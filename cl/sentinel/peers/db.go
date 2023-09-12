package peers

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (m *Manager) Tx(ctx context.Context, fn func(context.Context, *sql.Tx) error) error {
	ctx, cn := context.WithTimeout(ctx, 1*time.Minute)
	defer cn()
	do := func() error {
		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		err = fn(ctx, tx)
		if err != nil {
			return err
		}
		return tx.Commit()
	}

	offer := sqliteBackoff()
	for {
		err := do()
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "SQLITE_BUSY") {
			time.Sleep(offer.Delay())
			continue
		}
		return err
	}
}
