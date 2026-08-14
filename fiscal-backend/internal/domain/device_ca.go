package domain

import (
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/url"
	"time"
)

type X509DeviceCredentialIssuer struct {
	ca                      *x509.Certificate
	signer                  crypto.Signer
	caPEM, mqttTLS, mqttWSS string
	transportMaster         []byte
	clock                   func() time.Time
}

func NewX509DeviceCredentialIssuer(certificatePEM, keyPEM []byte, mqttTLS, mqttWSS string, transportMaster ...string) (*X509DeviceCredentialIssuer, error) {
	cb, _ := pem.Decode(certificatePEM)
	kb, _ := pem.Decode(keyPEM)
	if cb == nil || kb == nil || mqttTLS == "" {
		return nil, errors.New("device CA configuration incomplete")
	}
	ca, e := x509.ParseCertificate(cb.Bytes)
	if e != nil || !ca.IsCA {
		return nil, errors.New("invalid device CA certificate")
	}
	var key any
	if key, e = x509.ParsePKCS8PrivateKey(kb.Bytes); e != nil {
		if ec, ecErr := x509.ParseECPrivateKey(kb.Bytes); ecErr == nil {
			key = ec
			e = nil
		} else if rsa, rsaErr := x509.ParsePKCS1PrivateKey(kb.Bytes); rsaErr == nil {
			key = rsa
			e = nil
		}
	}
	signer, ok := key.(crypto.Signer)
	publicEqual, comparable := signerPublicEqual(signer, ca.PublicKey)
	if e != nil || !ok || !comparable || !publicEqual {
		return nil, errors.New("device CA key mismatch")
	}
	var master []byte
	if len(transportMaster) > 0 {
		master = []byte(transportMaster[0])
	}
	if len(master) < 32 {
		return nil, errors.New("device transport master key too short")
	}
	return &X509DeviceCredentialIssuer{ca: ca, signer: signer, caPEM: string(certificatePEM), mqttTLS: mqttTLS, mqttWSS: mqttWSS, transportMaster: master, clock: time.Now}, nil
}

func signerPublicEqual(signer crypto.Signer, expected crypto.PublicKey) (bool, bool) {
	if signer == nil {
		return false, false
	}
	public, ok := signer.Public().(interface{ Equal(crypto.PublicKey) bool })
	if !ok {
		return false, false
	}
	return public.Equal(expected), true
}
func (i *X509DeviceCredentialIssuer) Issue(v DeviceActivationRequest) (DeviceCredential, error) {
	_, public, _, e := activationPublicKeyJSON(v.PublicKeyJWK)
	if e != nil {
		return DeviceCredential{}, e
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, e := rand.Int(rand.Reader, serialLimit)
	if e != nil {
		return DeviceCredential{}, e
	}
	now := i.clock().UTC()
	spiffe, _ := url.Parse("spiffe://beefiscal/device/" + v.DeviceInstanceID)
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: v.DeviceInstanceID, Organization: []string{"BeeFiscal SmartDevice"}}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(90 * 24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, URIs: []*url.URL{spiffe}, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, template, i.ca, public, i.signer)
	if e != nil {
		return DeviceCredential{}, e
	}
	credentialID := base64.RawURLEncoding.EncodeToString(sha256Bytes(string(der)))[:32]
	binding := DeviceCredential{CredentialID: credentialID, ClientCertificatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), CAChainPEM: i.caPEM, MQTTTLSURI: i.mqttTLS, MQTTWSSURI: i.mqttWSS, DeviceInstanceID: v.DeviceInstanceID, OrganizationID: v.ClaimedTenantID, LocationID: v.ClaimedLocationID, RegisterID: v.ClaimedRegisterID, Roles: v.ClaimedRoles, BindingVersion: v.BindingVersion, CommandHMACKey: base64.RawURLEncoding.EncodeToString(DeriveDeviceTransportKey(i.transportMaster, "command", v.DeviceInstanceID, credentialID)), SyncAckHMACKey: base64.RawURLEncoding.EncodeToString(DeriveDeviceTransportKey(i.transportMaster, "sync-ack", v.DeviceInstanceID, credentialID)), BLETicketHMACKey: base64.RawURLEncoding.EncodeToString(DeriveDeviceTransportKey(i.transportMaster, "ble-ticket", v.DeviceInstanceID, credentialID))}
	canonical, _ := json.Marshal(map[string]any{"credential_id": credentialID, "device_instance_id": binding.DeviceInstanceID, "organization_id": binding.OrganizationID, "location_id": binding.LocationID, "register_id": binding.RegisterID, "roles": binding.Roles, "binding_version": binding.BindingVersion, "mqtt_tls_uri": binding.MQTTTLSURI, "mqtt_wss_uri": binding.MQTTWSSURI, "command_hmac_key": binding.CommandHMACKey, "sync_ack_hmac_key": binding.SyncAckHMACKey, "ble_ticket_hmac_key": binding.BLETicketHMACKey})
	digest := sha256.Sum256(canonical)
	sig, e := i.signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if e != nil {
		return DeviceCredential{}, e
	}
	binding.BindingSignature = base64.RawURLEncoding.EncodeToString(sig)
	return binding, nil
}

func (i *X509DeviceCredentialIssuer) SignActivationAcknowledgement(unsigned []byte) (string, error) {
	digest := sha256.Sum256(unsigned)
	signature, err := i.signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(signature), nil
}
func (i *X509DeviceCredentialIssuer) SignCompositeBinding(canonical []byte) (string, string, error) {
	digest := sha256.Sum256(canonical)
	signature, err := i.signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return "", "", err
	}
	certificateDigest := sha256.Sum256(i.ca.RawSubjectPublicKeyInfo)
	return base64.RawURLEncoding.EncodeToString(signature), "binding-" + base64.RawURLEncoding.EncodeToString(certificateDigest[:8]), nil
}
