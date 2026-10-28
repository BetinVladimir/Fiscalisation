// Package provisioningseal creates opaque provisioning packages for an already
// enrolled device. It does not enroll keys, authorize callers or activate devices.
package provisioningseal

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"io"
	"math/big"
)

const MaxPlaintext = 32768
const fixedHeader = 159
const domain = "BeeFiscalProvisioningV2\x00"

type Request struct {
	// EnrolledPublicKeyDER MUST come from the independently authenticated factory
	// record, never from a mobile/BLE caller-supplied key.
	EnrolledPublicKeyDER      []byte
	DeviceKID, IssuerKID      string
	Generation                uint64
	SetupNonce, BindingSHA256 [32]byte
	Plaintext                 []byte
}

func Seal(request Request, signer *ecdsa.PrivateKey) ([]byte, error) {
	return seal(request, signer, rand.Reader)
}
func seal(r Request, signer *ecdsa.PrivateKey, random io.Reader) ([]byte, error) {
	if signer == nil || signer.Curve != elliptic.P256() || r.Generation == 0 || !validID(r.DeviceKID) || !validID(r.IssuerKID) || len(r.Plaintext) == 0 || len(r.Plaintext) > MaxPlaintext || r.SetupNonce == [32]byte{} || r.BindingSHA256 == [32]byte{} {
		return nil, errors.New("invalid provisioning subject")
	}
	public, err := x509.ParsePKIXPublicKey(r.EnrolledPublicKeyDER)
	if err != nil {
		return nil, err
	}
	recipient, ok := public.(*ecdsa.PublicKey)
	if !ok || recipient.Curve != elliptic.P256() {
		return nil, errors.New("enrolled P-256 key required")
	}
	peer, err := recipient.ECDH()
	if err != nil {
		return nil, err
	}
	ephemeral, err := ecdh.P256().GenerateKey(random)
	if err != nil {
		return nil, err
	}
	shared, err := ephemeral.ECDH(peer)
	if err != nil {
		return nil, err
	}
	defer clear(shared)
	h := make([]byte, fixedHeader+len(r.DeviceKID)+len(r.IssuerKID))
	copy(h, "BFP2")
	binary.BigEndian.PutUint64(h[4:12], r.Generation)
	copy(h[12:44], r.SetupNonce[:])
	copy(h[44:76], r.BindingSHA256[:])
	copy(h[76:141], ephemeral.PublicKey().Bytes())
	if _, err = io.ReadFull(random, h[141:153]); err != nil {
		return nil, err
	}
	h[153] = byte(len(r.DeviceKID))
	h[154] = byte(len(r.IssuerKID))
	binary.BigEndian.PutUint32(h[155:159], uint32(len(r.Plaintext)+16))
	copy(h[159:], r.DeviceKID)
	copy(h[159+len(r.DeviceKID):], r.IssuerKID)
	key, err := hkdf.Key(sha256.New, shared, r.SetupNonce[:], domain+string(h), 32)
	if err != nil {
		return nil, err
	}
	defer clear(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	packet := append(h, gcm.Seal(nil, h[141:153], r.Plaintext, h)...)
	digest := sha256.Sum256(packet)
	a, b, err := ecdsa.Sign(random, signer, digest[:])
	if err != nil {
		return nil, err
	}
	// Fixed-width P1363 signature, low-S to avoid alternate signature encodings.
	half := new(big.Int).Rsh(new(big.Int).Set(signer.Params().N), 1)
	if b.Cmp(half) > 0 {
		b.Sub(signer.Params().N, b)
	}
	signature := make([]byte, 64)
	a.FillBytes(signature[:32])
	b.FillBytes(signature[32:])
	return append(packet, signature...), nil
}
func validID(s string) bool {
	if len(s) < 1 || len(s) > 96 {
		return false
	}
	for _, c := range []byte(s) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}
