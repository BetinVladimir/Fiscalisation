package persistence

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const integrationTemplateMarker = "beefiscal-disposable-integration-template-v1"

func fixtureDatabaseURL(raw, database string) (string, error) {
	u, err := neturl.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" || u.Path != "/app" || u.Fragment != "" {
		return "", fmt.Errorf("requires explicit disposable PostgreSQL URL for template app")
	}
	if u.Query().Has("dbname") || u.Query().Has("database") {
		return "", fmt.Errorf("fixture URL cannot override selected database through query parameters")
	}
	u.Path = "/" + database
	u.RawPath = ""
	return u.String(), nil
}

// Never resets evidence in a shared database. The runner marks its own synthetic
// template; every test clones it, and cleanup drops only the random clone it owns.
func isolatedPostgresFixture(t *testing.T) (string, string) {
	t.Helper()
	source := os.Getenv("PG_INTEGRATION_URL")
	if source == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	if os.Getenv("PG_INTEGRATION_ISOLATED_DATABASES") != "1" {
		t.Fatal("PG integration requires explicit PG_INTEGRATION_ISOLATED_DATABASES=1 disposable-fixture opt-in")
	}
	maintenanceURL, err := fixtureDatabaseURL(source, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	maintenance, err := sql.Open("pgx", maintenanceURL)
	if err != nil {
		t.Fatal("open disposable fixture maintenance connection:", err)
	}
	maintenance.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var marker string
	if err = maintenance.QueryRowContext(ctx, `select coalesce(shobj_description(oid,'pg_database'),'') from pg_database where datname='app'`).Scan(&marker); err != nil || marker != integrationTemplateMarker {
		maintenance.Close()
		t.Fatal("refusing unmarked PostgreSQL fixture template", err)
	}
	random := make([]byte, 12)
	if _, err = rand.Read(random); err != nil {
		maintenance.Close()
		t.Fatal(err)
	}
	name := "fiscal_it_" + hex.EncodeToString(random)
	owned := pgx.Identifier{name}.Sanitize()
	if _, err = maintenance.ExecContext(ctx, `CREATE DATABASE `+owned+` TEMPLATE app`); err != nil {
		maintenance.Close()
		t.Fatal("clone disposable fixture:", err)
	}
	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(context.Background(), 20*time.Second)
		defer done()
		// Test defers close its writer/reader before Cleanup. FORCE handles only
		// accidental connections to this exact random owned database.
		if _, dropErr := maintenance.ExecContext(cleanupCtx, `DROP DATABASE `+owned+` WITH (FORCE)`); dropErr != nil {
			t.Errorf("drop owned disposable fixture %s: %v", name, dropErr)
		}
		maintenance.Close()
	})
	// CREATE DATABASE copies schema/data, but not ALTER DATABASE GUCs. Mirror
	// the canonical namespace used by this runner's clean template.
	if _, err = maintenance.ExecContext(ctx, `ALTER DATABASE `+owned+` SET search_path TO fiscal,public`); err != nil {
		t.Fatal("set owned fixture canonical namespace:", err)
	}
	writer, err := fixtureDatabaseURL(source, name)
	if err != nil {
		t.Fatal(err)
	}
	reader := ""
	if raw := os.Getenv("PG_RLS_INTEGRATION_URL"); raw != "" {
		reader, err = fixtureDatabaseURL(raw, name)
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Log("isolated disposable database:", name)
	return writer, reader
}

func TestFixtureDatabaseURLPreservesRestrictedIdentity(t *testing.T) {
	raw := "postgres://reader:synthetic%40password@127.0.0.1:15432/app?sslmode=disable&search_path=fiscal"
	got, err := fixtureDatabaseURL(raw, "fiscal_it_owned")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := neturl.Parse(got)
	old, _ := neturl.Parse(raw)
	if u.Path != "/fiscal_it_owned" || u.User.String() != old.User.String() || u.Host != old.Host || u.RawQuery != old.RawQuery {
		t.Fatal("fixture rewrite changed credentials, endpoint or runtime settings")
	}
	for _, bad := range []string{"postgres://reader@localhost/production", "file:///app", "host=localhost dbname=app", "postgres://localhost/app#fragment", "postgres://localhost/app?dbname=other", "postgres://localhost/app?database=other"} {
		if _, err = fixtureDatabaseURL(bad, "owned"); err == nil {
			t.Fatal("unsafe fixture URL accepted")
		}
	}
}

func isAuditImmutableError(err error) bool {
	var pgerr *pgconn.PgError
	return errors.As(err, &pgerr) && pgerr.Code == "P0001" && pgerr.Message == "FISCAL_AUDIT_IS_IMMUTABLE"
}
