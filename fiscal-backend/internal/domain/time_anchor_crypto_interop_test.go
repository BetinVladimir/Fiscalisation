package domain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fiscalisation/fiscal-backend/internal/provisioningseal"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Mandatory native runner sets the binary; generic Go tests do not claim C++ coverage.
func TestTimeAnchorActualCryptoInterop(t *testing.T) {
	binary := os.Getenv("TIME_ANCHOR_CRYPTO_HOST_BINARY")
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
	run := func(mode string) {
		t.Helper()
		out, err := exec.Command(binary, dir, mode).CombinedOutput()
		t.Log(string(out))
		if err != nil {
			t.Fatalf("actual time crypto %s: %v", mode, err)
		}
	}
	run("request")
	type envelope struct {
		Algorithm string `json:"algorithm"`
		Canonical string `json:"canonical_json"`
		KeyID     string `json:"key_id"`
		Signature string `json:"signature"`
	}
	responses := map[string]envelope{}
	for _, name := range []string{"a", "b", "c", "d"} {
		raw, err := os.ReadFile(filepath.Join(dir, "request-"+name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var request envelope
		if json.Unmarshal(raw, &request) != nil {
			t.Fatal("request JSON")
		}
		var body map[string]json.RawMessage
		if json.Unmarshal([]byte(request.Canonical), &body) != nil || len(body) != 17 {
			t.Fatal("request fields")
		}
		spki, _ := x509.MarshalPKIXPublicKey(&device.PublicKey)
		pin := sha256.Sum256(spki)
		for field, want := range map[string]string{"kind": "DEVICE_TIME_ANCHOR_REQUEST", "purpose": "trusted-device-time", "scope": "device.time.anchor", "signer_key_id": rootID, "device_id": bleFixtureDeviceID, "device_kid": request.KeyID, "device_spki_sha256": hex.EncodeToString(pin[:]), "tenant_id": tenant, "location_id": location, "register_id": register, "binding_sha256": bindingID} {
			var got string
			if json.Unmarshal(body[field], &got) != nil || got != want {
				t.Fatalf("request %s mismatch", field)
			}
		}
		for field, want := range map[string]int64{"version": 1, "policy_version": 1, "binding_generation": 7} {
			var got int64
			if json.Unmarshal(body[field], &got) != nil || got != want {
				t.Fatalf("request integer %s mismatch", field)
			}
		}
		var boot, nonce string
		_ = json.Unmarshal(body["boot_nonce"], &boot)
		_ = json.Unmarshal(body["request_nonce"], &nonce)
		for _, v := range []string{boot, nonce} {
			decoded, err := base64.RawURLEncoding.DecodeString(v)
			if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != v || bytes.Equal(decoded, make([]byte, 32)) {
				t.Fatal("request nonce invalid")
			}
		}
		if boot == nonce {
			t.Fatal("boot=request nonce")
		}
		var normalized any
		decoder := json.NewDecoder(bytes.NewBufferString(request.Canonical))
		decoder.UseNumber()
		if decoder.Decode(&normalized) != nil {
			t.Fatal("request decode")
		}
		exact, _ := edgeCanonical(normalized)
		if string(exact) != request.Canonical {
			t.Fatal("request canonical")
		}
		sig, err := base64.RawURLEncoding.DecodeString(request.Signature)
		if err != nil || len(sig) != 64 || request.Algorithm != "ES256_P1363" || request.KeyID != oldDevice.Data["transaction_signing_kid"] {
			t.Fatal("request signature format")
		}
		digest := sha256.Sum256([]byte("BEEFISCAL_DEVICE_TIME_ANCHOR_REQUEST_V1\n" + request.Canonical))
		r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
		half := new(big.Int).Rsh(new(big.Int).Set(device.Curve.Params().N), 1)
		if s.Cmp(half) > 0 || !ecdsa.Verify(&device.PublicKey, digest[:], r, s) {
			t.Fatal("actual request singlehash lowS proof rejected")
		}
		response := map[string]any{}
		decoder = json.NewDecoder(bytes.NewBufferString(request.Canonical))
		decoder.UseNumber()
		_ = decoder.Decode(&response)
		response["kind"] = "DEVICE_TIME_ANCHOR"
		response["request_sha256"] = hex.EncodeToString(digest[:])
		response["server_unix_ms"] = int64(1792555200000)
		response["uncertainty_ms"] = int64(1)
		response["max_rtt_ms"] = int64(5000)
		response["max_holdover_ms"] = int64(30000)
		sign := func(body map[string]any, high bool) envelope {
			t.Helper()
			canonical, err := edgeCanonical(body)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(append([]byte("BEEFISCAL_DEVICE_TIME_ANCHOR_V1\n"), canonical...))
			r, s, err := ecdsa.Sign(rand.Reader, root.signer.(*ecdsa.PrivateKey), hash[:])
			if err != nil {
				t.Fatal(err)
			}
			if (s.Cmp(new(big.Int).Rsh(new(big.Int).Set(device.Curve.Params().N), 1)) > 0) != high {
				s.Sub(device.Curve.Params().N, s)
			}
			der, _ := asn1.Marshal(struct{ R, S *big.Int }{r, s})
			return envelope{"ES256_DER", string(canonical), rootID, base64.RawURLEncoding.EncodeToString(der)}
		}
		responses[name] = sign(response, false)
		if name == "b" {
			responses["b-highS"] = sign(response, true)
			for _, field := range []string{"kind", "purpose", "scope", "signer_key_id", "request_sha256", "device_id", "device_kid", "device_spki_sha256", "tenant_id", "location_id", "register_id", "binding_sha256", "boot_nonce", "request_nonce", "binding_generation", "request_sequence", "version", "policy_version"} {
				copy := map[string]any{}
				for k, v := range response {
					copy[k] = v
				}
				if _, ok := response[field].(json.Number); ok {
					copy[field] = json.Number("9223372036854775807")
				} else if field == "version" || field == "policy_version" {
					copy[field] = 2
				} else {
					copy[field] = "foreign"
				}
				responses["bad-"+field] = sign(copy, false)
			}
			copy := map[string]any{}
			for k, v := range response {
				copy[k] = v
			}
			copy["extra"] = 1
			responses["bad-extra"] = sign(copy, false)
			for _, lex := range []string{"1.0", "1e0", "-0", "9223372036854775808"} {
				bad := responses[name]
				bad.Canonical = strings.Replace(bad.Canonical, `"version":1`, `"version":`+lex, 1)
				hash := sha256.Sum256([]byte("BEEFISCAL_DEVICE_TIME_ANCHOR_V1\n" + bad.Canonical))
				der, _ := ecdsa.SignASN1(rand.Reader, root.signer.(*ecdsa.PrivateKey), hash[:])
				bad.Signature = base64.RawURLEncoding.EncodeToString(der)
				responses["bad-integer-"+lex] = bad
			}
		}
	}
	// Parser-only signed golden vectors preserve int64 independently of the
	// installed legacy binding generation cap; they mint no core permit.
	for i, n := range []int64{9007199254740991, 9007199254740992, 9007199254740993, 9223372036854775807} {
		var body map[string]json.RawMessage
		_ = json.Unmarshal([]byte(responses["b"].Canonical), &body)
		body["binding_generation"] = json.RawMessage(jsonNumber(n))
		body["request_sequence"] = json.RawMessage(jsonNumber(n))
		body["server_unix_ms"] = json.RawMessage(jsonNumber(n))
		var value any
		decoder := json.NewDecoder(bytes.NewReader(mustJSON(body)))
		decoder.UseNumber()
		_ = decoder.Decode(&value)
		canonical, _ := edgeCanonical(value)
		hash := sha256.Sum256(append([]byte("BEEFISCAL_DEVICE_TIME_ANCHOR_V1\n"), canonical...))
		sig, _ := ecdsa.SignASN1(rand.Reader, root.signer.(*ecdsa.PrivateKey), hash[:])
		responses[fmt.Sprintf("parser-wide-%d", i)] = envelope{"ES256_DER", string(canonical), rootID, base64.RawURLEncoding.EncodeToString(sig)}
	}
	bad := responses["b"]
	bad.Algorithm = "ES256_P1363"
	responses["bad-algorithm"] = bad
	bad = responses["b"]
	bad.KeyID = "foreign"
	responses["bad-key"] = bad
	bad = responses["b"]
	badBytes, _ := base64.RawURLEncoding.DecodeString(bad.Signature)
	badBytes[len(badBytes)-1] ^= 1
	bad.Signature = base64.RawURLEncoding.EncodeToString(badBytes)
	responses["bad-signature"] = bad
	encoded, _ := json.Marshal(responses)
	write("responses.json", encoded)
	run("verify")
}

func mustJSON(value any) []byte {
	b, e := json.Marshal(value)
	if e != nil {
		panic(e)
	}
	return b
}
