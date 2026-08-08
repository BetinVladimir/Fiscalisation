package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Postgres struct{ db, reader *sql.DB }
type stateRow struct {
	Collection, Key string
	Payload         json.RawMessage
}

var mapCollections = map[string]bool{"products": true, "employees": true, "shifts": true, "orders": true, "checkouts": true, "checkout_hashes": true, "api_replays": true, "webhook_inbox": true, "configurations": true}

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
	rows, e := flattenSnapshot(raw)
	if e != nil {
		return e
	}
	tx, e := p.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	desired := make(map[string]bool, len(rows))
	for _, row := range rows {
		desired[stateRowID(row.Collection, row.Key)] = true
	}
	existing, e := tx.Query(`select collection,entity_key from minipos_state_rows for update`)
	if e != nil {
		return e
	}
	stale := make([][2]string, 0)
	for existing.Next() {
		var collection, key string
		if e = existing.Scan(&collection, &key); e != nil {
			existing.Close()
			return e
		}
		if !desired[stateRowID(collection, key)] {
			stale = append(stale, [2]string{collection, key})
		}
	}
	if e = existing.Err(); e != nil {
		existing.Close()
		return e
	}
	if e = existing.Close(); e != nil {
		return e
	}
	for _, key := range stale {
		if _, e = tx.Exec(`delete from minipos_state_rows where collection=$1 and entity_key=$2`, key[0], key[1]); e != nil {
			return e
		}
		if e = deleteTypedProjection(tx, key[0], key[1]); e != nil {
			return e
		}
	}
	stmt, e := tx.Prepare(`insert into minipos_state_rows(collection,entity_key,payload,updated_at) values($1,$2,$3::jsonb,now()) on conflict(collection,entity_key) do update set payload=excluded.payload,updated_at=now() where minipos_state_rows.payload is distinct from excluded.payload`)
	if e != nil {
		return e
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, e = stmt.Exec(row.Collection, row.Key, string(row.Payload)); e != nil {
			return e
		}
		if e = upsertTypedProjection(tx, row); e != nil {
			return fmt.Errorf("typed projection %s/%s: %w", row.Collection, row.Key, e)
		}
	}
	if _, e = tx.Exec(`insert into minipos_state_meta(singleton,generation,updated_at) values(true,1,now()) on conflict(singleton) do update set generation=minipos_state_meta.generation+1,updated_at=excluded.updated_at`); e != nil {
		return e
	}
	return tx.Commit()
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
		e = tx.QueryRow(`select jsonb_build_object('id',id,'tenant_id',organization_id,'sku',sku,'name',name,'unit',unit,'price',jsonb_build_object('amount',amount::text,'currency',currency),'tax_group',tax_group,'active',active,'status',status,'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_products where id=$1`, id).Scan(&raw)
	case "employees":
		e = tx.QueryRow(`select jsonb_build_object('id',id,'tenant_id',organization_id,'first_name',first_name,'last_name',last_name,'operator_code',operator_code,'roles',roles,'active',active,'status',status,'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_employees where id=$1`, id).Scan(&raw)
	case "shifts":
		e = tx.QueryRow(`select jsonb_build_object('id',id,'tenant_id',organization_id,'register_id',register_id,'employee_id',employee_id,'state',state,'version',version,'opened_at',opened_at,'closed_at',closed_at,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_shifts where id=$1`, id).Scan(&raw)
	case "orders":
		e = tx.QueryRow(`select jsonb_build_object('id',id,'tenant_id',organization_id,'external_id',external_id,'shift_id',shift_id,'register_id',register_id,'operator_code',operator_code,'state',state,'total',jsonb_build_object('amount',total::text,'currency',currency),'lines',lines,'fiscal_sale_id',coalesce(fiscal_sale_id,''),'fiscal_operation_id',coalesce(fiscal_operation_id,''),'fiscal_version',coalesce(fiscal_version,0),'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_orders where id=$1`, id).Scan(&raw)
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
		query = `select jsonb_build_object('id',id,'tenant_id',organization_id,'sku',sku,'name',name,'unit',unit,'price',jsonb_build_object('amount',amount::text,'currency',currency),'tax_group',tax_group,'active',active,'status',status,'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_products order by name,id`
	case "employees":
		query = `select jsonb_build_object('id',id,'tenant_id',organization_id,'first_name',first_name,'last_name',last_name,'operator_code',operator_code,'roles',roles,'active',active,'status',status,'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_employees order by last_name,first_name,id`
	case "shifts":
		query = `select jsonb_build_object('id',id,'tenant_id',organization_id,'register_id',register_id,'employee_id',employee_id,'state',state,'version',version,'opened_at',opened_at,'closed_at',closed_at,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_shifts order by opened_at,id`
	case "orders":
		query = `select jsonb_build_object('id',id,'tenant_id',organization_id,'external_id',external_id,'shift_id',shift_id,'register_id',register_id,'operator_code',operator_code,'state',state,'total',jsonb_build_object('amount',total::text,'currency',currency),'lines',lines,'fiscal_sale_id',coalesce(fiscal_sale_id,''),'fiscal_operation_id',coalesce(fiscal_operation_id,''),'fiscal_version',coalesce(fiscal_version,0),'version',version,'created_at',created_at,'updated_at',updated_at) from minipos_runtime_orders order by created_at,id`
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

