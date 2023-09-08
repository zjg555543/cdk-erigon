package stagedsync

import (
	"encoding/binary"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/length"
	"github.com/ledgerwatch/erigon-lib/kv"
)

func WriteOptimizedCounter(tx kv.RwTx, counterBucket string, addr []byte, count uint64, _debug bool) error {
	v := make([]byte, 1) // 1 byte (counter - 1) [0, 255]
	v[0] = byte(count - 1)
	if err := tx.Put(counterBucket, addr, v); err != nil {
		return err
	}
	return nil
}

func WriteLastCounter(tx kv.RwTx, counterBucket string, addr common.Address, count uint64) error {
	newValue := make([]byte, 16)
	binary.BigEndian.PutUint64(newValue, count)
	binary.BigEndian.PutUint64(newValue[8:], ^uint64(0))
	if err := tx.Put(counterBucket, addr.Bytes(), newValue); err != nil {
		return err
	}

	return nil
}

func WriteStandardCounter(tx kv.RwTx, counterBucket string, addr []byte, count uint64, chunk []byte) error {
	// key == address
	// value (dup) == accumulated counter uint64 + chunk uint64
	v := make([]byte, length.Counter+length.Chunk)
	binary.BigEndian.PutUint64(v, count)
	copy(v[length.Counter:], chunk)

	if err := tx.Put(counterBucket, addr, v); err != nil {
		return err
	}

	return nil
}
