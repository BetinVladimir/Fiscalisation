package migrations

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var files embed.FS

const advisoryLockID int64 = 0x4245454d494e4950 // "BEEMINIP"

func Run(ctx context.Context, databaseURL string) error {
	if strings.TrimSpace(databaseURL) == "" {
		return nil
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping migration database: %w", err)
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, advisoryLockID); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	if _, err = tx.Exec(ctx, `create table if not exists minipos_schema_migrations(name text primary key,checksum text not null,applied_at timestamptz not null default now())`); err != nil {
		return fmt.Errorf("create migration journal: %w", err)
	}
	entries, err := fs.Glob(files, "sql/*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)
	for _, name := range entries {
		body, readErr := files.ReadFile(name)
		if readErr != nil {
			return fmt.Errorf("read migration %s: %w", name, readErr)
		}
		sum := sha256.Sum256(body)
		checksum := hex.EncodeToString(sum[:])
		var stored string
		scanErr := tx.QueryRow(ctx, `select checksum from minipos_schema_migrations where name=$1`, name).Scan(&stored)
		if scanErr == nil {
			if stored != checksum {
				return fmt.Errorf("migration %s checksum changed", name)
			}
			continue
		}
		if scanErr != pgx.ErrNoRows {
			return fmt.Errorf("read migration journal %s: %w", name, scanErr)
		}
		if _, err = tx.Exec(ctx, string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err = tx.Exec(ctx, `insert into minipos_schema_migrations(name,checksum) values($1,$2)`, name, checksum); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}