func deleteTypedProjection(tx *sql.Tx, collection, key string) error {
	tables := map[string]string{"products": "minipos_runtime_products", "employees": "minipos_runtime_employees", "shifts": "minipos_runtime_shifts", "orders": "minipos_runtime_orders"}
	table := tables[collection]
	if table == "" {
		return nil
	}
	_, err := tx.Exec(`delete from `+table+` where id=$1`, key)
	return err
}
func upsertTypedProjection(tx *sql.Tx, row stateRow) error {
	p := string(row.Payload)
	switch row.Collection {
	case "products":
		_, e := tx.Exec(`insert into minipos_runtime_products(id,organization_id,sku,name,unit,amount,currency,tax_group,active,status,version,created_at,updated_at) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'sku',$2::jsonb->>'name',$2::jsonb->>'unit',($2::jsonb->'price'->>'amount')::numeric,$2::jsonb->'price'->>'currency',$2::jsonb->>'tax_group',($2::jsonb->>'active')::boolean,$2::jsonb->>'status',($2::jsonb->>'version')::bigint,($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz) on conflict(id) do update set organization_id=excluded.organization_id,sku=excluded.sku,name=excluded.name,unit=excluded.unit,amount=excluded.amount,currency=excluded.currency,tax_group=excluded.tax_group,active=excluded.active,status=excluded.status,version=excluded.version,created_at=excluded.created_at,updated_at=excluded.updated_at where minipos_runtime_products is distinct from excluded`, row.Key, p)
		return e
	case "employees":
		_, e := tx.Exec(`insert into minipos_runtime_employees(id,organization_id,first_name,last_name,operator_code,roles,active,status,version,created_at,updated_at) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'first_name',$2::jsonb->>'last_name',$2::jsonb->>'operator_code',$2::jsonb->'roles',($2::jsonb->>'active')::boolean,$2::jsonb->>'status',($2::jsonb->>'version')::bigint,($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz) on conflict(id) do update set organization_id=excluded.organization_id,first_name=excluded.first_name,last_name=excluded.last_name,operator_code=excluded.operator_code,roles=excluded.roles,active=excluded.active,status=excluded.status,version=excluded.version,created_at=excluded.created_at,updated_at=excluded.updated_at where minipos_runtime_employees is distinct from excluded`, row.Key, p)
		return e
	case "shifts":
		_, e := tx.Exec(`insert into minipos_runtime_shifts(id,organization_id,register_id,employee_id,state,version,opened_at,closed_at,created_at,updated_at) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'register_id',$2::jsonb->>'employee_id',$2::jsonb->>'state',($2::jsonb->>'version')::bigint,($2::jsonb->>'opened_at')::timestamptz,nullif($2::jsonb->>'closed_at','')::timestamptz,($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz) on conflict(id) do update set organization_id=excluded.organization_id,register_id=excluded.register_id,employee_id=excluded.employee_id,state=excluded.state,version=excluded.version,opened_at=excluded.opened_at,closed_at=excluded.closed_at,created_at=excluded.created_at,updated_at=excluded.updated_at where minipos_runtime_shifts is distinct from excluded`, row.Key, p)
		return e
	case "orders":
		_, e := tx.Exec(`insert into minipos_runtime_orders(id,organization_id,external_id,shift_id,register_id,operator_code,state,total,currency,lines,fiscal_sale_id,fiscal_operation_id,fiscal_version,version,created_at,updated_at) values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'external_id',$2::jsonb->>'shift_id',$2::jsonb->>'register_id',$2::jsonb->>'operator_code',$2::jsonb->>'state',($2::jsonb->'total'->>'amount')::numeric,$2::jsonb->'total'->>'currency',$2::jsonb->'lines',nullif($2::jsonb->>'fiscal_sale_id',''),nullif($2::jsonb->>'fiscal_operation_id',''),nullif($2::jsonb->>'fiscal_version','')::bigint,($2::jsonb->>'version')::bigint,($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz) on conflict(id) do update set organization_id=excluded.organization_id,external_id=excluded.external_id,shift_id=excluded.shift_id,register_id=excluded.register_id,operator_code=excluded.operator_code,state=excluded.state,total=excluded.total,currency=excluded.currency,lines=excluded.lines,fiscal_sale_id=excluded.fiscal_sale_id,fiscal_operation_id=excluded.fiscal_operation_id,fiscal_version=excluded.fiscal_version,version=excluded.version,created_at=excluded.created_at,updated_at=excluded.updated_at where minipos_runtime_orders is distinct from excluded`, row.Key, p)
		return e
	}
	return nil
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
		root[c] = v
	}
	return json.Marshal(root)
}
