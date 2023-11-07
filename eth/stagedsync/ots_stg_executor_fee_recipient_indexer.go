package stagedsync

import (
	"context"
	"time"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/ledgerwatch/erigon-lib/chain"
	"github.com/ledgerwatch/erigon-lib/etl"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/consensus"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/turbo/services"
	"github.com/ledgerwatch/log/v3"
)

func FeeRecipientExecutor(ctx context.Context, db kv.RoDB, tx kv.RwTx, isInternalTx bool, tmpDir string, chainConfig *chain.Config, blockReader services.FullBlockReader, engine consensus.Engine, startBlock, endBlock uint64, isShortInterval bool, logEvery *time.Ticker, s *StageState, logger log.Logger) (uint64, error) {
	feeRecipientHandler := NewFeeRecipientIndexerHandler(tmpDir, s, logger)
	defer feeRecipientHandler.Close()

	return runIncrementalHeaderIndexerExecutor(db, tx, blockReader, startBlock, endBlock, isShortInterval, logEvery, ctx, s, feeRecipientHandler)
}

// Implements HeaderIndexerHandler interface in order to index block fee recipients
type FeeRecipientIndexerHandler struct {
	IndexHandler
}

func NewFeeRecipientIndexerHandler(tmpDir string, s *StageState, logger log.Logger) HeaderIndexerHandler {
	collector := etl.NewCollector(s.LogPrefix(), tmpDir, etl.NewSortableBuffer(etl.BufferOptimalSize), logger)
	bitmaps := map[string]*roaring64.Bitmap{}

	return &FeeRecipientIndexerHandler{
		&StandardIndexHandler{kv.OtsFeeRecipientIndex, kv.OtsFeeRecipientCounter, collector, bitmaps},
	}
}

// Index fee recipient address -> blockNum
func (h *FeeRecipientIndexerHandler) HandleMatch(header *types.Header) {
	h.TouchIndex(header.Coinbase, header.Number.Uint64())
}
