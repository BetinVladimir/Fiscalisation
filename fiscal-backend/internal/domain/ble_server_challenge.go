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
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

const bleCertChallengeDomain = "BEEFISCAL_BLE_SERVER_CERT_CHALLENGE_V1\n"
const bleCertProofDomain = "BEEFISCAL_BLE_SERVER_CERT_PROOF_V1\n"

type BLEServerCertChallenge struct {
	Kind                string `json:"kind"`
	Version             int64  `json:"version"`
	Purpose             string `json:"purpose"`
	ChallengeID         string `json:"challenge_id"`
	Nonce               string `json:"nonce"`
	IssuedAt            int64  `json:"issued_at"`
	ExpiresAt           int64  `json:"expires_at"`
	DeviceID            string `json:"device_id"`
	DeviceSPKI          string `json:"device_spki_sha256"`
	TenantID            string `json:"tenant_id"`
	LocationID          string `json:"location_id"`
	RegisterID          string `json:"register_id"`
	BindingSHA256       string `json:"binding_sha256"`
	BindingGeneration   uint64 `json:"binding_generation"`
	ServerName          string `json:"server_name"`
	CertificateNotAfter int64  `json:"certificate_not_after"`
}
type BLEServerCertChallengeEnvelope struct {
	CanonicalJSON string `json:"canonical_json"`
	Signature     string `json:"signature"`
	KeyID         string `json:"key_id"`
}
type BLEServerCertProof struct {
	Kind            string `json:"kind"`
	Version         int64  `json:"version"`
	Purpose         string `json:"purpose"`
	ChallengeID     string `json:"challenge_id"`
	ChallengeSHA256 string `json:"challenge_sha256"`
	DeviceSPKI      string `json:"device_spki_sha256"`
}
type bleCertSource struct {
	Kind, ID, Tenant string
	Version          int64
}
type bleCertAuthority struct {
	public     *ecdsa.PublicKey
	activation DeviceActivationRequest
	sources    []bleCertSource
	challenge  BLEServerCertChallenge
}
type bleCertificateRepository interface {
	commitBLEServerChallenge(ResourceRecord, []bleCertSource, DeviceActivationRequest) error
	commitBLEServerCertificate(ResourceRecord, ResourceRecord, []bleCertSource, DeviceActivationRequest, time.Time) (ResourceRecord, error)
	confirmedBLEServerCertificate(string, string, []bleCertSource, DeviceActivationRequest) (ResourceRecord, error)
}
type bleChallengeSigner interface {
	SignBLEServerCertChallenge(string) (string, string, error)
}

