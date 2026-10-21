package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type webhookFaultStore struct {
	b               []byte
	saves, failAt   int
	failReload      bool
	reloadMode      string
	commitThenError bool
	committed       []byte
}

func (s *webhookFaultStore) Load() ([]byte, error) {
	if s.failReload && s.saves >= s.failAt {
		switch s.reloadMode {
		case "empty":
			return nil, nil
		case "corrupt":
			return []byte("invalid snapshot"), nil
		}
		return nil, errors.New("injected reload outage")
	}
	return append([]byte(nil), s.b...), nil
}
func (s *webhookFaultStore) Save(b []byte) error {
	s.saves++
	if s.saves == s.failAt {
		if s.commitThenError {
			s.b = append([]byte(nil), b...)
			s.committed = append([]byte(nil), b...)
		}
		return errors.New("injected save outage")
	}
	s.b = append([]byte(nil), b...)
	return nil
}
func webhookBoundaryFixture(t *testing.T) (*Service, *webhookFaultStore, Order) {
	t.Helper()
	store := &webhookFaultStore{}
	s, e := NewPersistentService("unused", "v1", store)
	if e != nil {
		t.Fatal(e)
	}
	o := Order{ID: "order", TenantID: "tenant", ExternalID: "external", State: "FISCAL_PENDING", FiscalSaleID: "sale", CheckoutOperationID: "operation", FiscalOperationID: "operation", Version: 7, FiscalVersion: 1}
	s.orders[o.ID] = o
	s.mu.Lock()
	e = s.persistLocked()
	s.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	store.saves = 0
	return s, store, o
}
func invokeWebhookBoundary(s *Service, wrapper string) error {
	raw := []byte(`{"tenant_id":"tenant","event":"receipt"}`)
	switch wrapper {
	case "plain":
		return s.ProcessFiscalWebhook("event", raw, "sale", "FISCALIZED", "operation", 2)
	case "linked":
		return s.ProcessFiscalWebhookLinked("event", raw, "sale", "external", "FISCALIZED", "operation", 2)
	case "tenant":
		return s.ProcessFiscalWebhookLinkedForTenant("tenant", "event", raw, "sale", "external", "FISCALIZED", "operation", 2)
	default:
		return s.ProcessFiscalWebhookLinkedForTenantWithReference("tenant", "event", raw, "sale", "external", "FISCALIZED", "operation", "receipt-17", 2)
	}
}
func TestWebhookPersistenceStageFailureDoesNotInventFiscalState(t *testing.T) {
	for _, wrapper := range []string{"plain", "linked", "tenant", "reference"} {
		for _, stage := range []int{1, 2, 3} {
			t.Run(wrapper+string(rune('0'+stage)), func(t *testing.T) {
				s, store, before := webhookBoundaryFixture(t)
				store.failAt = stage
				if invokeWebhookBoundary(s, wrapper) == nil {
					t.Fatal("save failure ignored")
				}
				got, _ := s.Order("order")
				if stage < 3 && !reflect.DeepEqual(got, before) {
					t.Fatal("unpersisted fiscal state escaped", got)
				}
				if stage == 3 && (got.State != "COMPLETED" || got.Version != 8 || got.FiscalVersion != 2) {
					t.Fatal("durable fiscal fact lost", got)
				}
				if record, ok := s.webhookInbox["event"]; ok && record.ProcessedAt != nil {
					t.Fatal("failed stage marked processed")
				}
				restart, e := NewPersistentService("unused", "v1", store)
				if e != nil {
					t.Fatal(e)
				}
				reloaded, _ := restart.Order("order")
				if !reflect.DeepEqual(got, reloaded) {
					t.Fatal("memory differs from persistence")
				}
				store.failAt = 0
				store.saves = 0
				if e = invokeWebhookBoundary(s, wrapper); e != nil {
					t.Fatal(e)
				}
				got, _ = s.Order("order")
				if got.State != "COMPLETED" || got.Version != 8 || got.FiscalVersion != 2 {
					t.Fatal("retry did not apply exactly once", got)
				}
				persisted := append([]byte(nil), store.b...)
				if e = invokeWebhookBoundary(s, wrapper); e != nil {
					t.Fatal(e)
				}
				if !bytes.Equal(persisted, store.b) {
					t.Fatal("processed replay wrote again")
				}
			})
		}
	}
}
func TestWebhookProjectionSaveAndReloadOutageDoesNotInventSuccess(t *testing.T) {
	for _, mode := range []string{"error", "empty", "corrupt"} {
		for _, stage := range []int{1, 2, 3} {
			t.Run(mode+string(rune('0'+stage)), func(t *testing.T) {
				s, store, before := webhookBoundaryFixture(t)
				store.failAt = stage
				store.failReload = true
				store.reloadMode = mode
				if invokeWebhookBoundary(s, "reference") == nil {
					t.Fatal("save failure ignored")
				}
				got, _ := s.Order("order")
				if stage < 3 && !reflect.DeepEqual(got, before) {
					t.Fatal("failed persist/reload left invented completion", got)
				}
				if stage == 3 && (got.State != "COMPLETED" || got.ReceiptReference != "receipt-17" || got.FiscalVersion != 2 || got.Version != 8) {
					t.Fatal("successful fiscal fact lost after inbox persistence failure", got)
				}
				var persisted struct {
					Orders map[string]Order `json:"orders"`
				}
				if e := json.Unmarshal(store.b, &persisted); e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(got, persisted.Orders["order"]) {
					t.Fatal("failed mutation was persisted by error recording", got, persisted.Orders["order"])
				}
				if inbox := s.webhookInbox["event"]; inbox.ProcessedAt != nil {
					t.Fatal("unpersisted inbox completion escaped")
				}
				store.failAt = 0
				store.failReload = false
				store.saves = 0
				_ = invokeWebhookBoundary(s, "reference")
				if e := invokeWebhookBoundary(s, "reference"); e != nil {
					t.Fatal(e)
				}
				got, _ = s.Order("order")
				if got.State != "COMPLETED" || got.Version != 8 || got.FiscalVersion != 2 {
					t.Fatal("retry failed exactly-once projection", got)
				}
				restart, e := NewPersistentService("unused", "v1", store)
				if e != nil {
					t.Fatal(e)
				}
				reloaded, _ := restart.Order("order")
				if !reflect.DeepEqual(got, reloaded) || restart.webhookInbox["event"].ProcessedAt == nil {
					t.Fatal("restart lost fiscal/inbox fact")
				}
			})
		}
	}
}

