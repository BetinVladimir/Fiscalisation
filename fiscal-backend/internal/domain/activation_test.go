package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"math/big"
	"strings"
	"testing"
	"time"
)

type activationIssuer struct{}

func (activationIssuer) Issue(v DeviceActivationRequest) (DeviceCredential, error) {
	return DeviceCredential{CredentialID: "credential-1", ClientCertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----", CAChainPEM: "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----", MQTTTLSURI: "ssl://mqtt.example:8883", MQTTWSSURI: "wss://mqtt.example/mqtt", BindingSignature: "signed", CommandHMACKey: "command-key", SyncAckHMACKey: "ack-key", BLETicketHMACKey: "ticket-key"}, nil
}

func TestProofBoundTenantlessActivationLifecycle(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	deviceID := "11111111-1111-4111-8111-111111111111"
	challenge, err := svc.NewDeviceActivationChallenge(deviceID)
	if err != nil {
		t.Fatal(err)
	}
	private, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	enc := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.FillBytes(make([]byte, 32))) }
	jwk := map[string]any{"kty": "EC", "crv": "P-256", "x": enc(private.X), "y": enc(private.Y)}
	input := CreateDeviceActivationInput{ChallengeID: challenge["challenge_id"].(string), Challenge: challenge["challenge"].(string), DeviceInstanceID: deviceID, PublicKeyJWK: jwk, Vendor: "DATECS", Model: "BLUECASH_50", Serial: "BC501234", FMIN: "12345678", Firmware: "1.0", CapabilityDigest: "a" + strings.Repeat("0", 63), RequestedRoles: []string{"PAYMENT_TERMINAL", "FISCAL_DEVICE"}}
	canonicalJWK, _, _, _ := activationPublicKey(jwk)
	sig, _ := ecdsa.SignASN1(rand.Reader, private, sha256Bytes(activationProof(input, canonicalJWK)))
	input.Signature = base64.RawURLEncoding.EncodeToString(sig)
	created, err := svc.CreateDeviceActivation(input)
	if err != nil {
		t.Fatal(err)
	}
	id := created["activation_request_id"].(string)
	secret := created["request_secret"].(string)
	code := created["user_code"].(string)
	lookup, err := svc.LookupDeviceActivation(code)
	if err != nil || lookup["activation_request_id"] != id || lookup["state"] != "PENDING" {
		t.Fatal("pending activation lookup failed", lookup, err)
	}
	if _, exposed := lookup["claimed_tenant_id"]; exposed {
		t.Fatal("tenant claim exposed by pre-confirmation lookup")
	}
	pending, _ := repo.ActivationRequest(id)
	if pending.State != "PENDING" || pending.ClaimedTenantID != "" || pending.SecretHash == secret || pending.UserCodeHash == code {
		t.Fatalf("secret/plain tenant leak: %+v", pending)
	}
	now := time.Now().UTC()
	if err = repo.PutResource(ResourceRecord{Kind: "location", TenantID: "tenant-1", ID: "location-1", Version: 1, Data: map[string]any{"status": "ACTIVE"}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err = repo.PutResource(ResourceRecord{Kind: "register", TenantID: "tenant-1", ID: "register-1", Version: 1, Data: map[string]any{"location_id": "location-1", "status": "ACTIVE"}, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.ConfirmDeviceActivation(id, ConfirmDeviceActivationInput{UserCode: "WRONG-CODE", LocationID: "location-1", RegisterID: "register-1", Roles: []string{"FISCAL_DEVICE"}, ActorSubject: "admin-1"}, "tenant-1"); err == nil {
		t.Fatal("wrong user code accepted")
	}
	confirmed, err := svc.ConfirmDeviceActivation(id, ConfirmDeviceActivationInput{UserCode: code, LocationID: "location-1", RegisterID: "register-1", Roles: []string{"FISCAL_DEVICE", "PAYMENT_TERMINAL"}, ActorSubject: "admin-1"}, "tenant-1")
	if err != nil || confirmed.State != "CONFIRMED" || confirmed.ClaimedTenantID != "tenant-1" {
		t.Fatal(confirmed, err)
	}
	view := DeviceActivationPublicView(confirmed)
	if _, ok := view["request_secret_hash"]; ok {
		t.Fatal("secret hash exposed")
	}
	if _, ok := view["user_code_hash"]; ok {
		t.Fatal("user code hash exposed")
	}
	if _, ok := view["device_public_key_jwk"]; ok {
		t.Fatal("bootstrap public key exposed to tenant UI")
	}
	proof := "credential\n" + id + "\nnonce-1\n" + digestString(secret)
	proofSig, _ := ecdsa.SignASN1(rand.Reader, private, sha256Bytes(proof))
	encoded := base64.RawURLEncoding.EncodeToString(proofSig)
	if _, err = svc.IssueDeviceActivationCredential(id, secret, "nonce-1", encoded); err == nil {
		t.Fatal("credential issued without CA issuer")
	}
	svc.SetDeviceCredentialIssuer(activationIssuer{})
	credential, err := svc.IssueDeviceActivationCredential(id, secret, "nonce-1", encoded)
	if err != nil || credential.CredentialID != "credential-1" {
		t.Fatal(credential, err)
	}
	issued, _ := repo.ActivationRequest(id)
	if issued.State != "CREDENTIAL_ISSUED" {
		t.Fatal(issued.State)
	}
	commitProof := "activate\n" + id + "\ncredential-1\n1\ncommit-nonce"
	commitSig, _ := ecdsa.SignASN1(rand.Reader, private, sha256Bytes(commitProof))
	active, err := svc.ActivateDeviceCredential(id, "credential-1", "commit-nonce", base64.RawURLEncoding.EncodeToString(commitSig))
	if err != nil || active.State != "ACTIVE" {
		t.Fatal(active, err)
	}
	device, err := repo.Resource("device", deviceID)
	if err != nil || device.TenantID != "tenant-1" || stringField(device.Data, "status") != "ACTIVE" {
		t.Fatal(device, err)
	}
	bound, _ := repo.Resource("register", "register-1")
	if stringField(bound.Data, "fiscal_device_id") != deviceID || stringField(bound.Data, "payment_terminal_id") != deviceID {
		t.Fatal("register roles not atomically bound", bound)
	}
	revoked, err := svc.DisconnectSmartDevice(deviceID, "tenant-1", "admin-1")
	if err != nil || revoked["state"] != "REVOKED" || revoked["binding_version"] != int64(2) {
		t.Fatal("device disconnect failed", revoked, err)
	}
	device, _ = repo.Resource("device", deviceID)
	bound, _ = repo.Resource("register", "register-1")
	if stringField(device.Data, "status") != "REVOKED" || stringField(bound.Data, "fiscal_device_id") != "" || stringField(bound.Data, "payment_terminal_id") != "" {
		t.Fatal("device bindings were not revoked atomically", device, bound)
	}
	if again, againErr := svc.DisconnectSmartDevice(deviceID, "tenant-1", "admin-1"); againErr != nil || again["state"] != "REVOKED" {
		t.Fatal("disconnect is not idempotent", again, againErr)
	}
}

func TestActivationChallengeAndIdentityAreOneTime(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	svc.SetBLESigningKey("01234567890123456789012345678901")
	id := "22222222-2222-4222-8222-222222222222"
	challenge, _ := svc.NewDeviceActivationChallenge(id)
	private, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	enc := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.FillBytes(make([]byte, 32))) }
	jwk := map[string]any{"kty": "EC", "crv": "P-256", "x": enc(private.X), "y": enc(private.Y)}
	input := CreateDeviceActivationInput{ChallengeID: challenge["challenge_id"].(string), Challenge: challenge["challenge"].(string), DeviceInstanceID: id, PublicKeyJWK: jwk, Vendor: "DATECS", Model: "BLUECASH_50", Serial: "S1", FMIN: "12345678", CapabilityDigest: "digest", RequestedRoles: []string{"FISCAL_DEVICE"}}
	canonical, _, _, _ := activationPublicKey(jwk)
	sig, _ := ecdsa.SignASN1(rand.Reader, private, sha256Bytes(activationProof(input, canonical)))
	input.Signature = base64.RawURLEncoding.EncodeToString(sig)
	if _, err := svc.CreateDeviceActivation(input); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateDeviceActivation(input); err == nil {
		t.Fatal("consumed challenge accepted twice")
	}
}