func (s *Service) SetBLEServerCertificateIssuer(issuer *BLEServerCertificateIssuer) {
	s.bleServerIssuer = issuer
}
func decodeBLECanonical(raw string, out any) error {
	if len(raw) == 0 || len(raw) > 8192 {
		return errors.New("BLE certificate canonical size invalid")
	}
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(out); e != nil {
		return e
	}
	var tail any
	if d.Decode(&tail) != io.EOF {
		return errors.New("BLE canonical trailing data")
	}
	canonical, e := edgeCanonical(out)
	if e != nil || !bytes.Equal(canonical, []byte(raw)) {
		return errors.New("BLE certificate noncanonical JSON")
	}
	return nil
}
func (i *X509DeviceCredentialIssuer) SignBLEServerCertChallenge(raw string) (string, string, error) {
	var challenge BLEServerCertChallenge
	if decodeBLECanonical(raw, &challenge) != nil || challenge.Kind != "BLE_SERVER_CERT_CHALLENGE" || !validBLEChallenge(challenge) {
		return "", "", errors.New("BLE challenge purpose rejected")
	}
	key, ok := i.signer.(*ecdsa.PrivateKey)
	if !ok || key.Curve != elliptic.P256() {
		return "", "", errors.New("P256 root challenge signer required")
	}
	hash := sha256.Sum256([]byte(bleCertChallengeDomain + raw))
	sig, e := ecdsa.SignASN1(rand.Reader, key, hash[:])
	if e != nil {
		return "", "", e
	}
	kid := sha256.Sum256(i.ca.RawSubjectPublicKeyInfo)
	return base64.RawURLEncoding.EncodeToString(sig), "binding-" + base64.RawURLEncoding.EncodeToString(kid[:8]), nil
}
func (s *Service) bleCertificateAuthority(deviceID, activationID, bindingID, tenant string) (bleCertAuthority, error) {
	deny := func() (bleCertAuthority, error) {
		return bleCertAuthority{}, errors.New("BLE certificate authority rejected")
	}
	var a bleCertAuthority
	activation, e := s.repo.ActivationRequest(activationID)
	if e != nil || activation.State != "ACTIVE" || activation.Model != "EDGE_AGENT_S3" || activation.DeviceInstanceID != deviceID || activation.ClaimedTenantID != tenant || activation.CredentialID == "" {
		return deny()
	}
	a.activation = activation
	resource := func(kind, id, scope string) (ResourceRecord, bool) {
		v, e := s.repo.Resource(kind, id)
		if e != nil || v.TenantID != scope {
			return v, false
		}
		a.sources = append(a.sources, bleCertSource{kind, id, scope, v.Version})
		return v, true
	}
	device, ok := resource("device", deviceID, tenant)
	if !ok || stringField(device.Data, "status") != "ACTIVE" || stringField(device.Data, "model") != "EDGE_AGENT_S3" || stringField(device.Data, "credential_id") != activation.CredentialID || stringField(device.Data, "serial") != activation.Serial {
		return deny()
	}
	matches := 0
	var enrolled ResourceRecord
	for _, v := range s.repo.Resources("platform_device", platformDeviceTenant) {
		if stringField(v.Data, "serial") == activation.Serial && stringField(v.Data, "tenant_id") == tenant && oneOf(stringField(v.Data, "state"), "ASSIGNED", "DEPLOYED") {
			matches++
			enrolled = v
		}
	}
	if matches != 1 {
		return deny()
	}
	if _, ok = resource("platform_device", enrolled.ID, platformDeviceTenant); !ok {
		return deny()
	}
	_, public, _, e := activationPublicKeyJSON(stringField(enrolled.Data, "device_public_key_jwk"))
	if e != nil {
		return deny()
	}
	_, activationKey, _, e := activationPublicKeyJSON(activation.PublicKeyJWK)
	if e != nil || !public.Equal(activationKey) {
		return deny()
	}
	der, e := x509.MarshalPKIXPublicKey(public)
	if e != nil {
		return deny()
	}
	encoded := base64.RawURLEncoding.EncodeToString(der)
	kidHash := sha256.Sum256([]byte(encoded))
	if stringField(device.Data, "transaction_signing_public_key") != encoded || stringField(device.Data, "transaction_signing_kid") != "esp32-p256-"+base64.RawURLEncoding.EncodeToString(kidHash[:12]) {
		return deny()
	}
	pin := sha256.Sum256(der)
	binding, ok := resource("composite_binding", bindingID, tenant)
	if !ok || stringField(binding.Data, "status") != "ACTIVE" || stringField(binding.Data, "adapter_device_id") != deviceID {
		return deny()
	}
	hash := stringField(binding.Data, "payload_sha256")
	if !validSHA256(hash) {
		return deny()
	}
	issued, ok := resource("issued_binding", hash, tenant)
	if !ok {
		return deny()
	}
	raw := stringField(issued.Data, "canonical_json")
	actual := sha256.Sum256([]byte(raw))
	if hex.EncodeToString(actual[:]) != hash {
		return deny()
	}
	var payload struct {
		Schema     int64  `json:"schema_version"`
		Generation uint64 `json:"generation"`
		Tenant     string `json:"tenant_id"`
		Device     string `json:"edge_device_id"`
		Register   string `json:"register_id"`
		Location   string `json:"location_id"`
		Authority  struct {
			KID string `json:"transaction_signing_kid"`
		} `json:"operational_authority"`
	}
	if json.Unmarshal([]byte(raw), &payload) != nil || payload.Schema != 3 || payload.Tenant != tenant || payload.Device != deviceID || payload.Register != activation.ClaimedRegisterID || payload.Location != activation.ClaimedLocationID || payload.Generation == 0 || payload.Generation != exactBLEGeneration(binding.Data["generation"]) || payload.Authority.KID != stringField(device.Data, "transaction_signing_kid") {
		return deny()
	}
	register, ok := resource("register", payload.Register, tenant)
	if !ok || stringField(register.Data, "status") != "ACTIVE" || stringField(register.Data, "composite_binding_id") != bindingID || stringField(register.Data, "adapter_device_id") != deviceID || stringField(register.Data, "location_id") != payload.Location || stringField(binding.Data, "register_id") != payload.Register {
		return deny()
	}
	location, ok := resource("location", payload.Location, tenant)
	if !ok || stringField(location.Data, "status") != "ACTIVE" {
		return deny()
	}
	a.public = public
	a.challenge = BLEServerCertChallenge{Kind: "BLE_SERVER_CERT_CHALLENGE", Version: 1, Purpose: bleServerCertificatePurpose, DeviceID: deviceID, DeviceSPKI: hex.EncodeToString(pin[:]), TenantID: tenant, LocationID: payload.Location, RegisterID: payload.Register, BindingSHA256: hash, BindingGeneration: payload.Generation, ServerName: bleServerName(deviceID)}
	return a, nil
}
func (s *Service) NewBLEServerCertificateChallenge(deviceID, activationID, bindingID, tenant string) (BLEServerCertChallengeEnvelope, error) {
	if s.bleServerIssuer == nil {
		return BLEServerCertChallengeEnvelope{}, errors.New("BLE server issuance disabled")
	}
	if _, ok := s.repo.(bleCertificateRepository); !ok {
		return BLEServerCertChallengeEnvelope{}, errors.New("BLE certificate atomic repository unavailable")
	}
	a, e := s.bleCertificateAuthority(deviceID, activationID, bindingID, tenant)
	if e != nil {
		return BLEServerCertChallengeEnvelope{}, e
	}
	id, e := newUUID()
	if e != nil {
		return BLEServerCertChallengeEnvelope{}, e
	}
	nonce, e := randomURL(32)
	if e != nil {
		return BLEServerCertChallengeEnvelope{}, e
	}
	now := s.bleServerIssuer.clock().UTC()
	if now.Before(s.bleServerIssuer.ca.NotBefore) || !now.Before(s.bleServerIssuer.ca.NotAfter) {
		return BLEServerCertChallengeEnvelope{}, errors.New("BLE CA outside validity")
	}
	c := a.challenge
	c.ChallengeID = id
	c.Nonce = nonce
	c.IssuedAt = now.Unix()
	c.ExpiresAt = c.IssuedAt + 120
	c.CertificateNotAfter = c.IssuedAt + 86400
	if time.Unix(c.CertificateNotAfter, 0).After(s.bleServerIssuer.ca.NotAfter) {
		return BLEServerCertChallengeEnvelope{}, errors.New("BLE CA lifetime insufficient")
	}
	raw, e := edgeCanonical(c)
	if e != nil {
		return BLEServerCertChallengeEnvelope{}, e
	}
	signer, ok := s.deviceCredentialIssuer.(bleChallengeSigner)
	if !ok {
		return BLEServerCertChallengeEnvelope{}, errors.New("BLE root challenge signer unavailable")
	}
	sig, kid, e := signer.SignBLEServerCertChallenge(string(raw))
	if e != nil {
		return BLEServerCertChallengeEnvelope{}, e
	}
	envelope := BLEServerCertChallengeEnvelope{string(raw), sig, kid}
	record := ResourceRecord{Kind: "ble_server_cert_challenge", ID: id, TenantID: tenant, Version: 1, CreatedAt: now, UpdatedAt: now, Data: map[string]any{"canonical_json": string(raw), "signature": sig, "key_id": kid, "activation_id": activationID, "binding_id": bindingID}}
	if e = s.repo.(bleCertificateRepository).commitBLEServerChallenge(record, a.sources, a.activation); e != nil {
		return BLEServerCertChallengeEnvelope{}, e
	}
	return envelope, nil
}
func (s *Service) IssueBLEServerCertificate(challengeID, tenant, proofJSON, signature string) (BLEServerCertificate, error) {
	deny := func() (BLEServerCertificate, error) {
		return BLEServerCertificate{}, errors.New("BLE certificate proof rejected")
	}
	if s.bleServerIssuer == nil {
		return deny()
	}
	repo, ok := s.repo.(bleCertificateRepository)
	if !ok {
		return deny()
	}
	challenge, e := s.repo.Resource("ble_server_cert_challenge", challengeID)
	if e != nil || challenge.TenantID != tenant {
		return deny()
	}
	var c BLEServerCertChallenge
	if decodeBLECanonical(stringField(challenge.Data, "canonical_json"), &c) != nil || c.ChallengeID != challengeID || c.TenantID != tenant || !validBLEChallenge(c) {
		return deny()
	}
	var proof BLEServerCertProof
	if decodeBLECanonical(proofJSON, &proof) != nil {
		return deny()
	}
	digest := sha256.Sum256([]byte(bleCertChallengeDomain + stringField(challenge.Data, "canonical_json")))
	if proof.Kind != "BLE_SERVER_CERT_PROOF" || proof.Version != 1 || proof.Purpose != bleServerCertificatePurpose || proof.ChallengeID != challengeID || proof.ChallengeSHA256 != hex.EncodeToString(digest[:]) || proof.DeviceSPKI != c.DeviceSPKI {
		return deny()
	}
	a, e := s.bleCertificateAuthority(c.DeviceID, stringField(challenge.Data, "activation_id"), stringField(challenge.Data, "binding_id"), tenant)
	if e != nil {
		return deny()
	}
	expected := a.challenge
	expected.ChallengeID = c.ChallengeID
	expected.Nonce = c.Nonce
	expected.IssuedAt = c.IssuedAt
	expected.ExpiresAt = c.ExpiresAt
	expected.CertificateNotAfter = c.CertificateNotAfter
	if expected != c {
		return deny()
	}
	if len(signature) != 86 {
		return deny()
	}
	sig, e := base64.RawURLEncoding.Strict().DecodeString(signature)
	proofHash := sha256.Sum256([]byte(bleCertProofDomain + proofJSON))
	if e != nil || len(sig) != 64 || base64.RawURLEncoding.EncodeToString(sig) != signature || !verifyP1363(a.public, proofHash[:], sig) {
		return deny()
	}
	now := s.bleServerIssuer.clock().UTC()
	if now.Unix() < c.IssuedAt || now.Unix() >= c.CertificateNotAfter {
		return deny()
	}
	// Replay requires an authoritative persisted record, never an optimistic cache.
	if record, e := repo.confirmedBLEServerCertificate(challengeID, tenant, a.sources, a.activation); e == nil {
		if stringField(record.Data, "proof_sha256") != hex.EncodeToString(proofHash[:]) {
			return deny()
		}
		return s.bleServerIssuer.validatePersistedBLECertificate(record, c, a.public)
	} else if !errors.Is(e, ErrNotFound) {
		return BLEServerCertificate{}, e
	}
	if now.Unix() < c.IssuedAt || now.Unix() >= c.ExpiresAt || c.ExpiresAt-c.IssuedAt != 120 || c.CertificateNotAfter-c.IssuedAt != 86400 {
		return deny()
	}
	nonce, e := base64.RawURLEncoding.DecodeString(c.Nonce)
	if e != nil || len(nonce) != 32 {
		return deny()
	}
	cert, e := s.bleServerIssuer.issue(bleServerIssuancePermit{deviceID: c.DeviceID, public: a.public, notAfter: time.Unix(c.CertificateNotAfter, 0)})
	if e != nil {
		return BLEServerCertificate{}, e
	}
	encoded, e := json.Marshal(cert)
	if e != nil {
		return BLEServerCertificate{}, e
	}
	record := ResourceRecord{Kind: "ble_server_certificate", ID: challengeID, TenantID: tenant, Version: 1, CreatedAt: now, UpdatedAt: now, Data: map[string]any{"certificate_json": string(encoded), "proof_sha256": hex.EncodeToString(proofHash[:]), "challenge_sha256": proof.ChallengeSHA256}}
	committed, e := repo.commitBLEServerCertificate(challenge, record, a.sources, a.activation, s.bleServerIssuer.clock().UTC())
	if e != nil {
		return BLEServerCertificate{}, e
	}
	return s.bleServerIssuer.validatePersistedBLECertificate(committed, c, a.public)
}
func (i *BLEServerCertificateIssuer) validatePersistedBLECertificate(record ResourceRecord, c BLEServerCertChallenge, public *ecdsa.PublicKey) (BLEServerCertificate, error) {
	deny := func() (BLEServerCertificate, error) {
		return BLEServerCertificate{}, errors.New("persisted BLE certificate validation failed")
	}
	var certificate BLEServerCertificate
	raw := stringField(record.Data, "certificate_json")
	if len(raw) == 0 || len(raw) > 65536 {
		return deny()
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&certificate) != nil {
		return deny()
	}
	var tail any
	if decoder.Decode(&tail) != io.EOF {
		return deny()
	}
	if record.Kind != "ble_server_certificate" || record.ID != c.ChallengeID || record.TenantID != c.TenantID || record.Version != 1 || certificate.Profile != bleServerCertificatePurpose || certificate.DeviceID != c.DeviceID || certificate.ServerName != c.ServerName || certificate.SPKISHA256 != c.DeviceSPKI || !certificate.NotAfter.Equal(time.Unix(c.CertificateNotAfter, 0)) || certificate.CAChainPEM != i.chain {
		return deny()
	}
	canonical, e := edgeCanonical(c)
	if e != nil {
		return deny()
	}
	challengeHash := sha256.Sum256([]byte(bleCertChallengeDomain + string(canonical)))
	if stringField(record.Data, "challenge_sha256") != hex.EncodeToString(challengeHash[:]) || !validSHA256(stringField(record.Data, "proof_sha256")) {
		return deny()
	}
	block, rest := pem.Decode([]byte(certificate.CertificatePEM))
	if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return deny()
	}
	leaf, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		return deny()
	}
	key, ok := leaf.PublicKey.(*ecdsa.PublicKey)
	if !ok || key.Curve != elliptic.P256() || public == nil || !key.Equal(public) || leaf.IsCA || !leaf.BasicConstraintsValid || leaf.KeyUsage != x509.KeyUsageDigitalSignature || len(leaf.ExtKeyUsage) != 1 || leaf.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth || len(leaf.UnknownExtKeyUsage) != 0 || len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != c.ServerName || len(leaf.URIs) != 1 || leaf.URIs[0].String() != fmt.Sprintf("spiffe://beefiscal/ble-server/device/%s", c.DeviceID) || leaf.Subject.CommonName != c.DeviceID || !leaf.NotAfter.Equal(certificate.NotAfter) || leaf.NotBefore.Before(time.Unix(c.IssuedAt-60, 0)) {
		return deny()
	}
	pin := sha256.Sum256(leaf.RawSubjectPublicKeyInfo)
	digest := sha256.Sum256(leaf.Raw)
	if hex.EncodeToString(pin[:]) != c.DeviceSPKI || hex.EncodeToString(digest[:]) != certificate.CertificateSHA256 {
		return deny()
	}
	roots := x509.NewCertPool()
	roots.AddCert(i.ca)
	if _, e = leaf.Verify(x509.VerifyOptions{Roots: roots, DNSName: c.ServerName, CurrentTime: i.clock().UTC(), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); e != nil {
		return deny()
	}
	return certificate, nil
}

