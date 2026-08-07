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
	if _, err = s.BindRegister(register["id"].(string), "tenant-1", device["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
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

func TestProductionDeviceCannotBeSimulated(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	_, err := s.CreateResource("device", "tenant-1", map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Daisy", "model": "Compact", "serial": "S1", "status": "ACTIVE", "environment": "PROD", "simulated": true})
	if err == nil {
		t.Fatal("simulated production device accepted")
	}
}
