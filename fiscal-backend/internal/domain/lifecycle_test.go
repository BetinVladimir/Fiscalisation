package domain

import "testing"

func TestCancelOnlyUnpaidSale(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	sale, err := s.CreateSale(CreateSale{ExternalID: "cancel-1", RegisterID: "FD000001", OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	op, err := s.CancelSale(sale.ID)
	if err != nil || op.State != "CANCELLED" {
		t.Fatalf("%+v %v", op, err)
	}
	if _, err = s.Receipt(sale.ID); err == nil {
		t.Fatal("cancelled sale exposed a fiscal receipt")
	}
	if _, err = s.CancelSale(sale.ID); err == nil {
		t.Fatal("second cancellation accepted")
	}
}

func TestReceiptAfterFiscalization(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	sale, _ := s.CreateSale(CreateSale{ExternalID: "receipt-1", RegisterID: "FD000001", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	op, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}})
	if err != nil || op.State != "FISCALIZED" {
		t.Fatal(op, err)
	}
	receipt, err := s.Receipt(sale.ID)
	if err != nil || receipt["fiscal_reference"] == "" {
		t.Fatal(receipt, err)
	}
}

func TestReconcileUnknownOperationOnly(t *testing.T) {
	r := NewMemoryRepository()
	s := NewService(r, NewSimulator(true))
	if err := r.PutOperation(Operation{ID: "unknown-1", TenantID: "t1", State: "UNKNOWN", Version: 1}); err != nil {
		t.Fatal(err)
	}
	op, err := s.ReconcileOperation("unknown-1")
	if err != nil || op.State != "RECONCILING" || op.Version != 2 {
		t.Fatal(op, err)
	}
	if _, err = s.ReconcileOperation("unknown-1"); err == nil {
		t.Fatal("duplicate reconciliation accepted")
	}
}
