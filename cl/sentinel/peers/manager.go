package peers

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	maxBadPeers       = 50000
	maxPeerRecordSize = 1000
	DefaultMaxPeers   = 33
	MaxBadResponses   = 50
)

type Manager struct {
	host        host.Host
	db          *sql.DB
	peerTimeout time.Duration

	mu sync.Mutex
}

func NewManager(ctx context.Context, host host.Host, db *sql.DB) (*Manager, error) {
	m := &Manager{
		db:          db,
		peerTimeout: 8 * time.Hour,
		host:        host,
	}
	return m, nil
}

func (m *Manager) getPeer(id peer.ID) (peer *Peer) {
	p := &Peer{
		pid: id,
		m:   m,
	}
	return p
}
func (m *Manager) TryPeer(id peer.ID, fn func(peer *Peer, ok bool)) {
	p := m.getPeer(id)
	fn(p, true)
}

// WithPeer will get the peer with id and run your lambda with it. it will update the last queried time
// It will do all synchronization and so you can use the peer thread safe inside
func (m *Manager) WithPeer(id peer.ID, fn func(peer *Peer)) {
	if fn == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.getPeer(id)
	fn(p)
}
