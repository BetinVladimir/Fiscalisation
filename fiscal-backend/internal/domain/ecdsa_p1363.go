package domain

import (
	"crypto/ecdsa"
	"crypto/rand"
	"math/big"
)

// verifyP1363 enforces the device wire format: fixed-width r||s for P-256.
// ASN.1 DER is deliberately not accepted, keeping firmware, BLE and MQTT
// signatures unambiguous.
func verifyP1363(key *ecdsa.PublicKey, digest, signature []byte) bool {
	if key == nil || key.Curve == nil || key.Curve.Params().Name != "P-256" || len(signature) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	return ecdsa.Verify(key, digest, r, s)
}

func signP1363(key *ecdsa.PrivateKey, digest []byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, key, digest)
	if err != nil {
		return nil, err
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return signature, nil
}
