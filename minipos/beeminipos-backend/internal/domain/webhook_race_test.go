package domain

import (
	"testing"
)

// TestWebhookBeforeLinkIsNotLostAfterZeroAffectedRows verifies that a fiscal
// webhook that arrives before any order is linked (0 rows affected) is
// persisted in the inbox and not silently discarded.
//
// Invariant: webhook records with "order link not found" are kept for later
// reconciliation, not silently dropped.
func TestWebhookBeforeLinkIsNotLostAfterZeroAffectedRows(t *testing.T) {
	store := &memoryStore{}
	svc, err := NewPersistentService("http://fiscal.test", "v1", store)
	if err != nil {
		t.Fatal(err)
	}

	const eventID = "evt-before-link-0001"
	const tenantID = "tenant-race-1"
	const resourceID = "fiscal-sale-not-yet-linked"

	raw := []byte(`{"tenant_id":"` + tenantID + `","state":"FISCALIZED","operation_id":"op-1"}`)

	// Webhook arrives before any order is linked → "order link not found".
	err = svc.ProcessFiscalWebhookLinkedForTenant(
		tenantID, eventID, raw, resourceID, "", "FISCALIZED", "op-1", 1)
	if err == nil {
		t.Fatal("expected error for unlinked order, got nil")
	}
	if err.Error() != "order link not found" {
		t.Fatalf("unexpected error: %v", err)
	}

	// Despite the error, the webhook inbox record MUST have been persisted.
	svc2, err2 := NewPersistentService("http://fiscal.test", "v1", store)
	if err2 != nil {
		t.Fatal("restart:", err2)
	}
	svc2.mu.RLock()
	record, found := svc2.webhookInbox[eventID]
	svc2.mu.RUnlock()
	if !found {
		t.Fatal("webhook inbox record not found after restart: record was silently lost (invariant violation)")
	}
	if record.EventID != eventID {
		t.Fatalf("wrong event id: %q", record.EventID)
	}
	if record.TenantID != tenantID {
		t.Fatalf("wrong tenant id: %q", record.TenantID)
	}
	if record.ProcessedAt != nil {
		t.Fatal("unprocessed webhook must not have ProcessedAt set")
	}
	if record.Error == "" {
		t.Fatal("unprocessed webhook must carry the error (order link not found)")
	}
	t.Logf("webhook_before_link_not_lost=pass: error=%q", record.Error)
}

// TestWebhookLinkRaceArrivesBeforeAndAfterLink verifies the full race:
//
//  1. Webhook arrives; order not yet linked → persisted with error.
//  2. FiscalSaleID is set on the order (simulates fiscal backend confirming).
//  3. Webhook re-processed (new event ID) → succeeds, order becomes COMPLETED.
func TestWebhookLinkRaceArrivesBeforeAndAfterLink(t *testing.T) {
	store := &memoryStore{}
	svc, err := NewPersistentService("http://fiscal.test", "v1", store)
	if err != nil {
		t.Fatal(err)
	}

	emp, err := svc.CreateEmployee(Employee{
		TenantID: "tenant-race-2", FirstName: "Ana", LastName: "Petrova", OperatorCode: "A001",
	})
	if err != nil {
		t.Fatal(err)
	}
	sh, err := svc.OpenShiftForTenant("00000000-0000-4000-8000-000000000099", emp.ID, "tenant-race-2")
	if err != nil {
		t.Fatal(err)
	}
	order, err := svc.CreateOrder(Order{ShiftID: sh.ID})
	if err != nil {
		t.Fatal(err)
	}

	const resourceID = "fiscal-sale-linked-post-event"

	raw := []byte(`{"tenant_id":"tenant-race-2","state":"FISCALIZED","operation_id":"op-race-2"}`)

	// Phase 1: webhook before FiscalSaleID is set → error, record persisted.
	err = svc.ProcessFiscalWebhookLinkedForTenant(
		"tenant-race-2", "evt-race-pre-link", raw, resourceID, "", "FISCALIZED", "op-race-2", 1)
	if err == nil {
		t.Fatal("want error before link, got nil")
	}

	// Phase 2: set FiscalSaleID on the order (simulates checkout having assigned the sale).
	svc.mu.Lock()
	o := svc.orders[order.ID]
	o.FiscalSaleID = resourceID
	svc.orders[order.ID] = o
	if err = svc.persistLocked(); err != nil {
		svc.mu.Unlock()
		t.Fatal("persist after link:", err)
	}
	svc.mu.Unlock()

	// Phase 3: retry webhook (new event ID, same resourceID) → must succeed.
	err = svc.ProcessFiscalWebhookLinkedForTenant(
		"tenant-race-2", "evt-race-post-link", raw, resourceID, "", "FISCALIZED", "op-race-2", 1)
	if err != nil {
		t.Fatalf("webhook after link: %v", err)
	}

	// Verify order is now COMPLETED.
	linked, err := svc.Order(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if linked.State != "COMPLETED" {
		t.Fatalf("expected COMPLETED after webhook post-link, got %q", linked.State)
	}
	t.Log("webhook_link_race_before_and_after=pass")
}
