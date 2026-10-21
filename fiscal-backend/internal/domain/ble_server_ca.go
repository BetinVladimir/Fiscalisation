package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"net/url"
	"time"
)

const bleServerCertificatePurpose = "ble-tls-server"

// BLEServerCertificateIssuer is deliberately separate from MQTT activation.
// It is not configured by default and has no public issue method or HTTP route.
type BLEServerCertificateIssuer struct {
	ca    *x509.Certificate
	key   *ecdsa.PrivateKey
	chain string
	clock func() time.Time
}
type BLEServerCertificate struct {
	Profile           string    `json:"profile"`
	DeviceID          string    `json:"device_id"`
	ServerName        string    `json:"server_name"`
	SPKISHA256        string    `json:"spki_sha256"`
	CertificatePEM    string    `json:"certificate_pem"`
	CAChainPEM        string    `json:"ca_chain_pem"`
	CertificateSHA256 string    `json:"certificate_sha256"`
	NotAfter          time.Time `json:"not_after"`
}

// This permit is internal; the service may mint it only after fresh purpose-PoP
// and atomic enrolled-key/binding/challenge checks. No caller-provided key API.
type bleServerIssuancePermit struct {
	deviceID string
	public   *ecdsa.PublicKey
	notAfter time.Time
}

func NewBLEServerCertificateIssuer(caPEM, keyPEM, mqttCAPEM []byte) (*BLEServerCertificateIssuer, error) {
	reject := func() (*BLEServerCertificateIssuer, error) { return nil, errors.New("BLE server CA profile invalid") }
	cb, rest := pem.Decode(caPEM)
	kb, keyRest := pem.Decode(keyPEM)
	mb, mqttRest := pem.Decode(mqttCAPEM)
	if cb == nil || kb == nil || mb == nil || len(rest) > 0 || len(keyRest) > 0 || len(mqttRest) > 0 {
		return reject()
	}
	ca, e := x509.ParseCertificate(cb.Bytes)
	if e != nil || !ca.IsCA || !ca.BasicConstraintsValid || ca.KeyUsage&x509.KeyUsageCertSign == 0 || len(ca.ExtKeyUsage) != 1 || ca.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth || len(ca.UnknownExtKeyUsage) != 0 {
		return reject()
	}
	mqtt, e := x509.ParseCertificate(mb.Bytes)
	if e != nil || !mqtt.IsCA {
		return reject()
	}
	if hex.EncodeToString(ca.RawSubjectPublicKeyInfo) == hex.EncodeToString(mqtt.RawSubjectPublicKeyInfo) {
		return reject()
	}
	parsed, e := x509.ParsePKCS8PrivateKey(kb.Bytes)
	if e != nil {
		parsed, e = x509.ParseECPrivateKey(kb.Bytes)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if e != nil || !ok || key.Curve != elliptic.P256() || !key.PublicKey.Equal(ca.PublicKey) {
		return reject()
	}
	return &BLEServerCertificateIssuer{ca: ca, key: key, chain: string(caPEM), clock: time.Now}, nil
}
func bleServerName(deviceID string) string { return "device-" + deviceID + ".ble.beefiscal.invalid" }
func (i *BLEServerCertificateIssuer) issue(permit bleServerIssuancePermit) (BLEServerCertificate, error) {
	reject := func() (BLEServerCertificate, error) {
		return BLEServerCertificate{}, errors.New("BLE server certificate issuance denied")
	}
	if i == nil || !uuidPattern.MatchString(permit.deviceID) || permit.public == nil || permit.public.X == nil || permit.public.Y == nil || permit.public.Curve != elliptic.P256() || !permit.public.Curve.IsOnCurve(permit.public.X, permit.public.Y) {
		return reject()
	}
	now := i.clock().UTC()
	if now.Before(i.ca.NotBefore) || !now.Before(i.ca.NotAfter) || !permit.notAfter.After(now) || permit.notAfter.After(now.Add(24*time.Hour)) || permit.notAfter.After(i.ca.NotAfter) {
		return reject()
	}
	serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if e != nil {
		return BLEServerCertificate{}, e
	}
	if serial.Sign() == 0 {
		serial.SetInt64(1)
	}
	name := bleServerName(permit.deviceID)
	uri, _ := url.Parse("spiffe://beefiscal/ble-server/device/" + permit.deviceID)
	leaf := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: permit.deviceID, Organization: []string{"BeeFiscal BLE Server"}}, DNSNames: []string{name}, URIs: []*url.URL{uri}, NotBefore: now.Add(-time.Minute), NotAfter: permit.notAfter.UTC(), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, leaf, i.ca, permit.public, i.key)
	if e != nil {
		return BLEServerCertificate{}, e
	}
	spki, e := x509.MarshalPKIXPublicKey(permit.public)
	if e != nil {
		return BLEServerCertificate{}, e
	}
	pin := sha256.Sum256(spki)
	digest := sha256.Sum256(der)
	return BLEServerCertificate{Profile: bleServerCertificatePurpose, DeviceID: permit.deviceID, ServerName: name, SPKISHA256: hex.EncodeToString(pin[:]), CertificatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), CAChainPEM: i.chain, CertificateSHA256: hex.EncodeToString(digest[:]), NotAfter: leaf.NotAfter}, nil
}