type webhookVersionedFaultStore struct {
	webhookFaultStore
	generation int64
}

func (s *webhookVersionedFaultStore) LoadVersioned() ([]byte, int64, error) {
	b, e := s.Load()
	g := s.generation
	if s.failReload && s.saves >= s.failAt {
		g = 999
	}
	return b, g, e
}
func (s *webhookVersionedFaultStore) SaveVersioned(b []byte, expected int64) (int64, error) {
	if expected != s.generation {
		return s.generation, errors.New("generation conflict")
	}
	e := s.Save(b)
	if e == nil || (s.saves == s.failAt && s.commitThenError) {
		s.generation++
	}
	return s.generation, e
}
func TestWebhookFailedReloadDoesNotAdoptInvalidGeneration(t *testing.T) {
	for _, mode := range []string{"error", "empty", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			store := &webhookVersionedFaultStore{}
			s, e := NewPersistentService("unused", "v1", store)
			if e != nil {
				t.Fatal(e)
			}
			s.orders["order"] = Order{ID: "order", TenantID: "tenant", State: "FISCAL_PENDING", FiscalSaleID: "sale", CheckoutOperationID: "operation", Version: 7}
			s.mu.Lock()
			e = s.persistLocked()
			s.mu.Unlock()
			if e != nil {
				t.Fatal(e)
			}
			store.saves = 0
			store.failAt = 2
			store.failReload = true
			store.reloadMode = mode
			if invokeWebhookBoundary(s, "reference") == nil {
				t.Fatal("save failure ignored")
			}
			if s.generation != store.generation {
				t.Fatal("invalid recovery generation adopted", s.generation, store.generation)
			}
			got, _ := s.Order("order")
			if got.State != "FISCAL_PENDING" || got.Version != 7 {
				t.Fatal("failed versioned mutation escaped", got)
			}
			store.failAt = 0
			store.failReload = false
			_ = invokeWebhookBoundary(s, "reference")
			if e = invokeWebhookBoundary(s, "reference"); e != nil {
				t.Fatal(e)
			}
			got, _ = s.Order("order")
			if got.State != "COMPLETED" || got.Version != 8 {
				t.Fatal(got)
			}
		})
	}
}

