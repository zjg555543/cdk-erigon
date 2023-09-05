package migrations

import (
	"context"
	"database/sql"
	"errors"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS peers (
		id TEXT NOT NULL,
		use_count integer default 0 not null,
		success_count integer default 0 not null,
		rewards integer default 0 not null,
		penalties integer default 0 not null,
		banned integer default 0 not null,
		last_touch INTEGER default 0 NOT NULL,
		removed integer default 0 not null,
		primary key (id)
	);`,
}

func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL")
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL);"); err != nil {
		return err
	}
	// Get current schema version
	var currentVersion int
	err = tx.QueryRow("SELECT version FROM schema_version").Scan(&currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		currentVersion = -1
	} else if err != nil {
		return err
	}

	// Apply missing migrations
	for i := currentVersion + 1; i < len(migrations); i++ {
		_, err = tx.Exec(migrations[i])
		if err != nil {
			return err
		}

		// Update schema version
		_, err = tx.Exec("UPDATE schema_version SET version = ?", i)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
