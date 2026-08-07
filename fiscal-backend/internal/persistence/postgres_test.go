package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestStateRowsRoundTripEveryRepositoryCollection(t *testing.T) {
	original := map[string]any{
		"sales":      map[string]any{"sale-1": map[string]any{"id": "sale-1", "tenant_id": "tenant-1"}},
		"operations": map[string]any{"op-1": map[string]any{"id": "op-1", "state": "UNKNOWN"}},
		"devices":    map[string]any{}, "shifts": map[string]any{}, "unp": map[string]any{"r:A001": 2},
		"replays": map[string]any{}, "outbox": map[string]any{"event": map[string]any{"id": "event"}},
		"ble_sessions": map[string]any{}, "sync_acks": map[string]any{}, "connectivity_probes": map[string]any{},
		"resources": map[string]any{}, "artifacts": map[string]any{}, "edge_pending": map[string]any{},
		"audit": []any{map[string]any{"event_id": "a1"}, map[string]any{"event_id": "a2"}},
	}
	raw, _ := json.Marshal(original)
	rows, err := flattenSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	rebuilt, err := rebuildSnapshot(rows)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if json.Unmarshal(rebuilt, &got) != nil {
		t.Fatal("invalid rebuilt JSON")
	}
	var want map[string]any
	_ = json.Unmarshal(raw, &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("row persistence changed state\nwant=%#v\ngot=%#v", want, got)
	}
}

func TestStateRowsRejectUnknownCollection(t *testing.T) {
	if _, err := flattenSnapshot([]byte(`{"unknown":{"x":1}}`)); err == nil {
		t.Fatal("unknown collection accepted")
	}
	if _, err := rebuildSnapshot([]stateRow{{Collection: "unknown", Key: "x", Payload: []byte(`1`)}}); err == nil {
		t.Fatal("unknown persisted collection accepted")
	}
}

func TestPostgresRowStoreRestart(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.db.Exec(`delete from fiscal_state_rows; delete from fiscal_state_meta; delete from runtime_snapshots where aggregate='fiscal'`); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"sales":{},"operations":{},"devices":{"device-1":{"id":"device-1"}},"shifts":{},"unp":{},"replays":{},"outbox":{"event-1":{"id":"event-1"}},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{}}`)
	if err = p.Save(raw); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = p.db.QueryRow(`select count(*) from fiscal_state_rows`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("expected two entity rows, got %d: %v", count, err)
	}
	_ = p.Close()
	p, err = Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	loaded, err := p.Load()
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	_ = json.Unmarshal(loaded, &got)
	_ = json.Unmarshal(raw, &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("restart changed state\nwant=%#v\ngot=%#v", want, got)
	}
}

func TestPostgresMigratesLegacySnapshotAtomically(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	raw := []byte(`{"sales":{},"operations":{},"devices":{"legacy-device":{"id":"legacy-device"}},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{}}`)
	if _, err = p.db.Exec(`delete from fiscal_state_rows; delete from fiscal_state_meta`); err != nil {
		t.Fatal(err)
	}
	if _, err = p.db.Exec(`insert into runtime_snapshots(aggregate,payload,version,updated_at) values('fiscal',$1::jsonb,1,now()) on conflict(aggregate) do update set payload=excluded.payload`, string(raw)); err != nil {
		t.Fatal(err)
	}
	loaded, err := p.Load()
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err = p.db.QueryRow(`select count(*) from fiscal_state_rows where collection='devices' and entity_key='legacy-device'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("legacy state not migrated: %d %v", count, err)
	}
	var got, want map[string]any
	_ = json.Unmarshal(loaded, &got)
	_ = json.Unmarshal(raw, &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("legacy load changed state")
	}
}

