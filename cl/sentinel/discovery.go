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

package sentinel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/ledgerwatch/erigon/cl/clparams"
	"github.com/ledgerwatch/erigon/cl/fork"
	"github.com/ledgerwatch/erigon/p2p/enode"
	"github.com/ledgerwatch/erigon/p2p/enr"
	"github.com/ledgerwatch/log/v3"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"

	"github.com/prysmaticlabs/go-bitfield"
)

// ConnectWithPeer connects to the peer
func (s *Sentinel) ConnectWithPeer(ctx context.Context, info peer.AddrInfo, skipHandshake bool) (err error) {
	return s.connectWithPeer(ctx, info, skipHandshake)
}

// connectWithPeer is the entrypoint for all peer connection
func (s *Sentinel) connectWithPeer(ctx context.Context, info peer.AddrInfo, skipHandshake bool) (err error) {
	if info.ID == s.host.ID() {
		return nil
	}
	if s.peers.IsPeerBad(info.ID) {
		return fmt.Errorf("refused to connect to bad peer")
	}
	err = s.peers.EnsurePeer(ctx, info.ID)
	if err != nil {
		return err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, clparams.MaxDialTimeout)
	defer cancel()
	// now attempt to connect
	if err := s.host.Connect(ctxWithTimeout, info); err != nil {
		s.peers.PenalizePeer(ctx, info.ID, 1)
		return err
	}
	return nil
}

func (s *Sentinel) connectWithAllPeers(multiAddrs []multiaddr.Multiaddr) error {
	addrInfos, err := peer.AddrInfosFromP2pAddrs(multiAddrs...)
	if err != nil {
		return err
	}
	for _, peerInfo := range addrInfos {
		go func(peerInfo peer.AddrInfo) {
			if err := s.ConnectWithPeer(s.ctx, peerInfo, true); err != nil {
				if !errors.Is(err, context.DeadlineExceeded) {
					log.Error("[Sentinel] Could not connect with peer", "err", err)
				}
			}
		}(peerInfo)
	}
	return nil
}

func (s *Sentinel) listenForPeers() {
	s.listenForPeersDoneCh = make(chan struct{}, 3)
	enodes := []*enode.Node{}
	for _, node := range s.cfg.NetworkConfig.StaticPeers {
		newNode, err := enode.Parse(enode.ValidSchemes, node)
		if err == nil {
			enodes = append(enodes, newNode)
		} else {
			log.Warn("Could not connect to static peer", "peer", node, "reason", err)
		}
	}
	log.Info("Static peers", "len", len(enodes))
	if s.cfg.NoDiscovery {
		return
	}
	multiAddresses := convertToMultiAddr(enodes)
	if err := s.connectWithAllPeers(multiAddresses); err != nil {
		log.Warn("Could not connect to static peers", "reason", err)
	}

	sf := &singleflight.Group{}

	iterator := s.listener.RandomNodes()
	defer iterator.Close()
	for {
		if err := s.ctx.Err(); err != nil {
			log.Debug("Stopping Ethereum 2.0 peer discovery", "err", err)
			break
		}
		select {
		case <-s.listenForPeersDoneCh:
			return
		default:
		}
		if s.HasTooManyPeers() {
			log.Trace("[Sentinel] Not looking for peers, at peer limit")
			time.Sleep(100 * time.Millisecond)
			continue
		}
		exists := iterator.Next()
		if !exists {
			continue
		}
		node := iterator.Node()
		peerInfo, _, err := convertToAddrInfo(node)
		if err != nil {
			log.Error("[Sentinel] Could not convert to peer info", "err", err)
			continue
		}

		// Skip Peer if IP was private.
		if node.IP().IsPrivate() {
			continue
		}
		go sf.Do(string(peerInfo.ID), func() (interface{}, error) {
			if err := s.ConnectWithPeer(s.ctx, *peerInfo, false); err != nil {
				log.Trace("[Sentinel] Could not connect with peer", "err", err)
			}
			return nil, err
		})
	}
}

func (s *Sentinel) connectToBootnodes() error {
	for i := range s.discoverConfig.Bootnodes {
		if err := s.discoverConfig.Bootnodes[i].Record().Load(enr.WithEntry("tcp", new(enr.TCP))); err != nil {
			if !enr.IsNotFound(err) {
				log.Error("[Sentinel] Could not retrieve tcp port")
			}
			continue
		}
	}
	multiAddresses := convertToMultiAddr(s.discoverConfig.Bootnodes)
	s.connectWithAllPeers(multiAddresses)
	return nil
}

func (s *Sentinel) setupENR(
	node *enode.LocalNode,
) (*enode.LocalNode, error) {
	forkId, err := fork.ComputeForkId(s.cfg.BeaconConfig, s.cfg.GenesisConfig)
	if err != nil {
		return nil, err
	}
	node.Set(enr.WithEntry(s.cfg.NetworkConfig.Eth2key, forkId))
	node.Set(enr.WithEntry(s.cfg.NetworkConfig.AttSubnetKey, bitfield.NewBitvector64().Bytes()))
	node.Set(enr.WithEntry(s.cfg.NetworkConfig.SyncCommsSubnetKey, bitfield.Bitvector4{byte(0x00)}.Bytes()))
	return node, nil
}

func (s *Sentinel) onDisconnection(net network.Network, conn network.Conn) {
	// apparently this needs to be async because of part of the libp2p design contract
	// i can't find a source for this, it would be great if someone could
	run := func() error {
		ctx := context.TODO()
		peerId := conn.RemotePeer()
		if net.Connectedness(conn.RemotePeer()) == network.Connected {
			return nil
		}
		var state int
		s.peers.Tx(ctx, func(ctx context.Context, tx *sql.Tx) error {
			tx.QueryRowContext(ctx, `select state from temp.session where id = ?`, peerId.String()).Scan(&state)
			return nil
		})
		if state == 3 {
			s.logger.Trace("disconnected peer", "id", peerId)
		}
		return s.peers.SetPeerState(context.TODO(), peerId, 0)
	}
	go func() {
		err := run()
		if err != nil {
			s.logger.Error("onDisconnection", "err", err)
		}
	}()

}

func (s *Sentinel) onConnection(net network.Network, conn network.Conn) {
	// apparently this needs to be async because of part of the libp2p design contract
	// i can't find a source for this, it would be great if someone could
	run := func() error {
		peerId := conn.RemotePeer()
		// grab the lock for start handshaking
		swapped, err := s.peers.CasPeerState(context.TODO(), peerId, 0, 2)
		if err != nil {
			return err
		}
		// we are already handshaking/done handshaking, so skip
		if !swapped {
			return nil
		}
		valid, reason := s.handshaker.ValidatePeer(peerId)
		if !valid {
			// they failed handshake, so we need to ban them
			err = s.peers.BanPeer(context.TODO(), peerId, "invalid peer", "bad handshake", reason)
			if err != nil {
				return err
			}
			// swap the state back from 2 to 0
			_, err := s.peers.CasPeerState(context.TODO(), peerId, 2, 0)
			if err != nil {
				return err
			}
			return err
		}
		// set to state 3 to indicate that we are connected
		_, err = s.peers.CasPeerState(context.TODO(), peerId, 2, 3)
		if err != nil {
			return err
		}
		return nil
	}
	go func() {
		err := run()
		if err != nil {
			s.logger.Error("onConnection", "err", err)
		}
	}()

}
