package ble

import (
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

type EphemeralKey struct{ private *ecdh.PrivateKey }

func NewEphemeralKey() (*EphemeralKey, []byte, error) {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return &EphemeralKey{private: key}, key.PublicKey().Bytes(), nil
}

// DeriveSessionSecret implements the BLE v1 X25519 + HKDF-SHA-256 binding.
// Both peers must supply the same ticket digest, client nonce, edge nonce and
// binding context (tenant/register/device/session/protocol).
func (k *EphemeralKey) DeriveSessionSecret(peerPublic, ticketDigest, clientNonce, edgeNonce []byte, context string) ([]byte, error) {
	if k == nil || k.private == nil || len(ticketDigest) != sha256.Size || len(clientNonce) < 16 || len(edgeNonce) < 16 || context == "" {
		return nil, errors.New("incomplete handshake context")
	}
	peer, err := ecdh.X25519().NewPublicKey(peerPublic)
	if err != nil {
		return nil, err
	}
	shared, err := k.private.ECDH(peer)
	if err != nil {
		return nil, err
	}
	saltInput := append(append(append([]byte(nil), ticketDigest...), clientNonce...), edgeNonce...)
	salt := sha256.Sum256(saltInput)
	return hkdf.Key(sha256.New, shared, salt[:], "BeeFiscal BLE v1|"+context, 32)
}
