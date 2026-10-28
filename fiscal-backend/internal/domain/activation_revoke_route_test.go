package domain

import (
	"testing"
	"time"
)

func TestRevokeActivatedAdapterInvalidatesAllDependentRoutes(t *testing.T) {
	r := NewMemoryRepository()
	now := time.Now().UTC()
	r.activationRequests["activation"] = DeviceActivationRequest{ID: "activation", DeviceInstanceID: "adapter", ClaimedTenantID: "tenant", ClaimedRegisterID: "reg", State: "ACTIVE"}
	put := func(kind, id, tenant string, data map[string]any) {
		t.Helper()
		data["code"] = id
		if kind == "device" {
			data["serial"] = id
		}
		if err := r.PutResource(ResourceRecord{Kind: kind, ID: id, TenantID: tenant, Version: 1, Data: data}); err != nil {
			t.Fatal(err)
		}
	}
	put("device", "adapter", "tenant", map[string]any{"status": "ACTIVE", "credential_id": "secret"})
	put("device", "physical", "tenant", map[string]any{"status": "ACTIVE"})
	put("composite_binding", "active", "tenant", map[string]any{"status": "ACTIVE", "adapter_device_id": "adapter", "fiscal_device_id": "physical", "register_id": "reg"})
	put("composite_binding", "pending", "tenant", map[string]any{"status": "PENDING", "adapter_device_id": "adapter", "register_id": "reg2"})
	put("composite_binding", "unrelated", "tenant", map[string]any{"status": "ACTIVE", "adapter_device_id": "other", "register_id": "reg2"})
	put("composite_binding", "foreign", "foreign", map[string]any{"status": "ACTIVE", "adapter_device_id": "adapter", "register_id": "foreign"})
	put("issued_binding", "hash", "tenant", map[string]any{"envelope": "immutable"})
	put("register", "reg", "tenant", map[string]any{"composite_binding_id": "active", "adapter_device_id": "adapter", "fiscal_device_id": "physical", "payment_terminal_id": "terminal", "location_id": "location", "device_profile": "profile"})
	put("register", "reg2", "tenant", map[string]any{"composite_binding_id": "unrelated", "adapter_device_id": "other", "fiscal_device_id": "physical"})
	put("register", "stale", "tenant", map[string]any{"adapter_device_id": "adapter", "fiscal_device_id": "physical"})
	put("register", "foreign", "foreign", map[string]any{"adapter_device_id": "adapter"})
	r.bleSessions["adapter"] = BLESessionRecord{TenantID: "tenant", DeviceID: "adapter"}
	r.bleSessions["route"] = BLESessionRecord{TenantID: "tenant", RegisterID: "reg", FiscalDeviceID: "physical"}
	r.bleSessions["other"] = BLESessionRecord{TenantID: "tenant", RegisterID: "reg2", DeviceID: "other"}
	r.bleSessions["foreign"] = BLESessionRecord{TenantID: "foreign", DeviceID: "adapter"}
	if _, err := r.RevokeActivatedDevice("adapter", "tenant", "admin", now); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"active", "pending"} {
		b, _ := r.Resource("composite_binding", id)
		if stringField(b.Data, "status") != "REVOKED" || b.Version != 2 {
			t.Fatalf("binding %s: %+v", id, b)
		}
	}
	for _, id := range []string{"reg", "stale"} {
		v, _ := r.Resource("register", id)
		for _, field := range []string{"composite_binding_id", "adapter_device_id", "fiscal_device_id", "payment_terminal_id", "device_profile"} {
			if stringField(v.Data, field) != "" {
				t.Fatalf("route %s retains %s", id, field)
			}
		}
	}
	v, _ := r.Resource("register", "reg")
	if stringField(v.Data, "location_id") != "location" {
		t.Fatal("unrelated register data removed")
	}
	for _, id := range []string{"reg2", "foreign"} {
		v, _ := r.Resource("register", id)
		if v.Version != 1 {
			t.Fatalf("unrelated route changed: %s", id)
		}
	}
	physical, _ := r.Resource("device", "physical")
	if stringField(physical.Data, "status") != "ACTIVE" {
		t.Fatal("physical resource revoked")
	}
	issued, _ := r.Resource("issued_binding", "hash")
	if issued.Version != 1 || stringField(issued.Data, "envelope") != "immutable" {
		t.Fatal("issued proof mutated")
	}
	if !r.bleSessions["adapter"].Revoked || !r.bleSessions["route"].Revoked || r.bleSessions["other"].Revoked || r.bleSessions["foreign"].Revoked {
		t.Fatal("session revocation scope")
	}
	before := v.Version
	if _, err := r.RevokeActivatedDevice("adapter", "tenant", "admin", now); err != nil {
		t.Fatal(err)
	}
	v, _ = r.Resource("register", "reg")
	if v.Version != before {
		t.Fatal("idempotent revocation mutated route")
	}
}
