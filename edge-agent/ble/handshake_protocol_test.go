package ble

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

func TestEncryptedHandshakeReachesReadyAndRejectsReplay(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	client, clientPublic, err := NewEphemeralKey()
	if err != nil {
		t.Fatal(err)
	}
	ticket := Ticket{SessionID: "session-1", TenantID: "tenant-1", RegisterID: "register-1", DeviceID: "device-1", AppInstanceID: "app-1", ClientPublicKey: base64.RawURLEncoding.EncodeToString(clientPublic), Scopes: []string{"fiscal.execute"}, FencingToken: 7, ExpiresAt: time.Now().Add(time.Hour)}
	rawTicket, err := IssueTicket(ticket, key)
	if err != nil {
		t.Fatal(err)
	}
	clientNonce := []byte("0123456789abcdef")
	server := NewHandshakeServer(key, 128, 4)
	challenge, err := server.HandleHello(ControlMessage{Type: "HELLO", ProtocolVersion: BLEProtocolVersion, SessionID: ticket.SessionID, Payload: map[string]any{
		"ticket": rawTicket, "client_nonce": base64.RawURLEncoding.EncodeToString(clientNonce), "ephemeral_public_key": base64.RawURLEncoding.EncodeToString(clientPublic),
	}})
	if err != nil || challenge.Type != "CHALLENGE" {
		t.Fatalf("challenge: %#v %v", challenge, err)
	}
	edgeNonce, _ := decodeHandshakeBytes(challenge.Payload, "edge_nonce", 16)
	edgePublic, _ := decodeHandshakeBytes(challenge.Payload, "ephemeral_public_key", 32)
	ticketDigest := sha256.Sum256([]byte(rawTicket))
	secret, err := client.DeriveSessionSecret(edgePublic, ticketDigest[:], clientNonce, edgeNonce, handshakeContext(ticket))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := SealHandshakeProof(secret, ticket.SessionID, clientNonce, edgeNonce)
	if err != nil {
		t.Fatal(err)
	}
	auth := ControlMessage{Type: "AUTH_PROOF", ProtocolVersion: BLEProtocolVersion, SessionID: ticket.SessionID, Counter: 1, Payload: map[string]any{"ciphertext": base64.RawURLEncoding.EncodeToString(sealed)}}
	ready, err := server.HandleAuthProof(auth)
	if err != nil || ready.Type != "READY" || !server.Ready() || server.Session() == nil {
		t.Fatalf("ready: %#v %v", ready, err)
	}
	if _, err = server.HandleAuthProof(auth); err == nil {
		t.Fatal("AUTH_PROOF replay accepted")
	}
}

func TestHandshakeRejectsWrongBindingScopeAndCorruptedProof(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	for _, scopes := range [][]string{{"fiscal.read"}, {"fiscal.execute"}} {
		_, pub, _ := NewEphemeralKey()
		ticket := Ticket{SessionID: "session-x", TenantID: "tenant", RegisterID: "register", DeviceID: "device", AppInstanceID: "app", ClientPublicKey: base64.RawURLEncoding.EncodeToString(pub), Scopes: scopes, FencingToken: 1, ExpiresAt: time.Now().Add(time.Hour)}
		raw, _ := IssueTicket(ticket, key)
		server := NewHandshakeServer(key, 128, 4)
		challenge, err := server.HandleHello(ControlMessage{Type: "HELLO", ProtocolVersion: BLEProtocolVersion, SessionID: ticket.SessionID, Payload: map[string]any{"ticket": raw, "client_nonce": base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")), "ephemeral_public_key": base64.RawURLEncoding.EncodeToString(pub)}})
		if len(scopes) == 1 && scopes[0] == "fiscal.read" {
			if err == nil {
				t.Fatal("ticket without execute scope accepted")
			}
			continue
		}
		if err != nil || challenge.Type != "CHALLENGE" {
			t.Fatal(err)
		}
		if _, err = server.HandleAuthProof(ControlMessage{Type: "AUTH_PROOF", ProtocolVersion: BLEProtocolVersion, SessionID: ticket.SessionID, Counter: 1, Payload: map[string]any{"ciphertext": base64.RawURLEncoding.EncodeToString(make([]byte, 32))}}); err == nil {
			t.Fatal("corrupted proof accepted")
		}
	}
}

func TestHandshakeRejectsRevokedTicketBeforeChallenge(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	_, public, _ := NewEphemeralKey()
	ticket := Ticket{SessionID: "revoked", TenantID: "tenant", RegisterID: "register", DeviceID: "device", AppInstanceID: "app", ClientPublicKey: base64.RawURLEncoding.EncodeToString(public), Scopes: []string{"fiscal.execute"}, FencingToken: 1, ExpiresAt: time.Now().Add(time.Hour)}
	raw, _ := IssueTicket(ticket, key)
	server := NewHandshakeServer(key, 128, 4)
	server.SetRevocationChecker(func(id string, _ time.Time) bool { return id == "revoked" })
	_, err := server.HandleHello(ControlMessage{Type: "HELLO", ProtocolVersion: BLEProtocolVersion, SessionID: ticket.SessionID, Payload: map[string]any{"ticket": raw, "client_nonce": base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")), "ephemeral_public_key": base64.RawURLEncoding.EncodeToString(public)}})
	if err == nil {
		t.Fatal("revoked session accepted")
	}
}

func TestHandshakeRejectsBearerTicketUsedWithDifferentClientKey(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	_, boundPublic, _ := NewEphemeralKey()
	_, attackerPublic, _ := NewEphemeralKey()
	ticket := Ticket{SessionID: "bound", TenantID: "tenant", RegisterID: "register", DeviceID: "device", AppInstanceID: "app", ClientPublicKey: base64.RawURLEncoding.EncodeToString(boundPublic), Scopes: []string{"fiscal.execute"}, FencingToken: 1, ExpiresAt: time.Now().Add(time.Hour)}
	raw, _ := IssueTicket(ticket, key)
	server := NewHandshakeServer(key, 128, 4)
	_, err := server.HandleHello(ControlMessage{Type: "HELLO", ProtocolVersion: BLEProtocolVersion, SessionID: ticket.SessionID, Payload: map[string]any{"ticket": raw, "client_nonce": base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef")), "ephemeral_public_key": base64.RawURLEncoding.EncodeToString(attackerPublic)}})
	if err == nil {
		t.Fatal("bearer ticket accepted from a different client private key")
	}
}
