package exec3

import (
	"github.com/holiman/uint256"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/state"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/core/vm"
)

type CallTracer struct {
	froms     map[common.Address]struct{}
	tos       map[common.Address]struct{}
	buffering bool
	agg       *state.AggregatorV3
}

func NewCallTracer(buffering bool, agg *state.AggregatorV3) *CallTracer {
	return &CallTracer{buffering: buffering, agg: agg}
}
func (ct *CallTracer) Reset() {
	ct.froms, ct.tos = nil, nil
}
func (ct *CallTracer) Froms() map[common.Address]struct{} { return ct.froms }
func (ct *CallTracer) Tos() map[common.Address]struct{}   { return ct.tos }

func (ct *CallTracer) CaptureTxStart(gasLimit uint64) {}
func (ct *CallTracer) CaptureTxEnd(restGas uint64)    {}
func (ct *CallTracer) CaptureStart(env vm.VMInterface, from common.Address, to common.Address, precompile bool, create bool, input []byte, gas uint64, value *uint256.Int, code []byte) {
	if !ct.buffering {
		_ = ct.agg.AddTraceFrom(from[:])
		_ = ct.agg.AddTraceTo(to[:])
		return
	}

	if ct.froms == nil {
		ct.froms = map[common.Address]struct{}{}
		ct.tos = map[common.Address]struct{}{}
	}
	ct.froms[from], ct.tos[to] = struct{}{}, struct{}{}
}
func (ct *CallTracer) CaptureEnter(typ vm.OpCode, from common.Address, to common.Address, precompile bool, create bool, input []byte, gas uint64, value *uint256.Int, code []byte) {
	if !ct.buffering {
		_ = ct.agg.AddTraceFrom(from[:])
		_ = ct.agg.AddTraceTo(to[:])
		return
	}

	if ct.froms == nil {
		ct.froms = map[common.Address]struct{}{}
		ct.tos = map[common.Address]struct{}{}
	}
	ct.froms[from], ct.tos[to] = struct{}{}, struct{}{}
}
func (ct *CallTracer) CaptureState(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, rData []byte, depth int, err error) {
}
func (ct *CallTracer) CaptureFault(pc uint64, op vm.OpCode, gas, cost uint64, scope *vm.ScopeContext, depth int, err error) {
}
func (ct *CallTracer) CaptureEnd(output []byte, usedGas uint64, err error) {
}
func (ct *CallTracer) CaptureExit(output []byte, usedGas uint64, err error) {
}
func (ct *CallTracer) AddCoinbase(coinbase common.Address, uncles []*types.Header) {
	if !ct.buffering {
		_ = ct.agg.AddTraceTo(coinbase[:])
		for _, uncle := range uncles {
			_ = ct.agg.AddTraceTo(uncle.Coinbase[:])
		}
		return
	}

	if ct.tos == nil {
		ct.tos = map[common.Address]struct{}{}
	}
	ct.tos[coinbase] = struct{}{}
	for _, uncle := range uncles {
		ct.tos[uncle.Coinbase] = struct{}{}
	}
}
