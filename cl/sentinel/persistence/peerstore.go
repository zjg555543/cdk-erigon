package persistence

import (
	"context"
	"database/sql"
	"time"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/core/record"
	ma "github.com/multiformats/go-multiaddr"
)

var _ peerstore.Peerstore = (*TrackingPeerstore)(nil)
var _ peerstore.CertifiedAddrBook = (*TrackingPeerstore)(nil)

type Peerstore interface {
	peerstore.Peerstore
	peerstore.CertifiedAddrBook
}

type TrackingPeerstore struct {
	Peerstore

	db *sql.DB
}

func (t *TrackingPeerstore) ConsumePeerRecord(s *record.Envelope, ttl time.Duration) (accepted bool, err error) {
	return t.Peerstore.ConsumePeerRecord(s, ttl)
}

func (t *TrackingPeerstore) GetPeerRecord(p peer.ID) *record.Envelope {
	return t.Peerstore.GetPeerRecord(p)
}

func NewTrackingPeerstore(ps Peerstore, db *sql.DB) *TrackingPeerstore {
	return &TrackingPeerstore{
		Peerstore: ps,
		db:        db,
	}
}

func (t *TrackingPeerstore) Close() error {
	return t.Peerstore.Close()
}

// // AddAddr calls AddAddrs(p, []ma.Multiaddr{addr}, ttl)
func (t *TrackingPeerstore) AddAddr(p peer.ID, addr ma.Multiaddr, ttl time.Duration) {
	t.db.Exec(`insert or ignore into peers(id) values (?)`, p.String())
	t.Peerstore.AddAddr(p, addr, ttl)
}

// // AddAddrs gives this AddrBook addresses to use, with a given ttl
// // (time-to-live), after which the address is no longer valid.
// // If the manager has a longer TTL, the operation is a no-op for that address
func (t *TrackingPeerstore) AddAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) {
	t.db.Exec(`insert or ignore into peers(id) values (?)`, p.String())
	t.Peerstore.AddAddrs(p, addrs, ttl)
}

// // SetAddr calls mgr.SetAddrs(p, addr, ttl)
func (t *TrackingPeerstore) SetAddr(p peer.ID, addr ma.Multiaddr, ttl time.Duration) {
	t.db.Exec(`insert or ignore into peers(id) values (?)`, p.String())
	t.Peerstore.SetAddr(p, addr, ttl)
}

//
// SetAddrs sets the ttl on addresses. This clears any TTL there previously.
// This is used when we receive the best estimate of the validity of an address.

func (t *TrackingPeerstore) SetAddrs(p peer.ID, addrs []ma.Multiaddr, ttl time.Duration) {
	t.db.Exec(`insert or ignore into peers(id) values (?)`, p.String())
	t.Peerstore.SetAddrs(p, addrs, ttl)
}

// // UpdateAddrs updates the addresses associated with the given peer that have
// // the given oldTTL to have the given newTTL.
//
//	func (t *TrackingPeerstore) UpdateAddrs(p peer.ID, oldTTL time.Duration, newTTL time.Duration) {
//		panic("not implemented") // TODO: Implement
//	}
//
// // Addrs returns all known (and valid) addresses for a given peer.
//
//	func (t *TrackingPeerstore) Addrs(p peer.ID) []ma.Multiaddr {
//		panic("not implemented") // TODO: Implement
//	}
//
// // AddrStream returns a channel that gets all addresses for a given
// // peer sent on it. If new addresses are added after the call is made
// // they will be sent along through the channel as well.
//
//	func (t *TrackingPeerstore) AddrStream(_ context.Context, _ peer.ID) <-chan ma.Multiaddr {
//		panic("not implemented") // TODO: Implement
//	}
//
// // ClearAddresses removes all previously stored addresses.
//
//	func (t *TrackingPeerstore) ClearAddrs(p peer.ID) {
//		panic("not implemented") // TODO: Implement
//	}
//
// // PeersWithAddrs returns all of the peer IDs stored in the AddrBook.
//
//	func (t *TrackingPeerstore) PeersWithAddrs() peer.IDSlice {
//		panic("not implemented") // TODO: Implement
//	}
//
// PubKey stores the public key of a peer.
//
//	func (t *TrackingPeerstore) PubKey(pid peer.ID) ic.PubKey {
//		return t.Peerstore.PubKey(pid)
//	}
//
// AddPubKey stores the public key of a peer.
func (t *TrackingPeerstore) AddPubKey(pid peer.ID, pk ic.PubKey) error {
	_, err := t.db.Exec(`insert or ignore into peers(id) values (?)`, pid.String())
	if err != nil {
		return err
	}
	return t.Peerstore.AddPubKey(pid, pk)
}

