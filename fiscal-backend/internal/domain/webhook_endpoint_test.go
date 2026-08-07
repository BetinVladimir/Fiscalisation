package domain

import "testing"

func TestWebhookLifecycleIsTenantScopedRedactedAndPersistent(t *testing.T) {
	store := &testStore{}
	repo, _ := NewPersistentRepository(store)
	s := NewService(repo, NewSimulator(true))
	created, err := s.CreateWebhookEndpoint("tenant-a", map[string]any{"url": "https://hooks.example.test/fiscal", "events": []any{"fiscal.operation.updated"}})
	if err != nil || created["secret"] == "" {
		t.Fatalf("create: %#v %v", created, err)
	}
	id := created["id"].(string)
	got, err := s.GetWebhookEndpoint(id, "tenant-a")
	if err != nil || got["secret"] != nil {
		t.Fatalf("GET leaked secret: %#v %v", got, err)
	}
	if _, err = s.GetWebhookEndpoint(id, "tenant-b"); err == nil {
		t.Fatal("cross-tenant endpoint leaked")
	}
	rotated, err := s.RotateWebhookSecret(id, "tenant-a")
	if err != nil || rotated["secret"] == created["secret"] {
		t.Fatalf("rotation failed: %#v %v", rotated, err)
	}
	repo, err = NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	s = NewService(repo, NewSimulator(true))
	if len(s.WebhookDeliveryEndpoints("tenant-a", "fiscal.operation.updated")) != 1 {
		t.Fatal("endpoint did not survive restart")
	}
	if err = s.DisableWebhookEndpoint(id, "tenant-a"); err != nil || len(s.WebhookDeliveryEndpoints("tenant-a", "fiscal.operation.updated")) != 0 {
		t.Fatal("disabled endpoint remains deliverable")
	}
}

func TestWebhookRejectsSSRFAndUnsupportedEvents(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	bad := []map[string]any{
		{"url": "http://example.test/hook", "events": []any{"fiscal.operation.updated"}},
		{"url": "https://127.0.0.1/hook", "events": []any{"fiscal.operation.updated"}},
		{"url": "https://hooks.example.test/hook", "events": []any{"unknown.event"}},
	}
	for _, input := range bad {
		if _, err := s.CreateWebhookEndpoint("tenant", input); err == nil {
			t.Fatalf("unsafe endpoint accepted: %#v", input)
		}
	}
}

func TestWebhookAllowsBLERevocationSubscription(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	v, err := s.CreateWebhookEndpoint("tenant", map[string]any{"url": "https://edge.example.test/control", "events": []any{"ble.session.revoked"}})
	if err != nil || v["secret"] == "" || len(s.WebhookDeliveryEndpoints("tenant", "ble.session.revoked")) != 1 {
		t.Fatalf("revocation subscription rejected: %#v %v", v, err)
	}
}
