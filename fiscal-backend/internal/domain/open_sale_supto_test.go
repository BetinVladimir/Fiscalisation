package domain

import "testing"

func TestOpenSaleWithFirstLineUsesVerifiedFMINAtomically(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	registerID, _ := prepareBLERegister(t, svc, "tenant-supto")
	ws, err := svc.OpenWorkstationSession(registerID, "A001", "app-instance", "actor", "tenant-supto")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenShift(registerID, "A001", "tenant-supto"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SyncWorkstationClock(registerID, "tenant-supto"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RefreshReadiness(registerID, "tenant-supto"); err != nil {
		t.Fatal(err)
	}
	line := SaleLine{LineID: "550e8400-e29b-41d4-a716-446655440001", Name: "Кафе", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}
	sale, err := svc.OpenSaleWithFirstLine(OpenSaleWithFirstLineRequest{TenantID: "tenant-supto", ClientSaleSurrogateID: "550e8400-e29b-41d4-a716-446655440000", WorkstationID: registerID, OperatorSessionID: ws.SessionID, Line: line})
	if err != nil {
		t.Fatal(err)
	}
	if sale.State != "OPEN" || len(sale.Lines) != 1 || sale.UNP != "BL000001-A001-0000001" {
		t.Fatalf("non-atomic or wrong authority: %#v", sale)
	}
	if len(sale.RegulatoryIdentifiers) != 1 || sale.RegulatoryIdentifiers[0].Value != sale.UNP {
		t.Fatalf("binding projection missing: %#v", sale.RegulatoryIdentifiers)
	}
	if events := repo.AuditEvents("tenant-supto"); len(events) == 0 || events[len(events)-1].Action != "SALE_OPENED" {
		t.Fatalf("SALE_OPENED audit missing: %#v", events)
	}
}

func TestOpenSaleWithFirstLineFailureLeavesNoSaleOrUNP(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	registerID, _ := prepareBLERegister(t, svc, "tenant-supto-fail")
	ws, err := svc.OpenWorkstationSession(registerID, "A001", "app-instance", "actor", "tenant-supto-fail")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SyncWorkstationClock(registerID, "tenant-supto-fail"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RefreshReadiness(registerID, "tenant-supto-fail"); err != nil {
		t.Fatal(err)
	}
	line := SaleLine{LineID: "line", Name: "Кафе", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}
	if _, err := svc.OpenSaleWithFirstLine(OpenSaleWithFirstLineRequest{TenantID: "tenant-supto-fail", ClientSaleSurrogateID: "surrogate", WorkstationID: registerID, OperatorSessionID: ws.SessionID, Line: line}); err == nil {
		t.Fatal("sale opened without an open shift")
	}
	if got := repo.Sales("tenant-supto-fail"); len(got) != 0 {
		t.Fatalf("failed open leaked sale: %#v", got)
	}
}

func TestSUPTOPaymentPerformsFreshReadiness(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	registerID, _ := prepareBLERegister(t, svc, "tenant-supto-payment")
	ws, err := svc.OpenWorkstationSession(registerID, "A001", "app-instance", "actor", "tenant-supto-payment")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.OpenShift(registerID, "A001", "tenant-supto-payment"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SyncWorkstationClock(registerID, "tenant-supto-payment"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RefreshReadiness(registerID, "tenant-supto-payment"); err != nil {
		t.Fatal(err)
	}
	line := SaleLine{LineID: "line-1", Name: "Кафе", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}
	sale, err := svc.OpenSaleWithFirstLine(OpenSaleWithFirstLineRequest{TenantID: "tenant-supto-payment", ClientSaleSurrogateID: "sale-surrogate", WorkstationID: registerID, OperatorSessionID: ws.SessionID, Line: line})
	if err != nil {
		t.Fatal(err)
	}
	before := len(repo.Resources("readiness_lease", "tenant-supto-payment"))
	op, err := svc.PayForTenant(sale.ID, PaymentRequest{PaymentID: "payment-1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}}, "tenant-supto-payment")
	if err != nil || op.State != "FISCALIZED" {
		t.Fatal(op, err)
	}
	if len(repo.Resources("readiness_lease", "tenant-supto-payment")) != before+1 {
		t.Fatal("payment did not perform a fresh readiness probe")
	}
}
