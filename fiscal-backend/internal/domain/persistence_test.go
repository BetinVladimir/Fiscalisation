package domain

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

type failingStore struct{ err error }

func (s *failingStore) Load() ([]byte, error) { return nil, nil }
func (s *failingStore) Save([]byte) error     { return s.err }

type typedReadStore struct {
	testStore
	entities map[string][]byte
}

type versionedTestStore struct {
	mu         sync.Mutex
	b          []byte
	generation int64
	deltaCalls int
}

func (s *versionedTestStore) Load() ([]byte, error) {
	b, _, err := s.LoadVersioned()
	return b, err
}
func (s *versionedTestStore) Save(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b, s.generation = append([]byte(nil), b...), s.generation+1
	return nil
}
func (s *versionedTestStore) LoadVersioned() ([]byte, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.b...), s.generation, nil
}
func (s *versionedTestStore) SaveVersioned(b []byte, expected int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if expected != s.generation {
		return s.generation, errors.New("generation conflict")
	}
	s.b, s.generation = append([]byte(nil), b...), s.generation+1
	return s.generation, nil
}
func (s *versionedTestStore) SaveDeltaVersioned(_, current []byte, expected int64) (int64, error) {
	s.mu.Lock()
	s.deltaCalls++
	s.mu.Unlock()
	return s.SaveVersioned(current, expected)
}

func (s *typedReadStore) LoadTenantEntity(collection, tenant, id string) ([]byte, error) {
	raw, ok := s.entities[collection+"\n"+tenant+"\n"+id]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), raw...), nil
}
func (s *typedReadStore) LoadTenantEntities(string, string) ([][]byte, error) {
	return nil, nil
}

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
	registerID, _ := prepareBLERegister(t, svc, "tenant-1")
	sale, e := svc.CreateSale(CreateSale{TenantID: "tenant-1", ExternalID: "external-1", RegisterID: registerID, OperatorID: "A001"})
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
	if e != nil || got.UNP != sale.UNP || got.ExternalID != "external-1" || got.RegisterID != registerID || got.CreatedAt.IsZero() {
		t.Fatalf("%+v %v", got, e)
	}
	unp, e := r2.NextUNP(registerID, "A001", "tenant-1")
	if e != nil || unp == sale.UNP {
		t.Fatalf("unp=%s err=%v", unp, e)
	}
}

func TestUNPSequenceAbsorbsLegacyRegisterKeyWithoutReuse(t *testing.T) {
	r := NewMemoryRepository()
	r.unp["FD000001"] = 41
	unp, err := r.NextUNP("FD000001", "A001", "tenant-1")
	if err != nil || unp != "FD000001-A001-0000042" {
		t.Fatalf("unp=%s err=%v", unp, err)
	}
	if _, legacy := r.unp["FD000001"]; legacy || r.unp["tenant-1\nFD000001"] != 42 {
		t.Fatal("legacy UNP key was not atomically absorbed", r.unp)
	}
}

func TestUNPConcurrentAllocationIsUniqueMonotonicAndTenantScoped(t *testing.T) {
	r := NewMemoryRepository()
	const allocations = 128
	values := make(chan string, allocations)
	errs := make(chan error, allocations)
	var wg sync.WaitGroup
	for i := 0; i < allocations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := r.NextUNP("FD000001", "A001", "tenant-a")
			if err != nil {
				errs <- err
				return
			}
			values <- value
		}()
	}
	wg.Wait()
	close(values)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	unique := map[string]bool{}
	for value := range values {
		if unique[value] {
			t.Fatal("duplicate UNP", value)
		}
		unique[value] = true
	}
	if len(unique) != allocations {
		t.Fatal("lost concurrent UNP allocations", len(unique))
	}
	next, err := r.NextUNP("FD000001", "A001", "tenant-a")
	if err != nil || next != "FD000001-A001-0000129" {
		t.Fatal("UNP sequence reused or skipped after concurrency", next, err)
	}
	other, err := r.NextUNP("FD000001", "A001", "tenant-b")
	if err != nil || other != "FD000001-A001-0000001" {
		t.Fatal("UNP sequence was not tenant scoped", other, err)
	}
}

