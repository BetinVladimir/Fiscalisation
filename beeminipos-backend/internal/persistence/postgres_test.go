package persistence

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestMiniPOSStateRowsRoundTrip(t *testing.T) {
	raw := []byte(`{"products":{"p":{"id":"p"}},"employees":{},"shifts":{},"orders":{"o":{"id":"o"}},"checkouts":{},"checkout_hashes":{"k":"h"},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":42}`)
	rows, e := flattenSnapshot(raw)
	if e != nil {
		t.Fatal(e)
	}
	rebuilt, e := rebuildSnapshot(rows)
	if e != nil {
		t.Fatal(e)
	}
	var got, want map[string]any
	_ = json.Unmarshal(rebuilt, &got)
	_ = json.Unmarshal(raw, &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip changed state: %#v %#v", want, got)
	}
}

func TestMiniPOSDifferentialSavePreservesUntouchedRows(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows; delete from minipos_state_meta`); e != nil {
		t.Fatal(e)
	}
	first := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{"same":{"id":"same"},"changed":{"id":"changed","version":1},"removed":{"id":"removed"}},"sequence":1}`)
	if e = p.Save(first); e != nil {
		t.Fatal(e)
	}
	var sameBefore, changedBefore time.Time
	if e = p.db.QueryRow(`select updated_at from minipos_state_rows where collection='configurations' and entity_key='same'`).Scan(&sameBefore); e != nil {
		t.Fatal(e)
	}
	if e = p.db.QueryRow(`select updated_at from minipos_state_rows where collection='configurations' and entity_key='changed'`).Scan(&changedBefore); e != nil {
		t.Fatal(e)
	}
	time.Sleep(10 * time.Millisecond)
	second := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{"same":{"id":"same"},"changed":{"id":"changed","version":2},"added":{"id":"added"}},"sequence":2}`)
	if e = p.Save(second); e != nil {
		t.Fatal(e)
	}
	var sameAfter, changedAfter time.Time
	if e = p.db.QueryRow(`select updated_at from minipos_state_rows where collection='configurations' and entity_key='same'`).Scan(&sameAfter); e != nil {
		t.Fatal(e)
	}
	if e = p.db.QueryRow(`select updated_at from minipos_state_rows where collection='configurations' and entity_key='changed'`).Scan(&changedAfter); e != nil {
		t.Fatal(e)
	}
	if !sameAfter.Equal(sameBefore) {
		t.Fatal("unchanged row was rewritten")
	}
	if !changedAfter.After(changedBefore) {
		t.Fatal("changed row was not updated")
	}
	var count int
	if e = p.db.QueryRow(`select count(*) from minipos_state_rows where collection='configurations' and entity_key in ('removed','added')`).Scan(&count); e != nil || count != 1 {
		t.Fatalf("targeted add/delete failed: %d %v", count, e)
	}
}
func TestMiniPOSTypedProjectionIsAtomicAndDifferential(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := OpenWithReader(url, os.Getenv("PG_RLS_INTEGRATION_URL"))
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from minipos_runtime_orders;delete from minipos_runtime_shifts;delete from minipos_runtime_employees;delete from minipos_runtime_products`); e != nil {
		t.Fatal(e)
	}
	base := `{"products":{"product-typed":{"id":"product-typed","tenant_id":"org-a","sku":"SKU-A","name":"Coffee","unit":"pcs","price":{"amount":"%s","currency":"EUR"},"tax_group":"B","active":true,"status":"ACTIVE","version":%d,"created_at":"2026-08-07T10:00:00Z","updated_at":"2026-08-07T10:01:00Z"}},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":1}`
	if e = p.Save([]byte(fmt.Sprintf(base, "2.50", 1))); e != nil {
		t.Fatal(e)
	}
	var tenant, amount string
	var version int64
	if e = p.db.QueryRow(`select organization_id,amount::text,version from minipos_runtime_products where id='product-typed'`).Scan(&tenant, &amount, &version); e != nil || tenant != "org-a" || amount != "2.50" || version != 1 {
		t.Fatal(tenant, amount, version, e)
	}
	raw, e := p.LoadTenantEntity("products", "org-a", "product-typed")
	if e != nil || !json.Valid(raw) {
		t.Fatal("tenant typed read failed", string(raw), e)
	}
	if _, e = p.LoadTenantEntity("products", "org-b", "product-typed"); e == nil {
		t.Fatal("RLS typed read exposed foreign product")
	}
	rows, e := p.LoadTenantEntities("products", "org-a")
	if e != nil || len(rows) != 1 {
		t.Fatal("tenant collection read failed", len(rows), e)
	}
	rows, e = p.LoadTenantEntities("products", "org-b")
	if e != nil || len(rows) != 0 {
		t.Fatal("RLS collection exposed foreign product", len(rows), e)
	}
	if e = p.Save([]byte(fmt.Sprintf(base, "3.00", 2))); e != nil {
		t.Fatal(e)
	}
	if e = p.db.QueryRow(`select amount::text,version from minipos_runtime_products where id='product-typed'`).Scan(&amount, &version); e != nil || amount != "3.00" || version != 2 {
		t.Fatal(amount, version, e)
	}
	invalid := []byte(`{"products":{"product-typed":{"id":"product-typed","tenant_id":"org-a","sku":"SKU-A","name":"Coffee","unit":"pcs","price":{"amount":"4.00","currency":"BGN"},"tax_group":"B","active":true,"status":"ACTIVE","version":3,"created_at":"2026-08-07T10:00:00Z","updated_at":"2026-08-07T10:02:00Z"}},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":2}`)
	if e = p.Save(invalid); e == nil {
		t.Fatal("non-EUR typed write accepted")
	}
	var persistedVersion int64
	if e = p.db.QueryRow(`select (payload->>'version')::bigint from minipos_state_rows where collection='products' and entity_key='product-typed'`).Scan(&persistedVersion); e != nil || persistedVersion != 2 {
		t.Fatal("compatibility row escaped rollback", persistedVersion, e)
	}
}
func TestMiniPOSStateRowsRejectUnknownData(t *testing.T) {
	if _, e := flattenSnapshot([]byte(`{"unknown":{}}`)); e == nil {
		t.Fatal("unknown collection accepted")
	}
	if _, e := rebuildSnapshot([]stateRow{{Collection: "sequence", Key: "wrong", Payload: []byte(`1`)}}); e == nil {
		t.Fatal("invalid singleton accepted")
	}
}

