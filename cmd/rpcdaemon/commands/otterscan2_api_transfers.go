package commands

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon-lib/common/length"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/bitmapdb"
	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/log/v3"
)

type TransactionListResult struct {
	BlocksSummary map[hexutil.Uint64]*BlockSummary `json:"blocksSummary"`
	Results       []interface{}                    `json:"results"`
}

type TransactionMatch struct {
	Hash        common.Hash            `json:"hash"`
	Transaction *RPCTransaction        `json:"transaction"`
	Receipt     map[string]interface{} `json:"receipt"`
}

// Implements a template method for API that expose an address-based search results,
// like ERC20 or ERC721 txs that contains transfers related to a certain address.
//
// Usually this method implements most part of the job, and caller methods just wrap
// it with corresponding DB tables.
//
// Semantics of corresponding parameters are the same in the caller methods, so it
// should be assumed this doc is the source of truth.
//
// The idx param is 0-based index of the first match that should be returned, considering
// the elements are numbered [0, numElem - 1].
//
// The count param determines the maximum of how many results should be returned. It may
// return less than count elements if the table's last record is reached and there are
// no more results available.
//
// Those 2 params allow for a flexible way to build paginated results, i.e., you can get the
// 3rd page of results in a 25 element page by passing: idx == (3 - 1) * 25, count == 25.
//
// Most likely, for a search results when the matches are shown backwards in time, and pages
// are dynamically numbered backwards from the last search results, getting the 3rd page
// would require the client code to use: idx == (totalMatches - 3 * 25), count == 25; the
// search results should then be reversed in the UI.
func (api *Otterscan2APIImpl) genericTransferList(ctx context.Context, addr common.Address, idx, count uint64, indexBucket, counterBucket string) (*TransactionListResult, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	transferChunks, err := tx.Cursor(indexBucket)
	if err != nil {
		return nil, err
	}
	defer transferChunks.Close()

	transferCounter, err := tx.CursorDupSort(counterBucket)
	if err != nil {
		return nil, err
	}
	defer transferCounter.Close()

	// Determine page [start, end]
	startIdx := idx + 1

	// Locate first chunk
	counterFound, chunk, err := findNextCounter(transferCounter, addr, startIdx)
	if err != nil {
		return nil, err
	}
	if counterFound == 0 {
		return &TransactionListResult{
			BlocksSummary: make(map[hexutil.Uint64]*BlockSummary, 0),
			Results:       make([]interface{}, 0),
		}, nil
	}

	// Locate first chunk
	chunkKey := make([]byte, length.Addr+8)
	copy(chunkKey, addr.Bytes())
	copy(chunkKey[length.Addr:], chunk)
	k, v, err := transferChunks.SeekExact(chunkKey)
	if err != nil {
		return nil, err
	}
	if k == nil {
		return nil, fmt.Errorf("db possibly corrupted, couldn't find chunkKey %s on bucket %s", hexutility.Encode(chunkKey), indexBucket)
	}

	bm := bitmapdb.NewBitmap64()
	defer bitmapdb.ReturnToPool64(bm)

	for i := 0; i < len(v); i += 8 {
		bm.Add(binary.BigEndian.Uint64(v[i : i+8]))
	}

	// bitmap contains idxs [counterFound - bm.GetCardinality(), counterFound - 1]
	startCounter := counterFound - bm.GetCardinality() + 1
	if startIdx > startCounter {
		endRange, err := bm.Select(startIdx - startCounter - 1)
		if err != nil {
			return nil, err
		}
		bm.RemoveRange(bm.Minimum(), endRange+1)
	}

	ret := make([]interface{}, 0)
	it := bm.Iterator()
	for c := uint64(0); c < count; c++ {
		// Look at next chunk?
		if !it.HasNext() {
			k, v, err = transferChunks.Next()
			if err != nil {
				return nil, err
			}
			if !bytes.HasPrefix(k, addr.Bytes()) {
				break
			}

			bm.Clear()
			for i := 0; i < len(v); i += 8 {
				bm.Add(binary.BigEndian.Uint64(v[i : i+8]))
			}
			it = bm.Iterator()
		}

		// Get tx from EthTx
		txId := it.Next()
		txn, err := api._blockReader.TxnByTxId(ctx, tx, txId)
		if err != nil {
			return nil, err
		}

		blockNum, _, err := api.txnLookup(ctx, tx, txn.Hash())
		if err != nil {
			return nil, err
		}
		block, err := api.blockByNumberWithSenders(ctx, tx, blockNum)
		if err != nil {
			return nil, err
		}
		if block == nil {
			return nil, nil // not error, see https://github.com/ledgerwatch/erigon/issues/1645
		}

		receipt, err := api._getTransactionReceipt(ctx, tx, txn.Hash())
		if err != nil {
			return nil, err
		}

		ret = append(ret, &TransactionMatch{
			Hash:        txn.Hash(),
			Transaction: newRPCTransaction(txn, block.Hash(), blockNum, 0, block.BaseFee()),
			Receipt:     receipt,
		})
	}

	blocks := make([]hexutil.Uint64, 0, len(ret))
	for _, r := range ret {
		blockNum, ok, err := api.txnLookup(ctx, tx, r.(*TransactionMatch).Hash)
		if err != nil {
			return nil, err
		}
		if !ok {
			log.Warn("unexpected error, couldn't find tx", "hash", r.(*TransactionMatch).Hash)
		}
		blocks = append(blocks, hexutil.Uint64(blockNum))
	}

	blocksSummary, err := api.newBlocksSummaryFromResults(ctx, tx, blocks)
	if err != nil {
		return nil, err
	}
	return &TransactionListResult{
		BlocksSummary: blocksSummary,
		Results:       ret,
	}, nil
}

