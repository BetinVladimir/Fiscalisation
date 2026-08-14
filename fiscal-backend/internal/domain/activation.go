package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
)

type DeviceActivationChallenge struct {
	ID               string     `json:"activation_challenge_id"`
	DeviceInstanceID string     `json:"device_instance_id"`
	NonceHash        string     `json:"nonce_hash"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
	ConsumedAt       *time.Time `json:"consumed_at,omitempty"`
}
type DeviceActivationRequest struct {
	ID                 string     `json:"activation_request_id"`
	SecretHash         string     `json:"request_secret_hash"`
	UserCodeHash       string     `json:"user_code_hash"`
	DeviceInstanceID   string     `json:"device_instance_id"`
	PublicKeyJWK       string     `json:"device_public_key_jwk"`
	KeyThumbprint      string     `json:"device_key_thumbprint"`
	Vendor             string     `json:"vendor"`
	Model              string     `json:"model"`
	Serial             string     `json:"serial"`
	FMIN               string     `json:"fmin"`
	Firmware           string     `json:"firmware,omitempty"`
	CapabilityDigest   string     `json:"capability_digest"`
	State              string     `json:"state"`
	RequestedRoles     []string   `json:"requested_roles"`
	ClaimedRoles       []string   `json:"claimed_roles,omitempty"`
	ClaimedTenantID    string     `json:"claimed_tenant_id,omitempty"`
	ClaimedLocationID  string     `json:"claimed_location_id,omitempty"`
	ClaimedRegisterID  string     `json:"claimed_register_id,omitempty"`
	ClaimedBySubject   string     `json:"claimed_by_subject,omitempty"`
	CredentialID       string     `json:"credential_id,omitempty"`
	MQTTTLSURI         string     `json:"mqtt_tls_uri,omitempty"`
	MQTTClientID       string     `json:"mqtt_client_id,omitempty"`
	UNPPrefix          string     `json:"unp_prefix,omitempty"`
	UNPRangeStart      int64      `json:"unp_range_start,omitempty"`
	UNPRangeEnd        int64      `json:"unp_range_end,omitempty"`
	BindingVersion     int64      `json:"binding_version,omitempty"`
	ExpiresAt          time.Time  `json:"expires_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	ClaimedAt          *time.Time `json:"claimed_at,omitempty"`
	CredentialIssuedAt *time.Time `json:"credential_issued_at,omitempty"`
}
type CreateDeviceActivationInput struct {
	ChallengeID      string         `json:"challenge_id"`
	Challenge        string         `json:"challenge"`
	DeviceInstanceID string         `json:"device_instance_id"`
	PublicKeyJWK     map[string]any `json:"device_public_key_jwk"`
	Vendor           string         `json:"vendor"`
	Model            string         `json:"model"`
	Serial           string         `json:"serial"`
	FMIN             string         `json:"fmin"`
	Firmware         string         `json:"firmware"`
	CapabilityDigest string         `json:"capability_digest"`
	Signature        string         `json:"signature"`
	RequestedRoles   []string       `json:"requested_roles"`
}
type ConfirmDeviceActivationInput struct {
	UserCode     string   `json:"user_code"`
	LocationID   string   `json:"location_id"`
	RegisterID   string   `json:"register_id"`
	ActorSubject string   `json:"-"`
	Roles        []string `json:"roles"`
}
type DeviceCredential struct {
	CredentialID                    string   `json:"credential_id"`
	ClientCertificatePEM            string   `json:"client_certificate_pem"`
	CAChainPEM                      string   `json:"ca_chain_pem"`
	MQTTTLSURI                      string   `json:"mqtt_tls_uri"`
	MQTTWSSURI                      string   `json:"mqtt_wss_uri,omitempty"`
	BindingSignature                string   `json:"binding_signature"`
	DeviceInstanceID                string   `json:"device_instance_id"`
	OrganizationID                  string   `json:"organization_id"`
	LocationID                      string   `json:"location_id"`
	RegisterID                      string   `json:"register_id"`
	Roles                           []string `json:"roles"`
	BindingVersion                  int64    `json:"binding_version"`
	CommandHMACKey                  string   `json:"command_hmac_key"`
	SyncAckHMACKey                  string   `json:"sync_ack_hmac_key"`
	BLETicketHMACKey                string   `json:"ble_ticket_hmac_key"`
	UNPPrefix                       string   `json:"unp_prefix"`
	UNPRangeStart                   int64    `json:"unp_range_start"`
	UNPRangeEnd                     int64    `json:"unp_range_end"`
	LocalTokenIssuer                string   `json:"local_token_issuer,omitempty"`
	LocalTokenSigningKID            string   `json:"local_token_signing_kid,omitempty"`
	LocalTokenPublicKeyDERBase64    string   `json:"local_token_public_key_der_base64,omitempty"`
	SPADeploymentDescriptorURL      string   `json:"spa_deployment_descriptor_url,omitempty"`
	SPADeploymentSigningKID         string   `json:"spa_deployment_signing_kid,omitempty"`
	SPADeploymentPublicKeyDERBase64 string   `json:"spa_deployment_public_key_der_base64,omitempty"`
}