func TestLegacyArtifactMigrationRollsBackWhenPersistenceFails(t *testing.T) {
	r := NewMemoryRepository()
	r.store = &failingStore{err: errors.New("disk unavailable")}
	r.artifacts["receipt-legacy"] = []byte("immutable")
	if _, err := r.Artifact("receipt-legacy", "tenant-1"); err == nil {
		t.Fatal("migration persistence failure was ignored")
	}
	if string(r.artifacts["receipt-legacy"]) != "immutable" {
		t.Fatal("legacy artifact was not restored")
	}
	if _, exists := r.artifacts["tenant-1\nreceipt-legacy"]; exists {
		t.Fatal("tenant artifact remained after rollback")
	}
}

func TestTenantAwareMutationUsesTypedAuthoritativeRead(t *testing.T) {
	now := time.Now().UTC()
	sale := Sale{ID: "sale-typed", TenantID: "tenant-a", ExternalID: "external-1", RegisterID: "register-1", OperatorID: "A001", State: "DRAFT", Version: 1, Lines: []SaleLine{}, Payments: []PaymentRecord{}, CreatedAt: now, UpdatedAt: now}
	rawSale, _ := json.Marshal(sale)
	snapshot, _ := json.Marshal(repositorySnapshot{Sales: map[string]Sale{sale.ID: sale}, Operations: map[string]Operation{}, Devices: map[string]Device{}, Shifts: map[string]Shift{}, UNP: map[string]int64{}, Replays: map[string]ReplayRecord{}, Outbox: map[string]OutboxItem{}, BLESessions: map[string]BLESessionRecord{}, SyncAcks: map[string]SyncAck{}, ConnectivityProbes: map[string]ConnectivityProbe{}, Resources: map[string]ResourceRecord{}, Artifacts: map[string][]byte{}, Audit: []AuditEvent{}, EdgePending: map[string]EdgePendingCommand{}})
	store := &typedReadStore{testStore: testStore{b: snapshot}, entities: map[string][]byte{"sales\ntenant-a\nsale-typed": rawSale}}
	repo, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(repo, NewSimulator(true))
	line := SaleLine{LineID: "line-1", Name: "Coffee", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}
	if _, err = svc.AddLineForTenant(sale.ID, line, "tenant-b"); err == nil {
		t.Fatal("mutation fell back to compatibility mirror for a foreign tenant")
	}
	if got, err := svc.AddLineForTenant(sale.ID, line, "tenant-a"); err != nil || len(got.Lines) != 1 {
		t.Fatal("authorized typed mutation failed", got, err)
	}
}

func TestFiscalCoordinatorReloadsAfterGenerationConflict(t *testing.T) {
	store := &versionedTestStore{}
	r1, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	first := ResourceRecord{Kind: "device", TenantID: "tenant-1", ID: "device-1", Version: 1, Data: map[string]any{"serial": "A"}, CreatedAt: now, UpdatedAt: now}
	if err = r1.PutResource(first); err != nil {
		t.Fatal(err)
	}
	stale := ResourceRecord{Kind: "register", TenantID: "tenant-1", ID: "register-1", Version: 1, Data: map[string]any{"code": "R1"}, CreatedAt: now, UpdatedAt: now}
	if err = r2.PutResource(stale); err == nil {
		t.Fatal("stale coordinator write was accepted")
	}
	if got, getErr := r2.Resource("device", "device-1"); getErr != nil || got.Data["serial"] != "A" {
		t.Fatal("coordinator did not reload authoritative state", got, getErr)
	}
	if _, getErr := r2.Resource("register", "register-1"); getErr == nil {
		t.Fatal("failed stale mutation remained in memory")
	}
	if store.deltaCalls != 2 {
		t.Fatal("Fiscal coordinator did not use exact delta path", store.deltaCalls)
	}
}
