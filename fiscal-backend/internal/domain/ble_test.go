package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

const testBLEClientPublicKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func TestBLETicketSignatureRefreshRevokePersistence(t *testing.T) {
	store := &testStore{}
	repo, e := NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	s := NewService(repo, NewSimulator(true))
	key := "01234567890123456789012345678901"
	s.SetBLESigningKey(key)
	registerID, deviceID := prepareBLERegister(t, s, "tenant1")
	if _, err := s.BLESession(registerID, "A001", "app1", "tenant1", "subject-1", ""); err == nil {
		t.Fatal("BLE bearer ticket issued without client proof-of-possession key")
	}
	v, e := s.BLESession(registerID, "A001", "app1", "tenant1", "subject-1", testBLEClientPublicKey)
	if e != nil {
		t.Fatal(e)
	}
	raw := v["signed_session_ticket"].(string)
	outer, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		t.Fatal(e)
	}
	var wrapped struct {
		Payload   string `json:"payload"`
		Signature string `json:"signature"`
	}
	if json.Unmarshal(outer, &wrapped) != nil {
		t.Fatal("wrapper")
	}
	payload, _ := base64.RawURLEncoding.DecodeString(wrapped.Payload)
	sig, _ := base64.RawURLEncoding.DecodeString(wrapped.Signature)
	m := hmac.New(sha256.New, []byte(key))
	m.Write(payload)
	if !hmac.Equal(sig, m.Sum(nil)) {
		t.Fatal("bad signature")
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil || claims["TenantID"] != "tenant1" {
		t.Fatalf("%s", payload)
	}
	id := v["ble_session_id"].(string)
	repo, e = NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	s = NewService(repo, NewSimulator(true))
	s.SetBLESigningKey(key)
	if _, e = s.RefreshBLE(id, "tenant2", "subject-1"); e == nil {
		t.Fatal("cross tenant refresh")
	}
	if e = s.RevokeBLE(id, "tenant1", "subject-1"); e != nil {
		t.Fatal(e)
	}
	pending := s.PendingOutbox(time.Now().UTC().Add(time.Second))
	if len(pending) != 1 || pending[0].Event.EventType != "ble.session.revoked" || pending[0].Event.ResourceID != id {
		t.Fatalf("revocation event missing: %#v", pending)
	}
	if _, e = s.RefreshBLE(id, "tenant1", "subject-1"); e == nil {
		t.Fatal("revoked refresh accepted")
	}
	_ = deviceID
}

