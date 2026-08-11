package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Postgres struct{ db, reader *sql.DB }

var ErrConcurrentState = errors.New("MiniPOS repository generation conflict")

type stateRow struct {
	Collection, Key string
	Payload         json.RawMessage
}

var mapCollections = map[string]bool{"products": true, "employees": true, "identity_bindings": true, "operator_sessions": true, "shifts": true, "orders": true, "checkouts": true, "checkout_hashes": true, "api_replays": true, "webhook_inbox": true, "configurations": true}

func Open(url string) (*Postgres, error) {
	if url == "" {
		return nil, errors.New("database url required")
	}
	db, e := sql.Open("pgx", url)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	for _, q := range []string{`create table if not exists minipos_state_rows(collection text not null,entity_key text not null,payload jsonb not null,updated_at timestamptz not null default now(),primary key(collection,entity_key))`, `create table if not exists minipos_state_meta(singleton boolean primary key default true check(singleton),generation bigint not null,updated_at timestamptz not null default now())`} {
		if _, e = db.ExecContext(ctx, q); e != nil {
			db.Close()
			return nil, e
		}
	}
	if _, e = db.ExecContext(ctx, `alter table minipos_state_meta add column if not exists sequence bigint not null default 0, add column if not exists storage_mode smallint not null default 1`); e != nil {
		db.Close()
		return nil, e
	}
	return &Postgres{db: db, reader: db}, nil
}
func OpenWithReader(writeURL, readURL string) (*Postgres, error) {
	p, e := Open(writeURL)
	if e != nil {
		return nil, e
	}
	if readURL == "" || readURL == writeURL {
		return p, nil
	}
	reader, e := sql.Open("pgx", readURL)
	if e != nil {
		p.Close()
		return nil, e
	}
	reader.SetMaxOpenConns(20)
	reader.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e = reader.PingContext(ctx); e != nil {
		reader.Close()
		p.Close()
		return nil, e
	}
	p.reader = reader
	return p, nil
}
func (p *Postgres) Load() ([]byte, error) {
	rows, e := p.db.Query(`select collection,entity_key,payload from minipos_state_rows order by collection,entity_key`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	flat := []stateRow{}
	for rows.Next() {
		var x stateRow
		if e = rows.Scan(&x.Collection, &x.Key, &x.Payload); e != nil {
			return nil, e
		}
		flat = append(flat, x)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	var generation int64
	metaErr := p.db.QueryRow(`select generation from minipos_state_meta where singleton=true`).Scan(&generation)
	if metaErr != nil && !errors.Is(metaErr, sql.ErrNoRows) {
		return nil, metaErr
	}
	if len(flat) > 0 || metaErr == nil {
		return rebuildSnapshot(flat)
	}
	var legacy []byte
	e = p.db.QueryRow(`select payload from runtime_snapshots where aggregate='minipos'`).Scan(&legacy)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if e = p.Save(legacy); e != nil {
		return nil, fmt.Errorf("migrate legacy snapshot: %w", e)
	}
	return legacy, nil
}
func (p *Postgres) Save(raw []byte) error {
	_, e := p.saveVersioned(raw, nil)
	return e
}
func (p *Postgres) LoadVersioned() ([]byte, int64, error) {
	tx, e := p.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return nil, 0, e
	}
	defer tx.Rollback()
	var generation, sequence int64
	var storageMode int
	metaErr := tx.QueryRow(`select generation,sequence,storage_mode from minipos_state_meta where singleton=true`).Scan(&generation, &sequence, &storageMode)
	if metaErr != nil && !errors.Is(metaErr, sql.ErrNoRows) {
		return nil, 0, metaErr
	}
	if metaErr == nil && storageMode == 2 {
		b, loadErr := loadMiniPOSTypedSnapshot(tx, sequence)
		if loadErr != nil {
			return nil, 0, loadErr
		}
		if e = tx.Commit(); e != nil {
			return nil, 0, e
		}
		return b, generation, nil
	}
	rows, e := tx.Query(`select collection,entity_key,payload from minipos_state_rows order by collection,entity_key`)
	if e != nil {
		return nil, 0, e
	}
	flat := make([]stateRow, 0)
	for rows.Next() {
		var row stateRow
		if e = rows.Scan(&row.Collection, &row.Key, &row.Payload); e != nil {
			rows.Close()
			return nil, 0, e
		}
		flat = append(flat, row)
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return nil, 0, e
	}
	if e = rows.Close(); e != nil {
		return nil, 0, e
	}
	if len(flat) > 0 || metaErr == nil {
		b, rebuildErr := rebuildSnapshot(flat)
		if rebuildErr != nil {
			return nil, 0, rebuildErr
		}
		if e = tx.Commit(); e != nil {
			return nil, 0, e
		}
		return b, generation, nil
	}
	_ = tx.Rollback()
	b, e := p.Load()
	if e != nil || len(b) == 0 {
		return b, 0, e
	}
	return p.LoadVersioned()
}
func (p *Postgres) SaveVersioned(raw []byte, expected int64) (int64, error) {
	return p.saveVersioned(raw, &expected)
}
func (p *Postgres) SaveDeltaVersioned(previous, current []byte, expected int64) (int64, error) {
	oldRows, e := snapshotRowsOrEmpty(previous)
	if e != nil {
		return 0, e
	}
	newRows, e := snapshotRowsOrEmpty(current)
	if e != nil {
		return 0, e
	}
	oldByID := make(map[string]stateRow, len(oldRows))
	newByID := make(map[string]stateRow, len(newRows))
	for _, row := range oldRows {
		oldByID[stateRowID(row.Collection, row.Key)] = row
	}
	for _, row := range newRows {
		newByID[stateRowID(row.Collection, row.Key)] = row
	}
	deletes := make([]stateRow, 0)
	upserts := make([]stateRow, 0)
	for id, row := range oldByID {
		if _, exists := newByID[id]; !exists {
			deletes = append(deletes, row)
		}
	}
	for id, row := range newByID {
		old, exists := oldByID[id]
		if !exists || !bytes.Equal(old.Payload, row.Payload) {
			upserts = append(upserts, row)
		}
	}
	sort.Slice(deletes, func(i, j int) bool {
		return stateRowID(deletes[i].Collection, deletes[i].Key) < stateRowID(deletes[j].Collection, deletes[j].Key)
	})
	sort.Slice(upserts, func(i, j int) bool {
		return stateRowID(upserts[i].Collection, upserts[i].Key) < stateRowID(upserts[j].Collection, upserts[j].Key)
	})
	tx, e := p.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var actual, persistedSequence int64
	storageMode := 1
	e = tx.QueryRow(`select generation,sequence,storage_mode from minipos_state_meta where singleton=true for update`).Scan(&actual, &persistedSequence, &storageMode)
	if errors.Is(e, sql.ErrNoRows) {
		actual, e = 0, nil
	}
	if e != nil {
		return 0, e
	}
	if actual != expected {
		return actual, ErrConcurrentState
	}
	sequence, e := miniPOSSequence(current)
	if e != nil {
		return 0, e
	}
	for _, row := range deletes {
		if storageMode == 1 {
			if _, e = tx.Exec(`delete from minipos_state_rows where collection=$1 and entity_key=$2`, row.Collection, row.Key); e != nil {
				return 0, e
			}
		}
		if e = deleteTypedProjection(tx, row); e != nil {
			return 0, e
		}
	}
	for _, row := range upserts {
		if storageMode == 1 {
			if _, e = tx.Exec(`insert into minipos_state_rows(collection,entity_key,payload,updated_at) values($1,$2,$3::jsonb,now()) on conflict(collection,entity_key) do update set payload=excluded.payload,updated_at=now() where minipos_state_rows.payload is distinct from excluded.payload`, row.Collection, row.Key, string(row.Payload)); e != nil {
				return 0, e
			}
		}
		if e = upsertTypedProjection(tx, row); e != nil {
			return 0, fmt.Errorf("typed projection %s/%s: %w", row.Collection, row.Key, e)
		}
	}
	if storageMode == 1 {
		ready, coverageErr := miniPOSTypedCoverageComplete(tx)
		if coverageErr != nil {
			return 0, coverageErr
		}
		if ready {
			storageMode = 2
		}
	}
	var generation int64
	if e = tx.QueryRow(`insert into minipos_state_meta(singleton,generation,sequence,storage_mode,updated_at) values(true,1,$1,$2,now()) on conflict(singleton) do update set generation=minipos_state_meta.generation+1,sequence=excluded.sequence,storage_mode=excluded.storage_mode,updated_at=excluded.updated_at returning generation`, sequence, storageMode).Scan(&generation); e != nil {
		return 0, e
	}
	if e = tx.Commit(); e != nil {
		return 0, e
	}
	return generation, nil
}

func miniPOSSequence(raw []byte) (int64, error) {
	var root struct {
		Sequence int64 `json:"sequence"`
	}
	if err := json.Unmarshal(raw, &root); err != nil || root.Sequence < 0 {
		return 0, errors.New("invalid MiniPOS sequence")
	}
	return root.Sequence, nil
}

// loadMiniPOSTypedSnapshot reconstructs the coordinator cache exclusively from
// constrained typed tables. Compatibility rows are deliberately not consulted.
func loadMiniPOSTypedSnapshot(tx *sql.Tx, sequence int64) ([]byte, error) {
	rows, err := tx.Query(`
select collection,entity_key,payload from (
  select 'products'::text collection,id entity_key,payload from minipos_runtime_products
  union all select 'employees',id,payload from minipos_runtime_employees
  union all select 'identity_bindings',binding_key,payload from minipos_runtime_identity_bindings
  union all select 'operator_sessions',session_key,payload from minipos_runtime_operator_sessions
  union all select 'shifts',id,payload from minipos_runtime_shifts
  union all select 'orders',id,payload from minipos_runtime_orders
  union all select 'configurations',organization_id,payload from minipos_runtime_configurations
  union all select 'api_replays',replay_key,payload from minipos_runtime_api_replays
  union all select 'webhook_inbox',event_id,payload from minipos_runtime_webhook_inbox
  union all select 'checkouts',replay_key,payload from minipos_runtime_checkout_results
  union all select 'checkout_hashes',replay_key,to_jsonb(request_hash) from minipos_runtime_checkout_hashes
) typed where payload is not null order by collection,entity_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	flat := []stateRow{{Collection: "sequence", Key: "singleton", Payload: json.RawMessage(strconv.FormatInt(sequence, 10))}}
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
	return rebuildSnapshot(flat)
}

// miniPOSTypedCoverageComplete is the one-time cutover guard. Every active
// compatibility entity must have an exact typed payload before mode 2 is set.
func miniPOSTypedCoverageComplete(tx *sql.Tx) (bool, error) {
	var missing int
	err := tx.QueryRow(`select count(*) from minipos_state_rows s where s.collection <> 'sequence' and not (
  (s.collection='products' and exists(select 1 from minipos_runtime_products t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='employees' and exists(select 1 from minipos_runtime_employees t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='identity_bindings' and exists(select 1 from minipos_runtime_identity_bindings t where t.binding_key=s.entity_key and t.payload=s.payload)) or
  (s.collection='operator_sessions' and exists(select 1 from minipos_runtime_operator_sessions t where t.session_key=s.entity_key and t.payload=s.payload)) or
  (s.collection='shifts' and exists(select 1 from minipos_runtime_shifts t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='orders' and exists(select 1 from minipos_runtime_orders t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='configurations' and exists(select 1 from minipos_runtime_configurations t where t.organization_id=s.entity_key and t.payload=s.payload)) or
  (s.collection='api_replays' and exists(select 1 from minipos_runtime_api_replays t where t.replay_key=s.entity_key and t.payload=s.payload)) or
  (s.collection='webhook_inbox' and exists(select 1 from minipos_runtime_webhook_inbox t where t.event_id=s.entity_key and t.payload=s.payload)) or
  (s.collection='checkouts' and exists(select 1 from minipos_runtime_checkout_results t where t.replay_key=s.entity_key and t.payload=s.payload)) or
  (s.collection='checkout_hashes' and exists(select 1 from minipos_runtime_checkout_hashes t where t.replay_key=s.entity_key and to_jsonb(t.request_hash)=s.payload))
)`).Scan(&missing)
	return missing == 0, err
}

func snapshotRowsOrEmpty(raw []byte) ([]stateRow, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	return flattenSnapshot(raw)
}
func (p *Postgres) saveVersioned(raw []byte, expected *int64) (int64, error) {
	rows, e := flattenSnapshot(raw)
	if e != nil {
		return 0, e
	}
	tx, e := p.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var current int64
	e = tx.QueryRow(`select generation from minipos_state_meta where singleton=true for update`).Scan(&current)
	if errors.Is(e, sql.ErrNoRows) {
		current, e = 0, nil
	}
	if e != nil {
		return 0, e
	}
	if expected != nil && current != *expected {
		return current, ErrConcurrentState
	}
	desired := make(map[string]bool, len(rows))
	for _, row := range rows {
		desired[stateRowID(row.Collection, row.Key)] = true
	}
	existing, e := tx.Query(`select collection,entity_key,payload from minipos_state_rows for update`)
	if e != nil {
		return 0, e
	}
	stale := make([]stateRow, 0)
	for existing.Next() {
		var row stateRow
		if e = existing.Scan(&row.Collection, &row.Key, &row.Payload); e != nil {
			existing.Close()
			return 0, e
		}
		if !desired[stateRowID(row.Collection, row.Key)] {
			stale = append(stale, row)
		}
	}
	if e = existing.Err(); e != nil {
		existing.Close()
		return 0, e
	}
	if e = existing.Close(); e != nil {
		return 0, e
	}
	for _, row := range stale {
		if _, e = tx.Exec(`delete from minipos_state_rows where collection=$1 and entity_key=$2`, row.Collection, row.Key); e != nil {
			return 0, e
		}
		if e = deleteTypedProjection(tx, row); e != nil {
			return 0, e
		}
	}
	stmt, e := tx.Prepare(`insert into minipos_state_rows(collection,entity_key,payload,updated_at) values($1,$2,$3::jsonb,now()) on conflict(collection,entity_key) do update set payload=excluded.payload,updated_at=now() where minipos_state_rows.payload is distinct from excluded.payload`)
	if e != nil {
		return 0, e
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, e = stmt.Exec(row.Collection, row.Key, string(row.Payload)); e != nil {
			return 0, e
		}
		if e = upsertTypedProjection(tx, row); e != nil {
			return 0, fmt.Errorf("typed projection %s/%s: %w", row.Collection, row.Key, e)
		}
	}
	var generation int64
	if e = tx.QueryRow(`insert into minipos_state_meta(singleton,generation,updated_at) values(true,1,now()) on conflict(singleton) do update set generation=minipos_state_meta.generation+1,updated_at=excluded.updated_at returning generation`).Scan(&generation); e != nil {
		return 0, e
	}
	if e = tx.Commit(); e != nil {
		return 0, e
	}
	return generation, nil
}
func (p *Postgres) Close() error {
	if p.reader != nil && p.reader != p.db {
		_ = p.reader.Close()
	}
	return p.db.Close()
}

func (p *Postgres) LoadTenantEntity(collection, tenant, id string) ([]byte, error) {
	if tenant == "" || id == "" {
		return nil, sql.ErrNoRows
	}
	tx, e := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelReadCommitted})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`set local role beeminipos_tenant`); e != nil {
		return nil, e
	}
	if _, e = tx.Exec(`select set_config('app.organization_id',$1,true)`, tenant); e != nil {
		return nil, e
	}
	var raw []byte
	switch collection {
	case "products":
		e = tx.QueryRow(`select jsonb_strip_nulls(jsonb_build_object('id',id,'tenant_id',organization_id,'sku',sku,'barcode',barcode,'name',name,'unit',unit,'price',jsonb_build_object('amount',amount::text,'currency',currency),'tax_group',tax_group,'active',active,'status',status,'version',version,'created_at',created_at,'updated_at',updated_at)) from minipos_runtime_products where id=$1`, id).Scan(&raw)
	case "employees":
		e = tx.QueryRow(`select jsonb_build_object('id',id,'tenant_id',organization_id,'first_name',first_name,'last_name',last_name,'operator_code',operator_code,'roles',roles,'active',active,'status',status,'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_employees where id=$1`, id).Scan(&raw)
	case "shifts":
		e = tx.QueryRow(`select jsonb_strip_nulls(jsonb_build_object('id',id,'tenant_id',organization_id,'register_id',register_id,'employee_id',employee_id,'state',state,'version',version,'opened_at',opened_at,'closed_at',closed_at,'z_operation_id',z_operation_id,'z_fiscal_reference',z_fiscal_reference,'close_error',close_error,'created_at',created_at,'updated_at',updated_at)) from minipos_runtime_shifts where id=$1`, id).Scan(&raw)
	case "orders":
		e = tx.QueryRow(`select jsonb_build_object('id',id,'tenant_id',organization_id,'external_id',external_id,'shift_id',shift_id,'register_id',register_id,'operator_code',operator_code,'state',state,'total',jsonb_build_object('amount',total::text,'currency',currency),'lines',lines,'fiscal_sale_id',coalesce(fiscal_sale_id,''),'fiscal_operation_id',coalesce(fiscal_operation_id,''),'receipt_reference',coalesce(fiscal_reference,''),'reversal_operation_id',coalesce(reversal_operation_id,''),'reversal_fiscal_reference',coalesce(reversal_fiscal_reference,''),'reversal_reason_code',coalesce(reversal_reason_code,''),'fiscal_version',coalesce(fiscal_version,0),'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_orders where id=$1`, id).Scan(&raw)
	case "configurations":
		e = tx.QueryRow(`select jsonb_build_object('id',id,'tenant_id',organization_id,'location_name',location_name,'location_address',location_address,'workstation_name',workstation_name,'fiscal_register_id',fiscal_register_id,'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_configurations where id=$1`, id).Scan(&raw)
	case "api_replays":
		e = tx.QueryRow(`select payload from minipos_runtime_api_replays where replay_key=$1`, id).Scan(&raw)
	case "webhook_inbox":
		e = tx.QueryRow(`select payload from minipos_runtime_webhook_inbox where event_id=$1`, id).Scan(&raw)
	case "checkouts":
		e = tx.QueryRow(`select payload from minipos_runtime_checkout_results where replay_key=$1`, id).Scan(&raw)
	case "checkout_hashes":
		e = tx.QueryRow(`select to_json(request_hash) from minipos_runtime_checkout_hashes where replay_key=$1`, id).Scan(&raw)
	default:
		return nil, errors.New("unsupported typed collection")
	}
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return raw, nil
}
func (p *Postgres) LoadTenantEntities(collection, tenant string) ([][]byte, error) {
	if tenant == "" {
		return nil, errors.New("tenant required")
	}
	tx, e := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelReadCommitted})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`set local role beeminipos_tenant`); e != nil {
		return nil, e
	}
	if _, e = tx.Exec(`select set_config('app.organization_id',$1,true)`, tenant); e != nil {
		return nil, e
	}
	query := ""
	switch collection {
	case "products":
		query = `select jsonb_strip_nulls(jsonb_build_object('id',id,'tenant_id',organization_id,'sku',sku,'barcode',barcode,'name',name,'unit',unit,'price',jsonb_build_object('amount',amount::text,'currency',currency),'tax_group',tax_group,'active',active,'status',status,'version',version,'created_at',created_at,'updated_at',updated_at)) from minipos_runtime_products order by name,id`
	case "employees":
		query = `select jsonb_build_object('id',id,'tenant_id',organization_id,'first_name',first_name,'last_name',last_name,'operator_code',operator_code,'roles',roles,'active',active,'status',status,'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_employees order by last_name,first_name,id`
	case "shifts":
		query = `select jsonb_strip_nulls(jsonb_build_object('id',id,'tenant_id',organization_id,'register_id',register_id,'employee_id',employee_id,'state',state,'version',version,'opened_at',opened_at,'closed_at',closed_at,'z_operation_id',z_operation_id,'z_fiscal_reference',z_fiscal_reference,'close_error',close_error,'created_at',created_at,'updated_at',updated_at)) from minipos_runtime_shifts order by opened_at,id`
	case "orders":
		query = `select jsonb_build_object('id',id,'tenant_id',organization_id,'external_id',external_id,'shift_id',shift_id,'register_id',register_id,'operator_code',operator_code,'state',state,'total',jsonb_build_object('amount',total::text,'currency',currency),'lines',lines,'payments',payments,'fiscal_sale_id',coalesce(fiscal_sale_id,''),'fiscal_operation_id',coalesce(fiscal_operation_id,''),'receipt_reference',coalesce(fiscal_reference,''),'reversal_operation_id',coalesce(reversal_operation_id,''),'reversal_fiscal_reference',coalesce(reversal_fiscal_reference,''),'reversal_reason_code',coalesce(reversal_reason_code,''),'fiscal_version',coalesce(fiscal_version,0),'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_orders order by created_at,id`
	case "configurations":
		query = `select jsonb_build_object('id',id,'tenant_id',organization_id,'location_name',location_name,'location_address',location_address,'workstation_name',workstation_name,'fiscal_register_id',fiscal_register_id,'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_configurations order by id`
	default:
		return nil, errors.New("unsupported typed collection")
	}
	rows, e := tx.Query(query)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([][]byte, 0)
	for rows.Next() {
		var raw []byte
		if e = rows.Scan(&raw); e != nil {
			return nil, e
		}
		out = append(out, raw)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	if e = rows.Close(); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	return out, nil
}

func stateRowID(collection, key string) string { return collection + "\x00" + key }

func deleteTypedProjection(tx *sql.Tx, row stateRow) error {
	if row.Collection == "configurations" {
		return withOrganizationRole(tx, row, func() error {
			var identity struct {
				ID string `json:"id"`
			}
			if e := json.Unmarshal(row.Payload, &identity); e != nil || identity.ID == "" {
				return errors.New("typed configuration id required")
			}
			_, e := tx.Exec(`delete from minipos_runtime_configurations where id=$1`, identity.ID)
			return e
		})
	}
	tables := map[string]string{"products": "minipos_runtime_products", "employees": "minipos_runtime_employees", "identity_bindings": "minipos_runtime_identity_bindings", "operator_sessions": "minipos_runtime_operator_sessions", "shifts": "minipos_runtime_shifts", "orders": "minipos_runtime_orders", "configurations": "minipos_runtime_configurations", "api_replays": "minipos_runtime_api_replays", "webhook_inbox": "minipos_runtime_webhook_inbox", "checkouts": "minipos_runtime_checkout_results", "checkout_hashes": "minipos_runtime_checkout_hashes"}
	table := tables[row.Collection]
	if table == "" {
		return nil
	}
	return withOrganizationRole(tx, row, func() error {
		idColumn := "id"
		if row.Collection == "api_replays" {
			idColumn = "replay_key"
		} else if row.Collection == "webhook_inbox" {
			idColumn = "event_id"
		} else if row.Collection == "checkouts" || row.Collection == "checkout_hashes" {
			idColumn = "replay_key"
		} else if row.Collection == "identity_bindings" {
			idColumn = "binding_key"
		} else if row.Collection == "operator_sessions" {
			idColumn = "session_key"
		}
		_, err := tx.Exec(`delete from `+table+` where `+idColumn+`=$1`, row.Key)
		return err
	})
}
func upsertTypedProjection(tx *sql.Tx, row stateRow) error {
	return withOrganizationRole(tx, row, func() error { return upsertTypedProjectionAsOrganization(tx, row) })
}

func upsertTypedProjectionAsOrganization(tx *sql.Tx, row stateRow) error {
	p := string(row.Payload)
	switch row.Collection {
	case "products":
		_, e := tx.Exec(`insert into minipos_runtime_products(id,organization_id,sku,barcode,name,unit,amount,currency,tax_group,active,status,version,created_at,updated_at,payload) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'sku',nullif($2::jsonb->>'barcode',''),$2::jsonb->>'name',$2::jsonb->>'unit',($2::jsonb->'price'->>'amount')::numeric,$2::jsonb->'price'->>'currency',$2::jsonb->>'tax_group',($2::jsonb->>'active')::boolean,$2::jsonb->>'status',($2::jsonb->>'version')::bigint,($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz,$2::jsonb) on conflict(id) do update set organization_id=excluded.organization_id,sku=excluded.sku,barcode=excluded.barcode,name=excluded.name,unit=excluded.unit,amount=excluded.amount,currency=excluded.currency,tax_group=excluded.tax_group,active=excluded.active,status=excluded.status,version=excluded.version,created_at=excluded.created_at,updated_at=excluded.updated_at,payload=excluded.payload where minipos_runtime_products is distinct from excluded`, row.Key, p)
		return e
	case "employees":
		_, e := tx.Exec(`insert into minipos_runtime_employees(id,organization_id,first_name,last_name,operator_code,roles,active,status,version,created_at,updated_at,payload) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'first_name',$2::jsonb->>'last_name',$2::jsonb->>'operator_code',$2::jsonb->'roles',($2::jsonb->>'active')::boolean,$2::jsonb->>'status',($2::jsonb->>'version')::bigint,($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz,$2::jsonb) on conflict(id) do update set organization_id=excluded.organization_id,first_name=excluded.first_name,last_name=excluded.last_name,operator_code=excluded.operator_code,roles=excluded.roles,active=excluded.active,status=excluded.status,version=excluded.version,created_at=excluded.created_at,updated_at=excluded.updated_at,payload=excluded.payload where minipos_runtime_employees is distinct from excluded`, row.Key, p)
		return e
	case "identity_bindings":
		_, e := tx.Exec(`insert into minipos_runtime_identity_bindings(binding_key,organization_id,employee_id,subject_hash,identity_issuer,bound_at,payload) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'employee_id',$2::jsonb->>'subject_hash',$2::jsonb->>'identity_issuer',($2::jsonb->>'bound_at')::timestamptz,$2::jsonb) on conflict(binding_key) do update set organization_id=excluded.organization_id,employee_id=excluded.employee_id,subject_hash=excluded.subject_hash,identity_issuer=excluded.identity_issuer,bound_at=excluded.bound_at,payload=excluded.payload where minipos_runtime_identity_bindings is distinct from excluded`, row.Key, p)
		return e
	case "operator_sessions":
		_, e := tx.Exec(`insert into minipos_runtime_operator_sessions(session_key,organization_id,employee_id,app_instance_id,credential_fingerprint,state,first_seen,expires_at,revoked_at,payload) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'employee_id',$2::jsonb->>'app_instance_id',$2::jsonb->>'token_hash',$2::jsonb->>'state',($2::jsonb->>'first_seen')::timestamptz,($2::jsonb->>'expires_at')::timestamptz,nullif($2::jsonb->>'revoked_at','')::timestamptz,$2::jsonb) on conflict(session_key) do update set organization_id=excluded.organization_id,employee_id=excluded.employee_id,app_instance_id=excluded.app_instance_id,credential_fingerprint=excluded.credential_fingerprint,state=excluded.state,first_seen=excluded.first_seen,expires_at=excluded.expires_at,revoked_at=excluded.revoked_at,payload=excluded.payload where minipos_runtime_operator_sessions is distinct from excluded`, row.Key, p)
		return e
	case "shifts":
		_, e := tx.Exec(`insert into minipos_runtime_shifts(id,organization_id,register_id,employee_id,state,version,opened_at,closed_at,z_operation_id,z_fiscal_reference,close_error,created_at,updated_at,payload) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'register_id',$2::jsonb->>'employee_id',$2::jsonb->>'state',($2::jsonb->>'version')::bigint,($2::jsonb->>'opened_at')::timestamptz,nullif($2::jsonb->>'closed_at','')::timestamptz,nullif($2::jsonb->>'z_operation_id',''),nullif($2::jsonb->>'z_fiscal_reference',''),nullif($2::jsonb->>'close_error',''),($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz,$2::jsonb) on conflict(id) do update set organization_id=excluded.organization_id,register_id=excluded.register_id,employee_id=excluded.employee_id,state=excluded.state,version=excluded.version,opened_at=excluded.opened_at,closed_at=excluded.closed_at,z_operation_id=excluded.z_operation_id,z_fiscal_reference=excluded.z_fiscal_reference,close_error=excluded.close_error,created_at=excluded.created_at,updated_at=excluded.updated_at,payload=excluded.payload where minipos_runtime_shifts is distinct from excluded`, row.Key, p)
		return e
	case "orders":
		_, e := tx.Exec(`insert into minipos_runtime_orders(id,organization_id,external_id,shift_id,register_id,operator_code,state,total,currency,lines,payments,fiscal_sale_id,fiscal_operation_id,fiscal_reference,reversal_operation_id,reversal_fiscal_reference,reversal_reason_code,fiscal_version,version,created_at,updated_at,payload) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'external_id',$2::jsonb->>'shift_id',$2::jsonb->>'register_id',$2::jsonb->>'operator_code',$2::jsonb->>'state',($2::jsonb->'total'->>'amount')::numeric,$2::jsonb->'total'->>'currency',$2::jsonb->'lines',coalesce($2::jsonb->'payments','[]'::jsonb),nullif($2::jsonb->>'fiscal_sale_id',''),nullif($2::jsonb->>'fiscal_operation_id',''),nullif($2::jsonb->>'receipt_reference',''),nullif($2::jsonb->>'reversal_operation_id',''),nullif($2::jsonb->>'reversal_fiscal_reference',''),nullif($2::jsonb->>'reversal_reason_code',''),nullif($2::jsonb->>'fiscal_version','')::bigint,($2::jsonb->>'version')::bigint,($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz,$2::jsonb) on conflict(id) do update set organization_id=excluded.organization_id,external_id=excluded.external_id,shift_id=excluded.shift_id,register_id=excluded.register_id,operator_code=excluded.operator_code,state=excluded.state,total=excluded.total,currency=excluded.currency,lines=excluded.lines,payments=excluded.payments,fiscal_sale_id=excluded.fiscal_sale_id,fiscal_operation_id=excluded.fiscal_operation_id,fiscal_reference=excluded.fiscal_reference,reversal_operation_id=excluded.reversal_operation_id,reversal_fiscal_reference=excluded.reversal_fiscal_reference,reversal_reason_code=excluded.reversal_reason_code,fiscal_version=excluded.fiscal_version,version=excluded.version,created_at=excluded.created_at,updated_at=excluded.updated_at,payload=excluded.payload where minipos_runtime_orders is distinct from excluded`, row.Key, p)
		return e
	case "configurations":
		_, e := tx.Exec(`insert into minipos_runtime_configurations(id,organization_id,location_name,location_address,workstation_name,fiscal_register_id,version,created_at,updated_at,payload) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'location_name',$2::jsonb->>'location_address',$2::jsonb->>'workstation_name',$2::jsonb->>'fiscal_register_id',($2::jsonb->>'version')::bigint,($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz,$2::jsonb) on conflict(id) do update set organization_id=excluded.organization_id,location_name=excluded.location_name,location_address=excluded.location_address,workstation_name=excluded.workstation_name,fiscal_register_id=excluded.fiscal_register_id,version=excluded.version,created_at=excluded.created_at,updated_at=excluded.updated_at,payload=excluded.payload where minipos_runtime_configurations is distinct from excluded`, jsonID(row.Payload), p)
		return e
	case "api_replays":
		_, e := tx.Exec(`insert into minipos_runtime_api_replays(replay_key,organization_id,method,path,request_hash,status,pending,payload)
values($1,split_part($1,E'\n',1),split_part($1,E'\n',2),split_part($1,E'\n',3),$2::jsonb->>'hash',($2::jsonb->>'status')::integer,coalesce(($2::jsonb->>'pending')::boolean,false),$2::jsonb)
on conflict(replay_key) do update set organization_id=excluded.organization_id,method=excluded.method,path=excluded.path,request_hash=excluded.request_hash,status=excluded.status,pending=excluded.pending,payload=excluded.payload where minipos_runtime_api_replays is distinct from excluded`, row.Key, p)
		return e
	case "webhook_inbox":
		_, e := tx.Exec(`insert into minipos_runtime_webhook_inbox(event_id,organization_id,request_hash,received_at,processed_at,error,payload)
values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'hash',($2::jsonb->>'received_at')::timestamptz,nullif($2::jsonb->>'processed_at','')::timestamptz,nullif($2::jsonb->>'error',''),$2::jsonb)
on conflict(event_id) do update set organization_id=excluded.organization_id,request_hash=excluded.request_hash,received_at=excluded.received_at,processed_at=excluded.processed_at,error=excluded.error,payload=excluded.payload where minipos_runtime_webhook_inbox is distinct from excluded`, row.Key, p)
		return e
	case "checkouts":
		_, e := tx.Exec(`insert into minipos_runtime_checkout_results(replay_key,organization_id,order_id,state,version,fiscal_sale_id,fiscal_operation_id,fiscal_reference,payload)
values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'id',$2::jsonb->>'state',($2::jsonb->>'version')::bigint,nullif($2::jsonb->>'fiscal_sale_id',''),nullif($2::jsonb->>'fiscal_operation_id',''),nullif($2::jsonb->>'receipt_reference',''),$2::jsonb)
on conflict(replay_key) do update set organization_id=excluded.organization_id,order_id=excluded.order_id,state=excluded.state,version=excluded.version,fiscal_sale_id=excluded.fiscal_sale_id,fiscal_operation_id=excluded.fiscal_operation_id,fiscal_reference=excluded.fiscal_reference,payload=excluded.payload where minipos_runtime_checkout_results is distinct from excluded`, row.Key, p)
		return e
	case "checkout_hashes":
		_, e := tx.Exec(`insert into minipos_runtime_checkout_hashes(replay_key,organization_id,request_hash)
values($1,split_part($1,':',1),$2::jsonb#>>'{}')
on conflict(replay_key) do update set organization_id=excluded.organization_id,request_hash=excluded.request_hash where minipos_runtime_checkout_hashes is distinct from excluded`, row.Key, p)
		return e
	}
	return nil
}

func jsonID(raw json.RawMessage) string {
	var identity struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &identity)
	return identity.ID
}

func withOrganizationRole(tx *sql.Tx, row stateRow, mutate func() error) error {
	if _, ok := map[string]bool{"products": true, "employees": true, "identity_bindings": true, "operator_sessions": true, "shifts": true, "orders": true, "configurations": true, "api_replays": true, "webhook_inbox": true, "checkouts": true, "checkout_hashes": true}[row.Collection]; !ok {
		return nil
	}
	var identity struct {
		OrganizationID string `json:"tenant_id"`
	}
	if row.Collection == "checkout_hashes" {
		parts := strings.SplitN(row.Key, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return errors.New("typed checkout hash key must contain tenant and idempotency key")
		}
		identity.OrganizationID = parts[0]
	} else if row.Collection == "api_replays" {
		parts := strings.Split(row.Key, "\n")
		if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
			return errors.New("typed API replay key must contain tenant, method, path and idempotency key")
		}
		identity.OrganizationID = parts[0]
	} else if e := json.Unmarshal(row.Payload, &identity); e != nil {
		return e
	}
	if identity.OrganizationID == "" {
		if row.Collection == "configurations" {
			return nil // legacy pre-typed configuration rows remain readable until rewritten by the domain API
		}
		return errors.New("typed projection organization_id required")
	}
	if _, e := tx.Exec(`set local role beeminipos_tenant`); e != nil {
		return e
	}
	if _, e := tx.Exec(`select set_config('app.organization_id',$1,true)`, identity.OrganizationID); e != nil {
		return e
	}
	if e := mutate(); e != nil {
		return e
	}
	_, e := tx.Exec(`set local role none`)
	return e
}

func flattenSnapshot(raw []byte) ([]stateRow, error) {
	var root map[string]json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &root) != nil {
		return nil, errors.New("invalid MiniPOS repository state")
	}
	rows := []stateRow{}
	for collection, payload := range root {
		if collection == "sequence" {
			rows = append(rows, stateRow{collection, "singleton", append(json.RawMessage(nil), payload...)})
			continue
		}
		if !mapCollections[collection] {
			return nil, fmt.Errorf("unsupported state collection %q", collection)
		}
		var entries map[string]json.RawMessage
		if e := json.Unmarshal(payload, &entries); e != nil {
			return nil, e
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
	for c := range mapCollections {
		maps[c] = map[string]json.RawMessage{}
	}
	root := map[string]any{}
	root["sequence"] = json.RawMessage(`0`)
	for _, row := range rows {
		if row.Collection == "sequence" {
			if row.Key != "singleton" {
				return nil, errors.New("invalid sequence row")
			}
			root["sequence"] = append(json.RawMessage(nil), row.Payload...)
			continue
		}
		entries, ok := maps[row.Collection]
		if !ok {
			return nil, fmt.Errorf("unsupported persisted collection %q", row.Collection)
		}
		entries[row.Key] = append(json.RawMessage(nil), row.Payload...)
	}
	for c, v := range maps {
		// Identity bindings were added after the original snapshots. Preserve
		// byte/semantic compatibility for legacy states until the first binding
		// exists; the domain initializes an absent collection to an empty map.
		if (c == "identity_bindings" || c == "operator_sessions") && len(v) == 0 {
			continue
		}
		root[c] = v
	}
	return json.Marshal(root)
}
