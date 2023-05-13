package raw

import (
	"testing"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/cl/clparams"
	"github.com/ledgerwatch/erigon/cl/cltypes"
	"github.com/ledgerwatch/erigon/cl/cltypes/solid"
	"github.com/stretchr/testify/require"
)

const (
	blockRootsLength = 8192
	stateRootsLength = 8192
	randoMixesLength = 65536
	slashingsLength  = 8192
)

func CompareBeaconState(t *testing.T, a, b *BeaconState) error {

	_, err := a.HashSSZ()
	require.NoError(t, err)
	_, err = b.HashSSZ()
	require.NoError(t, err)

	require.EqualValues(t, a.inactivityScores.Length(), b.inactivityScores.Length())
	require.EqualValues(t, a.GenesisTime(), b.GenesisTime())
	require.EqualValues(t, a.GenesisValidatorsRoot(), b.GenesisValidatorsRoot())
	require.EqualValues(t, a.Slot(), b.Slot())
	require.EqualValues(t, a.Fork(), b.Fork())
	require.EqualValues(t, a.LatestBlockHeader(), b.LatestBlockHeader())
	require.EqualValues(t, a.BlockRoots(), b.BlockRoots())
	require.EqualValues(t, a.StateRoots(), b.StateRoots())
	require.EqualValues(t, a.HistoricalRoots(), b.HistoricalRoots())
	require.EqualValues(t, a.Eth1Data(), b.Eth1Data())
	require.EqualValues(t, a.Eth1DataVotes(), b.Eth1DataVotes())
	require.EqualValues(t, a.Eth1DepositIndex(), b.Eth1DepositIndex())
	a.ForEachValidator(func(v *cltypes.Validator, idx, total int) bool {
		require.EqualValuesf(t, v, v, "validator: %d", idx)
		return true
	})
	require.EqualValues(t, a.balances.Length(), b.balances.Length())
	a.balances.Range(func(index int, value uint64, length int) bool {
		require.EqualValuesf(t, value, b.balances.Get(index), "index %d", value)
		return true
	})

	require.EqualValues(t, a.balances.Length(), b.balances.Length())

	require.EqualValues(t, a.randaoMixes, b.randaoMixes)
	require.EqualValues(t, a.slashings, b.slashings)
	require.EqualValues(t, a.previousEpochParticipation.Length(), b.previousEpochParticipation.Length())
	//require.EqualValues(t, a.currentEpochParticipation, b.currentEpochParticipation)

	require.EqualValues(t, a.currentJustifiedCheckpoint, b.currentJustifiedCheckpoint)
	require.EqualValues(t, a.finalizedCheckpoint, b.finalizedCheckpoint)
	require.EqualValues(t, a.inactivityScores.Length(), b.inactivityScores.Length())
	a.inactivityScores.Range(func(index int, value uint64, length int) bool {
		require.EqualValuesf(t, value, b.inactivityScores.Get(index), "index %d", value)
		return true
	})
	require.EqualValues(t, a.currentSyncCommittee, b.currentSyncCommittee)
	require.EqualValues(t, a.nextSyncCommittee, b.nextSyncCommittee)
	require.EqualValues(t, a.latestExecutionPayloadHeader, b.latestExecutionPayloadHeader)
	require.EqualValues(t, a.nextWithdrawalIndex, b.nextWithdrawalIndex)
	require.EqualValues(t, a.nextWithdrawalValidatorIndex, b.nextWithdrawalValidatorIndex)
	require.EqualValues(t, a.historicalSummaries, b.historicalSummaries)
	require.EqualValues(t, a.previousEpochAttestations, b.previousEpochAttestations)
	require.EqualValues(t, a.currentEpochAttestations, b.currentEpochAttestations)

	for idx := range a.leaves {
		require.EqualValuesf(t, a.leaves[idx], b.leaves[idx], "leaf %d", idx)
	}

	//av, err := a.BlockRoot()
	//if err != nil {
	//	return err
	//}
	//bv, err := b.BlockRoot()
	//if err != nil {
	//	return err
	//}
	//assert.EqualValues(t, av, bv)
	return nil
}

type BeaconState struct {
	// State fields
	genesisTime                uint64
	genesisValidatorsRoot      common.Hash
	slot                       uint64
	fork                       *cltypes.Fork
	latestBlockHeader          *cltypes.BeaconBlockHeader
	blockRoots                 [blockRootsLength]common.Hash
	stateRoots                 [stateRootsLength]common.Hash
	historicalRoots            []common.Hash
	eth1Data                   *cltypes.Eth1Data
	eth1DataVotes              []*cltypes.Eth1Data
	eth1DepositIndex           uint64
	validators                 []*cltypes.Validator
	balances                   solid.Uint64Slice
	randaoMixes                [randoMixesLength]common.Hash
	slashings                  [slashingsLength]uint64
	previousEpochParticipation solid.BitList
	currentEpochParticipation  solid.BitList
	justificationBits          cltypes.JustificationBits
	// Altair
	previousJustifiedCheckpoint *cltypes.Checkpoint
	currentJustifiedCheckpoint  *cltypes.Checkpoint
	finalizedCheckpoint         *cltypes.Checkpoint
	inactivityScores            solid.Uint64Slice
	currentSyncCommittee        *cltypes.SyncCommittee
	nextSyncCommittee           *cltypes.SyncCommittee
	// Bellatrix
	latestExecutionPayloadHeader *cltypes.Eth1Header
	// Capella
	nextWithdrawalIndex          uint64
	nextWithdrawalValidatorIndex uint64
	historicalSummaries          []*cltypes.HistoricalSummary
	// Phase0: genesis fork. these 2 fields replace participation bits.
	previousEpochAttestations []*cltypes.PendingAttestation
	currentEpochAttestations  []*cltypes.PendingAttestation

	//  leaves for computing hashes
	leaves        [32][32]byte            // Pre-computed leaves.
	touchedLeaves map[StateLeafIndex]bool // Maps each leaf to whether they were touched or not.

	// cl version
	version      clparams.StateVersion // State version
	beaconConfig *clparams.BeaconChainConfig
}

func New(cfg *clparams.BeaconChainConfig) *BeaconState {
	state := &BeaconState{
		beaconConfig: cfg,
		//inactivityScores: solid.NewSimpleUint64Slice(int(cfg.ValidatorRegistryLimit)),
		inactivityScores:           solid.NewUint64Slice(int(cfg.ValidatorRegistryLimit)),
		balances:                   solid.NewUint64Slice(int(cfg.ValidatorRegistryLimit)),
		previousEpochParticipation: solid.NewBitList(0, int(cfg.ValidatorRegistryLimit)),
		currentEpochParticipation:  solid.NewBitList(0, int(cfg.ValidatorRegistryLimit)),
	}
	state.init()
	return state
}

func (b *BeaconState) init() error {
	if b.touchedLeaves == nil {
		b.touchedLeaves = make(map[StateLeafIndex]bool)
	}
	return nil
}
