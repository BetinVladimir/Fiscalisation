package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run verifies the BeeloyDB-owned schema. Application startup must never apply
// or reconcile DDL on its own.
func Run(ctx context.Context, databaseURL string) error {
	if strings.TrimSpace(databaseURL) == "" {
		return nil
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("open schema-check database: %w", err)
	}
	defer pool.Close()
	var ready bool
	if err = pool.QueryRow(ctx, `select to_regclass('minipos.minipos_state_rows') is not null and to_regclass('minipos.organizations') is not null`).Scan(&ready); err != nil {
		return fmt.Errorf("verify canonical minipos schema: %w", err)
	}
	if !ready {
		return fmt.Errorf("canonical minipos schema is missing; apply BeeloyDB migrations")
	}
	return nil
}