func TestWebhookAmbiguousCommittedSaveBlocksWritesUntilRecovery(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		for _, stage := range []int{1, 2, 3} {
			for _, mode := range []string{"error", "empty", "corrupt"} {
				t.Run(fmt.Sprintf("store=%s/stage=%d/%s", kind, stage, mode), func(t *testing.T) {
					base := &webhookFaultStore{}
					var store Store = base
					if kind != "plain" {
						versionedStore := &webhookVersionedFaultStore{webhookFaultStore: *base}
						store = versionedStore
						if kind == "delta" {
							store = &webhookDeltaFaultStore{versionedStore}
						}
						base = &versionedStore.webhookFaultStore
					}
					s, e := NewPersistentService("unused", "v1", store)
					if e != nil {
						t.Fatal(e)
					}
					s.orders["order"] = Order{ID: "order", TenantID: "tenant", ExternalID: "external", State: "FISCAL_PENDING", FiscalSaleID: "sale", CheckoutOperationID: "operation", Version: 7, FiscalVersion: 1}
					s.mu.Lock()
					e = s.persistLocked()
					s.mu.Unlock()
					if e != nil {
						t.Fatal(e)
					}
					base.saves = 0
					base.failAt = stage
					base.commitThenError = true
					base.failReload = true
					base.reloadMode = mode
					if invokeWebhookBoundary(s, "reference") == nil {
						t.Fatal("ambiguous save must return error")
					}
					if !bytes.Equal(base.b, base.committed) {
						t.Fatal("committed state overwritten during error recording")
					}
					if e = invokeWebhookBoundary(s, "reference"); e == nil {
						t.Fatal("outage retry must remain uncertain")
					}
					if !bytes.Equal(base.b, base.committed) || base.saves != stage {
						t.Fatal("write attempted before authoritative recovery", base.saves, stage)
					}
					base.failReload = false
					base.failAt = 0
					base.commitThenError = false
					// Recovery refresh discards speculative memory and asks caller to retry.
					_ = invokeWebhookBoundary(s, "reference")
					if e = invokeWebhookBoundary(s, "reference"); e != nil {
						t.Fatal("retry after recovery", e)
					}
					got, _ := s.Order("order")
					if got.State != "COMPLETED" || got.ReceiptReference != "receipt-17" || got.Version != 8 || got.FiscalVersion != 2 {
						t.Fatal("committed fiscal proof lost or reapplied", got)
					}
					restart, e := NewPersistentService("unused", "v1", store)
					if e != nil {
						t.Fatal(e)
					}
					loaded, _ := restart.Order("order")
					if !reflect.DeepEqual(got, loaded) || restart.webhookInbox["event"].ProcessedAt == nil {
						t.Fatal("restart proof/inbox mismatch")
					}
					count := base.saves
					if e = invokeWebhookBoundary(restart, "reference"); e != nil || base.saves != count {
						t.Fatal("processed replay wrote again", e)
					}
				})
			}
		}
	}
}

