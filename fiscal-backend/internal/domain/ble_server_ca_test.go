package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func bleProfileCA(t *testing.T, usage []x509.ExtKeyUsage) ([]byte, []byte, *x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1024), Subject: pkix.Name{CommonName: "Disposable BLE-only CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(48 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, ExtKeyUsage: usage}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	ca, e := x509.ParseCertificate(der)
	if e != nil {
		t.Fatal(e)
	}
	private, e := x509.MarshalPKCS8PrivateKey(key)
	if e != nil {
		t.Fatal(e)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), ca, key
}
func TestBLEServerProfileIsSeparatePurposeAndEnrolledKeyBound(t *testing.T) {
	mqtt, mqttKey := testDeviceCA(t)
	serverCA, key, ca, caKey := bleProfileCA(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	issuer, e := NewBLEServerCertificateIssuer(serverCA, key, mqtt)
	if e != nil {
		t.Fatal(e)
	}
	device, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	id := "11111111-1111-4111-8111-111111111111"
	permit := bleServerIssuancePermit{deviceID: id, public: &device.PublicKey, notAfter: time.Now().Add(time.Hour).Truncate(time.Second)}
	issued, e := issuer.issue(permit)
	if e != nil {
		t.Fatal(e)
	}
	block, _ := pem.Decode([]byte(issued.CertificatePEM))
	leaf, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	if !leaf.PublicKey.(*ecdsa.PublicKey).Equal(&device.PublicKey) || leaf.KeyUsage != x509.KeyUsageDigitalSignature || len(leaf.ExtKeyUsage) != 1 || leaf.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth || leaf.IsCA || len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != bleServerName(id) || len(leaf.URIs) != 1 || leaf.URIs[0].String() != "spiffe://beefiscal/ble-server/device/"+id {
		t.Fatal("unconstrained BLE certificate", leaf)
	}
	digest := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	if issued.SPKISHA256 != hex.EncodeToString(digest[:]) {
		t.Fatal("SPKI mismatch")
	}
	roots := x509.NewCertPool()
	roots.AddCert(ca)
	if _, e = leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: issued.ServerName, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); e != nil {
		t.Fatal(e)
	}
	if _, e = leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: "backend.example", KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); e == nil {
		t.Fatal("generic backend name accepted")
	}
	if _, e = leaf.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); e == nil {
		t.Fatal("BLE leaf accepted as MQTT client")
	}
	mqttRoots := x509.NewCertPool()
	mqttRoots.AppendCertsFromPEM(mqtt)
	if _, e = leaf.Verify(x509.VerifyOptions{Roots: mqttRoots, DNSName: issued.ServerName}); e == nil {
		t.Fatal("MQTT authority accepted BLE leaf")
	}
	mqttIssuer, e := NewX509DeviceCredentialIssuer(mqtt, mqttKey, "ssl://mqtt.test:8883", "", "01234567890123456789012345678901")
	if e != nil {
		t.Fatal(e)
	}
	enc := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.FillBytes(make([]byte, 32))) }
	jwk, _ := json.Marshal(map[string]string{"kty": "EC", "crv": "P-256", "x": enc(device.X), "y": enc(device.Y)})
	mqttCredential, e := mqttIssuer.Issue(DeviceActivationRequest{DeviceInstanceID: id, PublicKeyJWK: string(jwk)})
	if e != nil {
		t.Fatal(e)
	}
	mqttBlock, _ := pem.Decode([]byte(mqttCredential.ClientCertificatePEM))
	mqttLeaf, e := x509.ParseCertificate(mqttBlock.Bytes)
	if e != nil || len(mqttLeaf.ExtKeyUsage) != 1 || mqttLeaf.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth || len(mqttLeaf.DNSNames) != 0 {
		t.Fatal("existing MQTT profile changed", e)
	}
	if dir := os.Getenv("BLE_PROFILE_SYNTHETIC_INTEROP_DIR"); dir != "" {
		if !filepath.IsAbs(dir) {
			t.Fatal("absolute disposable fixture directory required")
		}
		if e = os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		deviceDER, _ := x509.MarshalPKCS8PrivateKey(device)
		for name, data := range map[string][]byte{"ca.pem": serverCA, "server.pem": []byte(issued.CertificatePEM), "server-key.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: deviceDER}), "expected-name": []byte(issued.ServerName), "expected-spki": []byte(issued.SPKISHA256)} {
			if e = os.WriteFile(filepath.Join(dir, name), data, 0600); e != nil {
				t.Fatal(e)
			}
		}
		leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		wrong, e := x509.CreateCertificate(rand.Reader, leaf, ca, &device.PublicKey, caKey)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "client-only.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: wrong}), 0600); e != nil {
			t.Fatal(e)
		}

		leaf.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		_, _, otherCA, otherCAKey := bleProfileCA(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
		wrongIssuer, e := x509.CreateCertificate(rand.Reader, leaf, otherCA, &device.PublicKey, otherCAKey)
		if e != nil {
			t.Fatal(e)
		}
		otherDevice, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if e != nil {
			t.Fatal(e)
		}
		wrongKey, e := x509.CreateCertificate(rand.Reader, leaf, ca, &otherDevice.PublicKey, caKey)
		if e != nil {
			t.Fatal(e)
		}
		otherKeyDER, _ := x509.MarshalPKCS8PrivateKey(otherDevice)
		for name, data := range map[string][]byte{"wrong-issuer.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: wrongIssuer}), "wrong-key.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: wrongKey}), "wrong-key-private.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: otherKeyDER})} {
			if e = os.WriteFile(filepath.Join(dir, name), data, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestBLEServerCARejectsSharedAndWrongPurposeAuthorities(t *testing.T) {
	mqtt, _ := testDeviceCA(t)
	for _, usage := range [][]x509.ExtKeyUsage{nil, {x509.ExtKeyUsageClientAuth}, {x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, {x509.ExtKeyUsageAny}} {
		ca, key, _, _ := bleProfileCA(t, usage)
		if _, e := NewBLEServerCertificateIssuer(ca, key, mqtt); e == nil {
			t.Fatal("wrong CA purpose accepted", usage)
		}
	}
	ca, key, _, _ := bleProfileCA(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	if _, e := NewBLEServerCertificateIssuer(ca, key, ca); e == nil {
		t.Fatal("shared MQTT/BLE CA accepted")
	}
	_, wrong, _, _ := bleProfileCA(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	if _, e := NewBLEServerCertificateIssuer(ca, wrong, mqtt); e == nil {
		t.Fatal("CA key mismatch accepted")
	}
}
func TestBLEServerProfileRejectsInvalidPermit(t *testing.T) {
	mqtt, _ := testDeviceCA(t)
	ca, key, _, _ := bleProfileCA(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	issuer, e := NewBLEServerCertificateIssuer(ca, key, mqtt)
	if e != nil {
		t.Fatal(e)
	}
	device, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	base := bleServerIssuancePermit{deviceID: "11111111-1111-4111-8111-111111111111", public: &device.PublicKey, notAfter: time.Now().Add(time.Hour)}
	for _, field := range []string{"id", "key", "expiry", "lifetime"} {
		p := base
		switch field {
		case "id":
			p.deviceID = "arbitrary-backend.example"
		case "key":
			p.public = nil
		case "expiry":
			p.notAfter = time.Now().Add(-time.Minute)
		case "lifetime":
			p.notAfter = time.Now().Add(25 * time.Hour)
		}
		if _, e = issuer.issue(p); e == nil {
			t.Fatal("invalid permit accepted", field)
		}
	}
}
