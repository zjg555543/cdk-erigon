package commands

import (
	"context"
	"fmt"

	"github.com/ledgerwatch/erigon/common"
	"github.com/ledgerwatch/erigon/rpc"
	"github.com/ledgerwatch/erigon/turbo/rpchelper"
)

func (api *ErigonImpl) GetContractCreationInfo(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) ([]*common.Hash, error) {
	tx, err := api.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stateReader, err := rpchelper.CreateStateReader(ctx, tx, blockNrOrHash, api.filters, api.stateCache)
	if err != nil {
		return nil, err
	}

	acc, err := stateReader.ReadAccountData(address)
	if err != nil {
		return nil, err
	}

	if acc == nil {
		return nil, fmt.Errorf("couldn't find account with address: %s", address)
	}

	return nil, nil
}
