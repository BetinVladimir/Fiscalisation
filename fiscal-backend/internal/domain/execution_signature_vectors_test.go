package domain

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

type executionSignatureVector struct {
	Description  string `json:"description"`
	Message      string `json:"message"`
	SPKI         string `json:"spki"`
	SignatureDER string `json:"signature_der"`
}

func TestBackendAcceptsMobileDerSignatureVector(t *testing.T) {
	path := "../../../minipos/BeeMiniPOS/modules/beeloy-execution-crypto/fixtures/execution_signature_vectors.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture not found at %s: %v", path, err)
	}
	var vectors []executionSignatureVector
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(vectors) < 3 {
		t.Fatalf("want >=3 vectors, got %d", len(vectors))
	}
	for i, v := range vectors {
		msgBytes, err := base64.StdEncoding.DecodeString(v.Message)
		if err != nil {
			t.Errorf("vector %d: bad message base64: %v", i, err)
			continue
		}
		spkiBytes, err := base64.StdEncoding.DecodeString(v.SPKI)
		if err != nil {
			t.Errorf("vector %d: bad spki base64: %v", i, err)
			continue
		}
		sigBytes, err := base64.StdEncoding.DecodeString(v.SignatureDER)
		if err != nil {
			t.Errorf("vector %d: bad signature_der base64: %v", i, err)
			continue
		}
		pub, err := x509.ParsePKIXPublicKey(spkiBytes)
		if err != nil {
			t.Errorf("vector %d: parse SPKI: %v", i, err)
			continue
		}
		ecPub, ok := pub.(*ecdsa.PublicKey)
		if !ok {
			t.Errorf("vector %d: SPKI is not ECDSA key", i)
			continue
		}
		digest := sha256.Sum256(msgBytes)
		if !ecdsa.VerifyASN1(ecPub, digest[:], sigBytes) {
			t.Errorf("vector %d (%s): VerifyASN1 returned false", i, v.Description)
		}
	}
}

func TestBackendRejectsP1363ForMobilePath(t *testing.T) {
	path := "../../../minipos/BeeMiniPOS/modules/beeloy-execution-crypto/fixtures/execution_signature_vectors.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("fixture not found at %s: %v", path, err)
	}
	var vectors []executionSignatureVector
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(vectors) == 0 {
		t.Skip("no vectors to test")
	}
	v := vectors[0]
	msgBytes, _ := base64.StdEncoding.DecodeString(v.Message)
	spkiBytes, _ := base64.StdEncoding.DecodeString(v.SPKI)
	pub, err := x509.ParsePKIXPublicKey(spkiBytes)
	if err != nil {
		t.Fatalf("parse SPKI: %v", err)
	}
	ecPub := pub.(*ecdsa.PublicKey)
	digest := sha256.Sum256(msgBytes)
	// A P1363 r‖s signature (64 bytes of zeros) is not valid DER and must be rejected.
	p1363Fake := make([]byte, 64)
	if ecdsa.VerifyASN1(ecPub, digest[:], p1363Fake) {
		t.Fatal("P1363 (r||s) signature must not verify for mobile DER path")
	}
}
