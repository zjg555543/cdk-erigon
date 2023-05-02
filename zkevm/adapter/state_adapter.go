package adapter

import (
	"context"

	"github.com/jackc/pgx"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/zkevm/state"
	"github.com/ledgerwatch/erigon/zkevm/state/metrics"
	"github.com/ledgerwatch/erigon/zkevm/state/runtime/executor/pb"
)

// write me an adapter mock for stateInterface from zk_synchronizer.go
// Interface
type stateInterface interface {
	GetLastBlock(ctx context.Context, dbTx pgx.Tx) (*state.Block, error)
	AddGlobalExitRoot(ctx context.Context, exitRoot *state.GlobalExitRoot, dbTx pgx.Tx) error
	AddForcedBatch(ctx context.Context, forcedBatch *state.ForcedBatch, dbTx pgx.Tx) error
	AddBlock(ctx context.Context, block *state.Block, dbTx pgx.Tx) error
	AddVirtualBatch(ctx context.Context, virtualBatch *state.VirtualBatch, dbTx pgx.Tx) error
	GetPreviousBlock(ctx context.Context, offset uint64, dbTx pgx.Tx) (*state.Block, error)
	GetLastBatchNumber(ctx context.Context, dbTx pgx.Tx) (uint64, error)
	GetBatchByNumber(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) (*state.Batch, error)
	ResetTrustedState(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) error
	GetNextForcedBatches(ctx context.Context, nextForcedBatches int, dbTx pgx.Tx) ([]state.ForcedBatch, error)
	AddVerifiedBatch(ctx context.Context, verifiedBatch *state.VerifiedBatch, dbTx pgx.Tx) error
	ProcessAndStoreClosedBatch(ctx context.Context, processingCtx state.ProcessingContext, encodedTxs []byte, dbTx pgx.Tx, caller metrics.CallerLabel) (common.Hash, error)
	OpenBatch(ctx context.Context, processingContext state.ProcessingContext, dbTx pgx.Tx) error
	CloseBatch(ctx context.Context, receipt state.ProcessingReceipt, dbTx pgx.Tx) error
	ProcessSequencerBatch(ctx context.Context, batchNumber uint64, batchL2Data []byte, caller metrics.CallerLabel, dbTx pgx.Tx) (*state.ProcessBatchResponse, error)
	StoreTransactions(ctx context.Context, batchNum uint64, processedTxs []*state.ProcessTransactionResponse, dbTx pgx.Tx) error
	GetStateRootByBatchNumber(ctx context.Context, batchNum uint64, dbTx pgx.Tx) (common.Hash, error)
	ExecuteBatch(ctx context.Context, batch state.Batch, updateMerkleTree bool, dbTx pgx.Tx) (*pb.ProcessBatchResponse, error)
	GetLastVerifiedBatch(ctx context.Context, dbTx pgx.Tx) (*state.VerifiedBatch, error)
	GetLastVirtualBatchNum(ctx context.Context, dbTx pgx.Tx) (uint64, error)
	AddSequence(ctx context.Context, sequence state.Sequence, dbTx pgx.Tx) error
	AddAccumulatedInputHash(ctx context.Context, batchNum uint64, accInputHash common.Hash, dbTx pgx.Tx) error
	AddTrustedReorg(ctx context.Context, trustedReorg *state.TrustedReorg, dbTx pgx.Tx) error
	GetReorgedTransactions(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) ([]ethTypes.Transaction, error)
	ResetForkID(ctx context.Context, batchNumber, forkID uint64, version string, dbTx pgx.Tx) error
	GetForkIDTrustedReorgCount(ctx context.Context, forkID uint64, version string, dbTx pgx.Tx) (uint64, error)
	UpdateForkIDIntervals(intervals []state.ForkIDInterval)

	BeginStateTransaction(ctx context.Context) (pgx.Tx, error)
}

type stateInterfaceMock struct{}

var _ stateInterface = (*stateInterfaceMock)(nil)

func NewStateInterfaceMock() stateInterface {
	return &stateInterfaceMock{}
}

func (m *stateInterfaceMock) GetLastBlock(ctx context.Context, dbTx pgx.Tx) (*state.Block, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) AddGlobalExitRoot(ctx context.Context, exitRoot *state.GlobalExitRoot, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) AddForcedBatch(ctx context.Context, forcedBatch *state.ForcedBatch, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) AddBlock(ctx context.Context, block *state.Block, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) AddVirtualBatch(ctx context.Context, virtualBatch *state.VirtualBatch, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) GetPreviousBlock(ctx context.Context, offset uint64, dbTx pgx.Tx) (*state.Block, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) GetLastBatchNumber(ctx context.Context, dbTx pgx.Tx) (uint64, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) GetBatchByNumber(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) (*state.Batch, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) ResetTrustedState(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) GetNextForcedBatches(ctx context.Context, nextForcedBatches int, dbTx pgx.Tx) ([]state.ForcedBatch, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) AddVerifiedBatch(ctx context.Context, verifiedBatch *state.VerifiedBatch, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) ProcessAndStoreClosedBatch(ctx context.Context, processingCtx state.ProcessingContext, encodedTxs []byte, dbTx pgx.Tx, caller metrics.CallerLabel) (common.Hash, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) OpenBatch(ctx context.Context, processingContext state.ProcessingContext, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) CloseBatch(ctx context.Context, receipt state.ProcessingReceipt, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) ProcessSequencerBatch(ctx context.Context, batchNumber uint64, batchL2Data []byte, caller metrics.CallerLabel, dbTx pgx.Tx) (*state.ProcessBatchResponse, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) StoreTransactions(ctx context.Context, batchNum uint64, processedTxs []*state.ProcessTransactionResponse, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) GetStateRootByBatchNumber(ctx context.Context, batchNum uint64, dbTx pgx.Tx) (common.Hash, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) ExecuteBatch(ctx context.Context, batch state.Batch, updateMerkleTree bool, dbTx pgx.Tx) (*pb.ProcessBatchResponse, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) GetLastVerifiedBatch(ctx context.Context, dbTx pgx.Tx) (*state.VerifiedBatch, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) GetLastVirtualBatchNum(ctx context.Context, dbTx pgx.Tx) (uint64, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) AddSequence(ctx context.Context, sequence state.Sequence, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) AddAccumulatedInputHash(ctx context.Context, batchNum uint64, accInputHash common.Hash, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) AddTrustedReorg(ctx context.Context, trustedReorg *state.TrustedReorg, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) GetReorgedTransactions(ctx context.Context, batchNumber uint64, dbTx pgx.Tx) ([]ethTypes.Transaction, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) ResetForkID(ctx context.Context, batchNumber, forkID uint64, version string, dbTx pgx.Tx) error {
	panic("implement me")
}

func (m *stateInterfaceMock) GetForkIDTrustedReorgCount(ctx context.Context, forkID uint64, version string, dbTx pgx.Tx) (uint64, error) {
	panic("implement me")
}

func (m *stateInterfaceMock) UpdateForkIDIntervals(intervals []state.ForkIDInterval) {
	panic("implement me")
}

func (m *stateInterfaceMock) BeginStateTransaction(ctx context.Context) (pgx.Tx, error) {
	panic("implement me")
}