func DeriveDeviceTransportKey(master []byte, purpose, deviceID, credentialID string) []byte {
	mac := hmac.New(sha256.New, master)
	mac.Write([]byte("beefiscal-device-v1\x00" + purpose + "\x00" + deviceID + "\x00" + credentialID))
	return mac.Sum(nil)
}

type DeviceCredentialIssuer interface {
	Issue(DeviceActivationRequest) (DeviceCredential, error)
}
type DeviceActivationAcknowledgementSigner interface {
	SignActivationAcknowledgement([]byte) (string, error)
}
type CompositeBindingSigner interface {
	SignCompositeBinding([]byte) (string, string, error)
}

func (s *Service) signCompositeBinding(value []byte) (string, string, error) {
	v, ok := s.deviceCredentialIssuer.(CompositeBindingSigner)
	if !ok {
		return "", "", errors.New("composite binding signer unavailable")
	}
	return v.SignCompositeBinding(value)
}

func DeviceActivationPublicView(v DeviceActivationRequest) map[string]any {
	return map[string]any{"activation_request_id": v.ID, "device_instance_id": v.DeviceInstanceID, "device_key_thumbprint": v.KeyThumbprint, "vendor": v.Vendor, "model": v.Model, "serial": v.Serial, "fmin": v.FMIN, "capability_digest": v.CapabilityDigest, "state": v.State, "requested_roles": v.RequestedRoles, "claimed_tenant_id": v.ClaimedTenantID, "claimed_location_id": v.ClaimedLocationID, "claimed_register_id": v.ClaimedRegisterID, "claimed_roles": v.ClaimedRoles, "binding_version": v.BindingVersion, "expires_at": v.ExpiresAt, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt}
}

func (s *Service) SetDeviceCredentialIssuer(v DeviceCredentialIssuer) { s.deviceCredentialIssuer = v }
func (s *Service) SetLocalTokenTrust(issuer, kid, publicKeyDERBase64 string) {
	s.localTokenIssuer, s.localTokenSigningKID, s.localTokenPublicKeyDERBase64 = issuer, kid, publicKeyDERBase64
}
func (s *Service) SetSPADeploymentTrust(url, kid, publicKeyDERBase64 string) {
	s.spaDeploymentDescriptorURL, s.spaDeploymentSigningKID, s.spaDeploymentPublicKeyDERBase64 = url, kid, publicKeyDERBase64
}

func (s *Service) SignDeviceActivationAcknowledgement(unsigned []byte) (string, error) {
	signer, ok := s.deviceCredentialIssuer.(DeviceActivationAcknowledgementSigner)
	if !ok {
		return "", errors.New("device activation acknowledgement signer unavailable")
	}
	return signer.SignActivationAcknowledgement(unsigned)
}

