package domain

import (
	"sync"
	"testing"
)

type testStore struct {
	mu sync.Mutex
	b  []byte
}

func (s *testStore) Load() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.b...), nil
}
func (s *testStore) Save(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b = append([]byte(nil), b...)
	return nil
}
func TestPersistentRepositoryRestoresFiscalStateAndUNP(t *testing.T) {
	store := &testStore{}
	r, e := NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	svc := NewService(r, NewSimulator(true))
	sale, e := svc.CreateSale(CreateSale{ExternalID: "external-1", RegisterID: "FD000001", OperatorID: "A001"})
	if e != nil {
		t.Fatal(e)
	}
	sale, e = svc.AddLine(sale.ID, SaleLine{LineID: "1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if e != nil {
		t.Fatal(e)
	}
	r2, e := NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	got, e := r2.Sale(sale.ID)
	if e != nil || got.UNP != sale.UNP || got.ExternalID != "external-1" || got.RegisterID != "FD000001" || got.CreatedAt.IsZero() {
		t.Fatalf("%+v %v", got, e)
	}
	unp, e := r2.NextUNP("FD000001", "A001")
	if e != nil || unp == sale.UNP {
		t.Fatalf("unp=%s err=%v", unp, e)
	}
}
