package provisioningseal

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) (Request, *ecdsa.PrivateKey, *ecdsa.PrivateKey) {
	t.Helper()
	signer, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	device, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	der, e := x509.MarshalPKIXPublicKey(&device.PublicKey)
	if e != nil {
		t.Fatal(e)
	}
	r := Request{EnrolledPublicKeyDER: der, DeviceKID: "device-1", IssuerKID: "issuer-1", Generation: 7, SetupNonce: sha256.Sum256([]byte("setup nonce")), BindingSHA256: sha256.Sum256([]byte("exact signed schema3 BFPE")), Plaintext: []byte("wifi-password|fiscal-12345678|opaque-runtime-secrets")}
	return r, signer, device
}
func TestSealSubjectAndRandomness(t *testing.T) {
	r, k, _ := fixture(t)
	a, e := Seal(r, k)
	if e != nil {
		t.Fatal(e)
	}
	b, e := Seal(r, k)
	if e != nil {
		t.Fatal(e)
	}
	if string(a) == string(b) {
		t.Fatal("reused ephemeral key/IV")
	}
	if string(a[:4]) != "BFP2" || binary.BigEndian.Uint64(a[4:12]) != 7 {
		t.Fatal("header")
	}
	if len(a) != fixedHeader+len(r.DeviceKID)+len(r.IssuerKID)+len(r.Plaintext)+16+64 {
		t.Fatal("length")
	}
}
func TestSealRejectsInvalidSubjects(t *testing.T) {
	r, k, _ := fixture(t)
	for _, alter := range []func(*Request){func(r *Request) { r.Generation = 0 }, func(r *Request) { r.DeviceKID = "foreign/device" }, func(r *Request) { r.SetupNonce = [32]byte{} }, func(r *Request) { r.BindingSHA256 = [32]byte{} }, func(r *Request) { r.Plaintext = nil }, func(r *Request) { r.EnrolledPublicKeyDER = []byte("untrusted") }, func(r *Request) { r.Plaintext = make([]byte, MaxPlaintext+1) }} {
		v := r
		alter(&v)
		if _, e := Seal(v, k); e == nil {
			t.Fatal("invalid accepted")
		}
	}
}

// Export ephemeral vectors ONLY for cross-language host tests. Never production
// keys/evidence. The actual firmware crypto must open this Go-issued package.
func TestExportCrossLanguageFixture(t *testing.T) {
	dir := os.Getenv("PROVISIONING_VECTOR_DIR")
	if dir == "" {
		t.Skip("cross-language vector export not requested")
	}
	r, k, device := fixture(t)
	r.Plaintext = append(r.Plaintext, bytes.Repeat([]byte("x"), 9000)...)
	packet, e := Seal(r, k)
	if e != nil {
		t.Fatal(e)
	}
	pub, e := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if e != nil {
		t.Fatal(e)
	}
	bad := append([]byte(nil), packet...)
	bad[len(bad)-65] ^= 1
	digest := sha256.Sum256(bad[:len(bad)-64])
	a, b, e := ecdsa.Sign(rand.Reader, k, digest[:])
	if e != nil {
		t.Fatal(e)
	}
	half := new(big.Int).Rsh(new(big.Int).Set(k.Params().N), 1)
	if b.Cmp(half) > 0 {
		b.Sub(k.Params().N, b)
	}
	a.FillBytes(bad[len(bad)-64 : len(bad)-32])
	b.FillBytes(bad[len(bad)-32:])
	files := map[string][]byte{"signed-bad-tag.bin": bad, "package.bin": packet, "issuer.pem": pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}), "device.scalar": device.D.FillBytes(make([]byte, 32)), "nonce.hex": []byte(hex.EncodeToString(r.SetupNonce[:])), "binding.hex": []byte(hex.EncodeToString(r.BindingSHA256[:])), "plaintext.bin": r.Plaintext}
	for name, body := range files {
		if e = os.WriteFile(filepath.Join(dir, name), body, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
