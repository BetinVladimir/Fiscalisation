package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/jackc/pgx/v5/stdlib"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Postgres stores typed authoritative rows. reader uses the restricted tenant
// role so tests and runtime reads exercise FORCE RLS rather than owner bypass.
type Postgres struct{ db, reader *sql.DB }

// BeginOutboundEmail durably records the complete message before SMTP is attempted.
func (p *Postgres) BeginOutboundEmail(ctx context.Context, recipient, subject, messageText string) (string, error) {
	var id string
	err := p.db.QueryRowContext(ctx, `insert into outbound_email_messages(recipient,subject,message_text,status) values($1,$2,$3,'SENDING') returning id::text`, recipient, subject, messageText).Scan(&id)
	return id, err
}

func (p *Postgres) MarkOutboundEmailSent(ctx context.Context, id string) error {
	result, err := p.db.ExecContext(ctx, `update outbound_email_messages set status='SENT',sent_at=now(),updated_at=now(),last_error=null where id=$1 and status='SENDING'`, id)
	return ensureOutboundEmailUpdated(result, err)
}

func (p *Postgres) MarkOutboundEmailFailed(ctx context.Context, id, failure string) error {
	result, err := p.db.ExecContext(ctx, `update outbound_email_messages set status='FAILED',updated_at=now(),last_error=$2 where id=$1 and status='SENDING'`, id, failure)
	return ensureOutboundEmailUpdated(result, err)
}

func ensureOutboundEmailUpdated(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return errors.New("outbound email record was not updated")
	}
	return nil
}

var ErrConcurrentState = errors.New("fiscal repository generation conflict")
var allocationIdentityPattern = regexp.MustCompile(`^[A-Za-z0-9]{8}$`)
var allocationOperatorPattern = regexp.MustCompile(`^[A-Za-z0-9]{4}$`)

