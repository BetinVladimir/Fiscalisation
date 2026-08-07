package ble

import (
	"testing"
	"time"
)

func TestTicketExpirySignatureAndReplay(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	raw, e := IssueTicket(Ticket{SessionID: "s", TenantID: "t", RegisterID: "r", DeviceID: "d", ExpiresAt: time.Now().Add(time.Hour)}, key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ParseTicket(raw, key, time.Now()); e != nil {
		t.Fatal(e)
	}
	if _, e = ParseTicket(raw, []byte("wrong"), time.Now()); e == nil {
		t.Fatal("signature accepted")
	}
	s, e := NewSession(key)
	if e != nil {
		t.Fatal(e)
	}
	c, b := s.Seal([]byte("command"), []byte("header"))
	if p, e := s.Open(c, b, []byte("header")); e != nil || string(p) != "command" {
		t.Fatal(string(p), e)
	}
	if _, e = s.Open(c, b, []byte("header")); e == nil {
		t.Fatal("replay accepted")
	}
}

func TestDirectionalEndpoints(t *testing.T) {
	client, e := NewEndpoint([]byte("pairing-secret"), "session-1", "client")
	if e != nil {
		t.Fatal(e)
	}
	edge, e := NewEndpoint([]byte("pairing-secret"), "session-1", "edge")
	if e != nil {
		t.Fatal(e)
	}
	c, b := client.Seal([]byte("sale"), []byte("command"))
	p, e := edge.Open(c, b, []byte("command"))
	if e != nil || string(p) != "sale" {
		t.Fatalf("client->edge %q %v", p, e)
	}
	c, b = edge.Seal([]byte("result"), []byte("response"))
	p, e = client.Open(c, b, []byte("response"))
	if e != nil || string(p) != "result" {
		t.Fatalf("edge->client %q %v", p, e)
	}
}
