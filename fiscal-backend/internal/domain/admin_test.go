package domain

import "testing"

func TestAdminResourcesTenantVersionAndBinding(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(repo, NewSimulator(true))
	org, err := s.UpsertOrganization("tenant-1", 0, map[string]any{"legal_name": "Bee Ltd", "eik": "123456789", "country": "BG", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	if org["version"] != int64(1) {
		t.Fatal(org)
	}
	location, err := s.CreateResource("location", "tenant-1", map[string]any{"code": "SOF", "name": "Sofia", "address": "1 Main", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	register, err := s.CreateResource("register", "tenant-1", map[string]any{"location_id": location["id"], "code": "R01", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	device, err := s.CreateResource("device", "tenant-1", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "SN1", "status": "DRAFT", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	device = activateTestDevice(t, s, "tenant-1", device)
	if _, err = s.BindRegister(register["id"].(string), "tenant-1", device["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
	}
	bound, err := s.GetResource("register", register["id"].(string), "tenant-1")
	if err != nil || bound["fiscal_device_active_from"] == "" {
		t.Fatal("binding activation time was not persisted", bound, err)
	}
	activeFrom := bound["fiscal_device_active_from"]
	updatedRegister, err := s.UpdateResource("register", register["id"].(string), "tenant-1", bound["version"].(int64), map[string]any{"location_id": location["id"], "code": "R01", "status": "ACTIVE", "fiscal_device_id": device["id"]})
	if err != nil || updatedRegister["fiscal_device_active_from"] != activeFrom {
		t.Fatal("unchanged binding activation time was not preserved", updatedRegister, err)
	}
	if _, err = s.GetResource("device", device["id"].(string), "tenant-2"); err == nil {
		t.Fatal("cross tenant device leaked")
	}
	if _, err = s.UpdateResource("location", location["id"].(string), "tenant-1", 99, map[string]any{"code": "SOF", "name": "Changed", "address": "1 Main", "status": "ACTIVE"}); err == nil {
		t.Fatal("stale version accepted")
	}
	repo, err = NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s = NewService(repo, NewSimulator(true))
	got, err := s.GetResource("register", register["id"].(string), "tenant-1")
	if err != nil || got["fiscal_device_id"] != device["id"] {
		t.Fatal(got, err)
	}
}

func TestRegisterCannotBypassBindingWithInvalidDeviceReference(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	location, err := s.CreateResource("location", "tenant-1", map[string]any{"code": "SOF", "name": "Sofia", "address": "1 Main", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateResource("register", "tenant-1", map[string]any{"location_id": location["id"], "code": "R01", "status": "ACTIVE", "payment_terminal_id": "00000000-0000-4000-8000-000000000099"}); err == nil {
		t.Fatal("register accepted an unknown payment terminal reference")
	}
}

func TestProductionDeviceCannotBeSimulated(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	_, err := s.CreateResource("device", "tenant-1", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Daisy", "model": "Compact", "serial": "S1", "status": "DRAFT", "environment": "PROD", "simulated": true})
	if err == nil {
		t.Fatal("simulated production device accepted")
	}
}

func TestDeviceActivationAndBindingFailClosed(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	if _, err := s.CreateResource("device", "tenant-1", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "DIRECT-ACTIVE", "status": "ACTIVE", "environment": "DEV", "simulated": true}); err == nil {
		t.Fatal("device was created directly ACTIVE")
	}
	location, _ := s.CreateResource("location", "tenant-1", map[string]any{"code": "SOF", "name": "Sofia", "address": "1 Main", "status": "ACTIVE"})
	register, _ := s.CreateResource("register", "tenant-1", map[string]any{"location_id": location["id"], "code": "R01", "status": "ACTIVE"})
	draft, err := s.CreateResource("device", "tenant-1", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "DRAFT-1", "status": "DRAFT", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindRegister(register["id"].(string), "tenant-1", draft["id"].(string), "FISCAL_DEVICE", ""); err == nil {
		t.Fatal("DRAFT device was bound")
	}
	if _, err = s.UpdateResource("device", draft["id"].(string), "tenant-1", 1, map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "DRAFT-1", "status": "ACTIVE", "environment": "DEV", "simulated": true}); err == nil {
		t.Fatal("DRAFT skipped service activation")
	}
	pending, err := s.UpdateResource("device", draft["id"].(string), "tenant-1", 1, map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "DRAFT-1", "status": "PENDING_SERVICE_ACTIVATION", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	active, err := s.UpdateResource("device", draft["id"].(string), "tenant-1", pending["version"].(int64), map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "DRAFT-1", "status": "ACTIVE", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.BindRegister(register["id"].(string), "tenant-1", active["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ProvisioningSession(active["id"].(string), "tenant-1"); err == nil {
		t.Fatal("ACTIVE device was reprovisioned")
	}
}

func TestProductionActivationRequiresEvidence(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	base := map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Daisy", "model": "Compact", "serial": "PROD-1", "status": "DRAFT", "environment": "PROD", "simulated": false}
	draft, err := s.CreateResource("device", "tenant-1", base)
	if err != nil {
		t.Fatal(err)
	}
	base["status"] = "PENDING_SERVICE_ACTIVATION"
	pending, err := s.UpdateResource("device", draft["id"].(string), "tenant-1", draft["version"].(int64), base)
	if err != nil {
		t.Fatal(err)
	}
	base["status"] = "ACTIVE"
	if _, err = s.UpdateResource("device", draft["id"].(string), "tenant-1", pending["version"].(int64), base); err == nil {
		t.Fatal("active PROD device without evidence accepted")
	}
	base["approved_type_evidence_id"] = "BIM-2026-001"
	base["service_contract_evidence_id"] = "SERVICE-2026-001"
	if _, err = s.UpdateResource("device", draft["id"].(string), "tenant-1", pending["version"].(int64), base); err != nil {
		t.Fatal(err)
	}
}

func TestProvisioningSessionIsTenantBoundAndSurvivesRestart(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(repo, NewSimulator(true))
	device, err := s.CreateResource("device", "tenant-1", map[string]any{"kind": "EDGE", "vendor": "Beeloy", "model": "ESP32-S3", "serial": "EDGE-1", "status": "DRAFT", "environment": "DEV", "simulated": false})
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.ProvisioningSession(device["id"].(string), "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	id := session["session_id"].(string)
	repo, err = NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	record, err := repo.Resource("provisioning_session", id)
	if err != nil || record.TenantID != "tenant-1" || stringField(record.Data, "device_id") != device["id"] || stringField(record.Data, "state") != "CREATED" {
		t.Fatal(record, err)
	}
	if _, err = NewService(repo, NewSimulator(true)).GetResource("provisioning_session", id, "tenant-2"); err == nil {
		t.Fatal("cross-tenant provisioning session leaked")
	}
}
