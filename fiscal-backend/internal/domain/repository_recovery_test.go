package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
)

type repositoryFaultStore struct {
	mu                          sync.Mutex
	data                        []byte
	calls, failAt               int
	commitThenError, readOutage bool
	readRaw                     []byte
	generation                  int64
}

func (s *repositoryFaultStore) Load() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}
func (s *repositoryFaultStore) loadLocked() ([]byte, error) {
	if s.readOutage && s.calls >= s.failAt {
		return nil, errors.New("recovery read outage")
	}
	if s.readRaw != nil && s.calls >= s.failAt {
		return append([]byte(nil), s.readRaw...), nil
	}
	return append([]byte(nil), s.data...), nil
}
func (s *repositoryFaultStore) Save(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(b)
}
func (s *repositoryFaultStore) saveLocked(b []byte) error {
	s.calls++
	failed := s.calls == s.failAt
	if !failed || s.commitThenError {
		s.data = append([]byte(nil), b...)
		s.generation++
	}
	if failed {
		return errors.New("save outcome uncertain")
	}
	return nil
}

type repositoryVersionedFaultStore struct{ *repositoryFaultStore }

func (s *repositoryVersionedFaultStore) LoadVersioned() ([]byte, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := s.loadLocked()
	return b, s.generation, e
}
func (s *repositoryVersionedFaultStore) SaveVersioned(b []byte, expected int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if expected != s.generation {
		return s.generation, errors.New("generation conflict")
	}
	e := s.saveLocked(b)
	return s.generation, e
}

type repositoryDeltaFaultStore struct{ *repositoryVersionedFaultStore }