// AllocateUNP is the PostgreSQL authority path for online identifiers. The
// transaction-scoped advisory lock serializes one FMIN stream across all Core
// replicas; the unique constraints remain the final fail-closed protection.
func (p *Postgres) AllocateUNP(ctx context.Context, tenantID, allocationID, fiscalDeviceNumber, operatorCode string) (string, int64, error) {
	if tenantID == "" || allocationID == "" || !allocationIdentityPattern.MatchString(fiscalDeviceNumber) || !allocationOperatorPattern.MatchString(operatorCode) {
		return "", 0, errors.New("invalid UNP allocation identity")
	}
	tx, err := p.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, tenantID+":"+fiscalDeviceNumber); err != nil {
		return "", 0, err
	}
	var sequence int64
	if err = tx.QueryRowContext(ctx, `select coalesce(max(unp_sequence),0)+1 from unp_allocations where tenant_id=$1 and fiscal_device_number=$2`, tenantID, fiscalDeviceNumber).Scan(&sequence); err != nil {
		return "", 0, err
	}
	if sequence > 9999999 {
		return "", 0, errors.New("UNP sequence exhausted")
	}
	unp := fmt.Sprintf("%s-%s-%07d", fiscalDeviceNumber, operatorCode, sequence)
	if _, err = tx.ExecContext(ctx, `insert into unp_allocations(tenant_id,allocation_id,fiscal_device_number,operator_code,unp_sequence,unp,status) values($1,$2,$3,$4,$5,$6,'ALLOCATED')`, tenantID, allocationID, fiscalDeviceNumber, operatorCode, sequence, unp); err != nil {
		return "", 0, err
	}
	if err = tx.Commit(); err != nil {
		return "", 0, err
	}
	return unp, sequence, nil
}

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
	if _, e = db.ExecContext(ctx, `alter table fiscal_state_meta add column if not exists storage_mode smallint not null default 1`); e != nil {
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
	_, err := p.saveVersioned(b, nil)
	return err
}
func (p *Postgres) LoadVersioned() ([]byte, int64, error) {
	tx, err := p.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	defer tx.Rollback()
	var generation int64
	var storageMode int
	metaErr := tx.QueryRow(`select generation,storage_mode from fiscal_state_meta where singleton=true`).Scan(&generation, &storageMode)
	if metaErr != nil && !errors.Is(metaErr, sql.ErrNoRows) {
		return nil, 0, metaErr
	}
	if metaErr == nil && storageMode == 2 {
		b, loadErr := loadFiscalTypedSnapshot(tx)
		if loadErr != nil {
			return nil, 0, loadErr
		}
		if err = tx.Commit(); err != nil {
			return nil, 0, err
		}
		return b, generation, nil
	}
	rows, err := tx.Query(`select collection,entity_key,payload from fiscal_state_rows order by collection,entity_key`)
	if err != nil {
		return nil, 0, err
	}
	flat := make([]stateRow, 0)
	for rows.Next() {
		var row stateRow
		if err = rows.Scan(&row.Collection, &row.Key, &row.Payload); err != nil {
			rows.Close()
			return nil, 0, err
		}
		flat = append(flat, row)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, 0, err
	}
	if err = rows.Close(); err != nil {
		return nil, 0, err
	}
	if len(flat) > 0 || metaErr == nil {
		b, rebuildErr := rebuildSnapshot(flat)
		if rebuildErr != nil {
			return nil, 0, rebuildErr
		}
		if err = tx.Commit(); err != nil {
			return nil, 0, err
		}
		return b, generation, nil
	}
	_ = tx.Rollback()
	b, err := p.Load() // performs the one-time legacy migration, if present
	if err != nil || len(b) == 0 {
		return b, 0, err
	}
	return p.LoadVersioned()
}
func (p *Postgres) SaveVersioned(b []byte, expected int64) (int64, error) {
	return p.saveVersioned(b, &expected)
}
func (p *Postgres) SaveDeltaVersioned(previous, current []byte, expected int64) (int64, error) {
	oldRows, err := snapshotRowsOrEmpty(previous)
	if err != nil {
		return 0, err
	}
	newRows, err := snapshotRowsOrEmpty(current)
	if err != nil {
		return 0, err
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
	tx, err := p.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var actual int64
	storageMode := 1
	err = tx.QueryRow(`select generation,storage_mode from fiscal_state_meta where singleton=true for update`).Scan(&actual, &storageMode)
	if errors.Is(err, sql.ErrNoRows) {
		actual, err = 0, nil
	}
	if err != nil {
		return 0, err
	}
	if actual != expected {
		return actual, ErrConcurrentState
	}
	for _, row := range deletes {
		if storageMode == 1 {
			if _, err = tx.Exec(`delete from fiscal_state_rows where collection=$1 and entity_key=$2`, row.Collection, row.Key); err != nil {
				return 0, err
			}
		}
		if err = deleteTypedProjection(tx, row); err != nil {
			return 0, err
		}
	}
	for _, row := range upserts {
		if storageMode == 1 {
			if _, err = tx.Exec(`insert into fiscal_state_rows(collection,entity_key,payload,updated_at) values($1,$2,$3::jsonb,now()) on conflict(collection,entity_key) do update set payload=excluded.payload,updated_at=now() where fiscal_state_rows.payload is distinct from excluded.payload`, row.Collection, row.Key, string(row.Payload)); err != nil {
				return 0, err
			}
		}
		if err = upsertTypedProjection(tx, row); err != nil {
			return 0, fmt.Errorf("typed projection %s/%s: %w", row.Collection, row.Key, err)
		}
	}
	if storageMode == 1 {
		ready, coverageErr := fiscalTypedCoverageComplete(tx)
		if coverageErr != nil {
			return 0, coverageErr
		}
		if ready {
			storageMode = 2
		}
	}
	var generation int64
	if err = tx.QueryRow(`insert into fiscal_state_meta(singleton,generation,storage_mode,updated_at) values(true,1,$1,now()) on conflict(singleton) do update set generation=fiscal_state_meta.generation+1,storage_mode=excluded.storage_mode,updated_at=excluded.updated_at returning generation`, storageMode).Scan(&generation); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return generation, nil
}

// loadFiscalTypedSnapshot reconstructs the coordinator cache exclusively from
// constrained typed tables. Historical devices rows are intentionally excluded;
// active devices are resources(kind=device).
func loadFiscalTypedSnapshot(tx *sql.Tx) ([]byte, error) {
	rows, err := tx.Query(`
select collection,entity_key,payload from (
  select 'sales'::text collection,id entity_key,payload,'0'::text ordering from fiscal_runtime_sales
  union all select 'operations',id,payload,'0' from fiscal_runtime_operations
  union all select 'shifts',id,payload,'0' from fiscal_runtime_shifts
  union all select 'resources',kind||':'||id,payload,'0' from fiscal_runtime_resources where kind<>'composite_binding'
  union all select 'resources','composite_binding:'||binding_id,payload,'0' from fiscal_runtime_composite_bindings
  union all select 'outbox',id,payload,'0' from fiscal_runtime_outbox
  union all select 'ble_sessions',id,payload,'0' from fiscal_runtime_ble_sessions
  union all select 'connectivity_probes',id,payload,'0' from fiscal_runtime_connectivity_probes
  union all select 'edge_pending',operation_id,payload,'0' from fiscal_runtime_edge_pending
  union all select 'audit',event_id,payload,occurred_at::text from fiscal_runtime_audit
  union all select 'replays',replay_key,payload,'0' from fiscal_runtime_replays
  union all select 'unp',tenant_id||E'\n'||register_id,to_jsonb(last_sequence),'0' from fiscal_runtime_unp_sequences
  union all select 'artifacts',tenant_id||E'\n'||id,to_jsonb(encode(body,'base64')),'0' from fiscal_runtime_artifacts
  union all select 'sync_acks',tenant_id||E'\n'||edge_id,payload,'0' from fiscal_runtime_sync_acks
  union all select 'activation_challenges',activation_challenge_id::text,payload,'0' from fiscal_device_activation_challenges
  union all select 'activation_requests',activation_request_id::text,payload,'0' from fiscal_device_activation_requests
) typed where payload is not null order by collection,ordering,entity_key`)
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
	return rebuildSnapshot(flat)
}

// fiscalTypedCoverageComplete is the one-time cutover guard. Every supported
// compatibility entity must have an exact typed representation before mode 2.
func fiscalTypedCoverageComplete(tx *sql.Tx) (bool, error) {
	var missing int
	err := tx.QueryRow(`select count(*) from fiscal_state_rows s where s.collection <> 'devices' and not (
  (s.collection='sales' and exists(select 1 from fiscal_runtime_sales t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='operations' and exists(select 1 from fiscal_runtime_operations t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='shifts' and exists(select 1 from fiscal_runtime_shifts t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='resources' and exists(select 1 from fiscal_runtime_resources t where t.kind||':'||t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='outbox' and exists(select 1 from fiscal_runtime_outbox t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='ble_sessions' and exists(select 1 from fiscal_runtime_ble_sessions t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='connectivity_probes' and exists(select 1 from fiscal_runtime_connectivity_probes t where t.id=s.entity_key and t.payload=s.payload)) or
  (s.collection='edge_pending' and exists(select 1 from fiscal_runtime_edge_pending t where t.operation_id=s.entity_key and t.payload=s.payload)) or
  (s.collection='audit' and exists(select 1 from fiscal_runtime_audit t where t.payload=s.payload)) or
  (s.collection='replays' and exists(select 1 from fiscal_runtime_replays t where t.replay_key=s.entity_key and t.payload=s.payload)) or
  (s.collection='unp' and exists(select 1 from fiscal_runtime_unp_sequences t where t.tenant_id||E'\n'||t.register_id=s.entity_key and to_jsonb(t.last_sequence)=s.payload)) or
  (s.collection='artifacts' and exists(select 1 from fiscal_runtime_artifacts t where t.tenant_id||E'\n'||t.id=s.entity_key and to_jsonb(encode(t.body,'base64'))=s.payload)) or
  (s.collection='sync_acks' and exists(select 1 from fiscal_runtime_sync_acks t where t.tenant_id||E'\n'||t.edge_id=s.entity_key and t.payload=s.payload))
  or (s.collection='activation_challenges' and exists(select 1 from fiscal_device_activation_challenges t where t.activation_challenge_id::text=s.entity_key and t.payload=s.payload))
  or (s.collection='activation_requests' and exists(select 1 from fiscal_device_activation_requests t where t.activation_request_id::text=s.entity_key and t.payload=s.payload))
)`).Scan(&missing)
	return missing == 0, err
}

func snapshotRowsOrEmpty(raw []byte) ([]stateRow, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	return flattenSnapshot(raw)
}
func (p *Postgres) saveVersioned(b []byte, expected *int64) (int64, error) {
	rows, err := flattenSnapshot(b)
	if err != nil {
		return 0, err
	}
	tx, err := p.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var current int64
	err = tx.QueryRow(`select generation from fiscal_state_meta where singleton=true for update`).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		current, err = 0, nil
	}
	if err != nil {
		return 0, err
	}
	if expected != nil && current != *expected {
		return current, ErrConcurrentState
	}
	desired := make(map[string]bool, len(rows))
	for _, row := range rows {
		desired[stateRowID(row.Collection, row.Key)] = true
	}
	existing, err := tx.Query(`select collection,entity_key,payload from fiscal_state_rows for update`)
	if err != nil {
		return 0, err
	}
	stale := make([]stateRow, 0)
	for existing.Next() {
		var row stateRow
		if err = existing.Scan(&row.Collection, &row.Key, &row.Payload); err != nil {
			existing.Close()
			return 0, err
		}
		if !desired[stateRowID(row.Collection, row.Key)] {
			stale = append(stale, row)
		}
	}
	if err = existing.Err(); err != nil {
		existing.Close()
		return 0, err
	}
	if err = existing.Close(); err != nil {
		return 0, err
	}
	for _, row := range stale {
		if _, err = tx.Exec(`delete from fiscal_state_rows where collection=$1 and entity_key=$2`, row.Collection, row.Key); err != nil {
			return 0, err
		}
		if err = deleteTypedProjection(tx, row); err != nil {
			return 0, err
		}
	}
	stmt, err := tx.Prepare(`insert into fiscal_state_rows(collection,entity_key,payload,updated_at) values($1,$2,$3::jsonb,now()) on conflict(collection,entity_key) do update set payload=excluded.payload,updated_at=now() where fiscal_state_rows.payload is distinct from excluded.payload`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, err = stmt.Exec(row.Collection, row.Key, string(row.Payload)); err != nil {
			return 0, err
		}
		if err = upsertTypedProjection(tx, row); err != nil {
			return 0, fmt.Errorf("typed projection %s/%s: %w", row.Collection, row.Key, err)
		}
	}
	var generation int64
	if err = tx.QueryRow(`insert into fiscal_state_meta(singleton,generation,updated_at) values(true,1,now()) on conflict(singleton) do update set generation=fiscal_state_meta.generation+1,updated_at=excluded.updated_at returning generation`).Scan(&generation); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return generation, nil
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
		err = tx.QueryRow(`select jsonb_build_object('sale_id',id,'tenant_id',tenant_id,'external_id',external_id,'location_id',coalesce(location_id,''),'register_id',register_id,'operator_id',operator_id,'unp',coalesce(unp,''),'state',state,'version',version,'lines',lines,'payments',payments,'fiscal_operation_id',coalesce(fiscal_operation_id,''),'receipt_artifact_id',coalesce(receipt_artifact_id,''),'fiscal_device',jsonb_strip_nulls(jsonb_build_object('device_id',fiscal_device_id,'binding_version',coalesce((payload->'fiscal_device'->>'binding_version')::bigint,0),'serial',fiscal_device_serial,'fiscal_device_number',fiscal_device_number,'fiscal_memory_number',fiscal_memory_number,'vendor',fiscal_device_vendor,'model',fiscal_device_model,'firmware',fiscal_device_firmware)),'created_at',created_at,'updated_at',updated_at) from fiscal_runtime_sales where id=$1`, id).Scan(&raw)
	case "operations":
		err = tx.QueryRow(`select payload from fiscal_runtime_operations where id=$1`, id).Scan(&raw)
	case "shifts":
		err = tx.QueryRow(`select jsonb_build_object('id',id,'tenant_id',tenant_id,'register_id',register_id,'operator_id',operator_id,'state',state,'version',version,'opened_at',opened_at,'closed_at',closed_at) from fiscal_runtime_shifts where id=$1`, id).Scan(&raw)
	case "ble_sessions":
		err = tx.QueryRow(`select payload from fiscal_runtime_ble_sessions where id=$1`, id).Scan(&raw)
	case "connectivity_probes":
		err = tx.QueryRow(`select payload from fiscal_runtime_connectivity_probes where id=$1`, id).Scan(&raw)
	case "edge_pending":
		err = tx.QueryRow(`select payload from fiscal_runtime_edge_pending where operation_id=$1`, id).Scan(&raw)
	case "sync_acks":
		err = tx.QueryRow(`select payload from fiscal_runtime_sync_acks where edge_id=$1`, id).Scan(&raw)
	case "artifacts":
		err = tx.QueryRow(`select to_json(encode(body,'base64')) from fiscal_runtime_artifacts where id=$1`, id).Scan(&raw)
	case "replays":
		err = tx.QueryRow(`select payload from fiscal_runtime_replays where replay_key=$1`, id).Scan(&raw)
	default:
		if len(collection) > len("resources:") && collection[:len("resources:")] == "resources:" {
			err = tx.QueryRow(`select jsonb_build_object('kind',kind,'tenant_id',tenant_id,'id',id,'version',version,'data',data,'created_at',created_at,'updated_at',updated_at) from fiscal_runtime_resources where kind=$1 and id=$2`, collection[len("resources:"):], id).Scan(&raw)
		} else {
			return nil, errors.New("unsupported typed collection")
		}
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return raw, nil
}

