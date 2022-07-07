package commands

import (
	"context"
	"math/big"
	"testing"

	"github.com/ledgerwatch/erigon-lib/kv/kvcache"
	"github.com/ledgerwatch/erigon/cmd/rpcdaemon/rpcdaemontest"
	"github.com/ledgerwatch/erigon/common/hexutil"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/turbo/snapshotsync"
	"github.com/stretchr/testify/assert"
)

func TestEthGasPrice(t *testing.T) {
	ctx := context.Background()
	db := rpcdaemontest.CreateTestKV(t)
	defer db.Close()

	tx, err := db.BeginRw(ctx)
	if err != nil {
		t.Errorf("failed to begin RW tx: %s", tx)
	}

	header := rawdb.ReadCurrentHeader(tx)
	header.BaseFee = big.NewInt(100)
	rawdb.WriteHeader(tx, header)

	tx.Rollback()

	stateCache := kvcache.New(kvcache.DefaultCoherentConfig)
	api := NewEthAPI(NewBaseApi(nil, stateCache, snapshotsync.NewBlockReader(), false), db, nil, nil, nil, 5000000)

	price, err := api.GasPrice(ctx)
	if err != nil {
		t.Errorf("failed getting gas price: %s", err)
	}

	expected := (*hexutil.Big)(&big.Int{})

	assert.Equal(t, expected, price)
}
