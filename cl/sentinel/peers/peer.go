package peers

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/ledgerwatch/log/v3"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Record Peer data.
type Peer struct {
	InRequest bool
	// peer id
	pid peer.ID
	// backref to the manager that owns this peer
	m *Manager
}

func (p *Peer) ID() peer.ID {
	return p.pid
}

func (p *Peer) tx(ctx context.Context, fn func(context.Context, *sql.Tx) error) error {
	ctx, cn := context.WithTimeout(ctx, 15*time.Second)
	defer cn()
	do := func() error {
		tx, err := p.m.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		_, err = tx.Exec(`insert or ignore into peers(id) values (?)`, p.ID().String())
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx,
			`update peers set last_touch = ? where id = ?`,
			uint64(time.Now().Unix()), p.ID().String())
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
		err = fn(ctx, tx)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	for {
		err := do()
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "SQLITE_BUSY") {
			continue
		}
		return err
	}

}
func (p *Peer) Reward() {
	log.Trace("[Sentinel Peers] peer rewarded", "peer-id", p.pid)
	err := p.tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update peers set rewards = rewards + 1 where id = ?`, p.ID().String())
		return err
	})
	if err != nil {
		log.Error("[sentinel] fail penalize", "err", err)
	}
}
func (p *Peer) Penalize() {
	log.Trace("[Sentinel Peers] peer penalized", "peer-id", p.pid)
	err := p.tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update peers set penalties = penalties + 1 where id = ?`, p.ID().String())
		return err
	})
	if err != nil {
		log.Error("[sentinel] fail penalize", "err", err)
	}
}

func (p *Peer) Forgive() {
	log.Trace("[Sentinel Peers] peer forgiven", "peer-id", p.pid)
	p.tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update peers set penalties = penalties - 1 where id = ?`, p.ID().String())
		return err
	})
}

func (p *Peer) MarkUsed() {
	var useCount int
	err := p.tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update peers set use_count = use_count + 1 where id = ?`, p.ID().String())
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `select use_count from peers where id = ?`, p.ID().String()).Scan(&useCount)
		if err != nil {
			return err
		}
		return err
	})
	if err != nil {
		log.Error("[sentinel] fail mark peer used", "err", err)
	}
	log.Trace("[Sentinel Peers] peer used", "peer-id", p.pid, "uses", useCount)
}

func (p *Peer) MarkUnused() {
}

func (p *Peer) MarkReplied() {
	var useCount int
	var successCount int
	err := p.tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update peers set success_count = success_count + 1 where id = ?`, p.ID().String())
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `select use_count, success_count from peers where id = ?`, p.ID().String()).Scan(&useCount, &successCount)
		if err != nil {
			return err
		}
		return err
	})
	if err != nil {
		log.Error("[sentinel] fail mark peer success", "err", err)
	}
	log.Trace("[Sentinel Peers] peer replied", "peer-id", p.pid, "uses", useCount, "success", successCount)
}

func (p *Peer) IsAvailable() (available bool) {
	var banned int
	var penalties int
	p.m.db.QueryRowContext(context.TODO(), `
select banned, penalties from peers where id = ?
	`, p.ID()).Scan(&banned, &penalties)
	if banned == 1 {
		return false
	}
	if penalties > MaxBadResponses {
		return false
	}

	return !p.busy
}

func (p *Peer) IsBad() (bad bool) {
	var banned int
	var penalties int
	err := p.tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_ = p.m.db.QueryRowContext(ctx,
			`select banned, penalties from peers where id = ?
	`, p.ID()).Scan(&banned, &penalties)
		return nil
	})
	if err != nil {
		log.Error("[Sentinel] fail check peer", "err", err)
		return true
	}
	if banned == 1 {
		bad = true
		return
	}
	bad = penalties > MaxBadResponses
	return
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

func (p *Peer) Disconnect(reason ...string) {
	rzn := strings.Join(reason, " ")
	if !anySetInString(skipReasons, rzn) {
		log.Trace("[Sentinel Peers] disconnecting from peer", "peer-id", p.pid, "reason", strings.Join(reason, " "))
	}
	p.m.host.Peerstore().RemovePeer(p.pid)
	p.m.host.Network().ClosePeer(p.pid)
}
func (p *Peer) Ban(reason ...string) {
	log.Trace("[Sentinel Peers] bad peers has been banned", "peer-id", p.pid, "reason", strings.Join(reason, " "))
	err := p.tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(context.TODO(), `update peers set banned = 1 where id = ?`, p.ID().String())
		return err
	})
	if err != nil {
		log.Error("[Sentinel] fail ban", "err", err)
	}
	p.Disconnect(reason...)
	return
}

func (p *Peer) Unban() {
	log.Trace("[Sentinel Peers] peer unbanned", "peer-id", p.pid)
	p.tx(context.TODO(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `update peers set penalties = 0, banned = 0 where id = ?`, p.ID().String())
		return err
	})
}
