package state

import (
	"bytes"
	"fmt"

	"github.com/ledgerwatch/erigon-lib/commitment"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/order"
	"github.com/ledgerwatch/erigon/common/math"
	"github.com/ledgerwatch/erigon/core/types/accounts"
)

var _ StateReader = (*ReaderV4)(nil)

type ReaderV4 struct {
	tx kv.TemporalTx
}

func NewReaderV4(tx kv.TemporalTx) *ReaderV4 {
	return &ReaderV4{tx: tx}
}

func (r *ReaderV4) ReadAccountData(address libcommon.Address) (*accounts.Account, error) {
	enc, ok, err := r.tx.DomainGet(kv.AccountsDomain, address.Bytes(), nil)
	if err != nil {
		return nil, err
	}
	if !ok || len(enc) == 0 {
		return nil, nil
	}
	fmt.Printf("enc: %x, %x\n", address, enc)
	var a accounts.Account
	if err = accounts.DeserialiseV3(&a, enc); err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ReaderV4) ReadAccountStorage(address libcommon.Address, incarnation uint64, key *libcommon.Hash) (enc []byte, err error) {
	enc, ok, err := r.tx.DomainGet(kv.StorageDomain, address.Bytes(), key.Bytes())
	if err != nil {
		return nil, err
	}
	fmt.Printf("read storage: %x, %d, %x -> %x, ok=%t\n", address, incarnation, *key, enc, ok)
	if !ok || len(enc) == 0 {
		return nil, nil
	}
	return enc, nil
}

func (r *ReaderV4) ReadAccountCode(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) (code []byte, err error) {
	if codeHash == emptyCodeHashH {
		return nil, nil
	}
	code, ok, err := r.tx.DomainGet(kv.CodeDomain, address.Bytes(), nil)
	if err != nil {
		return nil, err
	}
	if !ok || len(code) == 0 {
		return nil, nil
	}
	return code, nil
}

func (r *ReaderV4) ReadAccountCodeSize(address libcommon.Address, incarnation uint64, codeHash libcommon.Hash) (int, error) {
	code, err := r.ReadAccountCode(address, incarnation, codeHash)
	return len(code), err
}

func (r *ReaderV4) ReadAccountIncarnation(address libcommon.Address) (uint64, error) {
	a, _ := r.ReadAccountData(address)
	if a == nil {
		var hasStorage bool
		it, err := r.tx.DomainRange(kv.StorageDomain, address[:], nil, math.MaxUint64, order.Asc, 1)
		if err != nil {
			panic(err)
		}
		hasStorage = it.HasNext()
		fmt.Printf("ReadAccountIncarnation: %x, %t\n", address, hasStorage)
		if hasStorage {
			return 1, nil
		}
		return 0, nil
	}
	if !bytes.Equal(a.CodeHash[:], commitment.EmptyCodeHash) {
		fmt.Printf("ReadAccountIncarnation2: %x, %t\n", address, 1)
		return 1, nil
	}
	fmt.Printf("ReadAccountIncarnation3: %x, %d, inc=%d, %d, %d\n", address, 0, a.Incarnation, &a.Balance, a.Nonce)
	return 0, nil
}

func (r *ReaderV4) ReadCommitment(prefix []byte) (enc []byte, err error) {
	enc, ok, err := r.tx.DomainGet(kv.CommitmentDomain, prefix, nil)
	if err != nil {
		return nil, err
	}
	if !ok || len(enc) == 0 {
		return nil, nil
	}
	return enc, nil
}
