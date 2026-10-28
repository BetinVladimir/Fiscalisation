package domain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fiscalisation/fiscal-backend/internal/provisioningseal"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

// Disposable real-CA vectors for actual firmware installation tests; no external evidence.
func TestExportProvisioningInstallerInterop(t *testing.T) {
	dir := os.Getenv("PROVISIONING_INSTALLER_VECTOR_DIR")
	if dir == "" {
		t.Skip("interop export not requested")
	}
	ca, key := testDeviceCA(t)
	issuer, e := NewX509DeviceCredentialIssuer(ca, key, "ssl://mqtt.test:8883", "", "01234567890123456789012345678901")
	if e != nil {
		t.Fatal(e)
	}
	device, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	enc := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.FillBytes(make([]byte, 32))) }
	jwk := `{"crv":"P-256","kty":"EC","x":"` + enc(device.X) + `","y":"` + enc(device.Y) + `"}`
	der, _ := x509.MarshalPKIXPublicKey(&device.PublicKey)
	pub := base64.RawURLEncoding.EncodeToString(der)
	dh := sha256.Sum256([]byte(pub))
	kid := "esp32-p256-" + base64.RawURLEncoding.EncodeToString(dh[:12])
	raw, e := os.ReadFile("../../../IoT/firmware/edge-agent-s3/idf/tests/money_host/datecs.json")
	if e != nil {
		t.Fatal(e)
	}
	var payload map[string]any
	if json.Unmarshal(raw, &payload) != nil {
		t.Fatal("fixture")
	}
	payload["operational_authority"].(map[string]any)["transaction_signing_kid"] = kid
	canonical, _ := json.Marshal(payload)
	hash := sha256.Sum256(canonical)
	sig, issuerKID, e := issuer.SignCompositeBinding(canonical)
	if e != nil {
		t.Fatal(e)
	}
	req := provisioningseal.Request{EnrolledPublicKeyDER: der, DeviceKID: kid, IssuerKID: issuerKID, Generation: 7, BindingSHA256: hash}
	copy(req.SetupNonce[:], bytes.Repeat([]byte{3}, 32))
	req.Plaintext, _ = json.Marshal(map[string]any{"schema_version": 1, "binding": map[string]any{"canonical_json": string(canonical), "signature": sig, "key_id": issuerKID}, "wifi": map[string]string{"ssid": "test-network", "password": "test-password"}, "fiscal_operator_password": "12345678"})
	packet, e := issuer.SealRuntimeProvisioning(req, payload["edge_device_id"].(string), "existing-credential", jwk)
	if e != nil {
		t.Fatal(e)
	}
	badHash := req
	badHash.BindingSHA256[0] ^= 1
	badPacket, e := issuer.SealRuntimeProvisioning(badHash, payload["edge_device_id"].(string), "existing-credential", jwk)
	if e != nil {
		t.Fatal(e)
	}
	badSecret := req
	badSecret.Plaintext = bytes.Replace(req.Plaintext, []byte(`"fiscal_operator_password":"12345678"`), []byte(`"fiscal_operator_password":"invalid!"`), 1)
	badSecretPacket, e := issuer.SealRuntimeProvisioning(badSecret, payload["edge_device_id"].(string), "existing-credential", jwk)
	if e != nil {
		t.Fatal(e)
	}
	issuerDER, _ := x509.MarshalPKIXPublicKey(issuer.signer.Public())
	files := map[string][]byte{"installer.bin": packet, "wrong-binding.bin": badPacket, "wrong-secret.bin": badSecretPacket, "issuer.pem": pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: issuerDER}), "issuer.kid": []byte(issuerKID), "device.scalar": device.D.FillBytes(make([]byte, 32)), "nonce.hex": []byte(hex.EncodeToString(req.SetupNonce[:])), "binding.hex": []byte(hex.EncodeToString(hash[:]))}
	for name, data := range files {
		if e = os.WriteFile(filepath.Join(dir, name), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}

func TestVerifyActualFirmwareInstallProof(t *testing.T) {
	dir := os.Getenv("PROVISIONING_INSTALLER_VECTOR_DIR")
	if dir == "" {
		t.Skip("actual firmware proof fixture not requested")
	}
	raw, e := os.ReadFile(filepath.Join(dir, "firmware-proof.json"))
	if e != nil {
		t.Fatal(e)
	}
	var p ProvisioningInstallProof
	if json.Unmarshal(raw, &p) != nil {
		t.Fatal("proof JSON")
	}
	scalar, e := os.ReadFile(filepath.Join(dir, "device.scalar"))
	if e != nil {
		t.Fatal(e)
	}
	x, y := elliptic.P256().ScalarBaseMult(scalar)
	public := &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}
	signature, e := base64.RawURLEncoding.DecodeString(p.Signature)
	proof := "beefiscal-provisioning-installed-v1\n" + p.DeviceKID + "\n" + p.BindingSHA256 + "\n7\n" + p.PackageSHA256
	if e != nil || p.State != "INSTALLED" || p.Generation != 7 || !verifyP1363(public, sha256Bytes(proof), signature) {
		t.Fatal("actual mbedTLS device proof fails backend verification")
	}
	if verifyP1363(public, sha256Bytes(proof+"tamper"), signature) {
		t.Fatal("tampered installation proof verified")
	}
}
