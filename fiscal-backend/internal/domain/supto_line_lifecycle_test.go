package domain

import (
	"encoding/json"
	"testing"
)

func openSUPTOSaleForLineTest(t *testing.T) (*Service, *MemoryRepository, Sale) {
	t.Helper()
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	register, _ := prepareBLERegister(t, svc, "tenant-lines")
	ws, e := svc.OpenWorkstationSession(register, "A001", "app-instance", "actor", "tenant-lines")
	if e != nil {
		t.Fatal(e)
	}
	if _, e := svc.OpenShift(register, "A001", "tenant-lines"); e != nil {
		t.Fatal(e)
	}
	if _, e := svc.SyncWorkstationClock(register, "tenant-lines"); e != nil {
		t.Fatal(e)
	}
	if _, e := svc.RefreshReadiness(register, "tenant-lines"); e != nil {
		t.Fatal(e)
	}
	line := SaleLine{LineID: "line-1", Name: "Кафе", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}
	sale, e := svc.OpenSaleWithFirstLine(OpenSaleWithFirstLineRequest{TenantID: "tenant-lines", ClientSaleSurrogateID: "surrogate-lines", WorkstationID: register, OperatorSessionID: ws.SessionID, Line: line})
	if e != nil {
		t.Fatal(e)
	}
	return svc, repo, sale
}

func TestSUPTOLineChangesAreVersionedAuditedAndTotaled(t *testing.T) {
	svc, repo, sale := openSUPTOSaleForLineTest(t)
	replacement := sale.Lines[0]
	replacement.Quantity = "2.000"
	changed, e := svc.ChangeLineExpectedForTenant(sale.ID, "line-1", sale.Version, replacement, sale.TenantID)
	if e != nil || changed.Version != 2 {
		t.Fatal(changed, e)
	}
	if _, e = svc.ChangeLineExpectedForTenant(sale.ID, "line-1", sale.Version, replacement, sale.TenantID); e == nil {
		t.Fatal("stale change accepted")
	}
	b, e := json.Marshal(changed)
	if e != nil || !json.Valid(b) || !containsBytes(b, []byte(`"amount":"5.00"`)) {
		t.Fatalf("authoritative total missing: %s %v", b, e)
	}
	events := repo.AuditEvents(sale.TenantID)
	if events[len(events)-1].Action != "SALE_LINE_CHANGED" {
		t.Fatalf("change audit missing: %#v", events)
	}
	cancelled, e := svc.CancelLineExpectedForTenant(changed.ID, "line-1", changed.Version, changed.TenantID)
	if e != nil || len(cancelled.Lines) != 0 || cancelled.UNP != sale.UNP {
		t.Fatal(cancelled, e)
	}
	events = repo.AuditEvents(sale.TenantID)
	if events[len(events)-1].Action != "SALE_LINE_CANCELLED" {
		t.Fatalf("cancel audit missing: %#v", events)
	}
}

func containsBytes(body, part []byte) bool {
	for i := 0; i+len(part) <= len(body); i++ {
		ok := true
		for j := range part {
			if body[i+j] != part[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func TestSUPTOSaleRecoveryFilters(t *testing.T) {
	svc, _, sale := openSUPTOSaleForLineTest(t)
	if got := svc.SalesForTenant(sale.TenantID, "A001", sale.RegisterID, "OPEN"); len(got) != 1 || got[0].ID != sale.ID {
		t.Fatalf("recovery failed: %#v", got)
	}
	if got := svc.SalesForTenant(sale.TenantID, "B002", "", "OPEN"); len(got) != 0 {
		t.Fatalf("foreign operator leaked: %#v", got)
	}
}
