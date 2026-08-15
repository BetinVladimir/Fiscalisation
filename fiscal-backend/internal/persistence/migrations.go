package persistence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"sort"

	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func RunMigrations(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended('beefiscal-schema-migrations',0))`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `create table if not exists fiscal_schema_migrations(name text primary key, checksum text not null, applied_at timestamptz not null default now())`); err != nil {
		return err
	}
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		body, readErr := migrationFiles.ReadFile("migrations/" + entry.Name())
		if readErr != nil {
			return readErr
		}
		sum := sha256.Sum256(body)
		checksum := hex.EncodeToString(sum[:])
		var stored string
		scanErr := tx.QueryRowContext(ctx, `select checksum from fiscal_schema_migrations where name=$1`, entry.Name()).Scan(&stored)
		if scanErr == nil {
			if stored != checksum {
				return fmt.Errorf("migration %s checksum changed", entry.Name())
			}
			continue
		}
		if scanErr != sql.ErrNoRows {
			return scanErr
		}
		if _, err = tx.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err = tx.ExecContext(ctx, `insert into fiscal_schema_migrations(name,checksum) values($1,$2)`, entry.Name(), checksum); err != nil {
			return err
		}
	}
	return tx.Commit()
}
