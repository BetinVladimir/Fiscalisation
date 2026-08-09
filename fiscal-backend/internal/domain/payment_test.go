package domain

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type blockingPaymentDriver struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

type failNextStore struct {
	testStore
	failNext atomic.Bool
}

func (s *failNextStore) Save(body []byte) error {
	if s.failNext.CompareAndSwap(true, false) {
		return errors.New("injected final commit failure")
	}
	return s.testStore.Save(body)
}

type commitFailureDriver struct{ store *failNextStore }

func (d commitFailureDriver) Probe() error { return nil }
func (d commitFailureDriver) Execute(Operation, Sale, PaymentRequest) (string, string) {
	d.store.failNext.Store(true)
	return "FISCAL-COMMITTED", ""
}

func (d *blockingPaymentDriver) Probe() error { return nil }
func (d *blockingPaymentDriver) Execute(Operation, Sale, PaymentRequest) (string, string) {
	if d.calls.Add(1) == 1 {
		close(d.entered)
		<-d.release
	}
	return "FISCAL-ONE", ""
}

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

func TestExactSaleDecimalArithmeticRejectsNegativeAndOverflow(t *testing.T) {
	discount := Money{Amount: "0.20", Currency: "EUR"}
	sale := Sale{Lines: []SaleLine{
		{Quantity: "1", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, Discount: &discount},
		{Quantity: "1.2", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}},
		{Quantity: "1.23", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}},
		{Quantity: "1.234", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}},
		{Quantity: "0.500", UnitPrice: Money{Amount: "0.01", Currency: "EUR"}},
	}}
	if total, err := saleTotal(sale); err != nil || total != 447 {
		t.Fatalf("exact half-up total mismatch: total=%d err=%v", total, err)
	}
	if _, err := lineTotalCents(Money{Amount: "-1.00", Currency: "EUR"}, "1.000"); err == nil {
		t.Fatal("negative fiscal line price accepted")
	}
	if _, err := lineTotalCents(Money{Amount: "92233720368547758.07", Currency: "EUR"}, "2.000"); err == nil {
		t.Fatal("overflowing fiscal line total accepted")
	}
	over := Money{Amount: "1.01", Currency: "EUR"}
	if _, err := discountedLineTotalCents(SaleLine{Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, Discount: &over}); err == nil {
		t.Fatal("over-discounted fiscal line accepted")
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

func TestConcurrentPaymentsReserveSaleBeforeDriverSideEffect(t *testing.T) {
	driver := &blockingPaymentDriver{entered: make(chan struct{}), release: make(chan struct{})}
	s := NewService(NewMemoryRepository(), driver)
	sale, _ := s.CreateSale(CreateSale{ExternalID: "concurrent-payment", RegisterID: "r1", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		_, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}})
		errs <- err
	}()
	<-driver.entered
	reserved, err := s.GetSale(sale.ID)
	if err != nil || reserved.State != "PAYMENT_PENDING" {
		t.Fatalf("payment was not durably reserved before driver execution: %+v %v", reserved, err)
	}
	go func() {
		defer wg.Done()
		_, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p2", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}})
		errs <- err
	}()
	close(driver.release)
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if driver.calls.Load() != 1 || successes != 1 {
		t.Fatalf("concurrent payment executed driver %d times with %d successes", driver.calls.Load(), successes)
	}
}

