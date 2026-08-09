package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReportArtifactPersistenceAndTenantIsolation(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(repo, NewSimulator(true))
	registerID, _ := prepareBLERegister(t, s, "tenant-1")
	op, err := s.CreateReport(registerID, "Z", "tenant-1")
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
	pending := s.PendingOutbox(time.Now().UTC().Add(time.Second))
	if len(pending) != 2 {
		t.Fatalf("report operation and completion webhooks were not committed atomically: %+v", pending)
	}
	types := map[string]bool{}
	for _, item := range pending {
		types[item.Event.EventType] = true
	}
	if !types["fiscal.operation.updated"] || !types["register.report.completed"] {
		t.Fatalf("report webhook types incomplete: %+v", types)
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

func TestReportIsNotPublishedForUnknownDeviceResult(t *testing.T) {
	repo := NewMemoryRepository()
	driver := NewSimulator(true)
	driver.SetOutcomeUnknown(true)
	s := NewService(repo, driver)
	op, err := s.CreateReport("register-1", "Z", "")
	if err != nil || op.State != "UNKNOWN" {
		t.Fatal(op, err)
	}
	if len(repo.Resources("report", "")) != 0 || len(repo.artifacts) != 0 {
		t.Fatal("ambiguous device result published a completed report")
	}
}

func TestReportFinalPublicationIsAtomicAndRestartRecoversUnknown(t *testing.T) {
	store := &failNextStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(repo, NewSimulator(true))
	registerID, _ := prepareBLERegister(t, s, "tenant-1")
	s.driver = commitFailureDriver{store: store}
	if _, err = s.CreateReport(registerID, "Z", "tenant-1"); err == nil {
		t.Fatal("injected report publication failure was ignored")
	}
	if len(repo.Resources("report", "tenant-1")) != 0 || len(repo.artifacts) != 0 || len(repo.outbox) != 0 {
		t.Fatal("failed report publication leaked resource or artifact")
	}
	operations := repo.Operations()
	if len(operations) != 1 || operations[0].State != "EXECUTING" {
		t.Fatalf("durable report reservation was not retained: %+v", operations)
	}
	restarted, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	recovered, _ := restarted.Operation(operations[0].ID)
	if recovered.State != "UNKNOWN" || len(recovered.AllowedActions) != 1 || recovered.AllowedActions[0] != "RECONCILE" {
		t.Fatalf("interrupted report did not recover fail-closed: %+v", recovered)
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
