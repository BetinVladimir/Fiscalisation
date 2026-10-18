package domain

import (
	"testing"
)

func TestDurableFinalizePreservesTenderPlanAndOperationLink(t *testing.T) {
	repo := NewMemoryRepository()
	driver := &durableQueueDriver{repo: repo}
	svc := NewService(repo, driver)
	sale := Sale{ID: "sale-finalize", State: "OPEN", UNP: "12345678-A001-0000001", RegisterID: "register-1", Lines: []SaleLine{{LineID: "line-1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}}, Version: 1}
	if err := repo.PutSale(sale); err != nil {
		t.Fatal(err)
	}
	request := SaleFinalizeRequest{ClientOperationID: "00000000-0000-4000-8000-000000000101", ReceiptSessionID: "00000000-0000-4000-8000-000000000102", Payments: []PaymentRequest{{PaymentID: "00000000-0000-4000-8000-000000000103", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}}}, ExpectedTotal: Money{Amount: "2.50", Currency: "EUR"}}
	op, err := svc.FinalizeSaleForTenant(sale.ID, request, "")
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := repo.Sale(sale.ID)
	if len(saved.Payments) != 1 || saved.Payments[0].PaymentID != request.Payments[0].PaymentID || saved.FiscalOperationID != op.ID {
		t.Fatalf("durable receipt plan lost: payments=%+v operation=%q", saved.Payments, saved.FiscalOperationID)
	}
}

func TestPreparedFinalizeHasNoDeviceIOOrPublishOutbox(t *testing.T) {
	repo := NewMemoryRepository()
	driver := &durableQueueDriver{repo: repo}
	svc := NewService(repo, driver)
	sale := Sale{ID: "local-prepare", State: "OPEN", UNP: "12345678-A001-0000001", RegisterID: "register-1", Lines: []SaleLine{{LineID: "line-1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}}, Version: 1}
	if err := repo.PutSale(sale); err != nil {
		t.Fatal(err)
	}
	request := SaleFinalizeRequest{ClientOperationID: "00000000-0000-4000-8000-000000000201", ReceiptSessionID: "00000000-0000-4000-8000-000000000202", Payments: []PaymentRequest{{PaymentID: "00000000-0000-4000-8000-000000000203", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}}}, ExpectedTotal: Money{Amount: "2.50", Currency: "EUR"}}
	op, err := svc.PrepareFinalizeSaleForTenant(sale.ID, request, "")
	if err != nil {
		t.Fatal(err)
	}
	if driver.published || len(repo.Resources("device_command_outbox", "")) != 0 {
		t.Fatal("prepare published money command")
	}
	saved, _ := repo.Sale(sale.ID)
	if saved.State != "PAYMENT_PENDING" || saved.FiscalOperationID != op.ID || len(saved.Payments) != 1 {
		t.Fatal("prepared plan absent")
	}
}