func TestFinalPaymentCommitIsAtomicAndRestartRecoversUnknown(t *testing.T) {
	store := &failNextStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(repo, commitFailureDriver{store: store})
	sale, _ := s.CreateSale(CreateSale{ExternalID: "final-commit-failure", RegisterID: "r1", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if _, err = s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}}); err == nil {
		t.Fatal("injected final persistence failure was ignored")
	}
	pending, _ := repo.Sale(sale.ID)
	if pending.State != "PAYMENT_PENDING" || len(pending.Payments) != 0 || pending.ReceiptArtifactID != "" {
		t.Fatalf("failed atomic commit leaked final sale fields: %+v", pending)
	}
	if len(repo.outbox) != 0 {
		t.Fatal("failed terminal payment commit leaked a webhook event")
	}
	operations := repo.Operations()
	if len(operations) != 1 || operations[0].State != "EXECUTING" {
		t.Fatalf("durable reservation was not retained: %+v", operations)
	}
	restarted, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	recoveredSale, _ := restarted.Sale(sale.ID)
	recoveredOperation, _ := restarted.Operation(operations[0].ID)
	if recoveredSale.State != "UNKNOWN" || recoveredOperation.State != "UNKNOWN" || recoveredOperation.ErrorCode != "INTERRUPTED_AFTER_DEVICE_DISPATCH" || len(recoveredOperation.AllowedActions) != 1 || recoveredOperation.AllowedActions[0] != "RECONCILE" {
		t.Fatalf("restart did not conservatively recover interrupted payment: sale=%+v operation=%+v", recoveredSale, recoveredOperation)
	}
	if _, err = restarted.Artifact("missing", ""); err == nil {
		t.Fatal("failed transaction left a receipt artifact")
	}
}

func TestTerminalPaymentAndWebhookOutboxCommitAtomically(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))
	sale, _ := s.CreateSale(CreateSale{ExternalID: "payment-outbox", RegisterID: "r1", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	op, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}})
	if err != nil || op.State != "FISCALIZED" {
		t.Fatal(op, err)
	}
	pending := s.PendingOutbox(time.Now().UTC().Add(time.Second))
	if len(pending) != 1 || pending[0].ID != "event-"+op.ID || pending[0].Event.ResourceID != sale.ID || pending[0].Event.ResourceVersion != op.Version {
		t.Fatalf("terminal payment and webhook evidence diverged: %+v", pending)
	}
	data, ok := pending[0].Event.Data.(map[string]any)
	if !ok || data["state"] != "FISCALIZED" {
		t.Fatalf("terminal webhook payload diverged: %+v", pending[0].Event.Data)
	}
	if err = s.QueueFiscalEvent(sale.ID, op); err != nil || len(s.PendingOutbox(time.Now().UTC().Add(time.Second))) != 1 {
		t.Fatal("legacy explicit queue path was not idempotent")
	}
}

