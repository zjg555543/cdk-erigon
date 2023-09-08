package stagedsync

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon-lib/common/length"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/bitmapdb"
	"github.com/ledgerwatch/erigon/turbo/services"
	"github.com/ledgerwatch/log/v3"
	"golang.org/x/exp/slices"
)

func NewGenericLogIndexerUnwinder() UnwindExecutor {
	return func(ctx context.Context, tx kv.RwTx, u *UnwindState, blockReader services.FullBlockReader, isShortInterval bool, logEvery *time.Ticker) error {
		erc20Unwinder, err := NewTransferLogIndexerUnwinder(tx, kv.OtsERC20TransferIndex, kv.OtsERC20TransferCounter, false)
		if err != nil {
			return err
		}
		defer erc20Unwinder.Dispose()

		erc721Unwinder, err := NewTransferLogIndexerUnwinder(tx, kv.OtsERC721TransferIndex, kv.OtsERC721TransferCounter, true)
		if err != nil {
			return err
		}
		defer erc721Unwinder.Dispose()

		return runLogUnwind(ctx, tx, blockReader, isShortInterval, logEvery, u, TRANSFER_TOPIC, []UnwindHandler{erc20Unwinder, erc721Unwinder})
	}
}

type LogIndexerUnwinder interface {
	UnwindAddress(tx kv.RwTx, addr common.Address, ethTx uint64) error
	UnwindAddressHolding(tx kv.RwTx, addr, token common.Address, ethTx uint64) error
	Dispose() error
}

type UnwindHandler interface {
	Unwind(tx kv.RwTx, results []*TransferAnalysisResult, ethTx uint64) error
}

type TransferLogIndexerUnwinder struct {
	indexBucket   string
	counterBucket string
	isNFT         bool
	target        kv.RwCursor
	targetDel     kv.RwCursor
	counter       kv.RwCursorDupSort
}

func NewTransferLogIndexerUnwinder(tx kv.RwTx, indexBucket, counterBucket string, isNFT bool) (*TransferLogIndexerUnwinder, error) {
	target, err := tx.RwCursor(indexBucket)
	if err != nil {
		return nil, err
	}

	targetDel, err := tx.RwCursor(indexBucket)
	if err != nil {
		return nil, err
	}

	counter, err := tx.RwCursorDupSort(counterBucket)
	if err != nil {
		return nil, err
	}

	return &TransferLogIndexerUnwinder{
		indexBucket,
		counterBucket,
		isNFT,
		target,
		targetDel,
		counter,
	}, nil
}

func (u *TransferLogIndexerUnwinder) Dispose() error {
	u.target.Close()
	u.targetDel.Close()
	u.counter.Close()

	return nil
}

