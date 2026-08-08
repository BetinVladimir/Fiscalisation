package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"sort"
	"time"
)

type Postgres struct{ db, reader *sql.DB }

func Open(url string) (*Postgres, error) {
	if url == "" {
		return nil, errors.New("database url required")
	}
	db, e := sql.Open("pgx", url)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(20)
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	if _, e = db.ExecContext(ctx, `create table if not exists fiscal_state_rows(collection text not null,entity_key text not null,payload jsonb not null,updated_at timestamptz not null default now(),primary key(collection,entity_key))`); e != nil {
		db.Close()
		return nil, e
	}
	if _, e = db.ExecContext(ctx, `create table if not exists fiscal_state_meta(singleton boolean primary key default true check(singleton),generation bigint not null,updated_at timestamptz not null default now())`); e != nil {
		db.Close()
		return nil, e
	}
	return &Postgres{db: db, reader: db}, nil
}
func OpenWithReader(writeURL, readURL string) (*Postgres, error) {
	p, err := Open(writeURL)
	if err != nil {
		return nil, err
	}
	if readURL == "" || readURL == writeURL {
		return p, nil
	}
	reader, err := sql.Open("pgx", readURL)
	if err != nil {
		p.Close()
		return nil, err
	}
	reader.SetMaxOpenConns(20)
	reader.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = reader.PingContext(ctx); err != nil {
		reader.Close()
		p.Close()
		return nil, err
	}
	p.reader = reader
	return p, nil
}
func (p *Postgres) Load() ([]byte, error) {
	rows, err := p.db.Query(`select collection,entity_key,payload from fiscal_state_rows order by collection,entity_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	flat := make([]stateRow, 0)
	for rows.Next() {
		var row stateRow
		if err = rows.Scan(&row.Collection, &row.Key, &row.Payload); err != nil {
			return nil, err
		}
		flat = append(flat, row)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	var generation int64
	metaErr := p.db.QueryRow(`select generation from fiscal_state_meta where singleton=true`).Scan(&generation)
	if metaErr != nil && !errors.Is(metaErr, sql.ErrNoRows) {
		return nil, metaErr
	}
	if len(flat) > 0 || metaErr == nil {
		return rebuildSnapshot(flat)
	}
	// One-time compatibility migration from releases that stored one JSON blob.
	var legacy []byte
	err = p.db.QueryRow(`select payload from runtime_snapshots where aggregate='fiscal'`).Scan(&legacy)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err = p.Save(legacy); err != nil {
		return nil, fmt.Errorf("migrate legacy snapshot: %w", err)
	}
	return legacy, nil
}
func (p *Postgres) Save(b []byte) error {
	rows, err := flattenSnapshot(b)
	if err != nil {
		return err
	}
	tx, err := p.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	desired := make(map[string]bool, len(rows))
	for _, row := range rows {
		desired[stateRowID(row.Collection, row.Key)] = true
	}
	existing, err := tx.Query(`select collection,entity_key from fiscal_state_rows for update`)
	if err != nil {
		return err
	}
	stale := make([][2]string, 0)
	for existing.Next() {
		var collection, key string
		if err = existing.Scan(&collection, &key); err != nil {
			existing.Close()
			return err
		}
		if !desired[stateRowID(collection, key)] {
			stale = append(stale, [2]string{collection, key})
		}
	}
	if err = existing.Err(); err != nil {
		existing.Close()
		return err
	}
	if err = existing.Close(); err != nil {
		return err
	}
	for _, key := range stale {
		if _, err = tx.Exec(`delete from fiscal_state_rows where collection=$1 and entity_key=$2`, key[0], key[1]); err != nil {
			return err
		}
		if err = deleteTypedProjection(tx, key[0], key[1]); err != nil {
			return err
		}
	}
	stmt, err := tx.Prepare(`insert into fiscal_state_rows(collection,entity_key,payload,updated_at) values($1,$2,$3::jsonb,now()) on conflict(collection,entity_key) do update set payload=excluded.payload,updated_at=now() where fiscal_state_rows.payload is distinct from excluded.payload`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, err = stmt.Exec(row.Collection, row.Key, string(row.Payload)); err != nil {
			return err
		}
		if err = upsertTypedProjection(tx, row); err != nil {
			return fmt.Errorf("typed projection %s/%s: %w", row.Collection, row.Key, err)
		}
	}
	if _, err = tx.Exec(`insert into fiscal_state_meta(singleton,generation,updated_at) values(true,1,now()) on conflict(singleton) do update set generation=fiscal_state_meta.generation+1,updated_at=excluded.updated_at`); err != nil {
		return err
	}
	return tx.Commit()
}
func (p *Postgres) Close() error {
	if p.reader != nil && p.reader != p.db {
		_ = p.reader.Close()
	}
	return p.db.Close()
}

// LoadTenantEntity is the typed, RLS-bound read path used by public hot-path GETs.
func (p *Postgres) LoadTenantEntity(collection, tenant, id string) ([]byte, error) {
	if tenant == "" || id == "" {
		return nil, sql.ErrNoRows
	}
	tx, err := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`set local role beefiscal_tenant`); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`select set_config('app.tenant_id',$1,true)`, tenant); err != nil {
		return nil, err
	}
	var raw []byte
	switch collection {
	case "sales":
		err = tx.QueryRow(`select jsonb_build_object('sale_id',id,'tenant_id',tenant_id,'external_id',external_id,'register_id',register_id,'operator_id',operator_id,'unp',coalesce(unp,''),'state',state,'version',version,'lines',lines,'payments',payments,'fiscal_operation_id',coalesce(fiscal_operation_id,''),'receipt_artifact_id',coalesce(receipt_artifact_id,''),'created_at',created_at,'updated_at',updated_at) from fiscal_runtime_sales where id=$1`, id).Scan(&raw)
	case "operations":
		err = tx.QueryRow(`select jsonb_build_object('operation_id',id,'tenant_id',tenant_id,'sale_id',coalesce(sale_id,''),'type',type,'state',state,'version',version,'fiscal_reference',coalesce(fiscal_reference,''),'simulated',simulated,'error_code',coalesce(error_code,''),'created_at',created_at,'updated_at',updated_at) from fiscal_runtime_operations where id=$1`, id).Scan(&raw)
	default:
		return nil, errors.New("unsupported typed collection")
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return raw, nil
}
func (p *Postgres) LoadTenantEntities(collection, tenant string) ([][]byte, error) {
	if tenant == "" {
		return nil, errors.New("tenant required")
	}
	tx, err := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`set local role beefiscal_tenant`); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(`select set_config('app.tenant_id',$1,true)`, tenant); err != nil {
		return nil, err
	}
	query := ""
	switch collection {
	case "sales":
		query = `select jsonb_build_object('sale_id',id,'tenant_id',tenant_id,'external_id',external_id,'register_id',register_id,'operator_id',operator_id,'unp',coalesce(unp,''),'state',state,'version',version,'lines',lines,'payments',payments,'fiscal_operation_id',coalesce(fiscal_operation_id,''),'receipt_artifact_id',coalesce(receipt_artifact_id,''),'created_at',created_at,'updated_at',updated_at) from fiscal_runtime_sales order by created_at,id`
	case "operations":
		query = `select jsonb_build_object('operation_id',id,'tenant_id',tenant_id,'sale_id',coalesce(sale_id,''),'type',type,'state',state,'version',version,'fiscal_reference',coalesce(fiscal_reference,''),'simulated',simulated,'error_code',coalesce(error_code,''),'created_at',created_at,'updated_at',updated_at) from fiscal_runtime_operations order by created_at,id`
	default:
		return nil, errors.New("unsupported typed collection")
	}
	rows, err := tx.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([][]byte, 0)
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func stateRowID(collection, key string) string { return collection + "\x00" + key }

func deleteTypedProjection(tx *sql.Tx, collection, key string) error {
	table := ""
	switch collection {
	case "sales":
		table = "fiscal_runtime_sales"
	case "operations":
		table = "fiscal_runtime_operations"
	default:
		return nil
	}
	_, err := tx.Exec(`delete from `+table+` where id=$1`, key)
	return err
}
func upsertTypedProjection(tx *sql.Tx, row stateRow) error {
	payload := string(row.Payload)
	switch row.Collection {
	case "sales":
		_, err := tx.Exec(`insert into fiscal_runtime_sales(id,tenant_id,external_id,register_id,operator_id,unp,state,version,lines,payments,fiscal_operation_id,receipt_artifact_id,created_at,updated_at)
values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'external_id',$2::jsonb->>'register_id',$2::jsonb->>'operator_id',nullif($2::jsonb->>'unp',''),$2::jsonb->>'state',($2::jsonb->>'version')::bigint,$2::jsonb->'lines',$2::jsonb->'payments',nullif($2::jsonb->>'fiscal_operation_id',''),nullif($2::jsonb->>'receipt_artifact_id',''),($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz)
on conflict(id) do update set tenant_id=excluded.tenant_id,external_id=excluded.external_id,register_id=excluded.register_id,operator_id=excluded.operator_id,unp=excluded.unp,state=excluded.state,version=excluded.version,lines=excluded.lines,payments=excluded.payments,fiscal_operation_id=excluded.fiscal_operation_id,receipt_artifact_id=excluded.receipt_artifact_id,created_at=excluded.created_at,updated_at=excluded.updated_at
where fiscal_runtime_sales is distinct from excluded`, row.Key, payload)
		return err
	case "operations":
		_, err := tx.Exec(`insert into fiscal_runtime_operations(id,tenant_id,sale_id,type,state,version,fiscal_reference,simulated,error_code,created_at,updated_at)
values($1,$2::jsonb->>'tenant_id',nullif($2::jsonb->>'sale_id',''),$2::jsonb->>'type',$2::jsonb->>'state',($2::jsonb->>'version')::bigint,nullif($2::jsonb->>'fiscal_reference',''),coalesce(($2::jsonb->>'simulated')::boolean,false),nullif($2::jsonb->>'error_code',''),($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz)
on conflict(id) do update set tenant_id=excluded.tenant_id,sale_id=excluded.sale_id,type=excluded.type,state=excluded.state,version=excluded.version,fiscal_reference=excluded.fiscal_reference,simulated=excluded.simulated,error_code=excluded.error_code,created_at=excluded.created_at,updated_at=excluded.updated_at
where fiscal_runtime_operations is distinct from excluded`, row.Key, payload)
		return err
	}
	return nil
}

type stateRow struct {
	Collection string
	Key        string
	Payload    json.RawMessage
}

var mapCollections = map[string]bool{
	"sales": true, "operations": true, "devices": true, "shifts": true,
	"unp": true, "replays": true, "outbox": true, "ble_sessions": true,
	"sync_acks": true, "connectivity_probes": true, "resources": true,
	"artifacts": true, "edge_pending": true,
}

func flattenSnapshot(raw []byte) ([]stateRow, error) {
	var root map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &root) != nil {
		return nil, errors.New("invalid fiscal repository state")
	}
	rows := make([]stateRow, 0)
	for collection, payload := range root {
		if collection == "audit" {
			var entries []json.RawMessage
			if err := json.Unmarshal(payload, &entries); err != nil {
				return nil, err
			}
			for i, entry := range entries {
				rows = append(rows, stateRow{collection, fmt.Sprintf("%020d", i), append(json.RawMessage(nil), entry...)})
			}
			continue
		}
		if !mapCollections[collection] {
			return nil, fmt.Errorf("unsupported state collection %q", collection)
		}
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(payload, &entries); err != nil {
			return nil, err
		}
		for key, entry := range entries {
			rows = append(rows, stateRow{collection, key, append(json.RawMessage(nil), entry...)})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Collection == rows[j].Collection {
			return rows[i].Key < rows[j].Key
		}
		return rows[i].Collection < rows[j].Collection
	})
	return rows, nil
}

func rebuildSnapshot(rows []stateRow) ([]byte, error) {
	maps := map[string]map[string]json.RawMessage{}
	audit := make([]json.RawMessage, 0)
	for collection := range mapCollections {
		maps[collection] = map[string]json.RawMessage{}
	}
	for _, row := range rows {
		if row.Collection == "audit" {
			audit = append(audit, append(json.RawMessage(nil), row.Payload...))
			continue
		}
		entries, ok := maps[row.Collection]
		if !ok {
			return nil, fmt.Errorf("unsupported persisted collection %q", row.Collection)
		}
		entries[row.Key] = append(json.RawMessage(nil), row.Payload...)
	}
	root := map[string]any{"audit": audit}
	for collection, entries := range maps {
		root[collection] = entries
	}
	return json.Marshal(root)
}
