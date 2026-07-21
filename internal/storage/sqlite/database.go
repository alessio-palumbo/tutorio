// Package sqlite implements durable local adapters.
package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

func Open(ctx context.Context, path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err = db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply sqlite schema: %w", err)
	}
	for _, statement := range []string{
		`ALTER TABLE job_segments ADD COLUMN raw_response TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE job_segments ADD COLUMN error TEXT NOT NULL DEFAULT ''`,
	} {
		if _, migrationErr := db.ExecContext(ctx, statement); migrationErr != nil && !isDuplicateColumn(migrationErr) {
			db.Close()
			return nil, fmt.Errorf("upgrade sqlite schema: %w", migrationErr)
		}
	}
	return db, nil
}

func isDuplicateColumn(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}