func (s *repositoryDeltaFaultStore) SaveDeltaVersioned(_, current []byte, expected int64) (int64, error) {
	return s.SaveVersioned(current, expected)
}
func faultRepository(t *testing.T, kind string) (*MemoryRepository, *repositoryFaultStore, Store) {
	t.Helper()
	base := &repositoryFaultStore{}
	var backing Store = base
	if kind != "plain" {
		v := &repositoryVersionedFaultStore{base}
		backing = v
		if kind == "delta" {
			backing = &repositoryDeltaFaultStore{v}
		}
	}
	r, e := NewPersistentRepository(backing)
	if e != nil {
		t.Fatal(e)
	}
	r.sales["sale"] = Sale{ID: "sale", TenantID: "tenant", RegisterID: "register", OperatorID: "A001", State: "OPEN", Version: 1, UNP: "DT000001-A001-0000001"}
	r.unp["tenant\nDT000001"] = 1
	r.mu.Lock()
	e = r.persistLocked()
	r.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	r, e = NewPersistentRepository(backing)
	if e != nil {
		t.Fatal(e)
	}
	base.calls = 0
	return r, base, backing
}
func recoveryResource() ResourceRecord {
	return ResourceRecord{Kind: "issued_binding", ID: "immutable", TenantID: "tenant", Version: 1, Data: map[string]any{"canonical_json": "{}", "signature": "signed"}}
}
func TestFiscalRepositoryFailedSaveCannotPublishPhantomAuthority(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		t.Run(kind, func(t *testing.T) {
			r, s, _ := faultRepository(t, kind)
			s.failAt = 1
			s.readOutage = true
			v := recoveryResource()
			if e := r.PutResource(v); e == nil {
				t.Fatal("failure ignored")
			}
			if e := r.PutResource(v); e == nil {
				t.Fatal("immutable duplicate published unpersisted authority")
			}
			if _, e := r.Resource(v.Kind, v.ID); e == nil {
				t.Fatal("read exposed unpersisted authority")
			}
			if got := r.Resources(v.Kind, "tenant"); len(got) != 0 {
				t.Fatal("list exposed phantom authority")
			}
			if s.calls != 1 {
				t.Fatal("write while recovery unavailable")
			}
		})
	}
}
func TestFiscalRepositoryClaimReplayCannotReturnPhantomSuccess(t *testing.T) {
	r, s, _ := faultRepository(t, "plain")
	s.failAt = 1
	s.readOutage = true
	if _, _, e := r.ClaimReplay("tenant key", "digest"); e == nil {
		t.Fatal("ClaimReplay turned failed save into successful cached claim")
	}
	if _, ok := r.Replay("tenant key"); ok {
		t.Fatal("phantom replay read")
	}
}
func TestFiscalRepositoryCommittedFinancialFactSurvivesRecoveryOutage(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		for _, committed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/commit=%v", kind, committed), func(t *testing.T) {
				r, s, backing := faultRepository(t, kind)
				s.failAt = 1
				s.commitThenError = committed
				s.readOutage = true
				sale, _ := r.Sale("sale")
				sale.State = "COMPLETED"
				sale.Version++
				sale.FiscalOperationID = "fiscal-operation"
				op := Operation{ID: "fiscal-operation", TenantID: "tenant", SaleID: "sale", Type: "FISCAL_SALE", State: "FISCALIZED", FiscalReference: "receipt-17", Version: 1}
				if e := r.CommitSaleOperation(sale, op); e == nil {
					t.Fatal("failed save ignored")
				}
				durable := append([]byte(nil), s.data...)
				if _, e := r.Sale("sale"); e == nil {
					t.Fatal("uncertain monetary state read as authoritative")
				}
				if e := r.AppendAudit("tenant", "A001", "later", "sale", "sale", sale.UNP, nil, nil); e == nil {
					t.Fatal("outage followup write accepted")
				}
				if !bytes.Equal(durable, s.data) || s.calls != 1 {
					t.Fatal("error-path write overwrote durable fiscal state")
				}
				s.readOutage = false
				s.failAt = 0
				_, _ = r.Sale("sale")
				fresh, e := r.Sale("sale")
				if e != nil {
					t.Fatal(e)
				}
				want := "OPEN"
				if committed {
					want = "COMPLETED"
				}
				if fresh.State != want {
					t.Fatal("wrong recovered fiscal fact", fresh.State, want)
				}
				if committed {
					actual, e := r.Operation(op.ID)
					if e != nil || actual.FiscalReference != "receipt-17" {
						t.Fatal("receipt proof lost", e)
					}
				}
				restart, e := NewPersistentRepository(backing)
				if e != nil {
					t.Fatal(e)
				}
				loaded, e := restart.Sale("sale")
				if e != nil || loaded.State != want {
					t.Fatal("restart mismatch", e)
				}
			})
		}
	}
}
func TestFiscalRepositoryPlainCommittedFirstLineCannotRewindUNP(t *testing.T) {
	r, s, _ := faultRepository(t, "plain")
	s.failAt = 1
	s.commitThenError = true
	v := Sale{ID: "new-sale", TenantID: "tenant", ExternalID: "external-2", RegisterID: "register", OperatorID: "A001"}
	line := SaleLine{LineID: "line", Name: "item", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}}
	if _, e := r.OpenSaleWithFirstLine(v, line, "DT000001"); e == nil {
		t.Fatal("commit timeout ignored")
	}
	s.failAt = 0
	next, e := r.NextUNP("DT000001", "A001", "tenant")
	if e != nil {
		t.Fatal(e)
	}
	if next != "DT000001-A001-0000003" {
		t.Fatal("committed firstline UNP rewound/reused", next)
	}
	got, e := r.Sale("new-sale")
	if e != nil || got.UNP != "DT000001-A001-0000002" {
		t.Fatal("committed sale erased by local rollback", got, e)
	}
}
func TestFiscalRepositoryRecoveryRejectsParseableCorruption(t *testing.T) {
	for _, raw := range []string{"null", "{}", `{"other":{}}`, `{"sales":{}}`} {
		t.Run(raw, func(t *testing.T) {
			r, s, _ := faultRepository(t, "plain")
			s.failAt = 1
			s.readRaw = []byte(raw)
			if e := r.PutResource(recoveryResource()); e == nil {
				t.Fatal("save failure ignored")
			}
			if _, e := r.Sale("sale"); e == nil {
				t.Fatal("corrupt recovery accepted")
			}
			if e := r.PutResource(recoveryResource()); e == nil || s.calls != 1 {
				t.Fatal("corrupt recovery enabled write", e)
			}
		})
	}
}

