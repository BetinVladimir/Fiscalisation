package ble

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestX25519HKDFHandshake(t *testing.T) {
	client, clientPublic, err := NewEphemeralKey()
	if err != nil {
		t.Fatal(err)
	}
	edge, edgePublic, err := NewEphemeralKey()
	if err != nil {
		t.Fatal(err)
	}
	ticket := sha256.Sum256([]byte("signed-ticket"))
	clientNonce := bytes.Repeat([]byte{1}, 16)
	edgeNonce := bytes.Repeat([]byte{2}, 16)
	context := "tenant-1|register-1|device-1|session-1|2026-08-07"
	a, err := client.DeriveSessionSecret(edgePublic, ticket[:], clientNonce, edgeNonce, context)
	if err != nil {
		t.Fatal(err)
	}
	b, err := edge.DeriveSessionSecret(clientPublic, ticket[:], clientNonce, edgeNonce, context)
	if err != nil || !bytes.Equal(a, b) {
		t.Fatal("peers derived different secrets", err)
	}
	c, _ := edge.DeriveSessionSecret(clientPublic, ticket[:], clientNonce, edgeNonce, context+"-other")
	if bytes.Equal(a, c) {
		t.Fatal("binding context did not affect key")
	}
}