// LoadSystemEntities is reserved for internal workers that must drain records
// across tenants. It uses the writer identity intentionally and never serves a
// public request; public reads always use LoadTenantEntity/LoadTenantEntities.
func (p *Postgres) LoadSystemEntities(collection string) ([][]byte, error) {
	query := ""
	switch collection {
	case "outbox":
		query = `select payload from fiscal_runtime_outbox order by next_attempt,id`
	default:
		return nil, errors.New("unsupported system collection")
	}
	rows, err := p.db.Query(query)
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
	return out, rows.Err()
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
	var queryArgs []any
	switch collection {
	case "sales":
		query = `select jsonb_build_object('sale_id',id,'tenant_id',tenant_id,'external_id',external_id,'location_id',coalesce(location_id,''),'register_id',register_id,'operator_id',operator_id,'unp',coalesce(unp,''),'state',state,'version',version,'lines',lines,'payments',payments,'fiscal_operation_id',coalesce(fiscal_operation_id,''),'receipt_artifact_id',coalesce(receipt_artifact_id,''),'fiscal_device',jsonb_strip_nulls(jsonb_build_object('device_id',fiscal_device_id,'binding_version',coalesce((payload->'fiscal_device'->>'binding_version')::bigint,0),'serial',fiscal_device_serial,'fiscal_device_number',fiscal_device_number,'fiscal_memory_number',fiscal_memory_number,'vendor',fiscal_device_vendor,'model',fiscal_device_model,'firmware',fiscal_device_firmware)),'created_at',created_at,'updated_at',updated_at) from fiscal_runtime_sales order by created_at,id`
	case "operations":
		query = `select payload from fiscal_runtime_operations order by created_at,id`
	case "shifts":
		query = `select jsonb_build_object('id',id,'tenant_id',tenant_id,'register_id',register_id,'operator_id',operator_id,'state',state,'version',version,'opened_at',opened_at,'closed_at',closed_at) from fiscal_runtime_shifts order by opened_at,id`
	case "ble_sessions":
		query = `select payload from fiscal_runtime_ble_sessions order by expires_at,id`
	case "connectivity_probes":
		query = `select payload from fiscal_runtime_connectivity_probes order by observed_at,id`
	case "edge_pending":
		query = `select payload from fiscal_runtime_edge_pending order by accepted_at,operation_id`
	case "sync_acks":
		query = `select payload from fiscal_runtime_sync_acks order by committed_at,edge_id`
	case "audit":
		query = `select payload from fiscal_runtime_audit order by occurred_at,event_id`
	default:
		if len(collection) > len("resources:") && collection[:len("resources:")] == "resources:" {
			query = `select jsonb_build_object('kind',kind,'tenant_id',tenant_id,'id',id,'version',version,'data',data,'created_at',created_at,'updated_at',updated_at) from fiscal_runtime_resources where kind=$1 order by updated_at,id`
			queryArgs = append(queryArgs, collection[len("resources:"):])
		} else {
			return nil, errors.New("unsupported typed collection")
		}
	}
	rows, err := tx.Query(query, queryArgs...)
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

func deleteTypedProjection(tx *sql.Tx, row stateRow) error {
	if row.Collection == "activation_challenges" {
		_, err := tx.Exec(`delete from fiscal_device_activation_challenges where activation_challenge_id=$1`, row.Key)
		return err
	}
	if row.Collection == "activation_requests" {
		_, err := tx.Exec(`delete from fiscal_device_activation_requests where activation_request_id=$1`, row.Key)
		return err
	}
	if row.Collection == "sync_acks" {
		parts := strings.SplitN(row.Key, "\n", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil
		}
		return withTenantRole(tx, row, func() error {
			_, err := tx.Exec(`delete from fiscal_runtime_sync_acks where tenant_id=$1 and edge_id=$2`, parts[0], parts[1])
			return err
		})
	}
	if row.Collection == "artifacts" {
		parts := strings.SplitN(row.Key, "\n", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil // pre-tenant legacy artifact; migrated by an authenticated domain read
		}
		return withTenantRole(tx, row, func() error {
			_, err := tx.Exec(`delete from fiscal_runtime_artifacts where tenant_id=$1 and id=$2`, parts[0], parts[1])
			return err
		})
	}
	if row.Collection == "unp" {
		parts := strings.SplitN(row.Key, "\n", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil // pre-tenant legacy sequence; migrated by the domain on first allocation
		}
		return withTenantRole(tx, row, func() error {
			_, err := tx.Exec(`delete from fiscal_runtime_unp_sequences where tenant_id=$1 and register_id=$2`, parts[0], parts[1])
			return err
		})
	}
	if row.Collection == "resources" {
		return withTenantRole(tx, row, func() error {
			var identity struct{ Kind, ID string }
			if err := json.Unmarshal(row.Payload, &identity); err != nil || identity.Kind == "" || identity.ID == "" {
				return errors.New("typed resource identity required")
			}
			if _, err := tx.Exec(`delete from fiscal_runtime_resources where kind=$1 and id=$2`, identity.Kind, identity.ID); err != nil {
				return err
			}
			if identity.Kind == "composite_binding" {
				_, err := tx.Exec(`delete from fiscal_runtime_composite_bindings where binding_id=$1`, identity.ID)
				return err
			}
			return nil
		})
	}
	if table, ok := map[string]string{
		"outbox":              "fiscal_runtime_outbox",
		"ble_sessions":        "fiscal_runtime_ble_sessions",
		"connectivity_probes": "fiscal_runtime_connectivity_probes",
		"edge_pending":        "fiscal_runtime_edge_pending",
		"audit":               "fiscal_runtime_audit",
		"replays":             "fiscal_runtime_replays",
	}[row.Collection]; ok {
		id := row.Key
		if row.Collection == "audit" {
			var identity struct {
				EventID string `json:"event_id"`
			}
			if err := json.Unmarshal(row.Payload, &identity); err != nil || identity.EventID == "" {
				return errors.New("typed audit event_id required")
			}
			id = identity.EventID
		}
		idColumn := "id"
		if row.Collection == "audit" {
			idColumn = "event_id"
		} else if row.Collection == "edge_pending" {
			idColumn = "operation_id"
		} else if row.Collection == "replays" {
			idColumn = "replay_key"
		}
		return withTenantRole(tx, row, func() error {
			_, err := tx.Exec(`delete from `+table+` where `+idColumn+`=$1`, id)
			return err
		})
	}
	table := ""
	switch row.Collection {
	case "sales":
		table = "fiscal_runtime_sales"
	case "operations":
		table = "fiscal_runtime_operations"
	case "shifts":
		table = "fiscal_runtime_shifts"
	default:
		return nil
	}
	return withTenantRole(tx, row, func() error {
		_, err := tx.Exec(`delete from `+table+` where id=$1`, row.Key)
		return err
	})
}
func upsertTypedProjection(tx *sql.Tx, row stateRow) error {
	if row.Collection == "activation_challenges" {
		_, err := tx.Exec(`insert into fiscal_device_activation_challenges(activation_challenge_id,device_instance_id,nonce_hash,expires_at,consumed_at,created_at,payload) values($1,($2::jsonb->>'device_instance_id')::uuid,$2::jsonb->>'nonce_hash',($2::jsonb->>'expires_at')::timestamptz,nullif($2::jsonb->>'consumed_at','')::timestamptz,($2::jsonb->>'created_at')::timestamptz,$2::jsonb) on conflict(activation_challenge_id) do update set consumed_at=excluded.consumed_at,payload=excluded.payload where fiscal_device_activation_challenges is distinct from excluded`, row.Key, string(row.Payload))
		return err
	}
	if row.Collection == "activation_requests" {
		_, err := tx.Exec(`insert into fiscal_device_activation_requests(activation_request_id,device_instance_id,request_secret_hash,user_code_hash,device_key_thumbprint,state,claimed_tenant_id,claimed_location_id,claimed_register_id,binding_version,expires_at,created_at,updated_at,payload) values($1,($2::jsonb->>'device_instance_id')::uuid,$2::jsonb->>'request_secret_hash',$2::jsonb->>'user_code_hash',$2::jsonb->>'device_key_thumbprint',$2::jsonb->>'state',nullif($2::jsonb->>'claimed_tenant_id',''),nullif($2::jsonb->>'claimed_location_id',''),nullif($2::jsonb->>'claimed_register_id',''),nullif($2::jsonb->>'binding_version','')::bigint,($2::jsonb->>'expires_at')::timestamptz,($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz,$2::jsonb) on conflict(activation_request_id) do update set state=excluded.state,claimed_tenant_id=excluded.claimed_tenant_id,claimed_location_id=excluded.claimed_location_id,claimed_register_id=excluded.claimed_register_id,binding_version=excluded.binding_version,updated_at=excluded.updated_at,payload=excluded.payload where fiscal_device_activation_requests is distinct from excluded`, row.Key, string(row.Payload))
		return err
	}
	if row.Collection == "unp" || row.Collection == "artifacts" || row.Collection == "sync_acks" {
		parts := strings.SplitN(row.Key, "\n", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil // compatibility row until an authenticated domain operation supplies tenant ownership
		}
	}
	return withTenantRole(tx, row, func() error { return upsertTypedProjectionAsTenant(tx, row) })
}

func upsertTypedProjectionAsTenant(tx *sql.Tx, row stateRow) error {
	payload := string(row.Payload)
	switch row.Collection {
	case "sales":
		_, err := tx.Exec(`insert into fiscal_runtime_sales(id,tenant_id,external_id,location_id,register_id,operator_id,unp,state,version,lines,payments,fiscal_operation_id,receipt_artifact_id,fiscal_device_id,fiscal_device_serial,fiscal_device_number,fiscal_memory_number,fiscal_device_vendor,fiscal_device_model,fiscal_device_firmware,created_at,updated_at,payload)
values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'external_id',nullif($2::jsonb->>'location_id',''),$2::jsonb->>'register_id',$2::jsonb->>'operator_id',nullif($2::jsonb->>'unp',''),$2::jsonb->>'state',($2::jsonb->>'version')::bigint,$2::jsonb->'lines',$2::jsonb->'payments',nullif($2::jsonb->>'fiscal_operation_id',''),nullif($2::jsonb->>'receipt_artifact_id',''),nullif($2::jsonb->'fiscal_device'->>'device_id',''),nullif($2::jsonb->'fiscal_device'->>'serial',''),nullif($2::jsonb->'fiscal_device'->>'fiscal_device_number',''),nullif($2::jsonb->'fiscal_device'->>'fiscal_memory_number',''),nullif($2::jsonb->'fiscal_device'->>'vendor',''),nullif($2::jsonb->'fiscal_device'->>'model',''),nullif($2::jsonb->'fiscal_device'->>'firmware',''),($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz,$2::jsonb)
on conflict(id) do update set tenant_id=excluded.tenant_id,external_id=excluded.external_id,location_id=excluded.location_id,register_id=excluded.register_id,operator_id=excluded.operator_id,unp=excluded.unp,state=excluded.state,version=excluded.version,lines=excluded.lines,payments=excluded.payments,fiscal_operation_id=excluded.fiscal_operation_id,receipt_artifact_id=excluded.receipt_artifact_id,fiscal_device_id=excluded.fiscal_device_id,fiscal_device_serial=excluded.fiscal_device_serial,fiscal_device_number=excluded.fiscal_device_number,fiscal_memory_number=excluded.fiscal_memory_number,fiscal_device_vendor=excluded.fiscal_device_vendor,fiscal_device_model=excluded.fiscal_device_model,fiscal_device_firmware=excluded.fiscal_device_firmware,created_at=excluded.created_at,updated_at=excluded.updated_at,payload=excluded.payload
where fiscal_runtime_sales is distinct from excluded`, row.Key, payload)
		return err
	case "operations":
		_, err := tx.Exec(`insert into fiscal_runtime_operations(id,tenant_id,sale_id,register_id,type,state,version,fiscal_reference,original_fiscal_reference,reason_code,simulated,error_code,created_at,updated_at,payload)
values($1,$2::jsonb->>'tenant_id',nullif($2::jsonb->>'sale_id',''),nullif($2::jsonb->>'register_id',''),$2::jsonb->>'type',$2::jsonb->>'state',($2::jsonb->>'version')::bigint,nullif($2::jsonb->>'fiscal_reference',''),nullif($2::jsonb->>'original_fiscal_reference',''),nullif($2::jsonb->>'reason_code',''),coalesce(($2::jsonb->>'simulated')::boolean,false),nullif($2::jsonb->>'error_code',''),($2::jsonb->>'created_at')::timestamptz,($2::jsonb->>'updated_at')::timestamptz,$2::jsonb)
on conflict(id) do update set tenant_id=excluded.tenant_id,sale_id=excluded.sale_id,register_id=excluded.register_id,type=excluded.type,state=excluded.state,version=excluded.version,fiscal_reference=excluded.fiscal_reference,original_fiscal_reference=excluded.original_fiscal_reference,reason_code=excluded.reason_code,simulated=excluded.simulated,error_code=excluded.error_code,created_at=excluded.created_at,updated_at=excluded.updated_at,payload=excluded.payload
where fiscal_runtime_operations is distinct from excluded`, row.Key, payload)
		return err
	case "shifts":
		_, err := tx.Exec(`insert into fiscal_runtime_shifts(id,tenant_id,register_id,operator_id,state,version,opened_at,closed_at,payload)
values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'register_id',$2::jsonb->>'operator_id',$2::jsonb->>'state',($2::jsonb->>'version')::bigint,($2::jsonb->>'opened_at')::timestamptz,nullif($2::jsonb->>'closed_at','')::timestamptz,$2::jsonb)
on conflict(id) do update set tenant_id=excluded.tenant_id,register_id=excluded.register_id,operator_id=excluded.operator_id,state=excluded.state,version=excluded.version,opened_at=excluded.opened_at,closed_at=excluded.closed_at,payload=excluded.payload
where fiscal_runtime_shifts is distinct from excluded`, row.Key, payload)
		return err
	case "resources":
		_, err := tx.Exec(`insert into fiscal_runtime_resources(kind,id,tenant_id,version,data,created_at,updated_at,payload)
values($1::jsonb->>'kind',$1::jsonb->>'id',$1::jsonb->>'tenant_id',($1::jsonb->>'version')::bigint,$1::jsonb->'data',($1::jsonb->>'created_at')::timestamptz,($1::jsonb->>'updated_at')::timestamptz,$1::jsonb)
on conflict(kind,id) do update set tenant_id=excluded.tenant_id,version=excluded.version,data=excluded.data,created_at=excluded.created_at,updated_at=excluded.updated_at,payload=excluded.payload
where fiscal_runtime_resources is distinct from excluded`, payload)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`insert into fiscal_runtime_composite_bindings(binding_id,tenant_id,register_id,profile,adapter_device_id,fiscal_device_id,payment_device_id,generation,applied_generation,status,concurrency_version,payload,updated_at)
select $1::jsonb->>'id',$1::jsonb->>'tenant_id',$1::jsonb->'data'->>'register_id',$1::jsonb->'data'->>'profile',$1::jsonb->'data'->>'adapter_device_id',$1::jsonb->'data'->>'fiscal_device_id',nullif($1::jsonb->'data'->>'payment_device_id',''),($1::jsonb->'data'->>'generation')::bigint,nullif($1::jsonb->'data'->>'applied_generation','')::bigint,$1::jsonb->'data'->>'status',($1::jsonb->>'version')::bigint,$1::jsonb,($1::jsonb->>'updated_at')::timestamptz where $1::jsonb->>'kind'='composite_binding'
on conflict(binding_id)do update set tenant_id=excluded.tenant_id,register_id=excluded.register_id,profile=excluded.profile,adapter_device_id=excluded.adapter_device_id,fiscal_device_id=excluded.fiscal_device_id,payment_device_id=excluded.payment_device_id,generation=excluded.generation,applied_generation=excluded.applied_generation,status=excluded.status,concurrency_version=excluded.concurrency_version,payload=excluded.payload,updated_at=excluded.updated_at where fiscal_runtime_composite_bindings is distinct from excluded`, payload)
		return err
	case "outbox":
		_, err := tx.Exec(`insert into fiscal_runtime_outbox(id,tenant_id,event_type,resource_id,attempts,next_attempt,delivered_at,payload)
values($1,$2::jsonb->'event'->>'tenant_id',$2::jsonb->'event'->>'event_type',$2::jsonb->'event'->>'resource_id',($2::jsonb->>'attempts')::integer,($2::jsonb->>'next_attempt')::timestamptz,nullif($2::jsonb->>'delivered_at','')::timestamptz,$2::jsonb)
on conflict(id) do update set tenant_id=excluded.tenant_id,event_type=excluded.event_type,resource_id=excluded.resource_id,attempts=excluded.attempts,next_attempt=excluded.next_attempt,delivered_at=excluded.delivered_at,payload=excluded.payload where fiscal_runtime_outbox is distinct from excluded`, row.Key, payload)
		return err
	case "ble_sessions":
		_, err := tx.Exec(`insert into fiscal_runtime_ble_sessions(id,tenant_id,location_id,register_id,operator_id,app_instance_id,actor_subject,client_public_key,device_id,fiscal_device_id,fencing_token,expires_at,revoked,payload)
values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'location_id',$2::jsonb->>'register_id',$2::jsonb->>'operator_id',$2::jsonb->>'app_instance_id',$2::jsonb->>'actor_subject',$2::jsonb->>'client_public_key',$2::jsonb->>'device_id',$2::jsonb->>'fiscal_device_id',($2::jsonb->>'fencing_token')::bigint,($2::jsonb->>'expires_at')::timestamptz,coalesce(($2::jsonb->>'revoked')::boolean,false),$2::jsonb)
on conflict(id) do update set tenant_id=excluded.tenant_id,location_id=excluded.location_id,register_id=excluded.register_id,operator_id=excluded.operator_id,app_instance_id=excluded.app_instance_id,actor_subject=excluded.actor_subject,client_public_key=excluded.client_public_key,device_id=excluded.device_id,fiscal_device_id=excluded.fiscal_device_id,fencing_token=excluded.fencing_token,expires_at=excluded.expires_at,revoked=excluded.revoked,payload=excluded.payload where fiscal_runtime_ble_sessions is distinct from excluded`, row.Key, payload)
		return err
	case "connectivity_probes":
		_, err := tx.Exec(`insert into fiscal_runtime_connectivity_probes(id,tenant_id,register_id,state,observed_at,recommended_transport,payload)
values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'register_id',$2::jsonb->>'state',($2::jsonb->>'observed_at')::timestamptz,$2::jsonb->>'recommended_transport',$2::jsonb)
on conflict(id) do update set tenant_id=excluded.tenant_id,register_id=excluded.register_id,state=excluded.state,observed_at=excluded.observed_at,recommended_transport=excluded.recommended_transport,payload=excluded.payload where fiscal_runtime_connectivity_probes is distinct from excluded`, row.Key, payload)
		return err
	case "edge_pending":
		_, err := tx.Exec(`insert into fiscal_runtime_edge_pending(operation_id,tenant_id,register_id,device_id,command_type,operation_sequence,unp_sequence,accepted_at,payload)
values($1,$2::jsonb->>'tenant_id',$2::jsonb->>'register_id',$2::jsonb->>'device_id',$2::jsonb->>'command_type',($2::jsonb->>'operation_sequence')::bigint,($2::jsonb->>'unp_sequence')::bigint,($2::jsonb->>'accepted_at')::timestamptz,$2::jsonb)
on conflict(operation_id) do update set tenant_id=excluded.tenant_id,register_id=excluded.register_id,device_id=excluded.device_id,command_type=excluded.command_type,operation_sequence=excluded.operation_sequence,unp_sequence=excluded.unp_sequence,accepted_at=excluded.accepted_at,payload=excluded.payload where fiscal_runtime_edge_pending is distinct from excluded`, row.Key, payload)
		return err
	case "audit":
		_, err := tx.Exec(`insert into fiscal_runtime_audit(event_id,tenant_id,actor_id,action,object_type,object_id,occurred_at,event_hash,payload)
values($1::jsonb->>'event_id',$1::jsonb->>'tenant_id',$1::jsonb->>'actor_id',$1::jsonb->>'action',$1::jsonb->>'object_type',$1::jsonb->>'object_id',($1::jsonb->>'occurred_at')::timestamptz,$1::jsonb->>'event_hash',$1::jsonb)
on conflict(event_id) do update set tenant_id=excluded.tenant_id,actor_id=excluded.actor_id,action=excluded.action,object_type=excluded.object_type,object_id=excluded.object_id,occurred_at=excluded.occurred_at,event_hash=excluded.event_hash,payload=excluded.payload where fiscal_runtime_audit is distinct from excluded`, payload)
		return err
	case "replays":
		_, err := tx.Exec(`insert into fiscal_runtime_replays(replay_key,tenant_id,method,path,request_hash,status,pending,payload)
values($1,split_part($1,' ',1),split_part($1,' ',2),split_part($1,' ',3),$2::jsonb->>'hash',($2::jsonb->>'status')::integer,coalesce(($2::jsonb->>'pending')::boolean,false),$2::jsonb)
on conflict(replay_key) do update set tenant_id=excluded.tenant_id,method=excluded.method,path=excluded.path,request_hash=excluded.request_hash,status=excluded.status,pending=excluded.pending,payload=excluded.payload where fiscal_runtime_replays is distinct from excluded`, row.Key, payload)
		return err
	case "unp":
		_, err := tx.Exec(`insert into fiscal_runtime_unp_sequences(tenant_id,register_id,last_sequence)
values(split_part($1,E'\n',1),split_part($1,E'\n',2),$2::bigint)
on conflict(tenant_id,register_id) do update set last_sequence=excluded.last_sequence where fiscal_runtime_unp_sequences.last_sequence is distinct from excluded.last_sequence`, row.Key, payload)
		return err
	case "artifacts":
		_, err := tx.Exec(`insert into fiscal_runtime_artifacts(tenant_id,id,body,sha256)
values(split_part($1,E'\n',1),split_part($1,E'\n',2),decode($2::jsonb#>>'{}','base64'),encode(digest(decode($2::jsonb#>>'{}','base64'),'sha256'),'hex'))
on conflict(tenant_id,id) do update set body=excluded.body,sha256=excluded.sha256
where fiscal_runtime_artifacts is distinct from excluded`, row.Key, payload)
		return err
	case "sync_acks":
		_, err := tx.Exec(`insert into fiscal_runtime_sync_acks(tenant_id,edge_id,ack_id,committed_through_seq,committed_event_hash,committed_at,payload)
values(split_part($1,E'\n',1),split_part($1,E'\n',2),$2::jsonb->>'ack_id',($2::jsonb->>'committed_through_seq')::bigint,$2::jsonb->>'committed_event_hash',($2::jsonb->>'committed_at')::timestamptz,$2::jsonb)
on conflict(tenant_id,edge_id) do update set ack_id=excluded.ack_id,committed_through_seq=excluded.committed_through_seq,committed_event_hash=excluded.committed_event_hash,committed_at=excluded.committed_at,payload=excluded.payload
where fiscal_runtime_sync_acks is distinct from excluded`, row.Key, payload)
		return err
	}
	return nil
}