func TestPostgresEmptyStateDoesNotResurrectLegacySnapshot(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	legacy := `{"sales":{"must-not-return":{"id":"must-not-return"}},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{}}`
	if _, err = p.db.Exec(`delete from fiscal_state_rows; delete from fiscal_state_meta`); err != nil {
		t.Fatal(err)
	}
	if _, err = p.db.Exec(`insert into runtime_snapshots(aggregate,payload,version,updated_at) values('fiscal',$1::jsonb,1,now()) on conflict(aggregate) do update set payload=excluded.payload`, legacy); err != nil {
		t.Fatal(err)
	}
	empty := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{}}`)
	if err = p.Save(empty); err != nil {
		t.Fatal(err)
	}
	loaded, err := p.Load()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(loaded, &got)
	if len(got["sales"].(map[string]any)) != 0 {
		t.Fatal("legacy state resurrected after authoritative empty save")
	}
}

func TestPostgresDifferentialSavePreservesUntouchedRows(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.db.Exec(`delete from fiscal_state_rows; delete from fiscal_state_meta`); err != nil {
		t.Fatal(err)
	}
	first := []byte(`{"sales":{},"operations":{},"devices":{"same":{"id":"same"},"changed":{"id":"changed","version":1},"removed":{"id":"removed"}},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{}}`)
	if err = p.Save(first); err != nil {
		t.Fatal(err)
	}
	var sameBefore, changedBefore time.Time
	if err = p.db.QueryRow(`select updated_at from fiscal_state_rows where collection='devices' and entity_key='same'`).Scan(&sameBefore); err != nil {
		t.Fatal(err)
	}
	if err = p.db.QueryRow(`select updated_at from fiscal_state_rows where collection='devices' and entity_key='changed'`).Scan(&changedBefore); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	second := []byte(`{"sales":{},"operations":{},"devices":{"same":{"id":"same"},"changed":{"id":"changed","version":2},"added":{"id":"added"}},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{}}`)
	if err = p.Save(second); err != nil {
		t.Fatal(err)
	}
	var sameAfter, changedAfter time.Time
	if err = p.db.QueryRow(`select updated_at from fiscal_state_rows where collection='devices' and entity_key='same'`).Scan(&sameAfter); err != nil {
		t.Fatal(err)
	}
	if err = p.db.QueryRow(`select updated_at from fiscal_state_rows where collection='devices' and entity_key='changed'`).Scan(&changedAfter); err != nil {
		t.Fatal(err)
	}
	if !sameAfter.Equal(sameBefore) {
		t.Fatal("unchanged row was rewritten")
	}
	if !changedAfter.After(changedBefore) {
		t.Fatal("changed row was not updated")
	}
	var count int
	if err = p.db.QueryRow(`select count(*) from fiscal_state_rows where collection='devices' and entity_key in ('removed','added')`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("targeted add/delete failed: %d %v", count, err)
	}
}

func TestPostgresTypedProjectionIsAtomicAndDifferential(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.db.Exec(`delete from fiscal_state_rows;delete from fiscal_state_meta;delete from fiscal_runtime_operations;delete from fiscal_runtime_sales`); err != nil {
		t.Fatal(err)
	}
	base := `{"sales":{"sale-typed":{"sale_id":"sale-typed","tenant_id":"tenant-a","external_id":"external-a","register_id":"register-a","operator_id":"A001","state":"DRAFT","version":%d,"lines":[],"payments":[],"created_at":"2026-08-07T10:00:00Z","updated_at":"%s"}},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{}}`
	if err = p.Save([]byte(fmt.Sprintf(base, 1, "2026-08-07T10:00:00Z"))); err != nil {
		t.Fatal(err)
	}
	var tenant, state string
	var version int64
	if err = p.db.QueryRow(`select tenant_id,state,version from fiscal_runtime_sales where id='sale-typed'`).Scan(&tenant, &state, &version); err != nil || tenant != "tenant-a" || state != "DRAFT" || version != 1 {
		t.Fatal(tenant, state, version, err)
	}
	raw, err := p.LoadTenantEntity("sales", "tenant-a", "sale-typed")
	if err != nil || !json.Valid(raw) {
		t.Fatal("tenant typed read failed", string(raw), err)
	}
	if _, err = p.LoadTenantEntity("sales", "tenant-b", "sale-typed"); err == nil {
		t.Fatal("RLS typed read exposed foreign sale")
	}
	rows, err := p.LoadTenantEntities("sales", "tenant-a")
	if err != nil || len(rows) != 1 {
		t.Fatal("tenant collection read failed", len(rows), err)
	}
	rows, err = p.LoadTenantEntities("sales", "tenant-b")
	if err != nil || len(rows) != 0 {
		t.Fatal("RLS collection exposed foreign sale", len(rows), err)
	}
	if err = p.Save([]byte(fmt.Sprintf(base, 2, "2026-08-07T10:01:00Z"))); err != nil {
		t.Fatal(err)
	}
	if err = p.db.QueryRow(`select version from fiscal_runtime_sales where id='sale-typed'`).Scan(&version); err != nil || version != 2 {
		t.Fatal(version, err)
	}
	conflict := []byte(`{"sales":{"sale-typed":{"sale_id":"sale-typed","tenant_id":"tenant-a","external_id":"external-a","register_id":"register-a","operator_id":"A001","state":"DRAFT","version":3,"lines":[],"payments":[],"created_at":"2026-08-07T10:00:00Z","updated_at":"2026-08-07T10:02:00Z"},"sale-conflict":{"sale_id":"sale-conflict","tenant_id":"tenant-a","external_id":"external-a","register_id":"register-a","operator_id":"A001","state":"DRAFT","version":1,"lines":[],"payments":[],"created_at":"2026-08-07T10:00:00Z","updated_at":"2026-08-07T10:02:00Z"}},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{}}`)
	if err = p.Save(conflict); err == nil {
		t.Fatal("typed uniqueness violation accepted")
	}
	var stateCount int
	if err = p.db.QueryRow(`select count(*) from fiscal_state_rows where collection='sales'`).Scan(&stateCount); err != nil || stateCount != 1 {
		t.Fatal("compatibility rows were not rolled back", stateCount, err)
	}
	empty := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{}}`)
	if err = p.Save(empty); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = p.db.QueryRow(`select count(*) from fiscal_runtime_sales`).Scan(&count)
	if count != 0 {
		t.Fatal("typed delete not applied")
	}
}
