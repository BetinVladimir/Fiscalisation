package domain

import (
	"encoding/json"
	"testing"
)

func TestReportArtifactPersistenceAndTenantIsolation(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(repo, NewSimulator(true))
	op, err := s.CreateReport("register-1", "Z", "tenant-1")
	if err != nil || op.State != "FISCALIZED" {
		t.Fatal(op, err)
	}
	reports := s.Reports("tenant-1")
	if len(reports) != 1 {
		t.Fatal(reports)
	}
	manifest := reports[0]["artifacts"].([]any)[0].(map[string]any)
	artifactID := manifest["artifact_id"].(string)
	reportID := reports[0]["id"].(string)
	b, err := s.ReportArtifact(reportID, artifactID, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if json.Unmarshal(b, &payload) != nil || payload["type"] != "Z" {
		t.Fatal(string(b))
	}
	if _, err = s.ReportArtifact(reportID, artifactID, "tenant-2"); err == nil {
		t.Fatal("cross tenant artifact leaked")
	}
	repo, err = NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s = NewService(repo, NewSimulator(true))
	if _, err = s.ReportArtifact(reportID, artifactID, "tenant-1"); err != nil {
		t.Fatal("artifact did not survive restart", err)
	}
}

func TestAuditChainIsAppendOnlyAndPersistent(t *testing.T) {
	store := &testStore{}
	repo, _ := NewPersistentRepository(store)
	s := NewService(repo, NewSimulator(true))
	location, err := s.CreateResource("location", "tenant-1", map[string]any{"code": "SOF", "name": "Sofia", "address": "1 Main", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.UpdateResource("location", location["id"].(string), "tenant-1", 1, map[string]any{"code": "SOF", "name": "Sofia 2", "address": "1 Main", "status": "ACTIVE"})
	if err != nil {
		t.Fatal(err)
	}
	events := s.AuditEvents("tenant-1")
	if len(events) != 2 || events[0].EventHash == "" || events[1].PrevHash != events[0].EventHash {
		t.Fatalf("broken chain: %+v", events)
	}
	repo, _ = NewPersistentRepository(store)
	if len(repo.AuditEvents("tenant-1")) != 2 {
		t.Fatal("audit did not survive restart")
	}
}
