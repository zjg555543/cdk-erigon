package migrations

import (
	"context"
	"database/sql"
	"errors"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS peers (
		id BLOB NOT NULL,
		last_touch INTEGER NOT NULL,
		PRIMARY KEY (id)
	);`,
}

func ApplyMigrations(ctx context.Context, db *sql.DB) error {
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
