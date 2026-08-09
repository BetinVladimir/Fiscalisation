package domain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func TestTenantShiftCloseRequiresFinalZReport(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/registers/00000000-0000-4000-8000-000000000001/reports" || r.Method != http.MethodPost {
			t.Fatalf("unexpected fiscal request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["type"] != "Z" {
			t.Fatal("shift close did not request Z report")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"operation_id": "z-operation-1", "state": "FISCALIZED", "fiscal_reference": "Z-000001"})
	}))
	defer server.Close()
	s := NewService(server.URL, "2026-08-07")
	employee, err := s.CreateEmployee(Employee{TenantID: "tenant-1", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	shift, err := s.OpenShiftForTenant("00000000-0000-4000-8000-000000000001", employee.ID, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	closed, err := s.CloseShiftForTenant(shift.ID, "tenant-1")
	if err != nil || closed.State != "CLOSED" || closed.ZOperationID != "z-operation-1" || closed.ZFiscalReference != "Z-000001" || closed.ClosedAt == nil || requests != 1 {
		t.Fatalf("close=%+v requests=%d err=%v", closed, requests, err)
	}
}

func TestOpenShiftRejectsNonUUIDFiscalRegister(t *testing.T) {
	s := NewService("http://invalid", "2026-08-07")
	employee, err := s.CreateEmployee(Employee{TenantID: "tenant-1", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.OpenShiftForTenant("FD000001", employee.ID, "tenant-1"); err == nil {
		t.Fatal("non-UUID Fiscal register reached shift domain")
	}
}

func TestConcurrentShiftCloseEmitsExactlyOneZCommand(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			close(entered)
			<-release
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"operation_id": "z-operation-1", "state": "FISCALIZED", "fiscal_reference": "Z-000001"})
	}))
	defer server.Close()
	s := NewService(server.URL, "2026-08-07")
	employee, err := s.CreateEmployee(Employee{TenantID: "tenant-1", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	shift, err := s.OpenShiftForTenant("00000000-0000-4000-8000-000000000001", employee.ID, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	go func() { defer wg.Done(); _, closeErr := s.CloseShiftForTenant(shift.ID, "tenant-1"); errs <- closeErr }()
	<-entered
	go func() { defer wg.Done(); _, closeErr := s.CloseShiftForTenant(shift.ID, "tenant-1"); errs <- closeErr }()
	close(release)
	wg.Wait()
	close(errs)
	successes := 0
	for closeErr := range errs {
		if closeErr == nil {
			successes++
		}
	}
	if requests.Load() != 1 || successes != 1 {
		t.Fatalf("concurrent close emitted %d Z commands with %d successes", requests.Load(), successes)
	}
}

func TestAmbiguousZReportBlocksShiftAndNextShift(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "timeout", http.StatusGatewayTimeout) }))
	defer server.Close()
	s := NewService(server.URL, "2026-08-07")
	employee, err := s.CreateEmployee(Employee{TenantID: "tenant-1", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	shift, err := s.OpenShiftForTenant("00000000-0000-4000-8000-000000000001", employee.ID, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CloseShiftForTenant(shift.ID, "tenant-1"); err == nil {
		t.Fatal("ambiguous Z report closed shift")
	}
	blocked, err := s.ShiftForTenant(shift.ID, "tenant-1")
	if err != nil || blocked.State != "BLOCKED_RECONCILIATION" || blocked.CloseError == "" || blocked.ClosedAt != nil {
		t.Fatalf("blocked=%+v err=%v", blocked, err)
	}
	if _, err = s.OpenShiftForTenant("00000000-0000-4000-8000-000000000001", employee.ID, "tenant-1"); err == nil {
		t.Fatal("new shift opened over blocked Z reconciliation")
	}
}

func TestBlockedZCloseReconcilesByReadingOriginalOperationOnly(t *testing.T) {
	postRequests, getRequests := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/registers/00000000-0000-4000-8000-000000000001/reports":
			postRequests++
			_ = json.NewEncoder(w).Encode(map[string]any{"operation_id": "z-operation-unknown", "state": "UNKNOWN"})
		case r.Method == http.MethodGet && r.URL.Path == "/operations/z-operation-unknown":
			getRequests++
			if r.ContentLength > 0 {
				t.Fatal("GET reconciliation sent a request body")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"operation_id": "z-operation-unknown", "state": "FISCALIZED", "fiscal_reference": "Z-RECOVERED-1"})
		default:
			t.Fatalf("unexpected fiscal request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	s := NewService(server.URL, "2026-08-07")
	employee, err := s.CreateEmployee(Employee{TenantID: "tenant-1", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	shift, err := s.OpenShiftForTenant("00000000-0000-4000-8000-000000000001", employee.ID, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CloseShiftForTenant(shift.ID, "tenant-1"); err == nil {
		t.Fatal("UNKNOWN Z result closed shift")
	}
	blocked, err := s.ShiftForTenant(shift.ID, "tenant-1")
	if err != nil || blocked.State != "BLOCKED_RECONCILIATION" || blocked.ZOperationID != "z-operation-unknown" || len(blocked.AllowedActions) != 2 || blocked.AllowedActions[1] != "RECONCILE" {
		t.Fatalf("blocked shift does not expose reconciliation: %+v err=%v", blocked, err)
	}
	closed, err := s.ReconcileShiftCloseForTenant(shift.ID, "tenant-1")
	if err != nil || closed.State != "CLOSED" || closed.ZFiscalReference != "Z-RECOVERED-1" || closed.ClosedAt == nil || len(closed.AllowedActions) != 0 {
		t.Fatalf("reconciled close=%+v err=%v", closed, err)
	}
	if postRequests != 1 || getRequests != 1 {
		t.Fatalf("reconciliation repeated Z command: POST=%d GET=%d", postRequests, getRequests)
	}
}
