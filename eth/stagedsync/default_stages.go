package stagedsync

import (
	"context"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/eth/ethconfig"
	"github.com/ledgerwatch/erigon/eth/stagedsync/stages"
	"github.com/ledgerwatch/log/v3"
)

func DefaultStages(ctx context.Context, snapshots SnapshotsCfg, headers HeadersCfg, cumulativeIndex CumulativeIndexCfg, blockHashCfg BlockHashesCfg, bodies BodiesCfg, senders SendersCfg, exec ExecuteBlockCfg, hashState HashStateCfg, trieCfg TrieCfg, history HistoryCfg, logIndex LogIndexCfg, callTraces CallTracesCfg, txLookup TxLookupCfg, finish FinishCfg, caCfg ContractAnalyzerCfg, ots2Enabled bool, test bool) []*Stage {
	defaultStages := []*Stage{
		{
			ID:          stages.Snapshots,
			Description: "Download snapshots",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				if badBlockUnwind {
					return nil
				}
				return SpawnStageSnapshots(s, ctx, tx, snapshots, firstCycle, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return nil
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return SnapshotsPrune(p, firstCycle, snapshots, ctx, tx)
			},
		},
		{
			ID:          stages.Headers,
			Description: "Download headers",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				if badBlockUnwind {
					return nil
				}
				return SpawnStageHeaders(s, u, ctx, tx, headers, firstCycle, test, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return HeadersUnwind(u, s, tx, headers, test)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return HeadersPrune(p, tx, headers, ctx)
			},
		},
		{
			ID:          stages.CumulativeIndex,
			Description: "Write Cumulative Index",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnStageCumulativeIndex(cumulativeIndex, s, tx, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindCumulativeIndexStage(u, cumulativeIndex, tx, ctx)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneCumulativeIndexStage(p, tx, ctx)
			},
		},
		{
			ID:          stages.BlockHashes,
			Description: "Write block hashes",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnBlockHashStage(s, tx, blockHashCfg, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindBlockHashStage(u, tx, blockHashCfg, ctx)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneBlockHashStage(p, tx, blockHashCfg, ctx)
			},
		},
		{
			ID:          stages.Bodies,
			Description: "Download block bodies",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return BodiesForward(s, u, ctx, tx, bodies, test, firstCycle, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindBodiesStage(u, tx, bodies, ctx)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneBodiesStage(p, tx, bodies, ctx)
			},
		},
		{
			ID:          stages.Senders,
			Description: "Recover senders from tx signatures",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnRecoverSendersStage(senders, s, u, tx, 0, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindSendersStage(u, tx, senders, ctx)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneSendersStage(p, tx, senders, ctx)
			},
		},
		{
			ID:          stages.Execution,
			Description: "Execute blocks w/o hash checks",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnExecuteBlocksStage(s, u, tx, 0, ctx, exec, firstCycle, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindExecutionStage(u, s, tx, ctx, exec, firstCycle, logger)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneExecutionStage(p, tx, exec, ctx, firstCycle)
			},
		},
		{
			ID:          stages.HashState,
			Description: "Hash the key in the state",
			Disabled:    bodies.historyV3 && ethconfig.EnableHistoryV4InTest,
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnHashStateStage(s, tx, hashState, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindHashStateStage(u, s, tx, hashState, ctx, logger)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneHashStateStage(p, tx, hashState, ctx)
			},
		},
		{
			ID:          stages.IntermediateHashes,
			Description: "Generate intermediate hashes and computing state root",
			Disabled:    bodies.historyV3 && ethconfig.EnableHistoryV4InTest,
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				if exec.chainConfig.IsPrague(0) {
					_, err := SpawnVerkleTrie(s, u, tx, trieCfg, ctx, logger)
					return err
				}
				_, err := SpawnIntermediateHashesStage(s, u, tx, trieCfg, ctx, logger)
				return err
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				if exec.chainConfig.IsPrague(0) {
					return UnwindVerkleTrie(u, s, tx, trieCfg, ctx, logger)
				}
				return UnwindIntermediateHashesStage(u, s, tx, trieCfg, ctx, logger)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneIntermediateHashesStage(p, tx, trieCfg, ctx)
			},
		},
		{
			ID:                  stages.CallTraces,
			Description:         "Generate call traces index",
			DisabledDescription: "Work In Progress",
			Disabled:            bodies.historyV3,
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnCallTraces(s, tx, callTraces, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindCallTraces(u, s, tx, callTraces, ctx, logger)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneCallTraces(p, tx, callTraces, ctx, logger)
			},
		},
		{
			ID:          stages.AccountHistoryIndex,
			Description: "Generate account history index",
			Disabled:    bodies.historyV3,
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnAccountHistoryIndex(s, tx, history, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindAccountHistoryIndex(u, s, tx, history, ctx)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneAccountHistoryIndex(p, tx, history, ctx, logger)
			},
		},
		{
			ID:          stages.StorageHistoryIndex,
			Description: "Generate storage history index",
			Disabled:    bodies.historyV3,
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnStorageHistoryIndex(s, tx, history, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindStorageHistoryIndex(u, s, tx, history, ctx)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneStorageHistoryIndex(p, tx, history, ctx, logger)
			},
		},
		{
			ID:          stages.LogIndex,
			Description: "Generate receipt logs index",
			Disabled:    bodies.historyV3,
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnLogIndex(s, tx, logIndex, ctx, 0, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindLogIndex(u, s, tx, logIndex, ctx)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneLogIndex(p, tx, logIndex, ctx, logger)
			},
		},
		{
			ID:          stages.TxLookup,
			Description: "Generate tx lookup index",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnTxLookup(s, tx, 0 /* toBlock */, txLookup, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindTxLookup(u, s, tx, txLookup, ctx, logger)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneTxLookup(p, tx, txLookup, ctx, firstCycle, logger)
			},
		},
		{
			ID:          stages.Finish,
			Description: "Final: update current block for the RPC API",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, _ Unwinder, tx kv.RwTx, logger log.Logger) error {
				return FinishForward(s, tx, finish, firstCycle)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindFinish(u, tx, finish, ctx)
			},
			Prune: func(firstCycle bool, p *PruneState, tx kv.RwTx, logger log.Logger) error {
				return PruneFinish(p, tx, finish, ctx)
			},
		},
	}

	// If ots2 is enabled, inject ots2 stages before finish stage
	if ots2Enabled {
		ots2Stages := []*Stage{
			{
				ID:          stages.OtsContractIndexer,
				Description: "Index contract creation",
				Forward:     GenericStageForwardFunc(ctx, caCfg, stages.Bodies, ContractIndexerExecutor),
				Unwind: GenericStageUnwindFunc(ctx, caCfg,
					NewGenericIndexerUnwinder(
						kv.OtsAllContracts,
						kv.OtsAllContractsCounter,
						nil,
					),
				),
				Prune: NoopStagePrune(ctx, caCfg),
			},
			{
				ID:          stages.OtsERC20Indexer,
				Description: "ERC20 token indexer",
				Forward: GenericStageForwardFunc(ctx, caCfg, stages.OtsContractIndexer,
					NewConcurrentIndexerExecutor(
						NewERC20Prober,
						kv.OtsAllContracts,
						kv.OtsERC20,
						kv.OtsERC20Counter,
					)),
				Unwind: GenericStageUnwindFunc(ctx, caCfg,
					NewGenericIndexerUnwinder(
						kv.OtsERC20,
						kv.OtsERC20Counter,
						roaring64.BitmapOf(kv.ADDR_ATTR_ERC20),
					)),
				Prune: NoopStagePrune(ctx, caCfg),
			},
			{
				ID:          stages.OtsERC165Indexer,
				Description: "ERC165 indexer",
				Forward: GenericStageForwardFunc(ctx, caCfg, stages.OtsContractIndexer,
					NewConcurrentIndexerExecutor(
						NewERC165Prober,
						kv.OtsAllContracts,
						kv.OtsERC165,
						kv.OtsERC165Counter,
					)),
				Unwind: GenericStageUnwindFunc(ctx, caCfg,
					NewGenericIndexerUnwinder(
						kv.OtsERC165,
						kv.OtsERC165Counter,
						roaring64.BitmapOf(kv.ADDR_ATTR_ERC165),
					)),
				Prune: NoopStagePrune(ctx, caCfg),
			},
			{
				ID:          stages.OtsERC721Indexer,
				Description: "ERC721 token indexer",
				Forward: GenericStageForwardFunc(ctx, caCfg, stages.OtsERC165Indexer,
					NewConcurrentIndexerExecutor(
						NewERC721Prober,
						kv.OtsERC165,
						kv.OtsERC721,
						kv.OtsERC721Counter,
					)),
				Unwind: GenericStageUnwindFunc(ctx, caCfg,
					NewGenericIndexerUnwinder(
						kv.OtsERC721,
						kv.OtsERC721Counter,
						roaring64.BitmapOf(kv.ADDR_ATTR_ERC721, kv.ADDR_ATTR_ERC721_MD),
					)),
				Prune: NoopStagePrune(ctx, caCfg),
			},
			{
				ID:          stages.OtsERC1155Indexer,
				Description: "ERC1155 token indexer",
				Forward: GenericStageForwardFunc(ctx, caCfg, stages.OtsERC165Indexer,
					NewConcurrentIndexerExecutor(
						NewERC1155Prober,
						kv.OtsERC165,
						kv.OtsERC1155,
						kv.OtsERC1155Counter,
					)),
				Unwind: GenericStageUnwindFunc(ctx, caCfg,
					NewGenericIndexerUnwinder(
						kv.OtsERC1155,
						kv.OtsERC1155Counter,
						roaring64.BitmapOf(kv.ADDR_ATTR_ERC1155),
					)),
				Prune: NoopStagePrune(ctx, caCfg),
			},
			{
				ID:          stages.OtsERC1167Indexer,
				Description: "ERC1167 proxy indexer",
				Forward: GenericStageForwardFunc(ctx, caCfg, stages.OtsContractIndexer,
					NewConcurrentIndexerExecutor(
						NewERC1167Prober,
						kv.OtsAllContracts,
						kv.OtsERC1167,
						kv.OtsERC1167Counter,
					)),
				Unwind: GenericStageUnwindFunc(ctx, caCfg,
					NewGenericIndexerUnwinder(
						kv.OtsERC1167,
						kv.OtsERC1167Counter,
						roaring64.BitmapOf(kv.ADDR_ATTR_ERC1167),
					)),
				Prune: NoopStagePrune(ctx, caCfg),
			},
			{
				ID:          stages.OtsERC4626Indexer,
				Description: "ERC4626 token indexer",
				Forward: GenericStageForwardFunc(ctx, caCfg, stages.OtsERC20Indexer,
					NewConcurrentIndexerExecutor(
						NewERC4626Prober,
						kv.OtsERC20,
						kv.OtsERC4626,
						kv.OtsERC4626Counter,
					)),
				Unwind: GenericStageUnwindFunc(ctx, caCfg,
					NewGenericIndexerUnwinder(
						kv.OtsERC4626,
						kv.OtsERC4626Counter,
						roaring64.BitmapOf(kv.ADDR_ATTR_ERC4626),
					)),
				Prune: NoopStagePrune(ctx, caCfg),
			},
			{
				ID:          stages.OtsERC20And721Transfers,
				Description: "ERC20/721 token transfer indexer",
				// Binds itself to ERC721 contract classifier as the parent stage on purpose to ensure
				// both ERC20 and ERC721 stages are executed.
				Forward: GenericStageForwardFunc(ctx, caCfg, stages.OtsERC721Indexer, ERC20And721TransferIndexerExecutor),
				Unwind:  GenericStageUnwindFunc(ctx, caCfg, NewGenericLogIndexerUnwinder()),
				Prune:   NoopStagePrune(ctx, caCfg),
			},
			{
				ID:          stages.OtsERC20And721Holdings,
				Description: "ERC20/721 token holdings indexer",
				Forward:     GenericStageForwardFunc(ctx, caCfg, stages.OtsERC721Indexer, ERC20And721HolderIndexerExecutor),
				Unwind:      GenericStageUnwindFunc(ctx, caCfg, NewGenericLogHoldingsUnwinder()),
				Prune:       NoopStagePrune(ctx, caCfg),
			},
		}

		newStages := defaultStages[:len(defaultStages)-1]
		newStages = append(newStages, ots2Stages...)
		newStages = append(newStages, defaultStages[len(defaultStages)-1])
		defaultStages = newStages
	}

	return defaultStages
}

