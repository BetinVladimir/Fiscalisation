package domain

import "testing"

func TestSplitPaymentAndExactTotal(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	sale, e := s.CreateSale(CreateSale{ExternalID: "split", RegisterID: "r1", OperatorID: "A001"})
	if e != nil {
		t.Fatal(e)
	}
	sale, e = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "2.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if e != nil {
		t.Fatal(e)
	}
	op, e := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.00", Currency: "EUR"}})
	if e != nil || op.State != "PAYMENT_ACCEPTED" {
		t.Fatalf("%+v %v", op, e)
	}
	op, e = s.Pay(sale.ID, PaymentRequest{PaymentID: "p2", Type: "CASH", Amount: Money{Amount: "3.00", Currency: "EUR"}})
	if e != nil || op.State != "FISCALIZED" {
		t.Fatalf("%+v %v", op, e)
	}
	got, _ := s.GetSale(sale.ID)
	if got.State != "COMPLETED" || len(got.Payments) != 2 {
		t.Fatalf("%+v", got)
	}
}
func TestOverpaymentAndInvalidQuantityRejected(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	sale, _ := s.CreateSale(CreateSale{ExternalID: "over", RegisterID: "r1", OperatorID: "A001"})
	if _, e := s.AddLine(sale.ID, SaleLine{LineID: "bad", Name: "Item", Quantity: "0.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}); e == nil {
		t.Fatal("zero quantity")
	}
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "ok", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if _, e := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.51", Currency: "EUR"}}); e == nil {
		t.Fatal("overpayment")
	}
}

func TestUnknownOutcomeRequiresReconciliationAndNoBlindRetry(t *testing.T) {
	driver := NewSimulator(true)
	driver.SetOutcomeUnknown(true)
	s := NewService(NewMemoryRepository(), driver)
	sale, _ := s.CreateSale(CreateSale{ExternalID: "unknown", RegisterID: "r1", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	op, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}})
	if err != nil || op.State != "UNKNOWN" {
		t.Fatal(op, err)
	}
	if _, err = s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}}); err == nil {
		t.Fatal("unknown operation was blindly retried")
	}
	got, _ := s.GetSale(sale.ID)
	if got.State != "UNKNOWN" {
		t.Fatal(got)
	}
}

func TestLostFiscalDeviceBlocksBeforeOperation(t *testing.T) {
	r := NewMemoryRepository()
	s := NewService(r, NewSimulator(false))
	sale, _ := s.CreateSale(CreateSale{ExternalID: "offline", RegisterID: "r1", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B"})
	if _, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "1.00", Currency: "EUR"}}); err == nil {
		t.Fatal("lost device accepted payment")
	}
	if len(r.Operations()) != 0 {
		t.Fatal("operation created before device reachability was established")
	}
}
