package domain

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestDelegatedOnlinePreparationFreezesActualSaleWithoutMoneyIO(t *testing.T) {
	f, p, in := delegatedFixture(t)
	if _, err := f.svc.SyncWorkstationClock("register-1", p.TenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.RefreshReadiness("register-1", p.TenantID); err != nil {
		t.Fatal(err)
	}
	in.SalePreparation = &DelegatedSalePreparation{SourceOrderID: "minipos-order", SourceShiftID: "minipos-shift", Items: []SaleLine{{LineID: "30000000-0000-4000-8000-000000000001", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}}, Payments: []PaymentRequest{{PaymentID: "20000000-0000-4000-8000-000000000001", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}}}, ExpectedTotal: Money{Amount: "2.50", Currency: "EUR"}}
	first, err := f.svc.IssueDelegatedExecutionAuthorization(p, in, "prepared")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(first.Authorization.Payload)
	var auth ExecutionAuthorization
	if json.Unmarshal(raw, &auth) != nil || auth.UNPAuthority == nil {
		t.Fatal("no server authority")
	}
	if auth.UNPAuthority.ServerSaleID == in.SalePreparation.SourceOrderID || auth.UNPAuthority.PayloadSHA256 != edgeDigest(first.CanonicalPayload) {
		t.Fatal("caller ID or mutable plan")
	}
	sale, err := f.repo.Sale(auth.UNPAuthority.ServerSaleID)
	if err != nil || sale.State != "PAYMENT_PENDING" || len(sale.Payments) != 1 || sale.FiscalOperationID != auth.UNPAuthority.OperationID {
		t.Fatal("actual sale not frozen", err)
	}
	if len(f.repo.Resources("device_command_outbox", p.TenantID)) != 0 {
		t.Fatal("money command automatically published")
	}
	retry, err := f.svc.IssueDelegatedExecutionAuthorization(p, in, "prepared")
	if err != nil || retry.Authorization != first.Authorization || len(f.repo.Sales(p.TenantID)) != 1 {
		t.Fatal("preparation retry changed receipt", err)
	}
	in.SalePreparation.Items[0].Name = "Changed"
	if _, err = f.svc.IssueDelegatedExecutionAuthorization(p, in, "other-key"); err == nil {
		t.Fatal("changed order reused prepared authority")
	}
}
