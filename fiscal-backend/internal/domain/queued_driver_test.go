package domain

import (
	"errors"
	"testing"
	"time"
)

type queueDriver struct {
	calls   int
	op      Operation
	sale    Sale
	payment PaymentRequest
	fail    bool
}

func (d *queueDriver) Probe() error { return nil }
func (d *queueDriver) Execute(Operation, Sale, PaymentRequest) (string, string) {
	panic("synchronous side effect forbidden")
}
func (d *queueDriver) Queue(o Operation, s Sale, p PaymentRequest) error {
	d.calls++
	d.op = o
	d.sale = s
	d.payment = p
	if d.fail {
		return ErrNotFound
	}
	return nil
}
func TestPaymentQueuesAfterDurableReservation(t *testing.T) {
	r := NewMemoryRepository()
	d := &queueDriver{}
	s := NewService(r, d)
	sale, err := s.CreateSale(CreateSale{ExternalID: "queued", RegisterID: "r1", OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	sale, err = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "10.00", Currency: "EUR"}, TaxGroup: "B"})
	if err != nil {
		t.Fatal(err)
	}
	op, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p-queued", Type: "CASH", Amount: Money{Amount: "10.00", Currency: "EUR"}})
	if err != nil || op.State != "EXECUTING" || d.calls != 1 {
		t.Fatalf("queue result %+v calls=%d err=%v", op, d.calls, err)
	}
	stored, _ := r.Operation(op.ID)
	if stored.State != "EXECUTING" {
		t.Fatalf("operation not durable before queue: %+v", stored)
	}
}

type durableQueueDriver struct {
	repo      Repository
	published bool
}

func (d *durableQueueDriver) Probe() error { return nil }
func (d *durableQueueDriver) Execute(Operation, Sale, PaymentRequest) (string, string) {
	panic("synchronous side effect forbidden")
}
func (d *durableQueueDriver) Prepare(op Operation, sale Sale, _ PaymentRequest) (ResourceRecord, error) {
	return ResourceRecord{Kind: "device_command_outbox", TenantID: sale.TenantID, ID: op.ID, Version: 1, Data: map[string]any{"topic": "tenants/t/devices/d/commands", "body": "{}", "expires_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)}, CreatedAt: op.CreatedAt, UpdatedAt: op.CreatedAt}, nil
}
func (d *durableQueueDriver) Publish(command ResourceRecord) error {
	stored, err := d.repo.Resource("device_command_outbox", command.ID)
	if err != nil || stored.ID != command.ID {
		return errors.New("publish happened before durable outbox")
	}
	d.published = true
	return errors.New("broker unavailable")
}

func TestDurableQueueIsAtomicAndSurvivesRestartWithoutUnknown(t *testing.T) {
	store := &testStore{}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	driver := &durableQueueDriver{repo: repo}
	svc := NewService(repo, driver)
	sale, err := svc.CreateSale(CreateSale{ExternalID: "durable-queued", RegisterID: "r1", OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	sale, err = svc.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "10.00", Currency: "EUR"}, TaxGroup: "B"})
	if err != nil {
		t.Fatal(err)
	}
	op, err := svc.Pay(sale.ID, PaymentRequest{PaymentID: "p-durable", Type: "CASH", Amount: Money{Amount: "10.00", Currency: "EUR"}})
	if err != nil || !driver.published || op.State != "EXECUTING" {
		t.Fatalf("unexpected durable queue result: %+v published=%v err=%v", op, driver.published, err)
	}
	restarted, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := restarted.Operation(op.ID)
	if err != nil || stored.State != "EXECUTING" {
		t.Fatalf("durable command was made ambiguous on restart: %+v %v", stored, err)
	}
	if _, err = restarted.Resource("device_command_outbox", op.ID); err != nil {
		t.Fatal("durable command missing after restart", err)
	}
}

func TestReversalIsReservedWithDurableCommandBeforePublish(t *testing.T) {
	repo := NewMemoryRepository()
	driver := &durableQueueDriver{repo: repo}
	svc := NewService(repo, driver)
	now := time.Now().UTC().Add(-time.Hour)
	sale := Sale{ID: "sale-reversal", ExternalID: "external-reversal", RegisterID: "r1", OperatorID: "A001", UNP: "DY000600-A001-0000001", State: "COMPLETED", Version: 3, FiscalOperationID: "original-op", Lines: []SaleLine{{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "10.00", Currency: "EUR"}, TaxGroup: "B"}}, Payments: []PaymentRecord{{PaymentID: "p1", Type: "CARD", Amount: Money{Amount: "10.00", Currency: "EUR"}}}, FiscalDevice: FiscalDeviceSnapshot{DeviceID: "device-1", BindingVersion: 1, FiscalMemoryNumber: "12345678"}, CreatedAt: now, UpdatedAt: now}
	if err := repo.PutSale(sale); err != nil {
		t.Fatal(err)
	}
	if err := repo.PutOperation(Operation{ID: "original-op", SaleID: sale.ID, Type: "FISCAL_SALE", State: "FISCALIZED", Version: 2, FiscalReference: "269", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	op, err := svc.Reverse(sale.ID, "CUSTOMER_RETURN")
	if err != nil || op.State != "EXECUTING" || !driver.published {
		t.Fatalf("reversal not durably queued: op=%+v published=%v err=%v", op, driver.published, err)
	}
	stored, _ := repo.Sale(sale.ID)
	if stored.State != "FISCALIZATION_PENDING" {
		t.Fatalf("sale not reserved: %+v", stored)
	}
	if _, err = repo.Resource("device_command_outbox", op.ID); err != nil {
		t.Fatal("reversal outbox missing", err)
	}
}

func TestExpiredDurableCommandBecomesUnknownWithWebhook(t *testing.T) {
	repo := NewMemoryRepository()
	driver := &durableQueueDriver{repo: repo}
	svc := NewService(repo, driver)
	sale, err := svc.CreateSale(CreateSale{ExternalID: "expire-queued", RegisterID: "r1", OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	sale, err = svc.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "10.00", Currency: "EUR"}, TaxGroup: "B"})
	if err != nil {
		t.Fatal(err)
	}
	op, err := svc.Pay(sale.ID, PaymentRequest{PaymentID: "p-expire", Type: "CASH", Amount: Money{Amount: "10.00", Currency: "EUR"}})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.ExpireDeviceCommand(op.ID); err != nil {
		t.Fatal(err)
	}
	stored, _ := repo.Operation(op.ID)
	updated, _ := repo.Sale(sale.ID)
	if stored.State != "UNKNOWN" || updated.State != "UNKNOWN" || stored.ErrorCode != "DEVICE_COMMAND_EXPIRED_BEFORE_ACCEPTANCE" {
		t.Fatalf("expiry did not fail close: op=%+v sale=%+v", stored, updated)
	}
	if len(repo.PendingOutbox(time.Now().UTC())) != 1 {
		t.Fatal("expiry webhook was not committed")
	}
}
