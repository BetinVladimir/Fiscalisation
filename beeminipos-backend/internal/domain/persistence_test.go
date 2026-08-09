package domain

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestOperatorSessionRevocationSurvivesRestart(t *testing.T) {
	store := &memoryStore{}
	service, err := NewPersistentService("http://fiscal", "v1", store)
	if err != nil {
		t.Fatal(err)
	}
	employee, err := service.CreateEmployee(Employee{TenantID: "org-session", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RegisterOperatorSession("org-session", employee.ID, "00000000-0000-4000-8000-000000000001", "fingerprint-1", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RevokeOperatorSession("org-session", "fingerprint-1"); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewPersistentService("http://fiscal", "v1", store)
	if err != nil {
		t.Fatal(err)
	}
	if !restarted.OperatorSessionRevoked("org-session", "fingerprint-1") || restarted.OperatorSessionActive("org-session", "fingerprint-1", employee.ID, "00000000-0000-4000-8000-000000000001", time.Now()) {
		t.Fatal("revoked operator session became active after restart")
	}
}

type memoryStore struct {
	mu   sync.Mutex
	b    []byte
	fail bool
}

type versionedMemoryStore struct {
	mu         sync.Mutex
	b          []byte
	generation int64
	deltaCalls int
}

func (s *versionedMemoryStore) Load() ([]byte, error) {
	b, _, err := s.LoadVersioned()
	return b, err
}
func (s *versionedMemoryStore) Save(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b, s.generation = append([]byte(nil), b...), s.generation+1
	return nil
}
func (s *versionedMemoryStore) LoadVersioned() ([]byte, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.b...), s.generation, nil
}
func (s *versionedMemoryStore) SaveVersioned(b []byte, expected int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if expected != s.generation {
		return s.generation, errors.New("generation conflict")
	}
	s.b, s.generation = append([]byte(nil), b...), s.generation+1
	return s.generation, nil
}
func (s *versionedMemoryStore) SaveDeltaVersioned(_, current []byte, expected int64) (int64, error) {
	s.mu.Lock()
	s.deltaCalls++
	s.mu.Unlock()
	return s.SaveVersioned(current, expected)
}

func (m *memoryStore) Load() ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.b...), nil
}
func (m *memoryStore) Save(b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("injected persistence failure")
	}
	m.b = append([]byte(nil), b...)
	return nil
}

func TestAPIReplayMutationRollsBackMemoryWhenPersistenceFails(t *testing.T) {
	store := &memoryStore{fail: true}
	s, err := NewPersistentService("http://fiscal", "v1", store)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.PutAPIReplay("tenant\nPOST\n/path\nkey", APIReplay{Hash: "hash", Status: 201}); err == nil {
		t.Fatal("persistence failure was ignored")
	}
	if _, exists := s.APIReplay("tenant\nPOST\n/path\nkey"); exists {
		t.Fatal("failed replay mutation leaked into memory")
	}
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
	raw := []byte(`{"event_id":"event-1","tenant_id":"tenant-1","resource_id":"sale-1"}`)
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

func TestMiniPOSCoordinatorReloadsAfterGenerationConflict(t *testing.T) {
	store := &versionedMemoryStore{}
	s1, err := NewPersistentService("http://fiscal", "v1", store)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := NewPersistentService("http://fiscal", "v1", store)
	if err != nil {
		t.Fatal(err)
	}
	first, err := s1.CreateProduct(Product{TenantID: "tenant-1", SKU: "SKU-1", Name: "Coffee", Price: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s2.CreateEmployee(Employee{TenantID: "tenant-1", FirstName: "Ada", LastName: "Lovelace", OperatorCode: "A001"}); err == nil {
		t.Fatal("stale MiniPOS coordinator write was accepted")
	}
	if got, getErr := s2.Product(first.ID); getErr != nil || got.SKU != "SKU-1" {
		t.Fatal("MiniPOS coordinator did not reload authoritative state", got, getErr)
	}
	if len(s2.Employees()) != 0 {
		t.Fatal("failed stale employee mutation remained in memory")
	}
	if store.deltaCalls != 2 {
		t.Fatal("MiniPOS coordinator did not use exact delta path", store.deltaCalls)
	}
}
