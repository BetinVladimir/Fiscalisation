package domain

import (
	"testing"
	"time"
)

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

func TestAddLineRejectsStaleVersionWithoutMutation(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	sale, err := s.CreateSale(CreateSale{ExternalID: "concurrent-1", RegisterID: "FD000001", OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	line := SaleLine{LineID: "l1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}
	updated, err := s.AddLineExpectedForTenant(sale.ID, sale.Version, line, sale.TenantID)
	if err != nil || updated.Version != sale.Version+1 {
		t.Fatalf("first versioned mutation failed: %+v %v", updated, err)
	}
	if _, err = s.AddLineExpectedForTenant(sale.ID, sale.Version, SaleLine{LineID: "l2", Name: "Tea", Quantity: "1.000", UnitPrice: Money{Amount: "1.50", Currency: "EUR"}, TaxGroup: "B"}, sale.TenantID); err == nil {
		t.Fatal("stale sale version accepted")
	}
	stored, err := s.GetSale(sale.ID)
	if err != nil || stored.Version != updated.Version || len(stored.Lines) != 1 {
		t.Fatalf("stale mutation changed sale: %+v %v", stored, err)
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

func TestReversalPersistsReasonAndOriginalFiscalReference(t *testing.T) {
	r := NewMemoryRepository()
	s := NewService(r, NewSimulator(true))
	sale, _ := s.CreateSale(CreateSale{ExternalID: "reverse-1", RegisterID: "FD000001", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	original, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}})
	if err != nil || original.FiscalReference == "" {
		t.Fatal(original, err)
	}
	before := len(r.Operations())
	if _, err = s.ReverseForTenantWithReference(sale.ID, "CUSTOMER_RETURN", "wrong-reference", ""); err == nil || len(r.Operations()) != before {
		t.Fatal("mismatched original reference changed state", err)
	}
	reversal, err := s.ReverseForTenantWithReference(sale.ID, "CUSTOMER_RETURN", original.FiscalReference, "")
	if err != nil || reversal.Type != "REVERSAL" || reversal.ReasonCode != "CUSTOMER_RETURN" || reversal.OriginalFiscalReference != original.FiscalReference || reversal.SaleID != sale.ID {
		t.Fatal("incomplete reversal evidence", reversal, err)
	}
	persisted, err := r.Operation(reversal.ID)
	if err != nil || persisted.ReasonCode != reversal.ReasonCode || persisted.OriginalFiscalReference != original.FiscalReference {
		t.Fatal("reversal compliance fields were not persisted", persisted, err)
	}
	if _, err = s.Reverse(sale.ID, "SECOND_REVERSAL"); err == nil {
		t.Fatal("second reversal accepted")
	}
}

func TestReversalRejectsUnknownReasonAndExpiredOperatorErrorBeforeExecution(t *testing.T) {
	r := NewMemoryRepository()
	s := NewService(r, NewSimulator(true))
	sale, _ := s.CreateSale(CreateSale{ExternalID: "reverse-deadline", RegisterID: "FD000001", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	original, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}})
	if err != nil {
		t.Fatal(err)
	}
	before := len(r.Operations())
	if _, err = s.ReverseForTenantWithReference(sale.ID, "OTHER", original.FiscalReference, ""); err == nil || len(r.Operations()) != before {
		t.Fatal("unknown reversal reason reached execution", err)
	}
	original.CreatedAt = time.Now().UTC().AddDate(0, -3, 0)
	if err = r.PutOperation(original); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ReverseForTenantWithReference(sale.ID, "OPERATOR_ERROR", original.FiscalReference, ""); err == nil || len(r.Operations()) != before {
		t.Fatal("expired operator-error reversal reached execution", err)
	}
}
