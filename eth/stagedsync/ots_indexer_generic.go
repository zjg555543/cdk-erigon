package stagedsync

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/RoaringBitmap/roaring/roaring64"
	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon-lib/common/hexutility"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/turbo/services"
	"github.com/ledgerwatch/log/v3"
)

func NewGenericIndexerUnwinder(targetBucket, counterBucket string, attrs *roaring64.Bitmap) UnwindExecutor {
	return func(ctx context.Context, tx kv.RwTx, u *UnwindState, _ services.FullBlockReader, isShortInterval bool, logEvery *time.Ticker) error {
		return runUnwind(ctx, tx, isShortInterval, logEvery, u, targetBucket, counterBucket, attrs)
	}
}

func runUnwind(ctx context.Context, tx kv.RwTx, isShortInterval bool, logEvery *time.Ticker, u *UnwindState, targetBucket, counterBucket string, attrs *roaring64.Bitmap) error {
	target, err := tx.RwCursorDupSort(targetBucket)
	if err != nil {
		return err
	}
	defer target.Close()

	counter, err := tx.RwCursor(counterBucket)
	if err != nil {
		return err
	}
	defer counter.Close()

	// The unwind interval is ]u.UnwindPoint, EOF]
	startBlock := hexutility.EncodeTs(u.UnwindPoint + 1)

	// Delete all specified address attributes for affected addresses down to
	// unwind point + 1
	if attrs != nil {
		k, v, err := target.Seek(startBlock)
		if err != nil {
			return err
		}
		for k != nil {
			addr := common.BytesToAddress(v)
			if err := RemoveAttributes(tx, addr, attrs); err != nil {
				return err
			}

			k, v, err = target.NextDup()
			if err != nil {
				return err
			}
			if k == nil {
				k, v, err = target.NextNoDup()
				if err != nil {
					return err
				}
			}
		}
	}

	// Delete all block indexes backwards down to unwind point + 1
	unwoundBlock := uint64(0)
	k, _, err := target.Last()
	if err != nil {
		return err
	}
	for k != nil {
		blockNum := binary.BigEndian.Uint64(k)
		if blockNum <= u.UnwindPoint {
			unwoundBlock = blockNum
			break
		}

		if err := target.DeleteCurrentDuplicates(); err != nil {
			return err
		}

		k, _, err = target.PrevNoDup()
		if err != nil {
			return err
		}
	}

	// Delete all indexer counters backwards down to unwind point + 1
	k, v, err := counter.Last()
	if err != nil {
		return err
	}
	for k != nil {
		blockNum := binary.BigEndian.Uint64(v)
		if blockNum <= u.UnwindPoint {
			if blockNum != unwoundBlock {
				log.Error(fmt.Sprintf("[%s] Counter index is corrupt; please report as a bug", u.LogPrefix()), "unwindPoint", u.UnwindPoint, "blockNum", blockNum)
				return fmt.Errorf("[%s] Counter index is corrupt; please report as a bug: unwindPoint=%v blockNum=%v", u.LogPrefix(), u.UnwindPoint, blockNum)
			}
			break
		}

		if err := counter.DeleteCurrent(); err != nil {
			return err
		}

		// TODO: replace it by Prev(); investigate why it is not working (dupsort.PrevNoDup() works fine)
		k, v, err = counter.Last()
		if err != nil {
			return err
		}
	}

	return nil
}
