package domain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fiscalisation/fiscal-backend/internal/provisioningseal"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Mandatory native runner sets the binary; generic Go tests do not claim C++ coverage.
func TestBLECertificateActualFirmwareProofInterop(t *testing.T) {
	binary := os.Getenv("BLE_CERTIFICATE_PROOF_HOST_BINARY")
	if binary == "" {
		t.Skip("explicit actual firmware runner not requested")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("absolute native binary required")
	}
	dir := t.TempDir()
	svc, _, _, device := bleLifecycleFixture(t, "versioned")
	repo := svc.repo.(*MemoryRepository)
	root := svc.deviceCredentialIssuer.(*X509DeviceCredentialIssuer)
	raw, e := os.ReadFile("../../../IoT/firmware/edge-agent-s3/idf/tests/money_host/datecs.json")
	if e != nil {
		t.Fatal(e)
	}
	var payload map[string]any
	if e = json.Unmarshal(raw, &payload); e != nil {
		t.Fatal(e)
	}
	payload["edge_device_id"] = bleFixtureDeviceID
	payload["operational_authority"].(map[string]any)["transaction_signing_kid"] = repo.resources[resourceKey("device", bleFixtureDeviceID)].Data["transaction_signing_kid"]
	tenant := payload["tenant_id"].(string)
	location := payload["location_id"].(string)
	register := payload["register_id"].(string)
	canonical, _ := edgeCanonical(payload)
	bindingHash := sha256.Sum256(canonical)
	bindingID := hex.EncodeToString(bindingHash[:])
	signature, rootID, e := root.SignCompositeBinding(canonical)
	if e != nil {
		t.Fatal(e)
	}
	oldDevice := repo.resources[resourceKey("device", bleFixtureDeviceID)]
	oldDevice.TenantID = tenant
	oldFactory := repo.resources[resourceKey("platform_device", "factory")]
	oldFactory.Data["tenant_id"] = tenant
	repo.mu.Lock()
	repo.resources = map[string]ResourceRecord{}
	for _, record := range []ResourceRecord{oldDevice, oldFactory,
		{Kind: "location", ID: location, TenantID: tenant, Version: 1, Data: map[string]any{"status": "ACTIVE"}},
		{Kind: "register", ID: register, TenantID: tenant, Version: 1, Data: map[string]any{"status": "ACTIVE", "location_id": location, "composite_binding_id": "binding", "adapter_device_id": bleFixtureDeviceID}},
		{Kind: "issued_binding", ID: bindingID, TenantID: tenant, Version: 1, Data: map[string]any{"canonical_json": string(canonical), "payload_sha256": bindingID, "key_id": rootID, "signature": signature}},
		{Kind: "composite_binding", ID: "binding", TenantID: tenant, Version: 1, Data: map[string]any{"status": "ACTIVE", "register_id": register, "adapter_device_id": bleFixtureDeviceID, "generation": 7, "payload_sha256": bindingID}},
	} {
		repo.resources[resourceKey(record.Kind, record.ID)] = record
	}
	activation := repo.activationRequests["activation"]
	activation.ClaimedTenantID = tenant
	activation.ClaimedLocationID = location
	activation.ClaimedRegisterID = register
	repo.activationRequests[activation.ID] = activation
	e = repo.persistLocked()
	repo.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	write := func(name string, data []byte) {
		t.Helper()
		if e := os.WriteFile(filepath.Join(dir, name), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	rootDER, _ := x509.MarshalPKIXPublicKey(root.signer.Public())
	write("issuer.pem", pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: rootDER}))
	write("issuer.kid", []byte(rootID))
	write("device.scalar", device.D.FillBytes(make([]byte, 32)))
	write("binding.json", canonical)
	write("binding.signature", []byte(signature))
	makePacket := func(value map[string]any, generation int64) ([]byte, []byte, string) {
		t.Helper()
		canonical, _ := edgeCanonical(value)
		digest := sha256.Sum256(canonical)
		sig, kid, e := root.SignCompositeBinding(canonical)
		if e != nil {
			t.Fatal(e)
		}
		der, _ := x509.MarshalPKIXPublicKey(&device.PublicKey)
		req := provisioningseal.Request{EnrolledPublicKeyDER: der, DeviceKID: oldDevice.Data["transaction_signing_kid"].(string), IssuerKID: kid, Generation: uint64(generation), BindingSHA256: digest}
		copy(req.SetupNonce[:], bytes.Repeat([]byte{3}, 32))
		req.Plaintext, _ = json.Marshal(map[string]any{"schema_version": 1, "binding": map[string]any{"canonical_json": string(canonical), "signature": sig, "key_id": kid}, "wifi": map[string]string{"ssid": "test-network", "password": "test-password"}, "fiscal_operator_password": "12345678"})
		packet, e := root.SealRuntimeProvisioning(req, bleFixtureDeviceID, "existing-credential", activation.PublicKeyJWK)
		if e != nil {
			t.Fatal(e)
		}
		return packet, canonical, sig
	}
	packet, _, _ := makePacket(payload, 7)
	write("installer.bin", packet)
	payload["generation"] = 8
	nextPacket, nextBinding, nextSignature := makePacket(payload, 8)
	write("installer-next.bin", nextPacket)
	write("binding-next.json", nextBinding)
	write("binding-next.signature", []byte(nextSignature))
	payload["generation"] = 7
	envelopes := make([]BLEServerCertChallengeEnvelope, 6)
	for i := range envelopes {
		envelopes[i], e = svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", tenant)
		if e != nil {
			t.Fatal(e)
		}
		encoded, _ := json.Marshal(envelopes[i])
		write("challenge-"+string(rune('0'+i))+".json", encoded)
	}
	var challenge BLEServerCertChallenge
	if e = decodeBLECanonical(envelopes[0].CanonicalJSON, &challenge); e != nil {
		t.Fatal(e)
	}
	write("now", []byte(strings.TrimSpace(jsonNumber(challenge.IssuedAt))))
	// Sign hostile canonical/raw bytes with the real synthetic root, so native
	// rejection cannot be explained merely by a broken signature.
	vectors := map[string]BLEServerCertChallengeEnvelope{}
	sign := func(raw string) BLEServerCertChallengeEnvelope {
		h := sha256.Sum256([]byte(bleCertChallengeDomain + raw))
		sig, e := ecdsa.SignASN1(rand.Reader, root.signer.(*ecdsa.PrivateKey), h[:])
		if e != nil {
			t.Fatal(e)
		}
		return BLEServerCertChallengeEnvelope{CanonicalJSON: raw, Signature: base64.RawURLEncoding.EncodeToString(sig), KeyID: rootID}
	}
	for _, name := range []string{"purpose", "device", "tenant", "location", "register", "binding", "generation", "SPKI", "future", "expired", "unknown", "duplicate", "fractional", "exponent", "overflow", "null", "noncanonical", "nonce", "UUID", "name", "oversized"} {
		raw := envelopes[0].CanonicalJSON
		var c BLEServerCertChallenge
		_ = decodeBLECanonical(raw, &c)
		switch name {
		case "purpose":
			c.Purpose = "mqtt-client"
		case "device":
			c.DeviceID = "22222222-2222-4222-8222-222222222222"
		case "tenant":
			c.TenantID = "other"
		case "location":
			c.LocationID = "other"
		case "register":
			c.RegisterID = "other"
		case "binding":
			c.BindingSHA256 = strings.Repeat("0", 64)
		case "generation":
			c.BindingGeneration = 8
		case "SPKI":
			c.DeviceSPKI = strings.Repeat("0", 64)
		case "future":
			c.IssuedAt += 121
			c.ExpiresAt += 121
			c.CertificateNotAfter += 121
		case "expired":
			c.IssuedAt -= 120
			c.ExpiresAt -= 120
			c.CertificateNotAfter -= 120
		case "nonce":
			c.Nonce = "AA"
		case "UUID":
			c.ChallengeID = "not-a-uuid"
		case "name":
			c.ServerName = "arbitrary.example"
		}
		encoded, _ := edgeCanonical(c)
		raw = string(encoded)
		switch name {
		case "unknown":
			raw = strings.TrimSuffix(raw, "}") + `,"other":1}`
		case "duplicate":
			raw = strings.Replace(raw, `"version":1`, `"version":1,"version":1`, 1)
		case "fractional":
			raw = strings.Replace(raw, `"binding_generation":7`, `"binding_generation":7.0`, 1)
		case "exponent":
			raw = strings.Replace(raw, `"binding_generation":7`, `"binding_generation":7e0`, 1)
		case "overflow":
			raw = strings.Replace(raw, `"binding_generation":7`, `"binding_generation":9223372036854775808`, 1)
		case "null":
			raw = strings.Replace(raw, `"binding_generation":7`, `"binding_generation":null`, 1)
		case "noncanonical":
			raw = " " + raw
		case "oversized":
			raw = strings.Repeat(" ", 8193) + raw
		}
		vectors[name] = sign(raw)
	}
	foreign := envelopes[0]
	foreign.KeyID = "foreign-root"
	vectors["key-id"] = foreign
	foreign = envelopes[0]
	other, _ := ecdsa.GenerateKey(device.Curve, rand.Reader)
	h := sha256.Sum256([]byte(bleCertChallengeDomain + foreign.CanonicalJSON))
	sig, _ := ecdsa.SignASN1(rand.Reader, other, h[:])
	foreign.Signature = base64.RawURLEncoding.EncodeToString(sig)
	vectors["wrong-root"] = foreign
	foreign = envelopes[0]
	foreign.Signature = strings.Repeat("A", 1024*1024)
	vectors["huge-signature"] = foreign
	foreign = envelopes[0]
	foreign.Signature += "="
	vectors["padded-signature"] = foreign
	for {
		foreign = sign(envelopes[0].CanonicalJSON)
		if len(foreign.Signature)%4 != 0 {
			break
		}
	}
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, foreign.Signature[len(foreign.Signature)-1])
	foreign.Signature = foreign.Signature[:len(foreign.Signature)-1] + string(alphabet[last+1])
	vectors["signature-padbits"] = foreign
	encoded, _ := json.Marshal(vectors)
	write("negative-vectors.json", encoded)
	command := exec.Command(binary, dir)
	output, e := command.CombinedOutput()
	t.Log(string(output))
	if e != nil {
		t.Fatal("actual firmware proof fixture failed", e)
	}
	var proof struct {
		CanonicalJSON string `json:"canonical_json"`
		Signature     string `json:"signature"`
	}
	raw, e = os.ReadFile(filepath.Join(dir, "firmware-proof.json"))
	if e != nil || json.Unmarshal(raw, &proof) != nil {
		t.Fatal("native proof missing", e)
	}
	issued, e := svc.IssueBLEServerCertificate(challenge.ChallengeID, tenant, proof.CanonicalJSON, proof.Signature)
	if e != nil || issued.SPKISHA256 != challenge.DeviceSPKI {
		t.Fatal("actual C++ proof incompatible with Go issuance", e)
	}
	again, e := svc.IssueBLEServerCertificate(challenge.ChallengeID, tenant, proof.CanonicalJSON, proof.Signature)
	if e != nil || again != issued {
		t.Fatal("native proof replay rotated certificate", e)
	}
}
func jsonNumber(n int64) string { b, _ := json.Marshal(n); return string(b) }