func TestFiscalRepositoryInitialEmptyRecovery(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		for _, committed := range []bool{false, true} {
			for _, outage := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/committed=%v/outage=%v", kind, committed, outage), func(t *testing.T) {
					base := &repositoryFaultStore{failAt: 1, commitThenError: committed, readOutage: outage}
					var backing Store = base
					if kind != "plain" {
						v := &repositoryVersionedFaultStore{base}
						backing = v
						if kind == "delta" {
							backing = &repositoryDeltaFaultStore{v}
						}
					}
					r, e := NewPersistentRepository(backing)
					if e != nil {
						t.Fatal(e)
					}
					if e = r.PutResource(recoveryResource()); e == nil {
						t.Fatal("save error hidden")
					}
					if outage {
						if e = r.PutResource(recoveryResource()); !errors.Is(e, ErrRepositoryUnavailable) {
							t.Fatal("outage write", e)
						}
					}
					base.readOutage = false
					if outage {
						if e = r.PersistenceReady(); !errors.Is(e, ErrRepositoryUnavailable) {
							t.Fatal("refresh must require retry", e)
						}
					}
					if e = r.PutResource(recoveryResource()); e != nil {
						t.Fatal("healthy retry", e)
					}
					restarted, e := NewPersistentRepository(backing)
					if e != nil {
						t.Fatal(e)
					}
					if _, e = restarted.Resource("issued_binding", "immutable"); e != nil {
						t.Fatal("restart lost authority", e)
					}
				})
			}
		}
	}
}
func TestFiscalRepositoryConstructorTrustBoundary(t *testing.T) {
	for _, raw := range []string{"null", "{}", `{"unrelated":{}}`, `{"sales":null}`, `{"sales":[]}`} {
		if _, e := NewPersistentRepository(&repositoryFaultStore{data: []byte(raw)}); e == nil {
			t.Fatalf("accepted corrupt startup %s", raw)
		}
	}
	for _, raw := range []string{`{"sales":{}}`, `{"devices":{}}`, `{"sales":{},"edge_pending":null,"activation_requests":null,"activation_challenges":null}`} {
		if _, e := NewPersistentRepository(&repositoryFaultStore{data: []byte(raw)}); e != nil {
			t.Fatalf("sparse legacy rejected %s: %v", raw, e)
		}
	}
	for _, kind := range []string{"versioned", "delta"} {
		v := &repositoryVersionedFaultStore{&repositoryFaultStore{generation: 1}}
		var backing Store = v
		if kind == "delta" {
			backing = &repositoryDeltaFaultStore{v}
		}
		if _, e := NewPersistentRepository(backing); e == nil {
			t.Fatal("nonzero generation empty accepted", kind)
		}
	}
}
func TestFiscalRepositoryAuthoritativeNewerGenerationWins(t *testing.T) {
	r, s, backing := faultRepository(t, "versioned")
	s.failAt = 1
	s.readOutage = true
	if e := r.PutResource(recoveryResource()); e == nil {
		t.Fatal("failure hidden")
	}
	s.readOutage = false
	newer, e := NewPersistentRepository(backing)
	if e != nil {
		t.Fatal(e)
	}
	newer.mu.Lock()
	v := newer.sales["sale"]
	v.State = "COMPLETED"
	v.Version = 3
	newer.sales[v.ID] = v
	e = newer.persistLocked()
	newer.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	if e = r.PersistenceReady(); !errors.Is(e, ErrRepositoryUnavailable) {
		t.Fatal(e)
	}
	got, e := r.Sale("sale")
	if e != nil || got.State != "COMPLETED" || got.Version != 3 {
		t.Fatal("newer fact lost", got, e)
	}
	r.mu.RLock()
	generation := r.generation
	r.mu.RUnlock()
	if generation != s.generation {
		t.Fatal("generation not refreshed")
	}
}
func TestFiscalRepositoryRecoveryShapePreservesPopulatedOptional(t *testing.T) {
	r, _, _ := faultRepository(t, "plain")
	r.mu.RLock()
	baseline := append([]byte(nil), r.confirmedSnapshot...)
	r.mu.RUnlock()
	var full map[string]json.RawMessage
	if e := json.Unmarshal(baseline, &full); e != nil {
		t.Fatal(e)
	}
	full["activation_requests"] = json.RawMessage(`{"activation":{"id":"activation"}}`)
	populated, _ := json.Marshal(full)
	delete(full, "activation_requests")
	omitted, _ := json.Marshal(full)
	if validRepositorySnapshotShape(omitted, populated) {
		t.Fatal("populated optional disappeared")
	}
	full["activation_requests"] = json.RawMessage(`null`)
	nullified, _ := json.Marshal(full)
	if validRepositorySnapshotShape(nullified, populated) {
		t.Fatal("populated optional nullified")
	}
}