func TestMiniPOSPostgresRestartLegacyAndEmptyState(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL not set")
	}
	p, e := Open(url)
	if e != nil {
		t.Fatal(e)
	}
	defer p.Close()
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta;delete from runtime_snapshots where aggregate='minipos'`); e != nil {
		t.Fatal(e)
	}
	raw := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{"c":{"id":"c"}},"sequence":7}`)
	if e = p.Save(raw); e != nil {
		t.Fatal(e)
	}
	_ = p.Close()
	p, e = Open(url)
	if e != nil {
		t.Fatal(e)
	}
	loaded, e := p.Load()
	if e != nil {
		t.Fatal(e)
	}
	assertJSONEqual(t, raw, loaded)
	legacy := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{"legacy":{"id":"legacy"}},"sequence":8}`)
	if _, e = p.db.Exec(`delete from minipos_state_rows;delete from minipos_state_meta`); e != nil {
		t.Fatal(e)
	}
	if _, e = p.db.Exec(`insert into runtime_snapshots(aggregate,payload,version,updated_at) values('minipos',$1::jsonb,1,now()) on conflict(aggregate) do update set payload=excluded.payload`, string(legacy)); e != nil {
		t.Fatal(e)
	}
	loaded, e = p.Load()
	if e != nil {
		t.Fatal(e)
	}
	assertJSONEqual(t, legacy, loaded)
	empty := []byte(`{"products":{},"employees":{},"shifts":{},"orders":{},"checkouts":{},"checkout_hashes":{},"api_replays":{},"webhook_inbox":{},"configurations":{},"sequence":0}`)
	if e = p.Save(empty); e != nil {
		t.Fatal(e)
	}
	loaded, e = p.Load()
	if e != nil {
		t.Fatal(e)
	}
	assertJSONEqual(t, empty, loaded)
}
func assertJSONEqual(t *testing.T, wantRaw, gotRaw []byte) {
	t.Helper()
	var want, got map[string]any
	_ = json.Unmarshal(wantRaw, &want)
	_ = json.Unmarshal(gotRaw, &got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("state mismatch\nwant=%#v\ngot=%#v", want, got)
	}
}