func runLogUnwind(ctx context.Context, tx kv.RwTx, blockReader services.FullBlockReader, isShortInterval bool, logEvery *time.Ticker, u *UnwindState, topic []byte, unwinders []UnwindHandler) error {
	analyzer, err := NewTransferLogAnalyzer()
	if err != nil {
		return err
	}

	logs, err := tx.Cursor(kv.Log)
	if err != nil {
		return err
	}
	defer logs.Close()

	// The unwind interval is ]u.UnwindPoint, EOF]
	startBlock := u.UnwindPoint + 1

	// Traverse blocks logs [startBlock, EOF], determine txs that should've matched the criteria,
	// their logs, and their addresses.
	blocks, err := newBlockBitmapFromTopic(tx, startBlock, u.CurrentBlockNumber, topic)
	if err != nil {
		return err
	}
	defer bitmapdb.ReturnToPool(blocks)

	for it := blocks.Iterator(); it.HasNext(); {
		blockNum := uint64(it.Next())

		// Avoid recalculating txid from the block basetxid for each match
		baseTxId, err := blockReader.BaseTxIdForBlock(ctx, tx, blockNum)
		if err != nil {
			return err
		}

		// Inspect each block's tx logs
		logPrefix := hexutility.EncodeTs(blockNum)
		k, v, err := logs.Seek(logPrefix)
		if err != nil {
			return err
		}
		for k != nil && bytes.HasPrefix(k, logPrefix) {
			txLogs := newTxLogsFromRaw[TransferAnalysisResult](blockNum, baseTxId, k, v)
			results, err := AnalyzeLogs[TransferAnalysisResult](tx, analyzer, txLogs.rawLogs)
			if err != nil {
				return err
			}

			if len(results) > 0 {
				for _, unwinder := range unwinders {
					if err := unwinder.Unwind(tx, results, txLogs.ethTx); err != nil {
						return err
					}
				}
			}

			select {
			default:
			case <-ctx.Done():
				return common.ErrStopped
			case <-logEvery.C:
				log.Info("Unwinding log indexer", "blockNum", blockNum)
			}

			k, v, err = logs.Next()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (u *TransferLogIndexerUnwinder) Unwind(tx kv.RwTx, results []*TransferAnalysisResult, ethTx uint64) error {
	for _, r := range results {
		if err := r.Unwind(tx, u.isNFT, u, ethTx); err != nil {
			return err
		}
	}

	return nil
}

func (u *TransferLogIndexerUnwinder) UnwindAddressHolding(tx kv.RwTx, addr, token common.Address, ethTx uint64) error {
	return fmt.Errorf("NOT IMPLEMENTED; SHOULDN'T BE CALLED")
}

func (u *TransferLogIndexerUnwinder) UnwindAddress(tx kv.RwTx, addr common.Address, ethTx uint64) error {
	key := chunkKey(addr.Bytes(), false, ethTx)
	k, v, err := u.target.Seek(key)
	if err != nil {
		return err
	}
	if k == nil || !bytes.HasPrefix(k, addr.Bytes()) {
		// that's ok, because for unwind we take the shortcut and cut everything
		// onwards at the first occurrence, but there may be further occurrences
		// which will just be ignored
		return nil
	}

	// Skip potential rewrites of same chunk due to multiple matches on later blocks
	lastVal := binary.BigEndian.Uint64(v[len(v)-8:])
	if lastVal < ethTx {
		return nil
	}

	foundChunk := binary.BigEndian.Uint64(k[length.Addr:])
	bm := bitmapdb.NewBitmap64()
	defer bitmapdb.ReturnToPool64(bm)

	for i := 0; i < len(v); i += 8 {
		val := binary.BigEndian.Uint64(v[i : i+8])
		if val >= ethTx {
			break
		}
		bm.Add(val)
	}

	// Safe copy
	k = slices.Clone(k)
	v = slices.Clone(v)

	// Remove all following chunks
	for {
		if err := u.targetDel.Delete(k); err != nil {
			return err
		}

		k, _, err = u.target.Next()
		if err != nil {
			return err
		}
		k = slices.Clone(k)
		if k == nil || !bytes.HasPrefix(k, addr.Bytes()) {
			break
		}
	}

	if !bm.IsEmpty() {
		// Rewrite the found chunk as the last
		newKey := chunkKey(addr.Bytes(), true, 0)
		buf := bytes.NewBuffer(nil)
		b := make([]byte, 8)
		for it := bm.Iterator(); it.HasNext(); {
			val := it.Next()
			binary.BigEndian.PutUint64(b, val)
			buf.Write(b)
		}
		if err := tx.Put(u.indexBucket, newKey, buf.Bytes()); err != nil {
			return err
		}
	} else {
		// Rewrite the last remaining chunk as the last
		k, v, err := u.target.Prev()
		if err != nil {
			return err
		}
		k = slices.Clone(k)
		v = slices.Clone(v)
		if k != nil && bytes.HasPrefix(k, addr.Bytes()) {
			if err := u.targetDel.Delete(k); err != nil {
				return err
			}

			binary.BigEndian.PutUint64(k[length.Addr:], ^uint64(0))
			if err := tx.Put(u.indexBucket, k, v); err != nil {
				return err
			}
		}
	}

	// Delete counters backwards up to the chunk found
	k, _, err = u.counter.SeekExact(addr.Bytes())
	if err != nil {
		return err
	}
	k = slices.Clone(k)
	if k == nil {
		return fmt.Errorf("possible db corruption; can't find bucket=%s addr=%s data", u.counterBucket, addr)
	}

	// Determine if counter is stored in optimized format
	c, err := u.counter.CountDuplicates()
	if err != nil {
		return err
	}
	v, err = u.counter.LastDup()
	if err != nil {
		return err
	}
	v = slices.Clone(v)
	isSingleChunkOptimized := c == 1 && len(v) == 1

	// Delete last counter, it'll be replaced on the next step
	lastCounter := uint64(0)
	if isSingleChunkOptimized {
		if err := u.counter.DeleteCurrent(); err != nil {
			return err
		}
	} else {
		for {
			if len(v) == 1 {
				// DB corrupted
				return fmt.Errorf("db possibly corrupted, len(v) == 1: bucket=%s addr=%s k=%s v=%s", u.counterBucket, addr, hexutility.Encode(k), hexutility.Encode(v))
			}
			lastCounter = binary.BigEndian.Uint64(v[:length.Counter])
			chunk := binary.BigEndian.Uint64(v[length.Counter:])

			if chunk < foundChunk {
				break
			}
			if err := u.counter.DeleteCurrent(); err != nil {
				return err
			}
			k, v, err = u.counter.PrevDup()
			if err != nil {
				return err
			}
			k = slices.Clone(k)
			v = slices.Clone(v)
			if k == nil {
				lastCounter = 0
				isSingleChunkOptimized = true
				break
			}
		}
	}

	// Replace counter
	if !bm.IsEmpty() {
		newCounter := lastCounter + bm.GetCardinality()
		if isSingleChunkOptimized && newCounter <= 256 {
			// Write optimized counter
			if err := WriteOptimizedCounter(tx, u.counterBucket, addr.Bytes(), newCounter, true); err != nil {
				return err
			}
		} else {
			if err := WriteLastCounter(tx, u.counterBucket, addr, newCounter); err != nil {
				return err
			}
		}
	} else {
		// Rewrite previous counter (if it exists) pointing it to last chunk
		if k != nil && !isSingleChunkOptimized {
			if err := u.counter.DeleteCurrent(); err != nil {
				return err
			}

			binary.BigEndian.PutUint64(v[length.Counter:], ^uint64(0))
			if err := tx.Put(u.counterBucket, addr.Bytes(), v); err != nil {
				return err
			}
		}
	}

	return nil
}