// Given an index, locates the counter chunk which should contain the desired index (>= index)
func findNextCounter(transferCounter kv.CursorDupSort, addr common.Address, idx uint64) (uint64, []byte, error) {
	v, err := transferCounter.SeekBothRange(addr.Bytes(), hexutility.EncodeTs(idx))
	if err != nil {
		return 0, nil, err
	}

	// No occurrences
	if v == nil {
		return 0, nil, nil
	}

	// <= 256 matches-optimization
	if len(v) == 1 {
		c, err := transferCounter.CountDuplicates()
		if err != nil {
			return 0, nil, err
		}
		if c != 1 {
			return 0, nil, fmt.Errorf("db possibly corrupted, expected 1 duplicate, got %d", c)
		}
		return uint64(v[0]) + 1, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, nil
	}

	// Regular chunk
	return binary.BigEndian.Uint64(v[:length.Ts]), v[length.Ts:], nil
}

func (api *Otterscan2APIImpl) genericGetTransferCount(ctx context.Context, addr common.Address, counterBucket string) (uint64, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	transferCounter, err := tx.CursorDupSort(counterBucket)
	if err != nil {
		return 0, err
	}
	defer transferCounter.Close()

	k, _, err := transferCounter.SeekExact(addr.Bytes())
	if err != nil {
		return 0, err
	}
	if k == nil {
		return 0, nil
	}

	v, err := transferCounter.LastDup()
	if err != nil {
		return 0, err
	}

	// Check if it is in the <= 256 count optimization
	if len(v) == 1 {
		c, err := transferCounter.CountDuplicates()
		if err != nil {
			return 0, err
		}
		if c != 1 {
			return 0, fmt.Errorf("db possibly corrupted, expected 1 duplicate, got %d", c)
		}

		return uint64(v[0]) + 1, nil
	}

	return binary.BigEndian.Uint64(v[:8]), nil
}

// copied from eth_receipts.go
func (api *Otterscan2APIImpl) _getTransactionReceipt(ctx context.Context, tx kv.Tx, txnHash common.Hash) (map[string]interface{}, error) {
	var blockNum uint64
	var ok bool

	blockNum, ok, err := api.txnLookup(ctx, tx, txnHash)
	if err != nil {
		return nil, err
	}

	cc, err := api.chainConfig(tx)
	if err != nil {
		return nil, err
	}

	if !ok && cc.Bor == nil {
		return nil, nil
	}

	// if not ok and cc.Bor != nil then we might have a bor transaction.
	// Note that Private API returns 0 if transaction is not found.
	if !ok || blockNum == 0 {
		blockNumPtr, err := rawdb.ReadBorTxLookupEntry(tx, txnHash)
		if err != nil {
			return nil, err
		}
		if blockNumPtr == nil {
			return nil, nil
		}

		blockNum = *blockNumPtr
	}

	block, err := api.blockByNumberWithSenders(ctx, tx, blockNum)
	if err != nil {
		return nil, err
	}
	if block == nil {
		return nil, nil // not error, see https://github.com/ledgerwatch/erigon/issues/1645
	}

	var txnIndex uint64
	var txn types.Transaction
	for idx, transaction := range block.Transactions() {
		if transaction.Hash() == txnHash {
			txn = transaction
			txnIndex = uint64(idx)
			break
		}
	}

	var borTx types.Transaction
	if txn == nil {
		borTx, _, _, _ = rawdb.ReadBorTransactionForBlock(tx, block)
		if borTx == nil {
			return nil, nil
		}
	}

	receipts, err := api.getReceipts(ctx, tx, cc, block, block.Body().SendersFromTxs())
	if err != nil {
		return nil, fmt.Errorf("getReceipts error: %w", err)
	}

	if txn == nil {
		borReceipt, err := rawdb.ReadBorReceipt(tx, block.Hash(), blockNum, receipts)
		if err != nil {
			return nil, err
		}
		if borReceipt == nil {
			return nil, nil
		}
		return marshalReceipt(borReceipt, borTx, cc, block.HeaderNoCopy(), txnHash, false), nil
	}

	if len(receipts) <= int(txnIndex) {
		return nil, fmt.Errorf("block has less receipts than expected: %d <= %d, block: %d", len(receipts), int(txnIndex), blockNum)
	}

	return marshalReceipt(receipts[txnIndex], block.Transactions()[txnIndex], cc, block.HeaderNoCopy(), txnHash, true), nil
}
