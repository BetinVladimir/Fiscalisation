package persistence

import (
	"database/sql"
	"testing"
)

// mvpNewSchemaFixture returns an isolated PostgreSQL *sql.DB with all n
// migrations applied, cloned from the marked template. Skips if
// PG_INTEGRATION_URL is not set.
func mvpNewSchemaFixture(t *testing.T, _ int) *sql.DB {
	t.Helper()
	writer, _ := isolatedPostgresFixture(t)
	db, err := sql.Open("pgx", writer)
	if err != nil {
		t.Fatal("open schema fixture connection:", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
