package domain

import (
	"testing"
)

func lookupFixture() (*Service, Order, CheckoutLookup) {
	s := NewService("https://unused", "v")
	order := Order{ID: "order", TenantID: "tenant", Version: 4, State: "UNKNOWN", CheckoutOperationID: "op", ReceiptSessionID: "receipt", RegisterID: "register"}
	s.orders[order.ID] = order
	return s, order, CheckoutLookup{OperationID: "op", ReceiptSessionID: "receipt", RegisterID: "register", SaleID: "sale", State: "FISCALIZED", FiscalReference: "receipt17", Version: 2}
}
func TestValidatedCheckoutLookupBindsProjectsAndReplays(t *testing.T) {
	s, order, proof := lookupFixture()
	updated, err := s.ApplyValidatedCheckoutLookup("tenant", order, proof)
	if err != nil || updated.State != "COMPLETED" || updated.FiscalSaleID != "sale" || updated.ReceiptReference != "receipt17" || updated.Version != 5 {
		t.Fatalf("%+v %v", updated, err)
	}
	again, err := s.ApplyValidatedCheckoutLookup("tenant", updated, proof)
	if err != nil || again.Version != updated.Version {
		t.Fatalf("non-idempotent %+v %v", again, err)
	}
	if err = s.applyFiscalEventLinkedForTenant("tenant", "sale", "", "UNKNOWN", "op", "", 3); err == nil {
		t.Fatal("delayed unknown webhook regressed proof")
	}
	if got := s.orders[order.ID]; got.State != "COMPLETED" || got.ReceiptReference != "receipt17" {
		t.Fatalf("proof lost %+v", got)
	}
}
func TestValidatedCheckoutLookupCASAndTerminalConflicts(t *testing.T) {
	for _, scenario := range []string{"version", "reversed", "reference", "same version", "tenant"} {
		t.Run(scenario, func(t *testing.T) {
			s, order, proof := lookupFixture()
			current := order
			tenant := "tenant"
			switch scenario {
			case "version":
				current.Version++
			case "reversed":
				current.State = "REVERSED"
				current.ReversalOperationID = "reverse-op"
			case "reference":
				current.State = "COMPLETED"
				current.ReceiptReference = "other"
			case "same version":
				current.FiscalVersion = proof.Version
			case "tenant":
				tenant = "foreign"
			}
			s.orders[order.ID] = current
			got, err := s.ApplyValidatedCheckoutLookup(tenant, order, proof)
			if scenario == "reversed" {
				if err != nil || got.State != "REVERSED" {
					t.Fatal(got, err)
				}
			} else if err == nil {
				t.Fatal("accepted stale/conflicting proof")
			}
			if stored := s.orders[order.ID]; stored.State != current.State || stored.Version != current.Version || stored.ReceiptReference != current.ReceiptReference {
				t.Fatal("mutated after conflict")
			}
		})
	}
}
func TestValidatedLookupDoesNotUndoReversalWebhook(t *testing.T) {
	s, order, proof := lookupFixture()
	updated, err := s.ApplyValidatedCheckoutLookup("tenant", order, proof)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyFiscalEventLinkedForTenant("tenant", "sale", "", "REVERSED", "reverse-op", "refund", 3); err != nil {
		t.Fatal(err)
	}
	_, err = s.ApplyValidatedCheckoutLookup("tenant", updated, proof)
	if err == nil {
		t.Fatal("stale read should retry after version changed")
	}
	fresh := s.orders[order.ID]
	if fresh.State != "REVERSED" || fresh.Version <= updated.Version {
		t.Fatal(fresh)
	}
	if err = s.applyFiscalEventLinkedForTenant("tenant", "sale", "", "FISCALIZED", "op", "receipt17", 4); err != nil {
		t.Fatal(err)
	}
	if s.orders[order.ID].State != "REVERSED" {
		t.Fatal("old checkout resurrected reversed sale")
	}
}

func TestValidatedLookupCrossInstanceCASRetainsNewerWebhook(t *testing.T) {
	seed, order, proof := lookupFixture()
	store := &versionedMemoryStore{}
	seed.store = store
	if err := seed.persistLocked(); err != nil {
		t.Fatal(err)
	}
	a, err := NewPersistentService("unused", "v", store)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewPersistentService("unused", "v", store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.ApplyValidatedCheckoutLookup("tenant", order, proof); err != nil {
		t.Fatal(err)
	}
	if err = b.applyFiscalEventLinkedForTenant("tenant", "sale", "", "REVERSED", "reversal", "refund", 3); err != nil {
		t.Fatal(err)
	}
	if _, err = a.ApplyValidatedCheckoutLookup("tenant", order, proof); err == nil {
		t.Fatal("stale service should fail generation CAS")
	}
	got := a.orders[order.ID]
	if got.State != "REVERSED" || got.ReversalFiscalReference != "refund" {
		t.Fatalf("restored latest snapshot was overwritten %+v", got)
	}
	fresh, err := NewPersistentService("unused", "v", store)
	if err != nil || fresh.orders[order.ID].State != "REVERSED" {
		t.Fatal("durable state regressed", err)
	}
}
func TestValidatedLookupPersistenceFailureRollsBack(t *testing.T) {
	s, order, proof := lookupFixture()
	store := &memoryStore{}
	s.store = store
	if err := s.persistLocked(); err != nil {
		t.Fatal(err)
	}
	store.fail = true
	if _, err := s.ApplyValidatedCheckoutLookup("tenant", order, proof); err == nil {
		t.Fatal("accepted failed durable projection")
	}
	if got := s.orders[order.ID]; got.FiscalSaleID != "" || got.State != "UNKNOWN" || got.Version != order.Version {
		t.Fatal("failed projection leaked", got)
	}
}

func TestValidatedLookupCanCorrectUnprovenFailedProjection(t *testing.T) {
	s, order, proof := lookupFixture()
	order.State = "FAILED"
	s.orders[order.ID] = order
	got, err := s.ApplyValidatedCheckoutLookup("tenant", order, proof)
	if err != nil || got.State != "COMPLETED" {
		t.Fatal("unproven FAILED blocked positive proof", got, err)
	}
}
func TestValidatedLookupRetainsPriorSuccessWhenReadIsUnknown(t *testing.T) {
	s, order, proof := lookupFixture()
	good, err := s.ApplyValidatedCheckoutLookup("tenant", order, proof)
	if err != nil {
		t.Fatal(err)
	}
	proof.State = "UNKNOWN"
	proof.FiscalReference = ""
	proof.Version++
	got, err := s.ApplyValidatedCheckoutLookup("tenant", good, proof)
	if err != nil || got.State != "COMPLETED" || got.ReceiptReference != "receipt17" {
		t.Fatal(got, err)
	}
}
