package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

func testDeviceCA(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "BeeFiscal Test Device CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func TestX509DeviceIssuerCreatesKeyBoundClientCredentialAndSignsAck(t *testing.T) {
	ca, key := testDeviceCA(t)
	issuer, err := NewX509DeviceCredentialIssuer(ca, key, "ssl://mqtt.example:8883", "wss://mqtt.example/mqtt", "01234567890123456789012345678901")
	if err != nil {
		t.Fatal(err)
	}
	deviceKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	enc := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.FillBytes(make([]byte, 32))) }
	jwk := `{"crv":"P-256","kty":"EC","x":"` + enc(deviceKey.X) + `","y":"` + enc(deviceKey.Y) + `"}`
	credential, err := issuer.Issue(DeviceActivationRequest{DeviceInstanceID: "11111111-1111-4111-8111-111111111111", PublicKeyJWK: jwk, ClaimedTenantID: "tenant-1", ClaimedLocationID: "location-1", ClaimedRegisterID: "register-1", ClaimedRoles: []string{"FISCAL_DEVICE"}, BindingVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(credential.ClientCertificatePEM))
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil || !certificate.PublicKey.(*ecdsa.PublicKey).Equal(&deviceKey.PublicKey) || len(certificate.URIs) != 1 {
		t.Fatal("credential is not device-key bound", err)
	}
	if signature, signErr := issuer.SignActivationAcknowledgement([]byte(`{"state":"ACTIVE"}`)); signErr != nil || signature == "" {
		t.Fatal(signature, signErr)
	}
}
