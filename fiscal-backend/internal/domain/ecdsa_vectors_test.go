package domain

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// ecdsaVector is one entry from docs/MVP1/fixtures/ecdsa_vectors.json.
// The vectors cover two distinct signature formats:
//   - sig_der_b64:   ASN.1 DER format (Kotlin/JVM ecdsa.sign output)
//   - sig_p1363_b64: P1363 fixed-width r||s (device wire format, verified by verifyP1363)
//
// Cross-format rejection is tested in TestECDSACrossFormatRejection.
type ecdsaVector struct {
	Index       int    `json:"index"`
	DigestHex   string `json:"digest_hex"`
	PubKeySpki  string `json:"pubkey_spki_b64"`
	SigDer      string `json:"sig_der_b64"`
	SigP1363    string `json:"sig_p1363_b64"`
	WrongDer    string `json:"wrong_sig_der_b64"`
	WrongP1363  string `json:"wrong_sig_p1363_b64"`
}

func loadECDSAVectors(t *testing.T) []ecdsaVector {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	// domain → internal → fiscal-backend → Fiscalisation (repo root)
	repoRoot := filepath.Join(filepath.Dir(file), "..", "..", "..")
	path := filepath.Join(repoRoot, "docs", "MVP1", "fixtures", "ecdsa_vectors.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read ecdsa_vectors.json: %v (run gen_ecdsa.go to create it)", err)
	}
	var vectors []ecdsaVector
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("cannot unmarshal ecdsa_vectors.json: %v", err)
	}
	if len(vectors) < 10 {
		t.Fatalf("expected >= 10 vectors, got %d", len(vectors))
	}
	return vectors
}

// TestECDSAP1363Verify checks that P1363 signatures (device wire format) are
// accepted by verifyP1363 and rejected when a wrong-key signature is used.
func TestECDSAP1363Verify(t *testing.T) {
	vectors := loadECDSAVectors(t)
	for _, v := range vectors {
		v := v
		t.Run("v"+v.DigestHex[:8], func(t *testing.T) {
			digest, err := hex.DecodeString(v.DigestHex)
			if err != nil || len(digest) != 32 {
				t.Fatalf("bad digest: %v", err)
			}
			spkiBytes, err := base64.StdEncoding.DecodeString(v.PubKeySpki)
			if err != nil {
				t.Fatalf("bad pubkey base64: %v", err)
			}
			pub, err := x509.ParsePKIXPublicKey(spkiBytes)
			if err != nil {
				t.Fatalf("parse SPKI: %v", err)
			}
			ecPub, ok := pub.(*ecdsa.PublicKey)
			if !ok {
				t.Fatalf("key is not ECDSA")
			}

			// Valid P1363 signature must verify.
			sigP1363, err := base64.StdEncoding.DecodeString(v.SigP1363)
			if err != nil {
				t.Fatalf("bad p1363 base64: %v", err)
			}
			if !verifyP1363(ecPub, digest, sigP1363) {
				t.Errorf("verifyP1363 rejected a valid P1363 signature")
			}

			// Wrong-key P1363 signature must be rejected.
			wrongP1363, err := base64.StdEncoding.DecodeString(v.WrongP1363)
			if err != nil {
				t.Fatalf("bad wrong_p1363 base64: %v", err)
			}
			if verifyP1363(ecPub, digest, wrongP1363) {
				t.Errorf("verifyP1363 accepted a wrong-key P1363 signature")
			}
		})
	}
}

// TestECDSADERVerify checks that DER-format signatures (Kotlin/JVM output) are
// accepted by Go's ecdsa.VerifyASN1 and rejected when a wrong-key signature is used.
func TestECDSADERVerify(t *testing.T) {
	vectors := loadECDSAVectors(t)
	for _, v := range vectors {
		v := v
		t.Run("v"+v.DigestHex[:8], func(t *testing.T) {
			digest, _ := hex.DecodeString(v.DigestHex)
			spkiBytes, _ := base64.StdEncoding.DecodeString(v.PubKeySpki)
			pub, err := x509.ParsePKIXPublicKey(spkiBytes)
			if err != nil {
				t.Fatalf("parse SPKI: %v", err)
			}
			ecPub := pub.(*ecdsa.PublicKey)

			// Valid DER signature must verify.
			sigDer, _ := base64.StdEncoding.DecodeString(v.SigDer)
			if !ecdsa.VerifyASN1(ecPub, digest, sigDer) {
				t.Errorf("VerifyASN1 rejected a valid DER signature")
			}

			// Wrong-key DER signature must be rejected.
			wrongDer, _ := base64.StdEncoding.DecodeString(v.WrongDer)
			if ecdsa.VerifyASN1(ecPub, digest, wrongDer) {
				t.Errorf("VerifyASN1 accepted a wrong-key DER signature")
			}
		})
	}
}

// TestECDSACrossFormatRejection ensures that:
//   - A P1363 signature fed to DER verifier is rejected (no panic).
//   - A DER signature fed to P1363 verifier is rejected (no panic).
//
// Invariant 10: parsers/verifiers never throw externally (no panic).
func TestECDSACrossFormatRejection(t *testing.T) {
	vectors := loadECDSAVectors(t)
	if len(vectors) == 0 {
		t.Fatal("no vectors")
	}
	for _, v := range vectors[:min10(len(vectors))] {
		v := v
		t.Run("cross_"+v.DigestHex[:8], func(t *testing.T) {
			digest, _ := hex.DecodeString(v.DigestHex)
			spkiBytes, _ := base64.StdEncoding.DecodeString(v.PubKeySpki)
			pub, _ := x509.ParsePKIXPublicKey(spkiBytes)
			ecPub := pub.(*ecdsa.PublicKey)

			sigDer, _ := base64.StdEncoding.DecodeString(v.SigDer)
			sigP1363, _ := base64.StdEncoding.DecodeString(v.SigP1363)

			// P1363 (raw 64-byte r||s) must NOT parse as valid DER signature.
			if ecdsa.VerifyASN1(ecPub, digest, sigP1363) {
				t.Errorf("VerifyASN1 accepted a P1363 signature — format confusion")
			}

			// DER (variable-length ASN.1) must NOT verify as P1363 (wrong length).
			// verifyP1363 rejects anything not exactly 64 bytes; no panic.
			result := verifyP1363(ecPub, digest, sigDer)
			if result {
				t.Errorf("verifyP1363 accepted a DER signature — format confusion")
			}
		})
	}
}

func min10(n int) int {
	if n < 10 {
		return n
	}
	return 10
}