func withTenantRole(tx *sql.Tx, row stateRow, mutate func() error) error {
	if !map[string]bool{"sales": true, "operations": true, "shifts": true, "resources": true, "outbox": true, "ble_sessions": true, "connectivity_probes": true, "edge_pending": true, "audit": true, "replays": true, "unp": true, "artifacts": true, "sync_acks": true}[row.Collection] {
		return nil
	}
	var identity struct {
		TenantID string `json:"tenant_id"`
	}
	if row.Collection == "unp" || row.Collection == "artifacts" || row.Collection == "sync_acks" {
		parts := strings.SplitN(row.Key, "\n", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return errors.New("typed key must contain tenant and entity id")
		}
		identity.TenantID = parts[0]
	} else if row.Collection == "replays" {
		parts := strings.SplitN(row.Key, " ", 4)
		if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
			return errors.New("typed replay key must contain tenant, method, path and idempotency key")
		}
		identity.TenantID = parts[0]
	} else if row.Collection == "outbox" {
		var nested struct {
			Event struct {
				TenantID string `json:"tenant_id"`
			} `json:"event"`
		}
		if err := json.Unmarshal(row.Payload, &nested); err == nil {
			identity.TenantID = nested.Event.TenantID
		}
	} else if err := json.Unmarshal(row.Payload, &identity); err != nil {
		return err
	}
	if identity.TenantID == "" {
		return errors.New("typed projection tenant_id required")
	}
	if _, err := tx.Exec(`set local role beefiscal_tenant`); err != nil {
		return err
	}
	if _, err := tx.Exec(`select set_config('app.tenant_id',$1,true)`, identity.TenantID); err != nil {
		return err
	}
	if err := mutate(); err != nil {
		return err
	}
	_, err := tx.Exec(`set local role none`)
	return err
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
	"activation_challenges": true, "activation_requests": true,
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