// StateStages are all stages necessary for basic unwind and stage computation, it is primarily used to process side forks and memory execution.
func StateStages(ctx context.Context, headers HeadersCfg, bodies BodiesCfg, blockHashCfg BlockHashesCfg, senders SendersCfg, exec ExecuteBlockCfg, hashState HashStateCfg, trieCfg TrieCfg) []*Stage {
	return []*Stage{
		{
			ID:          stages.Headers,
			Description: "Download headers",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return nil
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return HeadersUnwind(u, s, tx, headers, false)
			},
		},
		{
			ID:          stages.Bodies,
			Description: "Download block bodies",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return BodiesForward(s, u, ctx, tx, bodies, false, false, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindBodiesStage(u, tx, bodies, ctx)
			},
		},
		{
			ID:          stages.BlockHashes,
			Description: "Write block hashes",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnBlockHashStage(s, tx, blockHashCfg, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindBlockHashStage(u, tx, blockHashCfg, ctx)
			},
		},
		{
			ID:          stages.Senders,
			Description: "Recover senders from tx signatures",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnRecoverSendersStage(senders, s, u, tx, 0, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindSendersStage(u, tx, senders, ctx)
			},
		},
		{
			ID:          stages.Execution,
			Description: "Execute blocks w/o hash checks",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnExecuteBlocksStage(s, u, tx, 0, ctx, exec, firstCycle, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindExecutionStage(u, s, tx, ctx, exec, firstCycle, logger)
			},
		},
		{
			ID:          stages.HashState,
			Description: "Hash the key in the state",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				return SpawnHashStateStage(s, tx, hashState, ctx, logger)
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindHashStateStage(u, s, tx, hashState, ctx, logger)
			},
		},
		{
			ID:          stages.IntermediateHashes,
			Description: "Generate intermediate hashes and computing state root",
			Forward: func(firstCycle bool, badBlockUnwind bool, s *StageState, u Unwinder, tx kv.RwTx, logger log.Logger) error {
				_, err := SpawnIntermediateHashesStage(s, u, tx, trieCfg, ctx, logger)
				return err
			},
			Unwind: func(firstCycle bool, u *UnwindState, s *StageState, tx kv.RwTx, logger log.Logger) error {
				return UnwindIntermediateHashesStage(u, s, tx, trieCfg, ctx, logger)
			},
		},
	}
}