func TestBLESessionRequiresActiveTenantRegisterAndFiscalDevice(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	s.SetBLESigningKey("01234567890123456789012345678901")
	registerID, deviceID := prepareBLERegister(t, s, "tenant1")
	if _, err := s.BLESession(registerID, "A001", "app1", "tenant2", "subject-1", testBLEClientPublicKey); err == nil {
		t.Fatal("cross-tenant BLE session accepted")
	}
	if _, err := s.BLESession("missing", "A001", "app1", "tenant1", "subject-1", testBLEClientPublicKey); err == nil {
		t.Fatal("unknown register BLE session accepted")
	}
	if _, err := s.BLESession(registerID, "NONE", "app1", "tenant1", "subject-1", testBLEClientPublicKey); err == nil {
		t.Fatal("unknown operator BLE session accepted")
	}
	if _, err := s.CreateResource("operator", "tenant1", map[string]any{"code": "F001", "first_name": "Future", "last_name": "Operator", "roles": []any{"CASHIER"}, "active_from": time.Now().UTC().Add(time.Hour).Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BLESession(registerID, "F001", "app1", "tenant1", "subject-1", testBLEClientPublicKey); err == nil {
		t.Fatal("future operator BLE session accepted")
	}
	v, err := s.BLESession(registerID, "A001", "app1", "tenant1", "subject-1", testBLEClientPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	device, err := s.GetResource("device", deviceID, "tenant1")
	if err != nil {
		t.Fatal(err)
	}
	device["status"] = "BLOCKED"
	if _, err = s.UpdateResource("device", deviceID, "tenant1", device["version"].(int64), device); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RefreshBLE(v["ble_session_id"].(string), "tenant1", "subject-1"); err == nil {
		t.Fatal("BLE session refreshed after fiscal device was blocked")
	}
}

func TestBLESessionRefreshRejectsDeactivatedOperator(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	s.SetBLESigningKey("01234567890123456789012345678901")
	registerID, deviceID := prepareBLERegister(t, s, "tenant1")
	v, err := s.BLESession(registerID, "A001", "app1", "tenant1", "subject-1", testBLEClientPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if v["device_id"] != deviceID || v["edge_id"] == deviceID || v["location_id"] == "" {
		t.Fatalf("BLE session did not distinguish edge/fiscal device or bind location: %#v", v)
	}
	operators := s.repo.Resources("operator", "tenant1")
	if len(operators) != 1 {
		t.Fatalf("operator fixture missing: %d", len(operators))
	}
	data := cloneMap(operators[0].Data)
	data["active_to"] = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if _, err = s.UpdateResource("operator", operators[0].ID, "tenant1", operators[0].Version, data); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RefreshBLE(v["ble_session_id"].(string), "tenant1", "subject-1"); err == nil {
		t.Fatal("BLE session refreshed after operator deactivation")
	}
}

func TestBLESessionLifecycleIsSubjectBoundAndRefreshRotatesAuthority(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	s.SetBLESigningKey("01234567890123456789012345678901")
	registerID, _ := prepareBLERegister(t, s, "tenant1")
	created, err := s.BLESession(registerID, "A001", "app1", "tenant1", "subject-1", testBLEClientPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	id := created["ble_session_id"].(string)
	if _, err = s.RefreshBLE(id, "tenant1", "subject-2"); err == nil {
		t.Fatal("different subject refreshed BLE authority")
	}
	if err = s.RevokeBLE(id, "tenant1", "subject-2"); err == nil {
		t.Fatal("different subject revoked BLE authority")
	}
	refreshed, err := s.RefreshBLE(id, "tenant1", "subject-1")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed["binding_version"].(int64) <= created["binding_version"].(int64) {
		t.Fatalf("fencing token did not advance: before=%v after=%v", created["binding_version"], refreshed["binding_version"])
	}
	if refreshed["signed_session_ticket"] == created["signed_session_ticket"] {
		t.Fatal("refresh reused signed session ticket")
	}
}

func TestBLESessionRefreshRejectsRegisterDeviceRebind(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	s.SetBLESigningKey("01234567890123456789012345678901")
	registerID, originalDeviceID := prepareBLERegister(t, s, "tenant-rebind")
	issued, err := s.BLESession(registerID, "A001", "app1", "tenant-rebind", "subject-1", testBLEClientPublicKey)
	if err != nil || issued["device_id"] != originalDeviceID || issued["edge_id"] == originalDeviceID || issued["location_id"] == "" {
		t.Fatalf("invalid BLE identity package: %#v %v", issued, err)
	}
	second, err := s.CreateResource("device", "tenant-rebind", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "BLE-FD-2", "status": "DRAFT", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	second = activateTestDevice(t, s, "tenant-rebind", second)
	if _, err = s.BindRegister(registerID, "tenant-rebind", second["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RefreshBLE(issued["ble_session_id"].(string), "tenant-rebind", "subject-1"); err == nil {
		t.Fatal("BLE authority refreshed after its immutable final-device binding changed")
	}
}

func prepareBLERegister(t *testing.T, s *Service, tenant string) (string, string) {
	t.Helper()
	location, err := s.CreateResource("location", tenant, map[string]any{"code": "SOF", "name": "Sofia", "address": "1 Main", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	register, err := s.CreateResource("register", tenant, map[string]any{"location_id": location["id"], "code": "R01", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	device, err := s.CreateResource("device", tenant, map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "BLE-FD-1", "status": "DRAFT", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	device = activateTestDevice(t, s, tenant, device)
	if _, err = s.BindRegister(register["id"].(string), tenant, device["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateResource("operator", tenant, map[string]any{"code": "A001", "first_name": "Ada", "last_name": "Lovelace", "roles": []any{"CASHIER"}, "active_from": "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	return register["id"].(string), device["id"].(string)
}

func activateTestDevice(t *testing.T, s *Service, tenant string, device map[string]any) map[string]any {
	t.Helper()
	data := cloneMap(device)
	for _, key := range []string{"id", "version", "created_at", "updated_at"} {
		delete(data, key)
	}
	data["status"] = "PENDING_SERVICE_ACTIVATION"
	pending, err := s.UpdateResource("device", device["id"].(string), tenant, device["version"].(int64), data)
	if err != nil {
		t.Fatal(err)
	}
	data["status"] = "ACTIVE"
	active, err := s.UpdateResource("device", device["id"].(string), tenant, pending["version"].(int64), data)
	if err != nil {
		t.Fatal(err)
	}
	return active
}
