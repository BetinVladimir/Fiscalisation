package domain

import (
	"sync"
	"testing"
)

type memoryStore struct {
	mu sync.Mutex
	b  []byte
}

func (m *memoryStore) Load() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.b...), nil
}
func (m *memoryStore) Save(b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.b = append([]byte(nil), b...)
	return nil
}
func TestPersistentServiceRestoresAutonomousStateAndSequence(t *testing.T) {
	store := &memoryStore{}
	s, e := NewPersistentService("http://fiscal", "v1", store)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.CreateProduct(Product{SKU: "1", Name: "Coffee", Price: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if e != nil {
		t.Fatal(e)
	}
	s2, e := NewPersistentService("http://fiscal", "v1", store)
	if e != nil {
		t.Fatal(e)
	}
	if got, e := s2.Product(p.ID); e != nil || got.Name != "Coffee" {
		t.Fatalf("%+v %v", got, e)
	}
	p2, e := s2.CreateProduct(Product{SKU: "2", Name: "Water", Price: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B"})
	if e != nil || p2.ID == p.ID {
		t.Fatalf("%+v %v", p2, e)
	}
}
func TestWebhookInboxPersistsRawBodyAndDeduplicates(t *testing.T) {
	store := &memoryStore{}
	s, err := NewPersistentService("http://fiscal", "v1", store)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.orders["order-1"] = Order{ID: "order-1", TenantID: "tenant-1", State: "FISCAL_PENDING", FiscalSaleID: "sale-1", Version: 7}
	if err = s.persistLocked(); err != nil {
		t.Fatal(err)
	}
	s.mu.Unlock()
	raw := []byte(`{"event_id":"event-1","resource_id":"sale-1"}`)
	if err = s.ProcessFiscalWebhook("event-1", raw, "sale-1", "FISCALIZED", "op-1", 2); err != nil {
		t.Fatal(err)
	}
	s, err = NewPersistentService("http://fiscal", "v1", store)
	if err != nil {
		t.Fatal(err)
	}
	order, _ := s.Order("order-1")
	if order.State != "COMPLETED" || order.FiscalVersion != 2 {
		t.Fatal(order)
	}
	inbox := s.webhookInbox["event-1"]
	if inbox.ProcessedAt == nil || string(inbox.Raw) != string(raw) {
		t.Fatal(inbox)
	}
	if err = s.ProcessFiscalWebhook("event-1", raw, "sale-1", "FAILED", "other", 3); err != nil {
		t.Fatal("same raw replay should be idempotent", err)
	}
	order, _ = s.Order("order-1")
	if order.State != "COMPLETED" {
		t.Fatal("replay reapplied event")
	}
	if err = s.ProcessFiscalWebhook("event-1", []byte(`different`), "sale-1", "FAILED", "other", 3); err == nil {
		t.Fatal("event id payload mismatch accepted")
	}
}
