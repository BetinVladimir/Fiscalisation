package domain

import "testing"

func sagaPlan() ReceiptFinalizePlan {
	return ReceiptFinalizePlan{ClientOperationID: "10000000-0000-4000-8000-000000000001", ReceiptSessionID: "20000000-0000-4000-8000-000000000001", SaleID: "sale-1", UNP: "AB123456-A001-0000001", Items: []SaleLine{{LineID: "l", Name: "x", Quantity: "1.000", UnitPrice: Money{Amount: "2.00", Currency: "EUR"}, TaxGroup: "B"}}, Payments: []PaymentRequest{{PaymentID: "30000000-0000-4000-8000-000000000001", Type: "CARD", Amount: Money{Amount: "2.00", Currency: "EUR"}}}, ExpectedTotal: Money{Amount: "2.00", Currency: "EUR"}}
}
func TestReceiptSagaReservationIsIdempotentAndDigestBound(t *testing.T) {
	s := NewMemoryReceiptSagaStore()
	v, owner, e := s.ReserveReceiptSaga("t", ReceiptSaga{Plan: sagaPlan()})
	if e != nil || !owner || v.State != ReceiptReserved {
		t.Fatal(v, owner, e)
	}
	_, owner, e = s.ReserveReceiptSaga("t", ReceiptSaga{Plan: sagaPlan()})
	if e != nil || owner {
		t.Fatal(owner, e)
	}
	bad := sagaPlan()
	bad.ExpectedTotal.Amount = "3.00"
	bad.Payments[0].Amount.Amount = "3.00"
	if _, _, e = s.ReserveReceiptSaga("t", ReceiptSaga{Plan: bad}); e == nil {
		t.Fatal("payload conflict accepted")
	}
}
func TestReceiptSagaHappyAndRecoveryTransitions(t *testing.T) {
	s := NewMemoryReceiptSagaStore()
	p := sagaPlan()
	s.ReserveReceiptSaga("t", ReceiptSaga{Plan: p})
	for _, x := range [][2]ReceiptSagaState{{ReceiptReserved, ReceiptCardAuthorizing}, {ReceiptCardAuthorizing, ReceiptCardApproved}, {ReceiptCardApproved, ReceiptFiscalOpening}, {ReceiptFiscalOpening, ReceiptFiscalOpen}, {ReceiptFiscalOpen, ReceiptLinesRegistering}, {ReceiptLinesRegistering, ReceiptPaymentsRegistering}, {ReceiptPaymentsRegistering, ReceiptFiscalClosing}, {ReceiptFiscalClosing, ReceiptCommitted}} {
		if _, e := s.AdvanceReceiptSaga("t", p.ClientOperationID, x[0], x[1]); e != nil {
			t.Fatal(x, e)
		}
	}
	if _, e := s.AdvanceReceiptSaga("t", p.ClientOperationID, ReceiptCommitted, ReceiptReserved); e == nil {
		t.Fatal("terminal state reopened")
	}
}

func TestFinalizeSalePublishesOneAggregateCommand(t *testing.T) {
	repo := NewMemoryRepository()
	driver := &capturingFinalizeDriver{}
	svc := NewService(repo, driver)
	// The aggregate coordinator validation itself is exercised independently of
	// tenant resource provisioning; this test proves one ordered command rather
	// than one command per tender.
	sale := Sale{ID: "sale-finalize", State: "OPEN", UNP: "12345678-A001-0000001", RegisterID: "register-1", Lines: []SaleLine{{LineID: "line-1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}}, Version: 1}
	if err := repo.PutSale(sale); err != nil {
		t.Fatal(err)
	}
	op, err := svc.FinalizeSaleForTenant(sale.ID, SaleFinalizeRequest{ClientOperationID: "00000000-0000-4000-8000-000000000101", ReceiptSessionID: "00000000-0000-4000-8000-000000000102", Payments: []PaymentRequest{{PaymentID: "00000000-0000-4000-8000-000000000103", Type: "CASH", Amount: Money{Amount: "1.00", Currency: "EUR"}}, {PaymentID: "00000000-0000-4000-8000-000000000104", Type: "CASH", Amount: Money{Amount: "1.50", Currency: "EUR"}}}, ExpectedTotal: Money{Amount: "2.50", Currency: "EUR"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if op.ID != "00000000-0000-4000-8000-000000000101" || driver.calls != 1 || driver.paymentCount != 2 {
		t.Fatalf("aggregate lost: op=%+v driver=%+v", op, driver)
	}
}

type capturingFinalizeDriver struct{ calls, paymentCount int }

func (d *capturingFinalizeDriver) Probe() error { return nil }
func (d *capturingFinalizeDriver) Execute(op Operation, s Sale, _ PaymentRequest) (string, string) {
	d.calls++
	d.paymentCount = len(s.Payments)
	return "receipt-1", ""
}
