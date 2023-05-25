/*
   Copyright 2022 Erigon-Lightclient contributors
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
       http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package handlers

import (
	"context"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/cl/clparams"
	"github.com/ledgerwatch/erigon/cl/cltypes"
	"github.com/ledgerwatch/erigon/cmd/sentinel/sentinel/communication"
	"github.com/ledgerwatch/erigon/cmd/sentinel/sentinel/peers"
	"github.com/ledgerwatch/log/v3"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
)

type ConsensusHandlers struct {
	handlers      map[protocol.ID]network.StreamHandler
	host          host.Host
	peers         *peers.Manager
	metadata      *cltypes.Metadata
	beaconConfig  *clparams.BeaconChainConfig
	genesisConfig *clparams.GenesisConfig
	ctx           context.Context

	db kv.RoDB // Read stuff from database to answer
}

const (
	SuccessfulResponsePrefix = 0x00
	ResourceUnavaiablePrefix = 0x03
)

func NewConsensusHandlers(ctx context.Context, db kv.RoDB, host host.Host,
	peers *peers.Manager, beaconConfig *clparams.BeaconChainConfig, genesisConfig *clparams.GenesisConfig, metadata *cltypes.Metadata) *ConsensusHandlers {
	c := &ConsensusHandlers{
		peers:         peers,
		host:          host,
		metadata:      metadata,
		db:            db,
		genesisConfig: genesisConfig,
		beaconConfig:  beaconConfig,
		ctx:           ctx,
	}
	c.handlers = map[protocol.ID]network.StreamHandler{
		protocol.ID(communication.PingProtocolV1):                wrapStreamHandler(c.pingHandler),
		protocol.ID(communication.GoodbyeProtocolV1):             wrapStreamHandler(c.goodbyeHandler),
		protocol.ID(communication.StatusProtocolV1):              wrapStreamHandler(c.statusHandler),
		protocol.ID(communication.MetadataProtocolV1):            wrapStreamHandler(c.metadataV1Handler),
		protocol.ID(communication.MetadataProtocolV2):            wrapStreamHandler(c.metadataV2Handler),
		protocol.ID(communication.BeaconBlocksByRangeProtocolV1): wrapStreamHandler(c.blocksByRangeHandler),
		protocol.ID(communication.BeaconBlocksByRootProtocolV1):  wrapStreamHandler(c.beaconBlocksByRootHandler),
	}
	return c
}

func (c *ConsensusHandlers) Start() {
	for id, handler := range c.handlers {
		c.host.SetStreamHandler(id, handler)
	}
}

func wrapStreamHandler(fn func(s network.Stream) error) func(s network.Stream) {
	return func(s network.Stream) {
		err := fn(s)
		if err != nil {
			log.Error("[pubsubhandler] stream handler", "err", err)
			// TODO: maybe we should log this
			_ = s.Reset()
			return
		}
		err = s.Close()
		if err != nil {
			log.Error("[pubsubhandler] close stream", "err", err)
		}
	}
}
