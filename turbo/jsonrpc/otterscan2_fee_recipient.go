package jsonrpc

import (
	"context"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutil"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/turbo/rpchelper"
	"github.com/ledgerwatch/erigon/turbo/services"
)

type FeeRecipientListResult struct {
	BlocksSummary map[hexutil.Uint64]*BlockSummary2 `json:"blocksSummary"`
	Results       []*FeeRecipientMatch              `json:"results"`
}

type FeeRecipientMatch struct {
	BlockNum hexutil.Uint64 `json:"blockNumber"`
	// Amount    hexutil.Uint64 `json:"amount"`
}

type feeRecipientSearchResultMaterializer struct {
	blockReader services.FullBlockReader
}

func NewFeeRecipientSearchResultMaterializer(tx kv.Tx, blockReader services.FullBlockReader) (*feeRecipientSearchResultMaterializer, error) {
	return &feeRecipientSearchResultMaterializer{blockReader}, nil
}

func (w *feeRecipientSearchResultMaterializer) Convert(ctx context.Context, tx kv.Tx, idx uint64) (*FeeRecipientMatch, error) {
	// hash, err := w.blockReader.CanonicalHash(ctx, tx, blockNum)
	// if err != nil {
	// 	return nil, err
	// }
	// TODO: replace by header
	// body, _, err := w.blockReader.Body(ctx, tx, hash, blockNum)
	// if err != nil {
	// 	return nil, err
	// }

	result := &FeeRecipientMatch{
		BlockNum: hexutil.Uint64(idx),
	}
	return result, nil
}

// TODO: remove
func (api *Otterscan2APIImpl) GetFeeRecipientList(ctx context.Context, addr common.Address, idx, count uint64) (*FeeRecipientListResult, error) {
	return api.GetBlocksRewardedList(ctx, addr, idx, count)
}

func (api *Otterscan2APIImpl) GetBlocksRewardedList(ctx context.Context, addr common.Address, idx, count uint64) (*FeeRecipientListResult, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	srm, err := NewFeeRecipientSearchResultMaterializer(tx, api._blockReader)
	if err != nil {
		return nil, err
	}

	ret, err := genericResultList(ctx, tx, addr, idx, count, kv.OtsFeeRecipientIndex, kv.OtsFeeRecipientCounter, (SearchResultMaterializer[FeeRecipientMatch])(srm))
	if err != nil {
		return nil, err
	}

	blocks := make([]hexutil.Uint64, 0, len(ret))
	for _, r := range ret {
		blocks = append(blocks, hexutil.Uint64(r.BlockNum))
	}

	blocksSummary, err := api.newBlocksSummary2FromResults(ctx, tx, blocks)
	if err != nil {
		return nil, err
	}
	return &FeeRecipientListResult{
		BlocksSummary: blocksSummary,
		Results:       ret,
	}, nil
}

// TODO: remove
func (api *Otterscan2APIImpl) GetFeeRecipientCount(ctx context.Context, addr common.Address) (uint64, error) {
	return api.GetBlocksRewardedCount(ctx, addr)
}

func (api *Otterscan2APIImpl) GetBlocksRewardedCount(ctx context.Context, addr common.Address) (uint64, error) {
	return api.genericGetCount(ctx, addr, kv.OtsFeeRecipientCounter)
}

func (api *Otterscan2APIImpl) getBlockWithSenders(ctx context.Context, number rpc.BlockNumber, tx kv.Tx) (*types.Block, []common.Address, error) {
	if number == rpc.PendingBlockNumber {
		return api.pendingBlock(), nil, nil
	}

	n, hash, _, err := rpchelper.GetBlockNumber(rpc.BlockNumberOrHashWithNumber(number), tx, api.filters)
	if err != nil {
		return nil, nil, err
	}

	block, senders, err := api._blockReader.BlockWithSenders(ctx, tx, hash, n)
	return block, senders, err
}

func (api *Otterscan2APIImpl) getBlockDetailsImpl(ctx context.Context, tx kv.Tx, b *types.Block, number rpc.BlockNumber, senders []common.Address) (*BlockSummary2, error) {
	var response BlockSummary2
	chainConfig, err := api.chainConfig(tx)
	if err != nil {
		return nil, err
	}

	// getBlockRes, err := delegateGetBlockByNumber(tx, b, number, false)
	// if err != nil {
	// 	return nil, err
	// }
	getIssuanceRes, err := delegateIssuance(tx, b, chainConfig)
	if err != nil {
		return nil, err
	}
	receipts, err := api.getReceipts(ctx, tx, chainConfig, b, senders)
	if err != nil {
		return nil, fmt.Errorf("getReceipts error: %v", err)
	}
	feesRes, err := delegateBlockFees(ctx, tx, b, senders, chainConfig, receipts)
	if err != nil {
		return nil, err
	}

	// response := map[string]interface{}{}
	// response["block"] = getBlockRes
	// response["issuance"] = getIssuanceRes
	response.internalIssuance = getIssuanceRes
	response.TotalFees = hexutil.Uint64(feesRes)
	return &response, nil
}