func TestRestartRecoversEveryInterruptedFiscalCommandWithoutReplay(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, op := range []Operation{
		{ID: "op-reversal", Type: "REVERSAL", State: "EXECUTING", Version: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "op-report", Type: "Z_REPORT", State: "EXECUTING", Version: 1, CreatedAt: now, UpdatedAt: now},
	} {
		if err = repo.PutOperation(op); err != nil {
			t.Fatal(err)
		}
	}
	restarted, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"op-reversal", "op-report"} {
		op, getErr := restarted.Operation(id)
		if getErr != nil || op.State != "UNKNOWN" || op.ErrorCode != "INTERRUPTED_AFTER_DEVICE_DISPATCH" || len(op.AllowedActions) != 1 || op.AllowedActions[0] != "RECONCILE" {
			t.Fatalf("interrupted %s was not recovered conservatively: %+v %v", id, op, getErr)
		}
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

func TestCardPaymentRequiresActiveRegisterTerminal(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulatorWithCardTerminal(true, true))
	registerID, _ := prepareBLERegister(t, s, "tenant-card")
	newSale := func(externalID string) Sale {
		sale, err := s.CreateSale(CreateSale{TenantID: "tenant-card", ExternalID: externalID, RegisterID: registerID, OperatorID: "A001"})
		if err != nil {
			t.Fatal(err)
		}
		sale, err = s.AddLineExpectedForTenant(sale.ID, sale.Version, SaleLine{LineID: externalID + "-line", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}, "tenant-card")
		if err != nil {
			t.Fatal(err)
		}
		return sale
	}
	sale := newSale("card-without-terminal")
	if _, err := s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "card-1", Type: "CARD", TerminalPolicy: "AUTO_IF_AVAILABLE", Amount: Money{Amount: "2.50", Currency: "EUR"}}, "tenant-card"); err == nil {
		t.Fatal("card payment accepted without an active bound terminal")
	}
	if len(repo.Operations()) != 0 {
		t.Fatal("operation created before payment-terminal registry gate")
	}
	terminal, err := s.CreateResource("device", "tenant-card", map[string]any{"kind": "PAYMENT_TERMINAL", "vendor": "Simulator", "model": "CARD-STUB", "serial": "CARD-001", "status": "DRAFT", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	terminal = activateTestDevice(t, s, "tenant-card", terminal)
	if _, err = s.BindRegister(registerID, "tenant-card", terminal["id"].(string), "OPTIONAL_PAYMENT_TERMINAL", time.Now().UTC().Add(time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	sale = newSale("card-with-future-terminal")
	if _, err = s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "card-future", Type: "CARD", TerminalPolicy: "AUTO_IF_AVAILABLE", Amount: Money{Amount: "2.50", Currency: "EUR"}}, "tenant-card"); err == nil {
		t.Fatal("future-dated payment-terminal binding was treated as active")
	}
	if _, err = s.BindRegister(registerID, "tenant-card", terminal["id"].(string), "OPTIONAL_PAYMENT_TERMINAL", ""); err != nil {
		t.Fatal(err)
	}
	sale = newSale("card-with-terminal")
	op, err := s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "card-2", Type: "CARD", TerminalPolicy: "AUTO_IF_AVAILABLE", Amount: Money{Amount: "2.50", Currency: "EUR"}}, "tenant-card")
	if err != nil || op.State != "FISCALIZED" {
		t.Fatalf("active bound terminal did not enable card payment: %+v %v", op, err)
	}
	sale = newSale("card-none-policy")
	if _, err = s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "card-3", Type: "CARD", TerminalPolicy: "NONE", Amount: Money{Amount: "2.50", Currency: "EUR"}}, "tenant-card"); err == nil {
		t.Fatal("CARD with terminal_policy=NONE was accepted")
	}
}

func TestEURTotalAndSplitPaymentPropertyMatrix(t *testing.T) {
	for i := 1; i <= 120; i++ {
		quantity := i%5 + 1
		unitCents := (i*37)%997 + 1
		totalCents := quantity * unitCents
		firstCents := (i * 13) % (totalCents + 1)
		secondCents := totalCents - firstCents
		money := func(cents int) Money {
			return Money{Amount: fmt.Sprintf("%d.%02d", cents/100, cents%100), Currency: "EUR"}
		}
		s := NewService(NewMemoryRepository(), NewSimulator(true))
		sale, err := s.CreateSale(CreateSale{ExternalID: fmt.Sprintf("property-%d", i), RegisterID: "r1", OperatorID: "A001"})
		if err != nil {
			t.Fatal(err)
		}
		sale, err = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Property item", Quantity: fmt.Sprintf("%d.000", quantity), UnitPrice: money(unitCents), TaxGroup: "B"})
		if err != nil {
			t.Fatalf("case %d line: %v", i, err)
		}
		if firstCents > 0 {
			op, payErr := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: money(firstCents)})
			if payErr != nil || (secondCents > 0 && op.State != "PAYMENT_ACCEPTED") {
				t.Fatalf("case %d first split: %+v %v", i, op, payErr)
			}
		}
		if secondCents > 0 {
			op, payErr := s.Pay(sale.ID, PaymentRequest{PaymentID: "p2", Type: "CASH", Amount: money(secondCents)})
			if payErr != nil || op.State != "FISCALIZED" {
				t.Fatalf("case %d final split: %+v %v", i, op, payErr)
			}
		}
		completed, err := s.GetSale(sale.ID)
		if err != nil || completed.State != "COMPLETED" {
			t.Fatalf("case %d did not complete exactly: %+v %v", i, completed, err)
		}
	}
}
