// Package dossier exports a bounded fiscal financial namespace without domain writes.
package dossier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// These authoritative runtime financial tables contain no bootstrap/session credentials.
// Generic resources, replays, outbox and audit payloads are deliberately not exported.
var Tables = []string{"fiscal_runtime_artifacts", "fiscal_runtime_operations", "fiscal_runtime_sales", "fiscal_runtime_shifts", "fiscal_runtime_sync_acks", "fiscal_runtime_unp_sequences"}

type Column struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Nullable bool    `json:"nullable"`
	Default  *string `json:"default"`
}
type Table struct {
	Name        string   `json:"name"`
	Columns     []Column `json:"columns"`
	RLS         bool     `json:"rls"`
	ForceRLS    bool     `json:"force_rls"`
	Policies    []string `json:"policies"`
	Constraints []string `json:"constraints"`
	Indexes     []string `json:"indexes"`
}
type Contract struct {
	Format string  `json:"format"`
	Schema string  `json:"schema"`
	Tables []Table `json:"tables"`
}
type File struct {
	SHA256 string `json:"sha256"`
	Rows   int64  `json:"rows"`
}
type Manifest struct {
	Format                  string          `json:"format"`
	Tenant                  string          `json:"tenant"`
	Schema                  string          `json:"schema"`
	Snapshot                string          `json:"snapshot"`
	CapturedAt              string          `json:"captured_at"`
	SourceCommit            string          `json:"source_commit_declared"`
	ProvenanceAuthenticated bool            `json:"provenance_authenticated"`
	Files                   map[string]File `json:"files"`
	Excluded                []string        `json:"excluded"`
}
type Options struct {
	Schema, Tenant, Output, SourceCommit string
	Contract                             Contract
}