func initialWebhookStore(kind string) (Store, *webhookFaultStore) {
	base := &webhookFaultStore{}
	if kind == "plain" {
		return base, base
	}
	versioned := &webhookVersionedFaultStore{}
	base = &versioned.webhookFaultStore
	if kind == "delta" {
		return &webhookDeltaFaultStore{versioned}, base
	}
	return versioned, base
}
func TestInitialEmptySnapshotAmbiguousSaveCannotEraseCommittedOrder(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		t.Run(kind, func(t *testing.T) {
			backing, store := initialWebhookStore(kind)
			store.failAt = 1
			store.commitThenError = true
			store.failReload = true
			s, e := NewPersistentService("unused", "v1", backing)
			if e != nil {
				t.Fatal(e)
			}
			original := Order{ID: "order", TenantID: "tenant", State: "COMPLETED", FiscalSaleID: "sale", ReceiptReference: "receipt", Version: 1}
			s.orders["order"] = original
			s.mu.Lock()
			e = s.persistLocked()
			s.mu.Unlock()
			if e == nil {
				t.Fatal("save timeout ignored")
			}
			durable := append([]byte(nil), store.b...)
			s.mu.Lock()
			e = s.persistLocked()
			s.mu.Unlock()
			if e == nil || !bytes.Equal(durable, store.b) || store.saves != 1 {
				t.Fatal("empty fallback erased committed first write", e)
			}
			store.failReload = false
			store.failAt = 0
			s.mu.Lock()
			e = s.persistLocked()
			s.mu.Unlock()
			if e == nil {
				t.Fatal("recovery refresh must require retry")
			}
			got, _ := s.Order("order")
			if !reflect.DeepEqual(got, original) {
				t.Fatal("authoritative first write lost", got)
			}
			restart, e := NewPersistentService("unused", "v1", backing)
			if e != nil {
				t.Fatal(e)
			}
			got, _ = restart.Order("order")
			if !reflect.DeepEqual(got, original) {
				t.Fatal("restart lost committed first write", got)
			}
		})
	}
}
func TestInitialEmptySnapshotDefiniteSaveFailureCanRecoverWithoutRestart(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		for _, mode := range []string{"healthy-empty", "error", "corrupt"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				backing, store := initialWebhookStore(kind)
				store.failAt = 1
				store.failReload = mode != "healthy-empty"
				store.reloadMode = mode
				s, e := NewPersistentService("unused", "v1", backing)
				if e != nil {
					t.Fatal(e)
				}
				original := Order{ID: "order", TenantID: "tenant", State: "FISCAL_PENDING", Version: 1}
				s.orders["order"] = original
				s.mu.Lock()
				e = s.persistLocked()
				s.mu.Unlock()
				if e == nil {
					t.Fatal("definite save failure ignored")
				}
				if _, ok := s.orders["order"]; ok {
					t.Fatal("first failed write escaped rollback")
				}
				if s.snapshotEstablished {
					t.Fatal("synthetic empty fallback became acknowledged baseline")
				}
				store.failReload = false
				store.failAt = 0
				if mode != "healthy-empty" {
					s.mu.Lock()
					e = s.persistLocked()
					s.mu.Unlock()
					if e == nil {
						t.Fatal("authoritative refresh requires retry")
					}
					if s.persistenceRecoveryPending {
						t.Fatal("legitimate empty generation0 failed to unblock recovery")
					}
					if store.saves != 1 {
						t.Fatal("recovery refresh wrote speculative state")
					}
				}
				s.orders["order"] = original
				s.mu.Lock()
				e = s.persistLocked()
				s.mu.Unlock()
				if e != nil {
					t.Fatal("healthy initial empty store remained blocked", e)
				}
				restart, e := NewPersistentService("unused", "v1", backing)
				if e != nil {
					t.Fatal(e)
				}
				got, _ := restart.Order("order")
				if !reflect.DeepEqual(got, original) {
					t.Fatal("retry/restart lost first acknowledged write", got)
				}
			})
		}
	}
}

type webhookDeltaFaultStore struct{ *webhookVersionedFaultStore }

func (s *webhookDeltaFaultStore) SaveDeltaVersioned(_, current []byte, expected int64) (int64, error) {
	return s.SaveVersioned(current, expected)
}

func TestInitialEmptyNonzeroGenerationCannotProveAbsence(t *testing.T) {
	for _, kind := range []string{"versioned", "delta"} {
		t.Run(kind, func(t *testing.T) {
			backing, store := initialWebhookStore(kind)
			switch v := backing.(type) {
			case *webhookVersionedFaultStore:
				v.generation = 7
			case *webhookDeltaFaultStore:
				v.generation = 7
			}
			s, e := NewPersistentService("unused", "v1", backing)
			if e != nil {
				t.Fatal(e)
			}
			store.failAt = 1
			s.orders["order"] = Order{ID: "order", Version: 1}
			s.mu.Lock()
			e = s.persistLocked()
			s.mu.Unlock()
			if e == nil || !s.persistenceRecoveryPending {
				t.Fatal("nonzero generation with empty payload accepted as authoritative absence", e)
			}
			store.failAt = 0
			s.mu.Lock()
			e = s.persistLocked()
			s.mu.Unlock()
			if e == nil || !s.persistenceRecoveryPending || store.saves != 1 {
				t.Fatal("invalid empty recovery permitted write", e)
			}
		})
	}
}
