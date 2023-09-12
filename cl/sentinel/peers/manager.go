package peers

import (
	"context"
	"database/sql"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ledgerwatch/log/v3"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/backoff"
)

const (
	maxBadPeers       = 50000
	maxPeerRecordSize = 1000
	DefaultMaxPeers   = 33
	MaxBadResponses   = 5
)

type Manager struct {
	host        host.Host
	db          *sql.DB
	peerTimeout time.Duration
	logger      log.Logger

	mu sync.Mutex
}

func NewManager(ctx context.Context, l log.Logger, host host.Host, db *sql.DB) (*Manager, error) {
	m := &Manager{
		db:          db,
		logger:      l,
		peerTimeout: 8 * time.Hour,
		host:        host,
	}

	go func() {
		tk := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-tk.C:
				m.snapshot(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
	return m, nil
}
func (m *Manager) MarkUsed(id peer.ID) error {
	ctx := context.TODO()
	var useCount int
	err := m.Tx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec(`insert or ignore into peers (id) values (?)`, id.String())
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `update peers set use_count = use_count + 1 where id = ?`, id.String())
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `select use_count from peers where id = ?`, id.String()).Scan(&useCount)
		if err != nil {
			return err
		}
		return err
	})
	if err != nil {
		return err
	}
	log.Trace("[Sentinel Peers] peer used", "peer-id", id, "uses", useCount)
	return nil
}
func (m *Manager) MarkReplied(id peer.ID) {
	var useCount int
	var successCount int
	err := m.Tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec(`insert or ignore into peers (id) values (?)`, id.String())
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `update peers set success_count = success_count + 1 where id = ?`, id.String())
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `select use_count, success_count from peers where id = ?`, id.String()).Scan(&useCount, &successCount)
		if err != nil {
			return err
		}
		return err
	})
	if err != nil {
		log.Error("[sentinel] fail mark peer success", "err", err)
	}
	log.Trace("[Sentinel Peers] peer replied", "peer-id", id, "uses", useCount, "success", successCount)
}

func (m *Manager) snapshot(ctx context.Context) {
	err := m.Tx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `drop table if exists main.session`)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `create table main.session as select * from temp.session`)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		m.logger.Error("could not snapshot", "err", err)
	}
}

func (m *Manager) CasPeerState(ctx context.Context, id peer.ID, from, to int) (swapped bool, err error) {
	var state int
	err = m.Tx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err = tx.Exec(`insert or ignore into session (id) values (?)`, id.String())
		if err != nil {
			return err
		}
		err := tx.QueryRowContext(ctx, `select state from temp.session where id = ?`, id.String()).Scan(&state)
		if err != nil {
			return nil
		}
		swapped = state == from
		if swapped {
			_, err := tx.ExecContext(ctx, `update temp.session set state = ? where id = ?`, to, id.String())
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return
	}
	return
}
func (m *Manager) SetPeerState(ctx context.Context, id peer.ID, to int) (err error) {
	err = m.Tx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err = tx.Exec(`insert or ignore into session (id) values (?)`, id.String())
		if err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `update temp.session set state = ? where id = ?`, to, id.String())
		if err != nil {
			return nil
		}
		return nil
	})
	if err != nil {
		return
	}
	return
}

func (m *Manager) EnsurePeer(ctx context.Context, id peer.ID) error {
	err := m.Tx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec(`insert or ignore into peers(id) values (?)`, id.String())
		if err != nil {
			return err
		}

		_, err = tx.Exec(`insert or ignore into session (id) values (?)`, id.String())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
func (m *Manager) BanPeer(ctx context.Context, id peer.ID, reason ...string) error {
	var state int
	err := m.Tx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec(`insert or ignore into peers(id) values (?)`, id.String())
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `select state from temp.session where id = ?`, id.String()).Scan(&state)
		if err != nil {
			return nil
		}
		_, err = tx.ExecContext(ctx, `update peers set banned = 1 where id = ?`, id.String())
		return err
	})
	if err != nil {
		log.Error("[Sentinel] fail ban", "err", err)
	}
	log.Info("[Sentinel Peers] bad peers has been banned", "peer-id", id, "state", state, "reason", strings.Join(reason, " "))
	m.DisconnectPeer(id, reason...)
	return err
}

var skipReasons = []string{
	"bad handshake",
	"context",
	"security protocol",
	"connect:",
	"dial backoff",
}

func anySetInString(set []string, in string) bool {
	for _, v := range skipReasons {
		if strings.Contains(in, v) {
			return true
		}
	}
	return false
}
func (m *Manager) DisconnectPeer(id peer.ID, reason ...string) {
	rzn := strings.Join(reason, " ")
	if !anySetInString(skipReasons, rzn) {
		m.logger.Trace("[Sentinel Peers] disconnecting from peer", "peer-id", id, "reason", strings.Join(reason, " "))
	}
	m.host.Peerstore().RemovePeer(id)
	m.host.Network().ClosePeer(id)
}

func (m *Manager) IsPeerBad(id peer.ID) (bad bool) {
	var banned int
	var penalties int
	err := m.Tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `insert or ignore into peers(id) values (?)`, id.String())
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx,
			`select banned, penalties from peers where id = ?
	`, id.String()).Scan(&banned, &penalties)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if banned == 1 {
			bad = true
			return nil
		}
		if penalties >= MaxBadResponses {
			bad = true
			return nil
		}
		return nil
	})
	if err != nil {
		m.logger.Error("failed to read peer info", "pid", id, "err", err)
		return true
	}
	return
}

var sqliteBackoff = backoff.NewExponentialDecorrelatedJitter(
	1*time.Millisecond,
	20*time.Millisecond,
	2,
	rand.NewSource(1),
)

func (m *Manager) IsPeerAvailable(id peer.ID) bool {
	ctx := context.TODO()
	var state int
	err := m.Tx(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.Exec(`insert or ignore into session (id) values (?)`, id.String())
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `select state from temp.session where id = ?`, id.String()).Scan(&state)
		if err != nil {
			return nil
		}
		return nil
	})
	if err != nil {
		return false
	}
	return state == 3
}
func (m *Manager) RewardPeer(ctx context.Context, id peer.ID, amount int) error {
	log.Trace("[Sentinel Peers] peer rewarded", "peer-id", id)
	return m.Tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update peers set rewards = rewards + ? where id = ?`, amount, id.String())
		return err
	})
}

func (m *Manager) PenalizePeer(ctx context.Context, id peer.ID, amount int) error {
	log.Trace("[Sentinel Peers] peer penalized", "peer-id", id)
	return m.Tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update peers set penalties = penalties + ? where id = ?`, amount, id.String())
		return err
	})
}