func identifier(s string) string { return pgx.Identifier{s}.Sanitize() }
func validSchema(s string) bool  { return s == "fiscal" || s == "public" }
func digest(b []byte) string     { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// Inspect is for a clean database created from reviewed migrations, not live-schema acceptance.
func Inspect(ctx context.Context, tx pgx.Tx, schema string) (Contract, error) {
	c := Contract{Format: "beefiscal-financial-pg-schema-v1", Schema: schema}
	if !validSchema(schema) {
		return c, errors.New("explicit fiscal/public schema required")
	}
	for _, name := range Tables {
		t := Table{Name: name, Columns: []Column{}, Policies: []string{}, Constraints: []string{}, Indexes: []string{}}
		var owner bool
		err := tx.QueryRow(ctx, `select relrowsecurity,relforcerowsecurity,relowner=(select oid from pg_roles where rolname=current_user) from pg_class where oid=to_regclass($1) and relkind='r'`, schema+"."+name).Scan(&t.RLS, &t.ForceRLS, &owner)
		if err != nil {
			return c, fmt.Errorf("missing table %s: %w", name, err)
		}
		if !t.RLS || !t.ForceRLS {
			return c, fmt.Errorf("FORCE RLS required: %s", name)
		}
		rows, err := tx.Query(ctx, `select a.attname,format_type(a.atttypid,a.atttypmod),not a.attnotnull,pg_get_expr(d.adbin,d.adrelid) from pg_attribute a left join pg_attrdef d on d.adrelid=a.attrelid and d.adnum=a.attnum where a.attrelid=to_regclass($1) and a.attnum>0 and not a.attisdropped order by a.attnum`, schema+"."+name)
		if err != nil {
			return c, err
		}
		for rows.Next() {
			var col Column
			if err = rows.Scan(&col.Name, &col.Type, &col.Nullable, &col.Default); err != nil {
				rows.Close()
				return c, err
			}
			t.Columns = append(t.Columns, col)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return c, err
		}
		for _, part := range []struct {
			query string
			dest  *[]string
		}{
			{`select polname||':'||polcmd::text||':'||polpermissive::text||':'||(select string_agg(case when x=0 then 'public' else (select rolname::text from pg_roles where oid=x) end,',' order by x) from unnest(polroles) x)||':'||coalesce(pg_get_expr(polqual,polrelid),'')||':'||coalesce(pg_get_expr(polwithcheck,polrelid),'') from pg_policy where polrelid=to_regclass($1) order by polname`, &t.Policies},
			{`select conname||':'||pg_get_constraintdef(oid) from pg_constraint where conrelid=to_regclass($1) order by conname`, &t.Constraints},
			{`select indexname||':'||indexdef from pg_indexes where schemaname=$1 and tablename=$2 order by indexname`, &t.Indexes},
		} {
			var r pgx.Rows
			if part.dest == &t.Indexes {
				r, err = tx.Query(ctx, part.query, schema, name)
			} else {
				r, err = tx.Query(ctx, part.query, schema+"."+name)
			}
			if err != nil {
				return c, err
			}
			for r.Next() {
				var value string
				if err = r.Scan(&value); err != nil {
					r.Close()
					return c, err
				}
				*part.dest = append(*part.dest, value)
			}
			err = r.Err()
			r.Close()
			if err != nil {
				return c, err
			}
		}
		c.Tables = append(c.Tables, t)
	}
	return c, nil
}

func Export(ctx context.Context, conn *pgx.Conn, o Options) (err error) {
	return export(ctx, conn, o, nil)
}
func export(ctx context.Context, conn *pgx.Conn, o Options, afterSnapshot func() error) (err error) {
	if o.Tenant == "" || strings.TrimSpace(o.Tenant) != o.Tenant || o.SourceCommit == "" || o.Output == "" {
		return errors.New("tenant, source commit and output required")
	}
	if _, err = os.Lstat(o.Output); err == nil || !os.IsNotExist(err) {
		return errors.New("output must not exist")
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var unsafe bool
	if err = tx.QueryRow(ctx, `select rolsuper or rolbypassrls from pg_roles where rolname=current_user`).Scan(&unsafe); err != nil {
		return err
	}
	if unsafe {
		return errors.New("superuser/BYPASSRLS connection refused")
	}
	if _, err = tx.Exec(ctx, `select set_config('app.tenant_id',$1,true),set_config('TimeZone','UTC',true),set_config('DateStyle','ISO, YMD',true),set_config('bytea_output','hex',true),set_config('extra_float_digits','3',true),set_config('row_security','on',true)`, o.Tenant); err != nil {
		return err
	}
	// Prevent concurrent DDL between dictionary comparison and row reads.
	if !validSchema(o.Schema) {
		return errors.New("explicit fiscal/public schema required")
	}
	for _, name := range Tables {
		if _, err = tx.Exec(ctx, "LOCK TABLE "+identifier(o.Schema)+"."+identifier(name)+" IN ACCESS SHARE MODE"); err != nil {
			return err
		}
	}
	var snapshot, at string
	if err = tx.QueryRow(ctx, `select pg_current_snapshot()::text,transaction_timestamp()::text`).Scan(&snapshot, &at); err != nil {
		return err
	}
	actual, err := Inspect(ctx, tx, o.Schema)
	if err != nil {
		return err
	}
	a, _ := json.Marshal(actual)
	b, _ := json.Marshal(o.Contract)
	if string(a) != string(b) {
		return errors.New("schema/policy drift against reviewed contract")
	}
	// Owners can alter policies; use an independent restricted reader, even with FORCE RLS.
	for _, t := range actual.Tables {
		var owner bool
		if err = tx.QueryRow(ctx, `select relowner=(select oid from pg_roles where rolname=current_user) from pg_class where oid=to_regclass($1)`, o.Schema+"."+t.Name).Scan(&owner); err != nil {
			return err
		}
		if owner {
			return errors.New("table-owner connection refused")
		}
	}
	if afterSnapshot != nil {
		if err = afterSnapshot(); err != nil {
			return err
		}
	}
	parent := filepath.Dir(o.Output)
	stage, err := os.MkdirTemp(parent, ".fiscal-pg-extract-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	manifest := Manifest{Format: "beefiscal-financial-pg-extract-v1", Tenant: o.Tenant, Schema: o.Schema, Snapshot: snapshot, CapturedAt: at, SourceCommit: o.SourceCommit, Files: map[string]File{}, Excluded: []string{"generic resources and credentials", "audit/outbox/replay payloads pending secret-safe export contract", "legacy/reporting typed tables", "outbound email and auth stores", "external artifact object bodies", "MiniPOS PostgreSQL", "S3/BlueCash/browser/native stores"}}
	write := func(name string, body []byte, count int64) error {
		if err := os.WriteFile(filepath.Join(stage, name), body, 0600); err != nil {
			return err
		}
		manifest.Files[name] = File{digest(body), count}
		return nil
	}
	if err = write("dictionary.json", append(a, '\n'), 0); err != nil {
		return err
	}
	for _, t := range actual.Tables {
		columns := []string{}
		for _, col := range t.Columns {
			columns = append(columns, identifier(col.Name)+"::text")
		}
		// Tenant predicate is defense in depth; RLS remains enabled and forced.
		query := "select " + strings.Join(columns, ",") + " from " + identifier(o.Schema) + "." + identifier(t.Name) + " where tenant_id::text=$1 order by " + strings.Join(columns, ", ") + " nulls first"
		rows, e := tx.Query(ctx, query, o.Tenant)
		if e != nil {
			return e
		}
		name := t.Name + ".jsonl"
		path := filepath.Join(stage, name)
		file, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			rows.Close()
			return e
		}
		h := sha256.New()
		var count int64
		for rows.Next() {
			cells := make([]*string, len(columns))
			targets := make([]any, len(columns))
			for i := range cells {
				targets[i] = &cells[i]
			}
			if e = rows.Scan(targets...); e != nil {
				break
			}
			var body []byte
			body, e = json.Marshal(cells)
			if e != nil {
				break
			}
			body = append(body, '\n')
			if _, e = file.Write(body); e != nil {
				break
			}
			h.Write(body)
			count++
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		closeErr := file.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
		manifest.Files[name] = File{hex.EncodeToString(h.Sum(nil)), count}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	// Atomic no-clobber directory reservation, manifest published last.
	if err = os.Mkdir(o.Output, 0700); err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			os.RemoveAll(o.Output)
		}
	}()
	names := []string{}
	for name := range manifest.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err = os.Rename(filepath.Join(stage, name), filepath.Join(o.Output, name)); err != nil {
			return err
		}
	}
	body, _ := json.MarshalIndent(manifest, "", "  ")
	if err = os.WriteFile(filepath.Join(o.Output, "manifest.json"), append(body, '\n'), 0600); err != nil {
		return err
	}
	complete = true
	return nil
}