func TestFiscalRepositoryAcknowledgedUNPNeverReissuedAfterAmbiguousSave(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		t.Run(kind, func(t *testing.T) {
			r, s, backing := faultRepository(t, kind)
			s.failAt = 1
			s.commitThenError = true
			s.readOutage = true
			value, e := r.NextUNP("DT000001", "A001", "tenant")
			if e == nil || value != "" {
				t.Fatal("uncertain UNP was published", value, e)
			}
			if value, e = r.NextUNP("DT000001", "A001", "tenant"); !errors.Is(e, ErrRepositoryUnavailable) || value != "" {
				t.Fatal("outage allocated", value, e)
			}
			s.readOutage = false
			if e = r.PersistenceReady(); !errors.Is(e, ErrRepositoryUnavailable) {
				t.Fatal(e)
			}
			value, e = r.NextUNP("DT000001", "A001", "tenant")
			if e != nil || value != "DT000001-A001-0000003" {
				t.Fatal("committed counter recycled", value, e)
			}
			restarted, e := NewPersistentRepository(backing)
			if e != nil {
				t.Fatal(e)
			}
			value, e = restarted.NextUNP("DT000001", "A001", "tenant")
			if e != nil || value != "DT000001-A001-0000004" {
				t.Fatal("restart recycled counter", value, e)
			}
		})
	}
}
func TestFiscalRepositoryActivationCannotEscapeUncertainPersistence(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		t.Run(kind, func(t *testing.T) {
			r, s, _ := faultRepository(t, kind)
			s.failAt = 1
			s.commitThenError = true
			s.readOutage = true
			if e := r.PutActivationChallenge(DeviceActivationChallenge{ID: "challenge", DeviceInstanceID: "device"}); e == nil {
				t.Fatal("failure hidden")
			}
			if e := r.ConsumeChallengeAndCreateActivation("challenge", "", DeviceActivationRequest{ID: "request"}); !errors.Is(e, ErrRepositoryUnavailable) {
				t.Fatal("activation escaped barrier", e)
			}
			if _, e := r.ActivationRequest("request"); !errors.Is(e, ErrRepositoryUnavailable) {
				t.Fatal("authority read escaped", e)
			}
			if s.calls != 1 {
				t.Fatal("activation wrote during uncertainty")
			}
		})
	}
}
func TestFiscalRepositoryConcurrentReadersAndRecoveryDoNotDeadlock(t *testing.T) {
	r, s, _ := faultRepository(t, "plain")
	s.failAt = 1
	s.readOutage = true
	if e := r.PutResource(recoveryResource()); e == nil {
		t.Fatal("failure hidden")
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				if _, e := r.Sale("sale"); !errors.Is(e, ErrRepositoryUnavailable) {
					t.Errorf("read escaped: %v", e)
				}
				_ = r.Resources("issued_binding", "tenant")
				_ = r.PersistenceReady()
			}
		}()
	}
	wg.Wait()
	if s.calls != 1 {
		t.Fatal("concurrent reads wrote snapshot")
	}
}

type blockingRepositoryReader struct {
	*repositoryFaultStore
	entered, release chan struct{}
}

func (s *blockingRepositoryReader) LoadTenantEntity(_, _, _ string) ([]byte, error) {
	close(s.entered)
	<-s.release
	return []byte(`{"id":"immutable","kind":"issued_binding","tenant_id":"tenant","version":1}`), nil
}
func (s *blockingRepositoryReader) LoadTenantEntities(_, _ string) ([][]byte, error) { return nil, nil }
func TestFiscalRepositoryDirectReaderCannotBypassConcurrentUncertainty(t *testing.T) {
	r, base, _ := faultRepository(t, "plain")
	// Force the direct SQL entity branch using a known tenant-owned cache identity.
	if e := r.PutResource(recoveryResource()); e != nil {
		t.Fatal(e)
	}
	store := &blockingRepositoryReader{base, make(chan struct{}), make(chan struct{})}
	r.store = store
	result := make(chan error, 1)
	go func() { _, e := r.Resource("issued_binding", "immutable"); result <- e }()
	<-store.entered
	base.failAt = base.calls + 1
	base.readOutage = true
	if e := r.AppendAudit("tenant", "actor", "TEST", "sale", "sale", "", nil, nil); e == nil {
		t.Fatal("failure hidden")
	}
	close(store.release)
	if e := <-result; !errors.Is(e, ErrRepositoryUnavailable) {
		t.Fatal("direct reader exposed authority during uncertainty", e)
	}
}

func TestFiscalRepositoryClaimReplayLoadErrorInvalidatesAvailability(t *testing.T) {
	r, store, _ := faultRepository(t, "versioned")
	before := r.PersistenceEpoch()
	store.failAt = 0
	store.readOutage = true
	if _, _, e := r.ClaimReplay("tenant claim", "digest"); e == nil {
		t.Fatal("claim load error hidden")
	}
	if r.PersistenceEpoch() == before {
		t.Fatal("claim load outage did not invalidate observations")
	}
	if _, e := r.Sale("sale"); !errors.Is(e, ErrRepositoryUnavailable) {
		t.Fatal("claim load outage did not fence reads", e)
	}
	if store.calls != 0 {
		t.Fatal("claim read outage performed Save")
	}
	store.readOutage = false
	if e := r.PersistenceReady(); !errors.Is(e, ErrRepositoryUnavailable) {
		t.Fatal(e)
	}
	if _, owner, e := r.ClaimReplay("tenant claim", "digest"); e != nil || !owner {
		t.Fatal("healthy claim retry stuck", owner, e)
	}
}