// // PrivKey returns the private key of a peer, if known. Generally this might only be our own
// // private key, see
// // https://discuss.libp2p.io/t/what-is-the-purpose-of-having-map-peer-id-privatekey-in-peerstore/74.
//
//	func (t *TrackingPeerstore) PrivKey(_ peer.ID) ic.PrivKey {
//		panic("not implemented") // TODO: Implement
//	}
//
// // AddPrivKey stores the private key of a peer.
//func (t *TrackingPeerstore) AddPrivKey(p peer.ID, pk ic.PrivKey) error {
//	return t.Peerstore.AddPrivKey(p, pk)
//}

// // PeersWithKeys returns all the peer IDs stored in the KeyBook.
//
//	func (t *TrackingPeerstore) PeersWithKeys() peer.IDSlice {
//		panic("not implemented") // TODO: Implement
//	}
//
// RemovePeer removes all keys associated with a peer.
func (t *TrackingPeerstore) RemovePeer(pid peer.ID) {
	t.db.ExecContext(context.TODO(),
		`update peers
		set removed = 1
		where peer_id = ?
		`, pid.String())
	t.Peerstore.RemovePeer(pid)
}

// // Get / Put is a simple registry for other peer-related key/value pairs.
// // If we find something we use often, it should become its own set of
// // methods. This is a last resort.
//
//	func (t *TrackingPeerstore) Get(p peer.ID, key string) (interface{}, error) {
//		panic("not implemented") // TODO: Implement
//	}
//
//	func (t *TrackingPeerstore) Put(p peer.ID, key string, val interface{}) error {
//		panic("not implemented") // TODO: Implement
//	}
//
// // RecordLatency records a new latency measurement
//
//	func (t *TrackingPeerstore) RecordLatency(_ peer.ID, _ time.Duration) {
//		panic("not implemented") // TODO: Implement
//	}
//
// // LatencyEWMA returns an exponentially-weighted moving avg.
// // of all measurements of a peer's latency.
//
//	func (t *TrackingPeerstore) LatencyEWMA(_ peer.ID) time.Duration {
//		panic("not implemented") // TODO: Implement
//	}
//
//	func (t *TrackingPeerstore) GetProtocols(_ peer.ID) ([]protocol.ID, error) {
//		panic("not implemented") // TODO: Implement
//	}
func (t *TrackingPeerstore) AddProtocols(p peer.ID, ids ...protocol.ID) error {
	_, err := t.db.Exec(`insert or ignore into peers(id) values (?)`, p.String())
	if err != nil {
		return err
	}
	return t.Peerstore.AddProtocols(p, ids...)
}
func (t *TrackingPeerstore) SetProtocols(p peer.ID, ids ...protocol.ID) error {
	_, err := t.db.Exec(`insert or ignore into peers(id) values (?)`, p.String())
	if err != nil {
		return err
	}
	return t.Peerstore.SetProtocols(p, ids...)
}

//
//func (t *TrackingPeerstore) RemoveProtocols(_ peer.ID, _ ...protocol.ID) error {
//	panic("not implemented") // TODO: Implement
//}
//
//// SupportsProtocols returns the set of protocols the peer supports from among the given protocols.
//// If the returned error is not nil, the result is indeterminate.
//func (t *TrackingPeerstore) SupportsProtocols(_ peer.ID, _ ...protocol.ID) ([]protocol.ID, error) {
//	panic("not implemented") // TODO: Implement
//}
//
//// FirstSupportedProtocol returns the first protocol that the peer supports among the given protocols.
//// If the peer does not support any of the given protocols, this function will return an empty protocol.ID and a nil error.
//// If the returned error is not nil, the result is indeterminate.
//func (t *TrackingPeerstore) FirstSupportedProtocol(_ peer.ID, _ ...protocol.ID) (protocol.ID, error) {
//	panic("not implemented") // TODO: Implement
//}
//
//// PeerInfo returns a peer.PeerInfo struct for given peer.ID.
//// This is a small slice of the information Peerstore has on
//// that peer, useful to other services.
//func (t *TrackingPeerstore) PeerInfo(_ peer.ID) peer.AddrInfo {
//	panic("not implemented") // TODO: Implement
//}
//
//// Peers returns all of the peer IDs stored across all inner stores.
//func (t *TrackingPeerstore) Peers() peer.IDSlice {
//	panic("not implemented") // TODO: Implement
//}