var DefaultForwardOrder = UnwindOrder{
	stages.Snapshots,
	stages.Headers,
	stages.BlockHashes,
	stages.Bodies,

	// Stages below don't use Internet
	stages.Senders,
	stages.Execution,
	stages.Translation,
	stages.HashState,
	stages.IntermediateHashes,
	stages.CallTraces,
	stages.AccountHistoryIndex,
	stages.StorageHistoryIndex,
	stages.LogIndex,
	stages.TxLookup,
	stages.Finish,
}

// UnwindOrder represents the order in which the stages needs to be unwound.
// The unwind order is important and not always just stages going backwards.
// Let's say, there is tx pool can be unwound only after execution.
// It's ok to remove some stage from here to disable only unwind of stage
type UnwindOrder []stages.SyncStage
type PruneOrder []stages.SyncStage

var DefaultUnwindOrder = UnwindOrder{
	stages.Finish,
	stages.TxLookup,
	stages.LogIndex,
	stages.StorageHistoryIndex,
	stages.AccountHistoryIndex,
	stages.CallTraces,

	// Unwinding of IHashes needs to happen after unwinding HashState
	stages.HashState,
	stages.IntermediateHashes,

	stages.Translation,
	stages.Execution,
	stages.Senders,

	stages.Bodies,
	stages.BlockHashes,
	stages.Headers,
}

var StateUnwindOrder = UnwindOrder{
	// Unwinding of IHashes needs to happen after unwinding HashState
	stages.HashState,
	stages.IntermediateHashes,
	stages.Execution,
	stages.Senders,
	stages.Bodies,
	stages.BlockHashes,
	stages.Headers,
}

var DefaultPruneOrder = PruneOrder{
	stages.Finish,
	stages.TxLookup,
	stages.LogIndex,
	stages.StorageHistoryIndex,
	stages.AccountHistoryIndex,
	stages.CallTraces,

	// Unwinding of IHashes needs to happen after unwinding HashState
	stages.HashState,
	stages.IntermediateHashes,

	stages.Translation,
	stages.Execution,
	stages.Senders,

	stages.Bodies,
	stages.BlockHashes,
	stages.Headers,
	stages.Snapshots,
}

var MiningUnwindOrder = UnwindOrder{} // nothing to unwind in mining - because mining does not commit db changes
var MiningPruneOrder = PruneOrder{}   // nothing to unwind in mining - because mining does not commit db changes
