package persistence

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
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
		"activation_challenges": map[string]any{}, "activation_requests": map[string]any{},
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
func assertJSONEqual(t *testing.T, wantRaw, gotRaw []byte) {
	t.Helper()
	var want, got any
	if json.Unmarshal(wantRaw, &want) != nil || json.Unmarshal(gotRaw, &got) != nil || !reflect.DeepEqual(want, got) {
		t.Fatalf("JSON mismatch\nwant=%s\ngot=%s", wantRaw, gotRaw)
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
	raw := []byte(`{"sales":{},"operations":{},"devices":{"device-1":{"id":"device-1"}},"shifts":{},"unp":{},"replays":{},"outbox":{"event-1":{"id":"event-1","event":{"event_id":"event-1","event_type":"sale.updated","tenant_id":"tenant-1","resource_id":"sale-1"},"attempts":0,"next_attempt":"2026-08-08T10:00:00Z"}},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
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

func TestPostgresVersionedSaveRejectsStaleInstance(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p1, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p1.Close()
	p2, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p2.Close()
	if _, err = p1.db.Exec(`delete from fiscal_state_rows;delete from fiscal_state_meta;delete from runtime_snapshots where aggregate='fiscal'`); err != nil {
		t.Fatal(err)
	}
	_, generation1, err := p1.LoadVersioned()
	if err != nil || generation1 != 0 {
		t.Fatal(generation1, err)
	}
	_, generation2, err := p2.LoadVersioned()
	if err != nil || generation2 != 0 {
		t.Fatal(generation2, err)
	}
	first := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if generation1, err = p1.SaveVersioned(first, generation1); err != nil || generation1 != 1 {
		t.Fatal(generation1, err)
	}
	stale := []byte(`{"sales":{},"operations":{},"devices":{"stale":{"id":"stale"}},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if _, err = p2.SaveVersioned(stale, generation2); !errors.Is(err, ErrConcurrentState) {
		t.Fatal("stale Fiscal instance overwrote current state", err)
	}
	loaded, current, err := p2.LoadVersioned()
	if err != nil || current != 1 {
		t.Fatal(current, err)
	}
	assertJSONEqual(t, first, loaded)
}

func TestPostgresDeltaSaveTouchesOnlyExplicitFiscalRows(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.db.Exec(`delete from fiscal_state_rows;delete from fiscal_state_meta;delete from runtime_snapshots where aggregate='fiscal'`); err != nil {
		t.Fatal(err)
	}
	baseline := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	generation, err := p.SaveVersioned(baseline, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.db.Exec(`insert into fiscal_state_rows(collection,entity_key,payload) values('devices','unmanaged','{"id":"unmanaged"}'::jsonb)`); err != nil {
		t.Fatal(err)
	}
	current := []byte(`{"sales":{},"operations":{},"devices":{"managed":{"id":"managed"}},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if _, err = p.SaveDeltaVersioned(baseline, current, generation); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = p.db.QueryRow(`select count(*) from fiscal_state_rows where collection='devices' and entity_key in('managed','unmanaged')`).Scan(&count); err != nil || count != 2 {
		t.Fatal("delta save scanned/deleted an unrelated Fiscal row", count, err)
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
	raw := []byte(`{"sales":{"legacy-sale":{"sale_id":"legacy-sale","tenant_id":"tenant-migration","external_id":"legacy-external","register_id":"register-1","operator_id":"A001","state":"DRAFT","version":1,"lines":[],"payments":[],"created_at":"2026-08-08T10:00:00Z","updated_at":"2026-08-08T10:00:00Z"}},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if _, err = p.db.Exec(`delete from fiscal_state_rows; delete from fiscal_state_meta; delete from fiscal_runtime_sales`); err != nil {
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
	if err = p.db.QueryRow(`select count(*) from fiscal_state_rows where collection='sales' and entity_key='legacy-sale'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("legacy state not migrated: %d %v", count, err)
	}
	var tenant string
	if err = p.db.QueryRow(`select tenant_id from fiscal_runtime_sales where id='legacy-sale'`).Scan(&tenant); err != nil || tenant != "tenant-migration" {
		t.Fatalf("legacy state missing authoritative typed projection: tenant=%q err=%v", tenant, err)
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
	legacy := `{"sales":{"must-not-return":{"id":"must-not-return"}},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`
	if _, err = p.db.Exec(`delete from fiscal_state_rows; delete from fiscal_state_meta`); err != nil {
		t.Fatal(err)
	}
	if _, err = p.db.Exec(`insert into runtime_snapshots(aggregate,payload,version,updated_at) values('fiscal',$1::jsonb,1,now()) on conflict(aggregate) do update set payload=excluded.payload`, legacy); err != nil {
		t.Fatal(err)
	}
	empty := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
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
	first := []byte(`{"sales":{},"operations":{},"devices":{"same":{"id":"same"},"changed":{"id":"changed","version":1},"removed":{"id":"removed"}},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
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
	second := []byte(`{"sales":{},"operations":{},"devices":{"same":{"id":"same"},"changed":{"id":"changed","version":2},"added":{"id":"added"}},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
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
	p, err := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.db.Exec(`delete from fiscal_state_rows;delete from fiscal_state_meta;delete from fiscal_runtime_operations;delete from fiscal_runtime_sales;delete from fiscal_runtime_shifts`); err != nil {
		t.Fatal(err)
	}
	base := `{"sales":{"sale-typed":{"sale_id":"sale-typed","tenant_id":"tenant-a","external_id":"external-a","register_id":"register-a","operator_id":"A001","state":"DRAFT","version":%d,"lines":[],"payments":[],"fiscal_device":{"device_id":"device-typed","serial":"SN-TYPED","fiscal_device_number":"FD-TYPED","fiscal_memory_number":"FM-TYPED","vendor":"Datecs","model":"DP-150 MX","firmware":"2026-EUR"},"created_at":"2026-08-07T10:00:00Z","updated_at":"%s"}},"operations":{},"devices":{},"shifts":{"shift-typed":{"id":"shift-typed","tenant_id":"tenant-a","register_id":"register-a","operator_id":"A001","state":"OPEN","version":1,"opened_at":"2026-08-07T09:00:00Z"}},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{"device:device-typed":{"kind":"device","tenant_id":"tenant-a","id":"device-typed","version":1,"data":{"vendor":"Datecs","serial":"SN-TYPED"},"created_at":"2026-08-07T08:00:00Z","updated_at":"2026-08-07T08:00:00Z"}},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`
	if err = p.Save([]byte(fmt.Sprintf(base, 1, "2026-08-07T10:00:00Z"))); err != nil {
		t.Fatal(err)
	}
	var tenant, state string
	var version int64
	if err = p.db.QueryRow(`select tenant_id,state,version from fiscal_runtime_sales where id='sale-typed'`).Scan(&tenant, &state, &version); err != nil || tenant != "tenant-a" || state != "DRAFT" || version != 1 {
		t.Fatal(tenant, state, version, err)
	}
	var deviceID, fiscalNumber, memoryNumber string
	if err = p.db.QueryRow(`select fiscal_device_id,fiscal_device_number,fiscal_memory_number from fiscal_runtime_sales where id='sale-typed'`).Scan(&deviceID, &fiscalNumber, &memoryNumber); err != nil || deviceID != "device-typed" || fiscalNumber != "FD-TYPED" || memoryNumber != "FM-TYPED" {
		t.Fatal("typed fiscal device snapshot missing", deviceID, fiscalNumber, memoryNumber, err)
	}
	raw, err := p.LoadTenantEntity("sales", "tenant-a", "sale-typed")
	if err != nil || !json.Valid(raw) {
		t.Fatal("tenant typed read failed", string(raw), err)
	}
	if !bytes.Contains(raw, []byte(`"fiscal_memory_number"`)) || !bytes.Contains(raw, []byte(`"FM-TYPED"`)) {
		t.Fatal("typed sale reconstruction lost fiscal memory snapshot", string(raw))
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
	shiftRaw, err := p.LoadTenantEntity("shifts", "tenant-a", "shift-typed")
	if err != nil || !json.Valid(shiftRaw) {
		t.Fatal("tenant typed shift read failed", string(shiftRaw), err)
	}
	if _, err = p.LoadTenantEntity("shifts", "tenant-b", "shift-typed"); err == nil {
		t.Fatal("RLS typed read exposed foreign shift")
	}
	resourceRaw, err := p.LoadTenantEntity("resources:device", "tenant-a", "device-typed")
	if err != nil || !json.Valid(resourceRaw) {
		t.Fatal("tenant typed resource read failed", string(resourceRaw), err)
	}
	if _, err = p.LoadTenantEntity("resources:device", "tenant-b", "device-typed"); err == nil {
		t.Fatal("RLS typed read exposed foreign resource")
	}
	reassigned := strings.Replace(fmt.Sprintf(base, 2, "2026-08-07T10:01:00Z"), "tenant-a", "tenant-b", 1)
	if err = p.Save([]byte(reassigned)); err == nil {
		t.Fatal("RLS-bound mutation reassigned an existing sale to another tenant")
	}
	if err = p.db.QueryRow(`select tenant_id,version from fiscal_runtime_sales where id='sale-typed'`).Scan(&tenant, &version); err != nil || tenant != "tenant-a" || version != 1 {
		t.Fatal("failed cross-tenant mutation was not atomic", tenant, version, err)
	}
	if err = p.Save([]byte(fmt.Sprintf(base, 2, "2026-08-07T10:01:00Z"))); err != nil {
		t.Fatal(err)
	}
	if err = p.db.QueryRow(`select version from fiscal_runtime_sales where id='sale-typed'`).Scan(&version); err != nil || version != 2 {
		t.Fatal(version, err)
	}
	conflict := []byte(`{"sales":{"sale-typed":{"sale_id":"sale-typed","tenant_id":"tenant-a","external_id":"external-a","register_id":"register-a","operator_id":"A001","state":"DRAFT","version":3,"lines":[],"payments":[],"created_at":"2026-08-07T10:00:00Z","updated_at":"2026-08-07T10:02:00Z"},"sale-conflict":{"sale_id":"sale-conflict","tenant_id":"tenant-a","external_id":"external-a","register_id":"register-a","operator_id":"A001","state":"DRAFT","version":1,"lines":[],"payments":[],"created_at":"2026-08-07T10:00:00Z","updated_at":"2026-08-07T10:02:00Z"}},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if err = p.Save(conflict); err == nil {
		t.Fatal("typed uniqueness violation accepted")
	}
	var stateCount int
	if err = p.db.QueryRow(`select count(*) from fiscal_state_rows where collection='sales'`).Scan(&stateCount); err != nil || stateCount != 1 {
		t.Fatal("compatibility rows were not rolled back", stateCount, err)
	}
	empty := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if err = p.Save(empty); err != nil {
		t.Fatal(err)
	}
	var count int
	_ = p.db.QueryRow(`select count(*) from fiscal_runtime_sales`).Scan(&count)
	if count != 0 {
		t.Fatal("typed delete not applied")
	}
}

func TestPostgresTypedTechnicalAggregatesAreTenantBound(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.db.Exec(`delete from fiscal_state_rows;delete from fiscal_state_meta;delete from fiscal_runtime_outbox;delete from fiscal_runtime_ble_sessions;delete from fiscal_runtime_connectivity_probes;delete from fiscal_runtime_edge_pending;delete from fiscal_runtime_audit;delete from fiscal_runtime_replays;delete from fiscal_runtime_unp_sequences`); err != nil {
		t.Fatal(err)
	}
	state := `{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{"tenant-tech\nregister-1":7},"replays":{"tenant-tech POST /public/v1/sales replay-key-00001":{"hash":"def","status":201,"body":"e30="}},"outbox":{"outbox-1":{"id":"outbox-1","event":{"event_id":"event-1","event_type":"sale.updated","api_version":"2026-08-07","tenant_id":"tenant-tech","resource_id":"sale-1","resource_version":1,"occurred_at":"2026-08-08T10:00:00Z","data":{}},"attempts":0,"next_attempt":"2026-08-08T10:01:00Z"}},"ble_sessions":{"ble-1":{"session_id":"ble-1","tenant_id":"tenant-tech","location_id":"location-1","register_id":"register-1","operator_id":"A001","app_instance_id":"app-1","actor_subject":"subject-1","device_id":"edge-1","fiscal_device_id":"fiscal-device-1","scopes":["fiscal.execute"],"fencing_token":1,"expires_at":"2026-08-08T18:00:00Z","revoked":false,"nonce":"nonce"}},"sync_acks":{},"connectivity_probes":{"probe-1":{"probe_id":"probe-1","tenant_id":"tenant-tech","register_id":"register-1","state":"SUCCEEDED","observed_at":"2026-08-08T10:00:00Z","hops":{},"recommended_transport":"REST"}},"resources":{},"artifacts":{},"audit":[{"event_id":"audit-1","tenant_id":"tenant-tech","actor_id":"A001","action":"UPSERT","object_type":"sale","object_id":"sale-1","occurred_at":"2026-08-08T10:00:00Z","before":{},"after":{},"event_hash":"abc"}],"edge_pending":{"operation-1":{"operation_id":"operation-1","tenant_id":"tenant-tech","register_id":"register-1","device_id":"edge-1","command_type":"FISCAL_SALE","payload":{},"operation_sequence":1,"unp_sequence":1,"accepted_at":"2026-08-08T10:00:00Z"}}}`
	if err = p.Save([]byte(state)); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"fiscal_runtime_outbox", "fiscal_runtime_ble_sessions", "fiscal_runtime_connectivity_probes", "fiscal_runtime_edge_pending", "fiscal_runtime_audit", "fiscal_runtime_replays", "fiscal_runtime_unp_sequences"} {
		var count int
		if err = p.db.QueryRow(`select count(*) from ` + table + ` where tenant_id='tenant-tech'`).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s projection: count=%d err=%v", table, count, err)
		}
	}
	var locationID, edgeID, fiscalDeviceID string
	if err = p.db.QueryRow(`select location_id,device_id,fiscal_device_id from fiscal_runtime_ble_sessions where id='ble-1'`).Scan(&locationID, &edgeID, &fiscalDeviceID); err != nil || locationID != "location-1" || edgeID != "edge-1" || fiscalDeviceID != "fiscal-device-1" || edgeID == fiscalDeviceID {
		t.Fatal("BLE authority identity projection collapsed edge/fiscal device", locationID, edgeID, fiscalDeviceID, err)
	}
	for collection, id := range map[string]string{
		"ble_sessions":        "ble-1",
		"connectivity_probes": "probe-1",
		"edge_pending":        "operation-1",
		"replays":             "tenant-tech POST /public/v1/sales replay-key-00001",
	} {
		raw, readErr := p.LoadTenantEntity(collection, "tenant-tech", id)
		if readErr != nil || len(raw) == 0 {
			t.Fatalf("typed %s orchestration read failed: %s %v", collection, raw, readErr)
		}
		if _, readErr = p.LoadTenantEntity(collection, "tenant-other", id); !errors.Is(readErr, sql.ErrNoRows) {
			t.Fatalf("typed %s read exposed foreign tenant: %v", collection, readErr)
		}
	}
	auditRows, readErr := p.LoadTenantEntities("audit", "tenant-tech")
	if readErr != nil || len(auditRows) != 1 {
		t.Fatal("typed audit orchestration read failed", len(auditRows), readErr)
	}
	foreignAudit, readErr := p.LoadTenantEntities("audit", "tenant-other")
	if readErr != nil || len(foreignAudit) != 0 {
		t.Fatal("typed audit read exposed foreign tenant", len(foreignAudit), readErr)
	}
	outboxRows, readErr := p.LoadSystemEntities("outbox")
	if readErr != nil || len(outboxRows) != 1 {
		t.Fatal("typed system outbox read failed", len(outboxRows), readErr)
	}
	tx, err := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`set local role beefiscal_tenant`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`select set_config('app.tenant_id','tenant-other',true)`); err != nil {
		t.Fatal(err)
	}
	var hidden int
	if err = tx.QueryRow(`select count(*) from fiscal_runtime_edge_pending`).Scan(&hidden); err != nil || hidden != 0 {
		t.Fatal("RLS exposed foreign technical aggregate", hidden, err)
	}
	_ = tx.Rollback()
	reassigned := strings.Replace(state, `"tenant_id":"tenant-tech","register_id":"register-1","device_id":"edge-1","command_type":"FISCAL_SALE"`, `"tenant_id":"tenant-other","register_id":"register-1","device_id":"edge-1","command_type":"FISCAL_SALE"`, 1)
	if err = p.Save([]byte(reassigned)); err == nil {
		t.Fatal("RLS-bound mutation reassigned edge pending command")
	}
	var tenant string
	if err = p.db.QueryRow(`select tenant_id from fiscal_runtime_edge_pending where operation_id='operation-1'`).Scan(&tenant); err != nil || tenant != "tenant-tech" {
		t.Fatal("failed technical mutation was not atomic", tenant, err)
	}
	empty := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if err = p.Save(empty); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresTypedArtifactsAreTenantBound(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.db.Exec(`delete from fiscal_state_rows;delete from fiscal_state_meta;delete from fiscal_runtime_artifacts`); err != nil {
		t.Fatal(err)
	}
	state := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{"tenant-artifact\nreceipt-1":"aGVsbG8="},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if err = p.Save(state); err != nil {
		t.Fatal(err)
	}
	var body []byte
	var digest string
	if err = p.db.QueryRow(`select body,sha256 from fiscal_runtime_artifacts where tenant_id='tenant-artifact' and id='receipt-1'`).Scan(&body, &digest); err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" || digest != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("unexpected artifact projection body=%q sha256=%s", body, digest)
	}
	raw, readErr := p.LoadTenantEntity("artifacts", "tenant-artifact", "receipt-1")
	if readErr != nil {
		t.Fatal("typed artifact read failed", readErr)
	}
	var typedBody []byte
	if err = json.Unmarshal(raw, &typedBody); err != nil || string(typedBody) != "hello" {
		t.Fatal("typed artifact read returned wrong body", string(typedBody), err)
	}
	if _, readErr = p.LoadTenantEntity("artifacts", "tenant-other", "receipt-1"); !errors.Is(readErr, sql.ErrNoRows) {
		t.Fatal("typed artifact read exposed foreign tenant", readErr)
	}
	tx, err := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`set local role beefiscal_tenant`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`select set_config('app.tenant_id','tenant-other',true)`); err != nil {
		t.Fatal(err)
	}
	var hidden int
	if err = tx.QueryRow(`select count(*) from fiscal_runtime_artifacts`).Scan(&hidden); err != nil || hidden != 0 {
		t.Fatal("RLS exposed foreign artifact", hidden, err)
	}
	_ = tx.Rollback()
	malformed := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{"tenant-artifact\nreceipt-1":"%%%"},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if err = p.Save(malformed); err == nil {
		t.Fatal("malformed artifact was accepted")
	}
	if err = p.db.QueryRow(`select body from fiscal_runtime_artifacts where tenant_id='tenant-artifact' and id='receipt-1'`).Scan(&body); err != nil || string(body) != "hello" {
		t.Fatal("failed artifact mutation was not atomic", string(body), err)
	}
	empty := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if err = p.Save(empty); err != nil {
		t.Fatal(err)
	}
	if err = p.db.QueryRow(`select count(*) from fiscal_runtime_artifacts`).Scan(&hidden); err != nil || hidden != 0 {
		t.Fatal("artifact typed delete failed", hidden, err)
	}
}

func TestPostgresTypedSyncAcksAreTenantBound(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.db.Exec(`delete from fiscal_state_rows;delete from fiscal_state_meta;delete from fiscal_runtime_sync_acks`); err != nil {
		t.Fatal(err)
	}
	state := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{"tenant-sync\nedge-1":{"ack_id":"ack-1","edge_id":"edge-1","committed_through_seq":3,"committed_event_hash":"2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824","committed_at":"2026-08-08T10:00:00Z","operation_results":[],"rejected":[],"signature":"sig"}},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if err = p.Save(state); err != nil {
		t.Fatal(err)
	}
	var seq int64
	if err = p.db.QueryRow(`select committed_through_seq from fiscal_runtime_sync_acks where tenant_id='tenant-sync' and edge_id='edge-1'`).Scan(&seq); err != nil || seq != 3 {
		t.Fatal("sync ack projection missing", seq, err)
	}
	raw, readErr := p.LoadTenantEntity("sync_acks", "tenant-sync", "edge-1")
	if readErr != nil || !strings.Contains(string(raw), `"ack_id": "ack-1"`) {
		t.Fatal("typed sync acknowledgement read failed", string(raw), readErr)
	}
	if _, readErr = p.LoadTenantEntity("sync_acks", "tenant-other", "edge-1"); !errors.Is(readErr, sql.ErrNoRows) {
		t.Fatal("typed sync acknowledgement exposed foreign tenant", readErr)
	}
	tx, err := p.reader.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`set local role beefiscal_tenant`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(`select set_config('app.tenant_id','tenant-other',true)`); err != nil {
		t.Fatal(err)
	}
	var hidden int
	if err = tx.QueryRow(`select count(*) from fiscal_runtime_sync_acks`).Scan(&hidden); err != nil || hidden != 0 {
		t.Fatal("RLS exposed foreign sync acknowledgement", hidden, err)
	}
	_ = tx.Rollback()
	empty := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	if err = p.Save(empty); err != nil {
		t.Fatal(err)
	}
}

func TestFiscalTypedOnlyRestartAndRollback(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err = p.db.Exec(`delete from fiscal_state_rows;delete from fiscal_state_meta;delete from fiscal_runtime_sales;delete from fiscal_runtime_operations;delete from fiscal_runtime_shifts;delete from fiscal_runtime_resources;delete from fiscal_runtime_outbox;delete from fiscal_runtime_ble_sessions;delete from fiscal_runtime_connectivity_probes;delete from fiscal_runtime_edge_pending;delete from fiscal_runtime_audit;delete from fiscal_runtime_replays;delete from fiscal_runtime_unp_sequences;delete from fiscal_runtime_artifacts;delete from fiscal_runtime_sync_acks`); err != nil {
		t.Fatal(err)
	}
	empty := []byte(`{"sales":{},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	generation, err := p.SaveVersioned(empty, 0)
	if err != nil {
		t.Fatal(err)
	}
	v1 := []byte(`{"sales":{"typed-only":{"sale_id":"typed-only","tenant_id":"tenant-typed","external_id":"typed-external","register_id":"register-1","operator_id":"A001","state":"DRAFT","version":1,"lines":[],"payments":[],"created_at":"2026-08-08T10:00:00Z","updated_at":"2026-08-08T10:00:00Z"}},"operations":{},"devices":{},"shifts":{},"unp":{},"replays":{},"outbox":{},"ble_sessions":{},"sync_acks":{},"connectivity_probes":{},"resources":{},"artifacts":{},"audit":[],"edge_pending":{},"activation_challenges":{},"activation_requests":{}}`)
	generation, err = p.SaveDeltaVersioned(empty, v1, generation)
	if err != nil {
		t.Fatal(err)
	}
	var mode int
	if err = p.db.QueryRow(`select storage_mode from fiscal_state_meta where singleton=true`).Scan(&mode); err != nil || mode != 2 {
		t.Fatal("typed-only mode was not activated", mode, err)
	}
	v2 := bytes.Replace(v1, []byte(`"version":1`), []byte(`"version":2`), 1)
	generation, err = p.SaveDeltaVersioned(v1, v2, generation)
	if err != nil {
		t.Fatal(err)
	}
	provisioning := []byte(`"resources":{"provisioning_session:11111111-1111-4111-8111-111111111111":{"kind":"provisioning_session","tenant_id":"tenant-typed","id":"11111111-1111-4111-8111-111111111111","version":1,"data":{"session_id":"11111111-1111-4111-8111-111111111111","device_id":"22222222-2222-4222-8222-222222222222","state":"CREATED","expires_at":"2026-08-08T10:15:00Z"},"created_at":"2026-08-08T10:00:00Z","updated_at":"2026-08-08T10:00:00Z"}}`)
	v3 := bytes.Replace(v2, []byte(`"resources":{}`), provisioning, 1)
	generation, err = p.SaveDeltaVersioned(v2, v3, generation)
	if err != nil {
		t.Fatal(err)
	}
	var compatibilityVersion, typedVersion int
	if err = p.db.QueryRow(`select (payload->>'version')::int from fiscal_state_rows where collection='sales' and entity_key='typed-only'`).Scan(&compatibilityVersion); err != nil || compatibilityVersion != 1 {
		t.Fatal("typed-only write still mutated compatibility state", compatibilityVersion, err)
	}
	if err = p.db.QueryRow(`select (payload->>'version')::int from fiscal_runtime_sales where id='typed-only'`).Scan(&typedVersion); err != nil || typedVersion != 2 {
		t.Fatal("typed-only projection not updated", typedVersion, err)
	}
	var provisioningTenant, provisioningState string
	if err = p.db.QueryRow(`select tenant_id,data->>'state' from fiscal_runtime_resources where kind='provisioning_session' and id='11111111-1111-4111-8111-111111111111'`).Scan(&provisioningTenant, &provisioningState); err != nil || provisioningTenant != "tenant-typed" || provisioningState != "CREATED" {
		t.Fatal("typed-only provisioning session not stored", provisioningTenant, provisioningState, err)
	}
	if _, err = p.db.Exec(`update fiscal_state_rows set payload=jsonb_set(payload,'{version}','99'::jsonb) where collection='sales' and entity_key='typed-only'`); err != nil {
		t.Fatal(err)
	}
	loaded, loadedGeneration, err := p.LoadVersioned()
	if err != nil || loadedGeneration != generation {
		t.Fatal(loadedGeneration, generation, err)
	}
	var got, want map[string]any
	_ = json.Unmarshal(loaded, &got)
	_ = json.Unmarshal(v3, &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("typed-only restart consulted compatibility state")
	}
	bad := bytes.Replace(v3, []byte(`"version":2`), []byte(`"version":0`), 1)
	if _, err = p.SaveDeltaVersioned(v3, bad, generation); err == nil {
		t.Fatal("typed-only constraint failure was accepted")
	}
	loaded, loadedGeneration, err = p.LoadVersioned()
	if err != nil || loadedGeneration != generation {
		t.Fatal("failed typed-only mutation changed generation", loadedGeneration, generation, err)
	}
	_ = json.Unmarshal(loaded, &got)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("failed typed-only mutation changed durable state")
	}
}