func randomURL(n int) (string, error) {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func digestString(v string) string { x := sha256.Sum256([]byte(v)); return fmt.Sprintf("%x", x) }
func (s *Service) codeHash(v string) string {
	m := hmac.New(sha256.New, s.bleSigningKey)
	m.Write([]byte(strings.ToUpper(strings.ReplaceAll(v, "-", ""))))
	return fmt.Sprintf("%x", m.Sum(nil))
}

func (s *Service) NewDeviceActivationChallenge(deviceInstanceID string) (map[string]any, error) {
	if !uuidPattern.MatchString(deviceInstanceID) {
		return nil, errors.New("invalid device instance id")
	}
	id, e := newUUID()
	if e != nil {
		return nil, e
	}
	nonce, e := randomURL(32)
	if e != nil {
		return nil, e
	}
	now := time.Now().UTC()
	v := DeviceActivationChallenge{ID: id, DeviceInstanceID: deviceInstanceID, NonceHash: digestString(nonce), ExpiresAt: now.Add(2 * time.Minute), CreatedAt: now}
	if e = s.repo.PutActivationChallenge(v); e != nil {
		return nil, e
	}
	return map[string]any{"challenge_id": id, "challenge": nonce, "expires_at": v.ExpiresAt}, nil
}

func (s *Service) CreateDeviceActivation(input CreateDeviceActivationInput) (map[string]any, error) {
	now := time.Now().UTC()
	if !uuidPattern.MatchString(input.DeviceInstanceID) || input.ChallengeID == "" || input.Challenge == "" || !oneOf(strings.ToUpper(input.Vendor), "DATECS", "DAISY") || input.Model == "" || input.Serial == "" || input.FMIN == "" || input.CapabilityDigest == "" {
		return nil, errors.New("invalid activation request")
	}
	jwk, key, thumb, e := activationPublicKey(input.PublicKeyJWK)
	if e != nil {
		return nil, e
	}
	roles := normalizedActivationRoles(input.RequestedRoles)
	if len(roles) == 0 {
		return nil, errors.New("activation role required")
	}
	canonical := activationProof(input, jwk)
	signature, e := base64.RawURLEncoding.DecodeString(input.Signature)
	if e != nil || !verifyP1363(key, sha256Bytes(canonical), signature) {
		return nil, errors.New("invalid activation proof")
	}
	id, e := newUUID()
	if e != nil {
		return nil, e
	}
	secret, e := randomURL(32)
	if e != nil {
		return nil, e
	}
	code, e := activationUserCode()
	if e != nil {
		return nil, e
	}
	v := DeviceActivationRequest{ID: id, SecretHash: digestString(secret), UserCodeHash: s.codeHash(code), DeviceInstanceID: input.DeviceInstanceID, PublicKeyJWK: jwk, KeyThumbprint: thumb, Vendor: strings.ToUpper(input.Vendor), Model: input.Model, Serial: input.Serial, FMIN: input.FMIN, Firmware: input.Firmware, CapabilityDigest: input.CapabilityDigest, State: "PENDING", RequestedRoles: roles, ExpiresAt: now.Add(10 * time.Minute), CreatedAt: now, UpdatedAt: now}
	if e = s.repo.ConsumeChallengeAndCreateActivation(input.ChallengeID, digestString(input.Challenge), v); e != nil {
		return nil, e
	}
	return map[string]any{"activation_request_id": id, "request_secret": secret, "user_code": code, "verification_uri": "https://fiscal.beeloy.com/activate", "expires_at": v.ExpiresAt, "interval": 5, "device_instance_id": v.DeviceInstanceID, "vendor": v.Vendor, "model": v.Model, "serial_suffix": suffix(v.Serial), "fmin_suffix": suffix(v.FMIN), "device_key_thumbprint": thumb}, nil
}

func (s *Service) ConfirmDeviceActivation(id string, input ConfirmDeviceActivationInput, tenant string) (DeviceActivationRequest, error) {
	if tenant == "" || input.ActorSubject == "" || input.LocationID == "" || input.RegisterID == "" {
		return DeviceActivationRequest{}, errors.New("invalid activation confirmation")
	}
	location, e := s.repo.Resource("location", input.LocationID)
	if e != nil || location.TenantID != tenant {
		return DeviceActivationRequest{}, ErrNotFound
	}
	register, e := s.repo.Resource("register", input.RegisterID)
	if e != nil || register.TenantID != tenant || stringField(register.Data, "location_id") != input.LocationID {
		return DeviceActivationRequest{}, ErrNotFound
	}
	return s.repo.ConfirmActivationRequest(id, s.codeHash(input.UserCode), tenant, input.LocationID, input.RegisterID, normalizedActivationRoles(input.Roles), input.ActorSubject, time.Now().UTC())
}
func (s *Service) LookupDeviceActivation(userCode string) (map[string]any, error) {
	v, e := s.repo.ActivationRequestByCode(s.codeHash(userCode))
	if e != nil || v.State != "PENDING" || time.Now().UTC().After(v.ExpiresAt) {
		return nil, ErrNotFound
	}
	view := DeviceActivationPublicView(v)
	delete(view, "claimed_tenant_id")
	delete(view, "claimed_location_id")
	delete(view, "claimed_register_id")
	delete(view, "claimed_roles")
	delete(view, "binding_version")
	return view, nil
}

func (s *Service) DisconnectSmartDevice(deviceID, tenant, actor string) (map[string]any, error) {
	if deviceID == "" || tenant == "" || actor == "" {
		return nil, errors.New("invalid device disconnect")
	}
	v, e := s.repo.RevokeActivatedDevice(deviceID, tenant, actor, time.Now().UTC())
	if e != nil {
		return nil, e
	}
	return DeviceActivationPublicView(v), nil
}

func (s *Service) IssueDeviceActivationCredential(id, requestSecret, nonce, signature string) (DeviceCredential, error) {
	v, e := s.repo.ActivationRequest(id)
	if e != nil {
		return DeviceCredential{}, e
	}
	if v.State != "CONFIRMED" || time.Now().UTC().After(v.ExpiresAt) || !hmac.Equal([]byte(v.SecretHash), []byte(digestString(requestSecret))) {
		return DeviceCredential{}, errors.New("activation credential denied")
	}
	_, key, _, e := activationPublicKeyJSON(v.PublicKeyJWK)
	if e != nil {
		return DeviceCredential{}, e
	}
	sig, e := base64.RawURLEncoding.DecodeString(signature)
	proof := "credential\n" + id + "\n" + nonce + "\n" + digestString(requestSecret)
	if e != nil || !verifyP1363(key, sha256Bytes(proof), sig) {
		return DeviceCredential{}, errors.New("invalid credential proof")
	}
	if s.deviceCredentialIssuer == nil {
		return DeviceCredential{}, errors.New("device credential issuer unavailable")
	}
	credential, e := s.deviceCredentialIssuer.Issue(v)
	if e != nil {
		return DeviceCredential{}, e
	}
	if credential.CredentialID == "" || credential.ClientCertificatePEM == "" || credential.CAChainPEM == "" || !strings.HasPrefix(credential.MQTTTLSURI, "ssl://") || credential.CommandHMACKey == "" || credential.SyncAckHMACKey == "" || credential.BLETicketHMACKey == "" {
		return DeviceCredential{}, errors.New("invalid issued credential")
	}
	credential.LocalTokenIssuer = s.localTokenIssuer
	credential.LocalTokenSigningKID = s.localTokenSigningKID
	credential.LocalTokenPublicKeyDERBase64 = s.localTokenPublicKeyDERBase64
	credential.SPADeploymentDescriptorURL = s.spaDeploymentDescriptorURL
	credential.SPADeploymentSigningKID = s.spaDeploymentSigningKID
	credential.SPADeploymentPublicKeyDERBase64 = s.spaDeploymentPublicKeyDERBase64
	credential.UNPPrefix = v.FMIN
	credential.UNPRangeStart, credential.UNPRangeEnd, e = s.repo.ReserveUNPRange(v.ClaimedTenantID, v.FMIN, 1000)
	if e != nil {
		return DeviceCredential{}, e
	}
	_, e = s.repo.MarkActivationCredentialIssued(id, credential, time.Now().UTC())
	return credential, e
}

func (s *Service) ActivateDeviceCredential(id, credentialID, nonce, signature string) (DeviceActivationRequest, error) {
	v, e := s.repo.ActivationRequest(id)
	if e != nil {
		return DeviceActivationRequest{}, e
	}
	if v.State != "CREDENTIAL_ISSUED" || v.CredentialID != credentialID {
		return DeviceActivationRequest{}, errors.New("activation commit denied")
	}
	_, key, _, e := activationPublicKeyJSON(v.PublicKeyJWK)
	if e != nil {
		return DeviceActivationRequest{}, e
	}
	proof := fmt.Sprintf("activate\n%s\n%s\n%d\n%s", id, credentialID, v.BindingVersion, nonce)
	sig, e := base64.RawURLEncoding.DecodeString(signature)
	if e != nil || !verifyP1363(key, sha256Bytes(proof), sig) {
		return DeviceActivationRequest{}, errors.New("invalid activation commit proof")
	}
	return s.repo.ActivateDeviceRequest(id, credentialID, time.Now().UTC())
}

func activationPublicKey(v map[string]any) (string, *ecdsa.PublicKey, string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", nil, "", e
	}
	return activationPublicKeyJSON(string(b))
}
func activationPublicKeyJSON(raw string) (string, *ecdsa.PublicKey, string, error) {
	var v map[string]any
	if json.Unmarshal([]byte(raw), &v) != nil || v["kty"] != "EC" || v["crv"] != "P-256" {
		return "", nil, "", errors.New("invalid activation jwk")
	}
	xS, xOK := v["x"].(string)
	yS, yOK := v["y"].(string)
	if !xOK || !yOK {
		return "", nil, "", errors.New("invalid activation jwk")
	}
	xb, e1 := base64.RawURLEncoding.DecodeString(xS)
	yb, e2 := base64.RawURLEncoding.DecodeString(yS)
	if e1 != nil || e2 != nil || len(xb) != 32 || len(yb) != 32 {
		return "", nil, "", errors.New("invalid activation jwk")
	}
	x, y := new(big.Int).SetBytes(xb), new(big.Int).SetBytes(yb)
	if !elliptic.P256().IsOnCurve(x, y) {
		return "", nil, "", errors.New("invalid activation jwk")
	}
	canonical := fmt.Sprintf(`{"crv":"P-256","kty":"EC","x":%q,"y":%q}`, xS, yS)
	thumb := base64.RawURLEncoding.EncodeToString(sha256Bytes(canonical))
	return canonical, &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, thumb, nil
}
func activationProof(v CreateDeviceActivationInput, jwk string) string {
	return strings.Join([]string{"activate", v.ChallengeID, v.Challenge, v.DeviceInstanceID, jwk, strings.ToUpper(v.Vendor), v.Model, v.Serial, v.FMIN, v.Firmware, v.CapabilityDigest, strings.Join(normalizedActivationRoles(v.RequestedRoles), ",")}, "\n")
}
func normalizedActivationRoles(v []string) []string {
	seen := map[string]bool{}
	for _, x := range v {
		if oneOf(x, "FISCAL_DEVICE", "PAYMENT_TERMINAL") {
			seen[x] = true
		}
	}
	out := make([]string, 0, len(seen))
	for x := range seen {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}
func activationUserCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	raw := make([]byte, 12)
	if _, e := rand.Read(raw); e != nil {
		return "", e
	}
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(raw[:4]) + "-" + string(raw[4:8]) + "-" + string(raw[8:]), nil
}
func suffix(v string) string {
	if len(v) <= 4 {
		return v
	}
	return v[len(v)-4:]
}
func sha256Bytes(v string) []byte { x := sha256.Sum256([]byte(v)); return x[:] }

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func (r *MemoryRepository) PutActivationChallenge(v DeviceActivationChallenge) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.activationChallenges[v.ID]; ok {
		return errors.New("activation challenge conflict")
	}
	r.activationChallenges[v.ID] = v
	return r.persistLocked()
}
func (r *MemoryRepository) ConsumeChallengeAndCreateActivation(challengeID, nonceHash string, v DeviceActivationRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.activationChallenges[challengeID]
	if !ok || c.ConsumedAt != nil || time.Now().UTC().After(c.ExpiresAt) || c.DeviceInstanceID != v.DeviceInstanceID || !hmac.Equal([]byte(c.NonceHash), []byte(nonceHash)) {
		return errors.New("activation challenge invalid")
	}
	for _, x := range r.activationRequests {
		if (x.DeviceInstanceID == v.DeviceInstanceID || x.KeyThumbprint == v.KeyThumbprint || x.Serial == v.Serial || x.FMIN == v.FMIN) && oneOf(x.State, "PENDING", "CONFIRMED", "CREDENTIAL_ISSUED", "ACTIVE") {
			return errors.New("activation identity conflict")
		}
	}
	now := time.Now().UTC()
	c.ConsumedAt = &now
	r.activationChallenges[challengeID] = c
	r.activationRequests[v.ID] = v
	return r.persistLocked()
}
func (r *MemoryRepository) ActivationRequest(id string) (DeviceActivationRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.activationRequests[id]
	if !ok {
		return DeviceActivationRequest{}, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) ActivationRequestByCode(hash string) (DeviceActivationRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.activationRequests {
		if hmac.Equal([]byte(v.UserCodeHash), []byte(hash)) {
			return v, nil
		}
	}
	return DeviceActivationRequest{}, ErrNotFound
}
func (r *MemoryRepository) ConfirmActivationRequest(id, codeHash, tenant, location, register string, roles []string, actor string, now time.Time) (DeviceActivationRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.activationRequests[id]
	if !ok {
		return DeviceActivationRequest{}, ErrNotFound
	}
	if v.State == "CONFIRMED" && v.ClaimedTenantID == tenant && v.ClaimedLocationID == location && v.ClaimedRegisterID == register {
		return v, nil
	}
	if v.State != "PENDING" || now.After(v.ExpiresAt) || !hmac.Equal([]byte(v.UserCodeHash), []byte(codeHash)) {
		return DeviceActivationRequest{}, errors.New("activation confirmation conflict")
	}
	for _, role := range roles {
		if !contains(v.RequestedRoles, role) {
			return DeviceActivationRequest{}, errors.New("activation capability role conflict")
		}
	}
	if !contains(roles, "FISCAL_DEVICE") {
		return DeviceActivationRequest{}, errors.New("fiscal role required")
	}
	v.State = "CONFIRMED"
	v.ClaimedTenantID = tenant
	v.ClaimedLocationID = location
	v.ClaimedRegisterID = register
	v.ClaimedRoles = roles
	v.ClaimedBySubject = actor
	v.BindingVersion = 1
	v.ClaimedAt = &now
	v.UpdatedAt = now
	r.activationRequests[id] = v
	return v, r.persistLocked()
}
func (r *MemoryRepository) MarkActivationCredentialIssued(id string, credential DeviceCredential, now time.Time) (DeviceActivationRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.activationRequests[id]
	if !ok {
		return DeviceActivationRequest{}, ErrNotFound
	}
	if v.State != "CONFIRMED" {
		return DeviceActivationRequest{}, errors.New("activation state conflict")
	}
	v.State = "CREDENTIAL_ISSUED"
	v.CredentialID = credential.CredentialID
	v.MQTTTLSURI = credential.MQTTTLSURI
	v.MQTTClientID = v.DeviceInstanceID
	v.UNPPrefix = credential.UNPPrefix
	v.UNPRangeStart = credential.UNPRangeStart
	v.UNPRangeEnd = credential.UNPRangeEnd
	v.CredentialIssuedAt = &now
	v.UpdatedAt = now
	r.activationRequests[id] = v
	return v, r.persistLocked()
}
func (r *MemoryRepository) ActivateDeviceRequest(id, credential string, now time.Time) (DeviceActivationRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.activationRequests[id]
	if !ok {
		return DeviceActivationRequest{}, ErrNotFound
	}
	if v.State == "ACTIVE" && v.CredentialID == credential {
		return v, nil
	}
	if v.State != "CREDENTIAL_ISSUED" || v.CredentialID != credential || v.ClaimedTenantID == "" {
		return DeviceActivationRequest{}, errors.New("activation state conflict")
	}
	register, key := r.resources[resourceKey("register", v.ClaimedRegisterID)], resourceKey("register", v.ClaimedRegisterID)
	if register.ID == "" || register.TenantID != v.ClaimedTenantID || stringField(register.Data, "location_id") != v.ClaimedLocationID {
		return DeviceActivationRequest{}, errors.New("activation register binding lost")
	}
	_, transactionKey, _, keyErr := activationPublicKeyJSON(v.PublicKeyJWK)
	if keyErr != nil {
		return DeviceActivationRequest{}, keyErr
	}
	transactionDER, keyErr := x509.MarshalPKIXPublicKey(transactionKey)
	if keyErr != nil {
		return DeviceActivationRequest{}, keyErr
	}
	device := ResourceRecord{Kind: "device", TenantID: v.ClaimedTenantID, ID: v.DeviceInstanceID, Version: 1, Data: map[string]any{"kind": "SMART_DEVICE", "vendor": v.Vendor, "model": v.Model, "serial": v.Serial, "fiscal_memory_number": v.FMIN, "firmware": v.Firmware, "status": "ACTIVE", "environment": "PROD", "device_key_thumbprint": v.KeyThumbprint, "credential_id": credential, "transaction_signing_public_key": base64.RawURLEncoding.EncodeToString(transactionDER), "transaction_signing_kid": v.KeyThumbprint, "capabilities": v.RequestedRoles, "binding_version": v.BindingVersion, "mqtt_uri": v.MQTTTLSURI, "mqtt_client_id": v.MQTTClientID, "ble_advertising_identity": v.DeviceInstanceID, "unp_prefix": v.UNPPrefix, "unp_range_start": v.UNPRangeStart, "unp_range_end": v.UNPRangeEnd}, CreatedAt: now, UpdatedAt: now}
	register.Data["fiscal_device_id"] = device.ID
	register.Data["fiscal_device_active_from"] = now.Format(time.RFC3339Nano)
	if contains(v.ClaimedRoles, "PAYMENT_TERMINAL") {
		register.Data["payment_terminal_id"] = device.ID
		register.Data["payment_terminal_active_from"] = now.Format(time.RFC3339Nano)
	}
	register.Version++
	register.UpdatedAt = now
	r.resources[resourceKey("device", device.ID)] = device
	r.resources[key] = register
	v.State = "ACTIVE"
	v.UpdatedAt = now
	r.activationRequests[id] = v
	r.appendAuditLocked(v.ClaimedTenantID, v.ClaimedBySubject, "SMART_DEVICE_ACTIVATED", "device", device.ID, "", nil, asMap(device))
	return v, r.persistLocked()
}

func (r *MemoryRepository) RevokeActivatedDevice(deviceID, tenant, actor string, now time.Time) (DeviceActivationRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var id string
	var v DeviceActivationRequest
	for candidateID, candidate := range r.activationRequests {
		if candidate.DeviceInstanceID == deviceID && candidate.ClaimedTenantID == tenant {
			id, v = candidateID, candidate
			break
		}
	}
	if id == "" {
		return DeviceActivationRequest{}, ErrNotFound
	}
	if v.State == "REVOKED" {
		return v, nil
	}
	if v.State != "ACTIVE" {
		return DeviceActivationRequest{}, errors.New("device is not active")
	}
	deviceKey := resourceKey("device", deviceID)
	device, ok := r.resources[deviceKey]
	if !ok || device.TenantID != tenant {
		return DeviceActivationRequest{}, ErrNotFound
	}
	registerKey := resourceKey("register", v.ClaimedRegisterID)
	register, ok := r.resources[registerKey]
	if ok && register.TenantID == tenant {
		if stringField(register.Data, "fiscal_device_id") == deviceID {
			delete(register.Data, "fiscal_device_id")
			delete(register.Data, "fiscal_device_active_from")
		}
		if stringField(register.Data, "payment_terminal_id") == deviceID {
			delete(register.Data, "payment_terminal_id")
			delete(register.Data, "payment_terminal_active_from")
		}
		register.Version++
		register.UpdatedAt = now
		r.resources[registerKey] = register
	}
	for sessionID, session := range r.bleSessions {
		if session.TenantID == tenant && session.FiscalDeviceID == deviceID && !session.Revoked {
			session.Revoked = true
			r.bleSessions[sessionID] = session
		}
	}
	device.Data["status"] = "REVOKED"
	device.Data["revoked_at"] = now.Format(time.RFC3339Nano)
	device.Data["credential_id"] = ""
	device.Version++
	device.UpdatedAt = now
	r.resources[deviceKey] = device
	v.State = "REVOKED"
	v.BindingVersion++
	v.UpdatedAt = now
	r.activationRequests[id] = v
	r.appendAuditLocked(tenant, actor, "SMART_DEVICE_REVOKED", "device", deviceID, "", nil, asMap(device))
	return v, r.persistLocked()
}
