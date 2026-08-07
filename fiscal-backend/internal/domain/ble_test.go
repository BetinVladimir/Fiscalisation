package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestBLETicketSignatureRefreshRevokePersistence(t *testing.T) {
	store := &testStore{}
	repo, e := NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	s := NewService(repo, NewSimulator(true))
	key := "01234567890123456789012345678901"
	s.SetBLESigningKey(key)
	v, e := s.BLESession("r1", "A001", "app1", "tenant1")
	if e != nil {
		t.Fatal(e)
	}
	raw := v["signed_session_ticket"].(string)
	outer, e := base64.RawURLEncoding.DecodeString(raw)
	if e != nil {
		t.Fatal(e)
	}
	var wrapped struct {
		Payload   string `json:"payload"`
		Signature string `json:"signature"`
	}
	if json.Unmarshal(outer, &wrapped) != nil {
		t.Fatal("wrapper")
	}
	payload, _ := base64.RawURLEncoding.DecodeString(wrapped.Payload)
	sig, _ := base64.RawURLEncoding.DecodeString(wrapped.Signature)
	m := hmac.New(sha256.New, []byte(key))
	m.Write(payload)
	if !hmac.Equal(sig, m.Sum(nil)) {
		t.Fatal("bad signature")
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil || claims["TenantID"] != "tenant1" {
		t.Fatalf("%s", payload)
	}
	id := v["ble_session_id"].(string)
	repo, e = NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	s = NewService(repo, NewSimulator(true))
	s.SetBLESigningKey(key)
	if _, e = s.RefreshBLE(id, "tenant2"); e == nil {
		t.Fatal("cross tenant refresh")
	}
	if e = s.RevokeBLE(id, "tenant1"); e != nil {
		t.Fatal(e)
	}
	pending := s.PendingOutbox(time.Now().UTC().Add(time.Second))
	if len(pending) != 1 || pending[0].Event.EventType != "ble.session.revoked" || pending[0].Event.ResourceID != id {
		t.Fatalf("revocation event missing: %#v", pending)
	}
	if _, e = s.RefreshBLE(id, "tenant1"); e == nil {
		t.Fatal("revoked refresh accepted")
	}
}
