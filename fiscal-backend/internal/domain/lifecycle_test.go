package domain

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type blockingReversalDriver struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (d *blockingReversalDriver) Probe() error { return nil }
func (d *blockingReversalDriver) Execute(Operation, Sale, PaymentRequest) (string, string) {
	if d.calls.Add(1) == 1 {
		close(d.entered)
		<-d.release
	}
	return "REVERSAL-ONE", ""
}

type durableCountingDriver struct{ calls atomic.Int32 }

func (d *durableCountingDriver) Probe() error { return nil }
func (d *durableCountingDriver) Execute(Operation, Sale, PaymentRequest) (string, string) {
	d.calls.Add(1)
	return "SHOULD-NOT-EXECUTE", ""
}

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

func TestReceiptKeepsFiscalDeviceIdentityAfterRegisterRebind(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))
	registerID, originalDeviceID := prepareBLERegister(t, s, "tenant-device-snapshot")
	sale, err := s.CreateSale(CreateSale{TenantID: "tenant-device-snapshot", ExternalID: "device-snapshot-sale", RegisterID: registerID, OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	sale, err = s.AddLineForTenant(sale.ID, SaleLine{LineID: "line-1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B"}, sale.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "payment-1", Type: "CASH", Amount: Money{Amount: "1.00", Currency: "EUR"}}, sale.TenantID); err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateResource("device", sale.TenantID, map[string]any{"kind": "FISCAL_DEVICE", "vendor": "Datecs", "model": "DP-150 MX", "serial": "SECOND-FD", "status": "DRAFT", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	second = activateTestDevice(t, s, sale.TenantID, second)
	if _, err = s.BindRegister(registerID, sale.TenantID, second["id"].(string), "FISCAL_DEVICE", ""); err != nil {
		t.Fatal(err)
	}
	receipt, err := s.ReceiptForTenant(sale.ID, sale.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, ok := receipt["fiscal_device"].(FiscalDeviceSnapshot)
	if !ok || snapshot.DeviceID != originalDeviceID || snapshot.Serial != "BLE-FD-1" || snapshot.DeviceID == second["id"] {
		t.Fatalf("receipt followed mutable register binding: %#v", receipt["fiscal_device"])
	}
	stored, err := repo.Sale(sale.ID)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := repo.Artifact(stored.ReceiptArtifactID, sale.TenantID)
	if err != nil || !bytes.Contains(artifact, []byte(originalDeviceID)) || bytes.Contains(artifact, []byte(second["id"].(string))) {
		t.Fatalf("immutable receipt artifact lost original device: %s %v", artifact, err)
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
	if err := r.PutOperation(Operation{ID: "unknown-1", TenantID: "t1", SaleID: "sale-1", Type: "FISCAL_SALE", State: "UNKNOWN", Version: 1}); err != nil {
		t.Fatal(err)
	}
	op, err := s.ReconcileOperation("unknown-1")
	if err != nil || op.State != "RECONCILING" || op.Version != 2 {
		t.Fatal(op, err)
	}
	pending := s.PendingOutbox(time.Now().UTC().Add(time.Second))
	if len(pending) != 1 || pending[0].Event.EventType != "fiscal.operation.reconciliation_required" || pending[0].Event.ResourceID != "sale-1" || pending[0].Event.ResourceVersion != op.Version {
		t.Fatalf("reconciliation transition and outbox diverged: %+v", pending)
	}
	data, ok := pending[0].Event.Data.(map[string]any)
	if !ok || data["lookup_only"] != true {
		t.Fatalf("reconciliation event does not prohibit blind replay: %+v", pending[0].Event.Data)
	}
	if _, err = s.ReconcileOperation("unknown-1"); err == nil {
		t.Fatal("duplicate reconciliation accepted")
	}
}

func TestReconciliationTransitionRollsBackWithOutboxFailure(t *testing.T) {
	store := &failNextStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	original := Operation{ID: "unknown-rollback", TenantID: "tenant-1", SaleID: "sale-1", Type: "FISCAL_SALE", State: "UNKNOWN", Version: 1, AllowedActions: []string{"RECONCILE"}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err = repo.PutOperation(original); err != nil {
		t.Fatal(err)
	}
	store.failNext.Store(true)
	s := NewService(repo, NewSimulator(true))
	if _, err = s.ReconcileOperationForTenant(original.ID, original.TenantID); err == nil {
		t.Fatal("injected reconciliation commit failure was ignored")
	}
	stored, err := repo.Operation(original.ID)
	if err != nil || stored.State != "UNKNOWN" || stored.Version != original.Version || len(repo.outbox) != 0 {
		t.Fatalf("failed reconciliation leaked state or event: operation=%+v outbox=%+v err=%v", stored, repo.outbox, err)
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
	reversedSale, err := r.Sale(sale.ID)
	if err != nil || reversedSale.State != "CANCELLED" {
		t.Fatal("successful reversal did not use the canonical Fiscal Sale state", reversedSale, err)
	}
	if _, err = s.Reverse(sale.ID, "SECOND_REVERSAL"); err == nil {
		t.Fatal("second reversal accepted")
	}
}

func TestConcurrentDirectReversalReservesSaleBeforeDriver(t *testing.T) {
	r := NewMemoryRepository()
	s := NewService(r, NewSimulator(true))
	sale, _ := s.CreateSale(CreateSale{ExternalID: "direct-reversal-race", RegisterID: "FD000001", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	original, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}})
	if err != nil {
		t.Fatal(err)
	}
	driver := &blockingReversalDriver{entered: make(chan struct{}), release: make(chan struct{})}
	s.driver = driver
	var wg sync.WaitGroup
	wg.Add(2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		_, reverseErr := s.ReverseForTenantWithReference(sale.ID, "CUSTOMER_RETURN", original.FiscalReference, "")
		errs <- reverseErr
	}()
	<-driver.entered
	reserved, _ := r.Sale(sale.ID)
	if reserved.State != "FISCALIZATION_PENDING" {
		t.Fatalf("reversal was not durably reserved before driver: %+v", reserved)
	}
	go func() {
		defer wg.Done()
		_, reverseErr := s.ReverseForTenantWithReference(sale.ID, "CUSTOMER_RETURN", original.FiscalReference, "")
		errs <- reverseErr
	}()
	close(driver.release)
	wg.Wait()
	close(errs)
	successes := 0
	for reverseErr := range errs {
		if reverseErr == nil {
			successes++
		}
	}
	if driver.calls.Load() != 1 || successes != 1 {
		t.Fatalf("direct reversal executed driver %d times with %d successes", driver.calls.Load(), successes)
	}
}

func TestFiscalOperationPersistsExecutingBeforeDriver(t *testing.T) {
	repo, err := NewPersistentRepository(&failingStore{err: errors.New("disk unavailable")})
	if err != nil {
		t.Fatal(err)
	}
	driver := &durableCountingDriver{}
	s := NewService(repo, driver)
	if _, err = s.FiscalOperation("register-1", "Z", ""); err == nil {
		t.Fatal("fiscal operation ignored durable reservation failure")
	}
	if driver.calls.Load() != 0 {
		t.Fatal("driver executed before operation reservation")
	}
}

func TestGenericFiscalOperationCommitsTerminalWebhookAtomically(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))
	op, err := s.FiscalOperation("register-1", "Z", "")
	if err != nil || op.State != "FISCALIZED" {
		t.Fatal(op, err)
	}
	pending := s.PendingOutbox(time.Now().UTC().Add(time.Second))
	if len(pending) != 1 || pending[0].Event.EventType != "fiscal.operation.updated" || pending[0].Event.ResourceID != op.ID || pending[0].Event.ResourceVersion != op.Version {
		t.Fatalf("generic fiscal operation webhook diverged: %+v", pending)
	}
}

func TestInterruptedReversalRestartMovesSaleAndOperationToUnknown(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	sale := Sale{ID: "sale-interrupted-reversal", ExternalID: "external", RegisterID: "register-1", OperatorID: "A001", State: "COMPLETED", Version: 4, Lines: []SaleLine{}, Payments: []PaymentRecord{}, CreatedAt: now, UpdatedAt: now}
	if err = repo.PutSale(sale); err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: "op-interrupted-reversal", SaleID: sale.ID, Type: "REVERSAL", State: "EXECUTING", Version: 1, CreatedAt: now, UpdatedAt: now}
	if _, err = repo.ReserveSaleReversal(sale.ID, "", sale.Version, op); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	recoveredSale, _ := restarted.Sale(sale.ID)
	recoveredOperation, _ := restarted.Operation(op.ID)
	if recoveredSale.State != "UNKNOWN" || recoveredOperation.State != "UNKNOWN" || len(recoveredOperation.AllowedActions) != 1 || recoveredOperation.AllowedActions[0] != "RECONCILE" {
		t.Fatalf("interrupted reversal did not recover fail-closed: sale=%+v operation=%+v", recoveredSale, recoveredOperation)
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
