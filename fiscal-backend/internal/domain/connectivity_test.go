package domain

import "testing"

type countingDriver struct{ executions int }

func (d *countingDriver) Execute(Operation, Sale, PaymentRequest) (string, string) {
	d.executions++
	return "COUNTED", ""
}
func (d *countingDriver) Probe() error { return nil }

func TestConnectivityProbePersistsAndBlocksWhenFiscalDeviceLost(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(repo, NewSimulator(false))
	location, _ := svc.CreateResource("location", "tenant-1", map[string]any{"code": "SOF", "name": "Sofia", "address": "1 Main", "status": "ACTIVE"})
	register, _ := svc.CreateResource("register", "tenant-1", map[string]any{"location_id": location["id"], "code": "R01", "status": "ACTIVE"})
	device, _ := svc.CreateResource("device", "tenant-1", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "SN-CONNECT", "status": "DRAFT", "environment": "DEV", "simulated": true})
	device = activateTestDevice(t, svc, "tenant-1", device)
	if _, err = svc.BindRegister(register["id"].(string), "tenant-1", device["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
	}
	probe, err := svc.Connectivity(register["id"].(string), "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if probe.State != "FAILED" || probe.RecommendedTransport != "BLOCK" || probe.Hops["fiscal_device"]["state"] != "UNAVAILABLE" {
		t.Fatalf("lost fiscal device must block, got %+v", probe)
	}
	repo, err = NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	svc = NewService(repo, NewSimulator(true))
	if _, err = svc.GetConnectivityProbe(probe.ProbeID, "tenant-2"); err == nil {
		t.Fatal("cross-tenant probe leaked")
	}
	got, err := svc.GetConnectivityProbe(probe.ProbeID, "tenant-1")
	if err != nil || got.ProbeID != probe.ProbeID {
		t.Fatal(got, err)
	}
}

func TestFiscalExecutionNeverReachesDriverWithoutActiveTenantBinding(t *testing.T) {
	driver := &countingDriver{}
	svc := NewService(NewMemoryRepository(), driver)
	registerID, deviceID := prepareBLERegister(t, svc, "tenant-1")
	if _, err := svc.CreateResource("operator", "tenant-1", map[string]any{"code": "B002", "first_name": "Future", "last_name": "Cashier", "roles": []any{"CASHIER"}, "active_from": "2099-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateResource("operator", "tenant-1", map[string]any{"code": "C003", "first_name": "Former", "last_name": "Cashier", "roles": []any{"CASHIER"}, "active_from": "2026-01-01T00:00:00Z", "active_to": "2026-02-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateSale(CreateSale{TenantID: "tenant-2", ExternalID: "foreign", RegisterID: registerID, OperatorID: "A001"}); err == nil {
		t.Fatal("foreign tenant created a sale on the register")
	}
	if _, err := svc.CreateSale(CreateSale{TenantID: "tenant-1", ExternalID: "future-operator", RegisterID: registerID, OperatorID: "B002"}); err == nil {
		t.Fatal("not-yet-active operator created a sale")
	}
	if _, err := svc.OpenShift(registerID, "C003", "tenant-1"); err == nil {
		t.Fatal("expired operator opened a shift")
	}
	sale, err := svc.CreateSale(CreateSale{TenantID: "tenant-1", ExternalID: "blocked-before-pay", RegisterID: registerID, OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	sale, err = svc.AddLineForTenant(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B"}, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	device, err := svc.GetResource("device", deviceID, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	device["status"] = "BLOCKED"
	if _, err = svc.UpdateResource("device", deviceID, "tenant-1", device["version"].(int64), device); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.PayForTenant(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "1.00", Currency: "EUR"}}, "tenant-1"); err == nil {
		t.Fatal("payment executed with blocked fiscal device")
	}
	if _, err = svc.FiscalOperation(registerID, "X", "tenant-1"); err == nil {
		t.Fatal("report command executed with blocked fiscal device")
	}
	if _, err = svc.OpenShift(registerID, "A001", "tenant-1"); err == nil {
		t.Fatal("shift opened with blocked fiscal device")
	}
	if driver.executions != 0 {
		t.Fatalf("driver called %d times after registry gate failure", driver.executions)
	}
}

var _ Driver = (*countingDriver)(nil)

func TestReadinessAndConnectivityRequireActiveTenantBinding(t *testing.T) {
	svc := NewService(NewMemoryRepository(), NewSimulator(true))
	if ready := svc.ReadinessForTenant("missing", "tenant-1"); ready["ready"] != false || ready["recommended_transport"] != "BLOCK" {
		t.Fatal("unknown device reported ready", ready)
	}
	location, _ := svc.CreateResource("location", "tenant-1", map[string]any{"code": "SOF", "name": "Sofia", "address": "1 Main", "status": "ACTIVE"})
	register, _ := svc.CreateResource("register", "tenant-1", map[string]any{"location_id": location["id"], "code": "R01", "status": "ACTIVE"})
	device, _ := svc.CreateResource("device", "tenant-1", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Daisy", "model": "Compact", "serial": "SN-READY", "status": "DRAFT", "environment": "DEV", "simulated": true})
	device = activateTestDevice(t, svc, "tenant-1", device)
	if ready := svc.ReadinessForTenant(device["id"].(string), "tenant-2"); ready["ready"] != false {
		t.Fatal("foreign device reported ready", ready)
	}
	probe, err := svc.Connectivity(register["id"].(string), "tenant-1")
	if err != nil || probe.RecommendedTransport != "BLOCK" {
		t.Fatal("unbound register reported ready", probe, err)
	}
	if _, err = svc.BindRegister(register["id"].(string), "tenant-1", device["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
	}
	if ready := svc.ReadinessForTenant(device["id"].(string), "tenant-1"); ready["ready"] != true || ready["recommended_transport"] != "REST" {
		t.Fatal("active tenant device not ready", ready)
	}
	blockedData := map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Daisy", "model": "Compact", "serial": "SN-READY", "status": "BLOCKED", "environment": "DEV", "simulated": true}
	if _, err = svc.UpdateResource("device", device["id"].(string), "tenant-1", device["version"].(int64), blockedData); err != nil {
		t.Fatal(err)
	}
	if ready := svc.ReadinessForTenant(device["id"].(string), "tenant-1"); ready["ready"] != false || ready["recommended_transport"] != "BLOCK" {
		t.Fatal("blocked device reported ready", ready)
	}
}
