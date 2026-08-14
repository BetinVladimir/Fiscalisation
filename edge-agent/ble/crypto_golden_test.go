package ble

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestGoTypeScriptEsp32CryptoGoldenVector(t *testing.T) {
	var v map[string]string
	raw, err := os.ReadFile("../../contracts/ble-crypto-golden-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	decode := func(name string) []byte {
		b, e := hex.DecodeString(v[name])
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	private, err := ecdh.X25519().NewPrivateKey(decode("private"))
	if err != nil {
		t.Fatal(err)
	}
	peer, err := ecdh.X25519().NewPublicKey(decode("peer_public"))
	if err != nil {
		t.Fatal(err)
	}
	shared, err := private.ECDH(peer)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(shared) != v["shared"] {
		t.Fatal("X25519 mismatch")
	}
	key, err := hkdf.Key(sha256.New, shared, decode("salt"), string(decode("info")), 32)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(key) != v["key"] {
		t.Fatal("HKDF mismatch")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nil, decode("nonce"), decode("plaintext"), decode("aad"))
	if hex.EncodeToString(sealed) != v["ciphertext"]+v["tag"] {
		t.Fatal("AES-GCM mismatch")
	}
}
