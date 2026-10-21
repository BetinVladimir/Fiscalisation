package domain

import (
	"encoding/json"
	"os"
	"testing"
)

// The SQL regression consumes the exact event emitted by the real service and
// repository, including actual resource IDs and the current Sale JSON shape.
func TestTypedProjectionDomainFixture(t *testing.T) {
	path := os.Getenv("TYPED_PROJECTION_FIXTURE")
	if path == "" {
		t.Skip("TYPED_PROJECTION_FIXTURE not set")
	}
	tenant := "10000000-0000-4000-8000-000000000001"
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	register, _ := prepareBLERegister(t, svc, tenant)
	ws, err := svc.OpenWorkstationSession(register, "A001", "projection-fixture-app", "actor", tenant)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.OpenShift(register, "A001", tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.SyncWorkstationClock(register, tenant); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.RefreshReadiness(register, tenant); err != nil {
		t.Fatal(err)
	}
	sale, err := svc.OpenSaleWithFirstLine(OpenSaleWithFirstLineRequest{TenantID: tenant, ClientSaleSurrogateID: "surrogate-opaque", WorkstationID: register, OperatorSessionID: ws.SessionID, Line: SaleLine{LineID: "line-opaque", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}})
	if err != nil {
		t.Fatal(err)
	}
	events := repo.AuditEvents(tenant)
	a := events[len(events)-1]
	if a.Action != "SALE_OPENED" || a.ObjectID != sale.ID || sale.OperatorCode != "A001" {
		t.Fatalf("unexpected event or snapshot: %#v", a)
	}
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
