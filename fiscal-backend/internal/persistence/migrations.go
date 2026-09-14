package persistence

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// RunMigrations is kept as the startup schema gate for compatibility. Schema
// ownership belongs to BeeloyDB; application binaries never mutate DDL.
func RunMigrations(ctx context.Context, databaseURL string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	var ready bool
	if err = db.QueryRowContext(ctx, `select to_regclass('fiscal.fiscal_state_rows') is not null and to_regclass('fiscal.fiscal_operations') is not null`).Scan(&ready); err != nil {
		return err
	}
	if !ready {
		return fmt.Errorf("canonical fiscal schema is missing; apply BeeloyDB migrations")
	}
	return nil
}
