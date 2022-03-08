package tool

import (
	"context"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/core/rawdb"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/rlp"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func ParseFloat64(str string) float64 {
	v, _ := strconv.ParseFloat(str, 64)
	return v
}

func ChainConfig(tx kv.Tx) *params.ChainConfig {
	genesisBlock, err := rawdb.ReadBlockByNumber(tx, 0)
	Check(err)
	chainConfig, err := rawdb.ReadChainConfig(tx, genesisBlock.Hash())
	Check(err)
	return chainConfig
}

func ChainConfigFromDB(db kv.RoDB) (cc *params.ChainConfig) {
	err := db.View(context.Background(), func(tx kv.Tx) error {

		if err := tx.ForAmount(kv.BlockBody, nil, 10, func(k, v []byte) error {
			bodyForStorage := new(types.BodyForStorage)
			err := rlp.DecodeBytes(v, bodyForStorage)
			if err != nil {
				return err
			}
			fmt.Printf("alex: %d, %d,%d\n", binary.BigEndian.Uint64(k), bodyForStorage.BaseTxId, bodyForStorage.TxAmount)

			return nil
		}); err != nil {
			panic(err)
		}

		cc = ChainConfig(tx)
		return nil
	})
	Check(err)
	return cc
}
