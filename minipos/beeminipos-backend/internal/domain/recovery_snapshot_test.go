package domain

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

type structuralRecoveryStore struct {
	*webhookFaultStore
	recovery []byte
}

func (s *structuralRecoveryStore) Load() ([]byte, error) {
	if s.saves >= s.failAt && s.failAt > 0 {
		return append([]byte(nil), s.recovery...), nil
	}
	return s.webhookFaultStore.Load()
}
func snapshotObject(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()
	var result map[string]json.RawMessage
	if e := json.Unmarshal(raw, &result); e != nil {
		t.Fatal(e)
	}
	return result
}
func encodeSnapshotObject(t *testing.T, value map[string]json.RawMessage) []byte {
	t.Helper()
	raw, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func structuralFixture(t *testing.T) (*Service, *structuralRecoveryStore, Order) {
	t.Helper()
	s, base, order := webhookBoundaryFixture(t)
	store := &structuralRecoveryStore{webhookFaultStore: base}
	s.store = store
	order.State = "COMPLETED"
	order.ReceiptReference = "receipt-17"
	order.FiscalVersion = 2
	order.Version = 8
	s.orders["order"] = order
	s.mu.Lock()
	e := s.persistLocked()
	s.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	base.saves = 0
	return s, store, order
}

func TestRecoveryRejectsStructurallyCorruptSnapshotAfterFiscalFacts(t *testing.T) {
	for _, kind := range []string{"null", "empty-object", "unrelated", "missing-orders", "missing-core", "missing-sequence", "null-orders", "array-orders"} {
		t.Run(kind, func(t *testing.T) {
			s, store, original := structuralFixture(t)
			candidate := snapshotObject(t, store.b)
			switch kind {
			case "null":
				store.recovery = []byte("null")
			case "empty-object":
				store.recovery = []byte("{}")
			case "unrelated":
				store.recovery = []byte(`{"other":{}}`)
			case "missing-orders":
				delete(candidate, "orders")
			case "missing-core":
				delete(candidate, "products")
			case "missing-sequence":
				delete(candidate, "sequence")
			case "null-orders":
				candidate["orders"] = json.RawMessage("null")
			case "array-orders":
				candidate["orders"] = json.RawMessage("[]")
			}
			if store.recovery == nil {
				store.recovery = encodeSnapshotObject(t, candidate)
			}
			durable := append([]byte(nil), store.b...)
			store.failAt = 1
			changed := original
			changed.State = "REVERSED"
			changed.Version++
			s.orders["order"] = changed
			s.mu.Lock()
			e := s.persistLocked()
			s.mu.Unlock()
			if e == nil {
				t.Fatal("save failure ignored")
			}
			got, _ := s.Order("order")
			if !reflect.DeepEqual(got, original) || !s.persistenceRecoveryPending {
				t.Fatal("invalid shape erased acknowledged fiscal fact", got)
			}
			s.mu.Lock()
			e = s.persistLocked()
			s.mu.Unlock()
			if e == nil || store.saves != 1 || !bytes.Equal(store.b, durable) {
				t.Fatal("invalid recovery permitted overwrite")
			}
			store.failAt = 0
			s.mu.Lock()
			e = s.persistLocked()
			s.mu.Unlock()
			if e == nil || s.persistenceRecoveryPending {
				t.Fatal("healthy recovery should refresh then require retry", e)
			}
			got, _ = s.Order("order")
			if !reflect.DeepEqual(got, original) {
				t.Fatal("recovery lost receipt", got)
			}
		})
	}
}

func TestRecoverySupportsPostgresNineMapsAndEmptyOptionalCompatibility(t *testing.T) {
	s, store, original := structuralFixture(t)
	candidate := snapshotObject(t, store.b)
	for _, key := range []string{"tax_groups", "identity_bindings", "operator_sessions"} {
		delete(candidate, key)
	}
	// This is rebuildSnapshot's real documented projection: nine core maps,
	// sequence, and no three empty historically optional collections.
	if len(candidate) != 10 {
		t.Fatal("PG fixture does not contain9 maps+sequence", len(candidate))
	}
	store.recovery = encodeSnapshotObject(t, candidate)
	store.failAt = 1
	s.orders["order"] = Order{ID: "order", State: "UNKNOWN"}
	s.mu.Lock()
	e := s.persistLocked()
	s.mu.Unlock()
	if e == nil {
		t.Fatal("save error ignored")
	}
	got, _ := s.Order("order")
	if !reflect.DeepEqual(got, original) || s.persistenceRecoveryPending {
		t.Fatal("legitimate PG9 recovery rejected", got)
	}
	// Reverse direction: the domain's full12 maps are valid after PG9 baseline.
	if !validRecoverySnapshotShape(store.b, store.recovery) {
		t.Fatal("full12 map shape incompatible with PG9 baseline")
	}
}
func TestRecoveryCannotOmitPopulatedHistoricalOptionalCollection(t *testing.T) {
	for _, key := range []string{"tax_groups", "identity_bindings", "operator_sessions"} {
		t.Run(key, func(t *testing.T) {
			_, store, _ := structuralFixture(t)
			baseline := snapshotObject(t, store.b)
			baseline[key] = json.RawMessage(`{"existing":{}}`)
			candidate := snapshotObject(t, encodeSnapshotObject(t, baseline))
			delete(candidate, key)
			if validRecoverySnapshotShape(encodeSnapshotObject(t, candidate), encodeSnapshotObject(t, baseline)) {
				t.Fatal("nonempty optional collection silently removed")
			}
		})
	}
}
func TestRecoverySparseLegacyBaselineDoesNotRequireNewCollections(t *testing.T) {
	baseline := []byte(`{"orders":{},"products":{},"employees":{},"shifts":{}}`)
	if !validRecoverySnapshotShape(baseline, baseline) {
		t.Fatal("sparse recognized legacy snapshot rejected")
	}
	if validRecoverySnapshotShape([]byte(`{"products":{},"employees":{},"shifts":{}}`), baseline) {
		t.Fatal("legacy acknowledged orders collection erased")
	}
}