func exactBLEGeneration(v any) uint64 {
	switch n := v.(type) {
	case int:
		if n > 0 {
			return uint64(n)
		}
	case int64:
		if n > 0 {
			return uint64(n)
		}
	case uint64:
		if n > 0 && n <= math.MaxInt64 {
			return n
		}
	case json.Number:
		x, e := strconv.ParseUint(string(n), 10, 63)
		if e == nil {
			return x
		}
	case float64:
		if n > 0 && n < 9007199254740992 && math.Trunc(n) == n {
			return uint64(n)
		}
	}
	return 0
}
func validBLEChallenge(c BLEServerCertChallenge) bool {
	nonce, e := base64.RawURLEncoding.DecodeString(c.Nonce)
	return c.Kind == "BLE_SERVER_CERT_CHALLENGE" && c.Version == 1 && c.Purpose == bleServerCertificatePurpose && uuidPattern.MatchString(c.ChallengeID) && uuidPattern.MatchString(c.DeviceID) && c.TenantID != "" && c.LocationID != "" && c.RegisterID != "" && validSHA256(c.DeviceSPKI) && validSHA256(c.BindingSHA256) && c.BindingGeneration > 0 && c.BindingGeneration <= math.MaxInt64 && c.ServerName == bleServerName(c.DeviceID) && e == nil && len(nonce) == 32 && base64.RawURLEncoding.EncodeToString(nonce) == c.Nonce && c.IssuedAt > 0 && c.IssuedAt <= math.MaxInt64-86400 && c.ExpiresAt == c.IssuedAt+120 && c.CertificateNotAfter == c.IssuedAt+86400
}
