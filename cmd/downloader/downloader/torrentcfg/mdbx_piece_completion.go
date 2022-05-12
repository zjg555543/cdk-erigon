package torrentcfg

import (
	"context"
	"encoding/binary"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"github.com/ledgerwatch/erigon-lib/kv"
)

const (
	complete   = "c"
	incomplete = "i"
)

type mdbxPieceCompletion struct {
	db kv.RwDB
}

var _ storage.PieceCompletion = (*mdbxPieceCompletion)(nil)

func NewMdbxPieceCompletion(db kv.RwDB) (ret storage.PieceCompletion, err error) {
	ret = &mdbxPieceCompletion{db}
	return
}

func (mc mdbxPieceCompletion) Get(pk metainfo.PieceKey) (cn storage.Completion, err error) {
	err = mc.db.View(context.Background(), func(tx kv.Tx) error {
		var key [metainfo.HashSize + 4]byte
		copy(key[:], pk.InfoHash[:])
		binary.BigEndian.PutUint32(key[metainfo.HashSize:], uint32(pk.Index))

		cn.Ok = true
		v, err := tx.GetOne(kv.BittorrentCompletion, key[:])
		if err != nil {
			return err
		}
		switch string(v) {
		case complete:
			cn.Complete = true
		case incomplete:
			cn.Complete = false
		default:
			cn.Ok = false
		}
		return nil
	})
	return
}

func (mc mdbxPieceCompletion) Set(pk metainfo.PieceKey, b bool) error {
	if c, err := mc.Get(pk); err == nil && c.Ok && c.Complete == b {
		return nil
	}
	return mc.db.Update(context.Background(), func(tx kv.RwTx) error {
		var key [metainfo.HashSize + 4]byte
		copy(key[:], pk.InfoHash[:])
		binary.BigEndian.PutUint32(key[metainfo.HashSize:], uint32(pk.Index))

		v := []byte(incomplete)
		if b {
			v = []byte(complete)
		}
		err := tx.Put(kv.BittorrentCompletion, key[:], v)
		if err != nil {
			return err
		}
		return nil
	})
}

func (mc *mdbxPieceCompletion) Close() error {
	mc.db.Close()
	return nil
}
