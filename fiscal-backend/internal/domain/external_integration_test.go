package domain

import "testing"

func TestApplyExternalResourceProjectsAndDeduplicates(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	loc, e := s.ApplyExternalResource("tenant-a", "system-a", "PUT", "location", "loc-1", 1, map[string]any{"name": "Store", "address": "Street", "status": "ACTIVE"})
	if e != nil {
		t.Fatal(e)
	}
	replay, e := s.ApplyExternalResource("tenant-a", "system-a", "PUT", "location", "loc-1", 1, map[string]any{"name": "Ignored", "address": "Other"})
	if e != nil || replay["id"] != loc["id"] || replay["name"] != "Store" {
		t.Fatalf("non-idempotent replay: %#v %v", replay, e)
	}
	reg, e := s.ApplyExternalResource("tenant-a", "system-a", "PUT", "register", "reg-1", 1, map[string]any{"name": "POS 1", "location_source_id": "loc-1", "status": "ACTIVE"})
	if e != nil {
		t.Fatal(e)
	}
	if reg["location_id"] != loc["id"] {
		t.Fatalf("register did not resolve source location: %#v", reg)
	}
}

func TestApplyExternalResourceRejectsMissingDependency(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	if _, e := s.ApplyExternalResource("tenant-a", "system-a", "PUT", "register", "reg-1", 1, map[string]any{"name": "POS", "location_source_id": "missing"}); e == nil {
		t.Fatal("missing location accepted")
	}
}
