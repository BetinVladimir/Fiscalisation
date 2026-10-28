package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const bleFixtureDeviceID = "11111111-1111-4111-8111-111111111111"

func bleLifecycleFixture(t *testing.T, kind string) (*Service, *repositoryFaultStore, Store, *ecdsa.PrivateKey) {
	t.Helper()
	base := &repositoryFaultStore{}
	var store Store = base
	if kind != "plain" {
		v := &repositoryVersionedFaultStore{base}
		store = v
		if kind == "delta" {
			store = &repositoryDeltaFaultStore{v}
		}
	}
	repo, e := NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	svc := NewService(repo, NewSimulator(true))
	mqtt, mqttKey := testDeviceCA(t)
	root, e := NewX509DeviceCredentialIssuer(mqtt, mqttKey, "ssl://mqtt.test:8883", "", "01234567890123456789012345678901")
	if e != nil {
		t.Fatal(e)
	}
	svc.SetDeviceCredentialIssuer(root)
	ca, key, _, _ := bleProfileCA(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	issuer, e := NewBLEServerCertificateIssuer(ca, key, mqtt)
	if e != nil {
		t.Fatal(e)
	}
	svc.SetBLEServerCertificateIssuer(issuer)
	private, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if e != nil {
		t.Fatal(e)
	}
	enc := func(n *big.Int) string { return base64.RawURLEncoding.EncodeToString(n.FillBytes(make([]byte, 32))) }
	jwk, _ := json.Marshal(map[string]string{"kty": "EC", "crv": "P-256", "x": enc(private.X), "y": enc(private.Y)})
	der, _ := x509.MarshalPKIXPublicKey(&private.PublicKey)
	encoded := base64.RawURLEncoding.EncodeToString(der)
	digest := sha256.Sum256([]byte(encoded))
	kid := "esp32-p256-" + base64.RawURLEncoding.EncodeToString(digest[:12])
	payload := map[string]any{"schema_version": 3, "generation": 7, "tenant_id": "tenant", "location_id": "location", "register_id": "register", "edge_device_id": bleFixtureDeviceID, "operational_authority": map[string]any{"transaction_signing_kid": kid}}
	raw, _ := edgeCanonical(payload)
	hash := sha256.Sum256(raw)
	pin := hex.EncodeToString(hash[:])
	sig, keyID, e := root.SignCompositeBinding(raw)
	if e != nil {
		t.Fatal(e)
	}
	records := []ResourceRecord{
		{Kind: "location", ID: "location", TenantID: "tenant", Version: 1, Data: map[string]any{"status": "ACTIVE"}},
		{Kind: "device", ID: bleFixtureDeviceID, TenantID: "tenant", Version: 1, Data: map[string]any{"status": "ACTIVE", "model": "EDGE_AGENT_S3", "serial": "S3-1", "credential_id": "existing-credential", "transaction_signing_public_key": encoded, "transaction_signing_kid": kid}},
		{Kind: "platform_device", ID: "factory", TenantID: platformDeviceTenant, Version: 1, Data: map[string]any{"state": "ASSIGNED", "tenant_id": "tenant", "serial": "S3-1", "device_public_key_jwk": string(jwk)}},
		{Kind: "register", ID: "register", TenantID: "tenant", Version: 1, Data: map[string]any{"status": "ACTIVE", "location_id": "location", "composite_binding_id": "binding", "adapter_device_id": bleFixtureDeviceID}},
		{Kind: "issued_binding", ID: pin, TenantID: "tenant", Version: 1, Data: map[string]any{"canonical_json": string(raw), "payload_sha256": pin, "signature": sig, "key_id": keyID}},
		{Kind: "composite_binding", ID: "binding", TenantID: "tenant", Version: 1, Data: map[string]any{"status": "ACTIVE", "adapter_device_id": bleFixtureDeviceID, "register_id": "register", "generation": 7, "payload_sha256": pin}},
	}
	for _, r := range records {
		if e = repo.PutResource(r); e != nil {
			t.Fatal(e)
		}
	}
	repo.mu.Lock()
	repo.activationRequests["activation"] = DeviceActivationRequest{ID: "activation", State: "ACTIVE", Model: "EDGE_AGENT_S3", DeviceInstanceID: bleFixtureDeviceID, PublicKeyJWK: string(jwk), Serial: "S3-1", CredentialID: "existing-credential", ClaimedTenantID: "tenant", ClaimedLocationID: "location", ClaimedRegisterID: "register", BindingVersion: 1}
	e = repo.persistLocked()
	repo.mu.Unlock()
	if e != nil {
		t.Fatal(e)
	}
	base.calls = 0
	return svc, base, store, private
}
func bleProof(t *testing.T, envelope BLEServerCertChallengeEnvelope, key *ecdsa.PrivateKey) (BLEServerCertChallenge, string, string) {
	t.Helper()
	var c BLEServerCertChallenge
	if e := decodeBLECanonical(envelope.CanonicalJSON, &c); e != nil {
		t.Fatal(e)
	}
	digest := sha256.Sum256([]byte(bleCertChallengeDomain + envelope.CanonicalJSON))
	raw, e := edgeCanonical(BLEServerCertProof{Kind: "BLE_SERVER_CERT_PROOF", Version: 1, Purpose: bleServerCertificatePurpose, ChallengeID: c.ChallengeID, ChallengeSHA256: hex.EncodeToString(digest[:]), DeviceSPKI: c.DeviceSPKI})
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256([]byte(bleCertProofDomain + string(raw)))
	r, s, e := ecdsa.Sign(rand.Reader, key, h[:])
	if e != nil {
		t.Fatal(e)
	}
	signature := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	return c, string(raw), base64.RawURLEncoding.EncodeToString(signature)
}
func TestBLEServerLifecycleFreshProofPersistsExactReplay(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		t.Run(kind, func(t *testing.T) {
			svc, _, store, key := bleLifecycleFixture(t, kind)
			envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
			if e != nil {
				t.Fatal(e)
			}
			c, proof, sig := bleProof(t, envelope, key)
			root := svc.deviceCredentialIssuer.(*X509DeviceCredentialIssuer)
			rootSig, e := base64.RawURLEncoding.DecodeString(envelope.Signature)
			h := sha256.Sum256([]byte(bleCertChallengeDomain + envelope.CanonicalJSON))
			if e != nil || !ecdsa.VerifyASN1(root.ca.PublicKey.(*ecdsa.PublicKey), h[:], rootSig) {
				t.Fatal("not purpose-bound root signed")
			}
			first, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
			if e != nil {
				t.Fatal(e)
			}
			again, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
			if e != nil || again != first {
				t.Fatal("replay rotated certificate", e)
			}
			restarted, e := NewPersistentRepository(store)
			if e != nil {
				t.Fatal(e)
			}
			svc.repo = restarted
			again, e = svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
			if e != nil || again != first {
				t.Fatal("restart lost exact certificate", e)
			}
			block, _ := pem.Decode([]byte(first.CertificatePEM))
			leaf, e := x509.ParseCertificate(block.Bytes)
			if e != nil || !leaf.PublicKey.(*ecdsa.PublicKey).Equal(&key.PublicKey) || leaf.DNSNames[0] != c.ServerName || !leaf.NotAfter.Equal(time.Unix(c.CertificateNotAfter, 0)) {
				t.Fatal("certificate not exact enrolled challenge", e)
			}
		})
	}
}
func TestBLEServerLifecycleRejectsBadProofAndAuthority(t *testing.T) {
	for _, name := range []string{"foreign tenant", "wrong proof key", "legacy double hash", "DER proof", "wrong purpose", "wrong challenge", "wrong SPKI", "noncanonical", "duplicate", "unknown", "expired", "future", "revoked activation", "revoked binding", "changed generation", "changed enrollment", "foreign register", "foreign devicekey"} {
		t.Run(name, func(t *testing.T) {
			svc, _, _, key := bleLifecycleFixture(t, "plain")
			envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
			if e != nil {
				t.Fatal(e)
			}
			c, proof, sig := bleProof(t, envelope, key)
			tenant := "tenant"
			repo := svc.repo.(*MemoryRepository)
			mutate := func(kind, id, field string, value any) {
				v, e := repo.Resource(kind, id)
				if e != nil {
					t.Fatal(e)
				}
				v.Version++
				v.Data[field] = value
				if e = repo.PutResource(v); e != nil {
					t.Fatal(e)
				}
			}
			switch name {
			case "foreign tenant":
				tenant = "other"
			case "wrong proof key":
				other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				_, _, sig = bleProof(t, envelope, other)
			case "legacy double hash":
				h := sha256.Sum256([]byte(bleCertProofDomain + proof))
				h = sha256.Sum256(h[:])
				r, s, _ := ecdsa.Sign(rand.Reader, key, h[:])
				sig = base64.RawURLEncoding.EncodeToString(append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...))
			case "DER proof":
				h := sha256.Sum256([]byte(bleCertProofDomain + proof))
				der, _ := ecdsa.SignASN1(rand.Reader, key, h[:])
				sig = base64.RawURLEncoding.EncodeToString(der)
			case "wrong purpose":
				proof = strings.Replace(proof, bleServerCertificatePurpose, "mqtt-client", 1)
			case "wrong challenge":
				proof = strings.Replace(proof, c.ChallengeID, "22222222-2222-4222-8222-222222222222", 1)
			case "wrong SPKI":
				proof = strings.Replace(proof, c.DeviceSPKI, strings.Repeat("0", 64), 1)
			case "noncanonical":
				proof = " " + proof
			case "duplicate":
				proof = strings.Replace(proof, `"version":1`, `"version":1,"version":1`, 1)
			case "unknown":
				proof = strings.TrimSuffix(proof, "}") + `,"unknown":1}`
			case "expired":
				svc.bleServerIssuer.clock = func() time.Time { return time.Unix(c.ExpiresAt, 0) }
			case "future":
				svc.bleServerIssuer.clock = func() time.Time { return time.Unix(c.IssuedAt-1, 0) }
			case "revoked activation":
				repo.mu.Lock()
				a := repo.activationRequests["activation"]
				a.State = "REVOKED"
				repo.activationRequests[a.ID] = a
				repo.mu.Unlock()
			case "revoked binding":
				mutate("composite_binding", "binding", "status", "REVOKED")
			case "changed generation":
				mutate("composite_binding", "binding", "generation", 8)
			case "changed enrollment":
				mutate("platform_device", "factory", "state", "SUSPENDED")
			case "foreign register":
				mutate("register", "register", "location_id", "other")
			case "foreign devicekey":
				mutate("device", bleFixtureDeviceID, "transaction_signing_public_key", "foreign")
			}
			certificate, e := svc.IssueBLEServerCertificate(c.ChallengeID, tenant, proof, sig)
			if e == nil || certificate.CertificatePEM != "" {
				t.Fatal("bad request issued", e)
			}
			if len(repo.Resources("ble_server_certificate", "tenant")) != 0 {
				t.Fatal("rejected proof consumed challenge")
			}
		})
	}
}
func TestBLEServerLifecycleAtomicPersistenceAmbiguity(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		for _, committed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/commit=%v", kind, committed), func(t *testing.T) {
				svc, store, backing, key := bleLifecycleFixture(t, kind)
				envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
				if e != nil {
					t.Fatal(e)
				}
				c, proof, sig := bleProof(t, envelope, key)
				store.failAt = store.calls + 1
				store.commitThenError = committed
				store.readOutage = true
				certificate, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
				if e == nil || certificate.CertificatePEM != "" {
					t.Fatal("uncertain cert returned", e)
				}
				before := store.calls
				certificate, e = svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
				if e == nil || certificate.CertificatePEM != "" || store.calls != before {
					t.Fatal("retry bypassed uncertainty", e)
				}
				store.readOutage = false
				_ = svc.PersistenceReady()
				if e = svc.PersistenceReady(); e != nil {
					t.Fatal(e)
				}
				certificate, e = svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
				if e != nil {
					t.Fatal("healthy retry", e)
				}
				if committed && store.calls != before {
					t.Fatal("committed retry reissued")
				}
				if !committed && store.calls != before+1 {
					t.Fatal("definite failure not retried")
				}
				repo, e := NewPersistentRepository(backing)
				if e != nil {
					t.Fatal(e)
				}
				svc.repo = repo
				again, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
				if e != nil || again != certificate {
					t.Fatal("restart certificate changed", e)
				}
			})
		}
	}
}

func TestBLEServerLifecycleRechecksAuthorityAndDeadlineAfterCA(t *testing.T) {
	for _, name := range []string{"generation changed during CA", "challenge expires during CA", "duplicate enrollment during CA"} {
		t.Run(name, func(t *testing.T) {
			svc, _, _, key := bleLifecycleFixture(t, "plain")
			envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
			if e != nil {
				t.Fatal(e)
			}
			c, proof, sig := bleProof(t, envelope, key)
			calls := 0
			svc.bleServerIssuer.clock = func() time.Time {
				calls++
				if name == "duplicate enrollment during CA" && calls == 2 {
					enrollment, e := svc.repo.Resource("platform_device", "factory")
					if e != nil {
						t.Fatal(e)
					}
					enrollment.ID = "duplicate-factory"
					if e = svc.repo.PutResource(enrollment); e != nil {
						t.Fatal(e)
					}
				}
				if name == "generation changed during CA" && calls == 2 {
					binding, e := svc.repo.Resource("composite_binding", "binding")
					if e != nil {
						t.Fatal(e)
					}
					binding.Version++
					binding.Data["generation"] = 8
					if e = svc.repo.PutResource(binding); e != nil {
						t.Fatal(e)
					}
				}
				if name == "challenge expires during CA" && calls >= 2 {
					return time.Unix(c.ExpiresAt, 0)
				}
				return time.Unix(c.IssuedAt, 0)
			}
			issued, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
			if e == nil || issued.CertificatePEM != "" {
				t.Fatal("stale authority/deadline issued", e)
			}
			if len(svc.repo.Resources("ble_server_certificate", "tenant")) != 0 {
				t.Fatal("stale cert consumed challenge")
			}
		})
	}
}
func TestBLEServerChallengeCreationRequiresCurrentAuthorityAndConfirmedSave(t *testing.T) {
	for _, kind := range []string{"plain", "versioned", "delta"} {
		for _, committed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/commit=%v", kind, committed), func(t *testing.T) {
				svc, store, _, _ := bleLifecycleFixture(t, kind)
				store.failAt = store.calls + 1
				store.commitThenError = committed
				store.readOutage = true
				envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
				if e == nil || envelope.CanonicalJSON != "" {
					t.Fatal("unconfirmed challenge published", e)
				}
				before := store.calls
				envelope, e = svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
				if e == nil || envelope.CanonicalJSON != "" || store.calls != before {
					t.Fatal("challenge bypassed uncertainty", e)
				}
				store.readOutage = false
				_ = svc.PersistenceReady()
				if e = svc.PersistenceReady(); e != nil {
					t.Fatal(e)
				}
				if _, e = svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant"); e != nil {
					t.Fatal("healthy fresh challenge failed", e)
				}
			})
		}
	}
}
func TestBLEServerCanonicalExactIntegerAndDuplicateBoundaries(t *testing.T) {
	svc, _, _, _ := bleLifecycleFixture(t, "plain")
	envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
	if e != nil {
		t.Fatal(e)
	}
	var challenge BLEServerCertChallenge
	for _, replace := range []string{`"binding_generation":7.0`, `"binding_generation":7e0`, `"binding_generation":9223372036854775808`, `"binding_generation":-1`, `"binding_generation":7,"binding_generation":7`, `"binding_generation":null`, `"binding_generation":NaN`} {
		raw := strings.Replace(envelope.CanonicalJSON, `"binding_generation":7`, replace, 1)
		if decodeBLECanonical(raw, &challenge) == nil && validBLEChallenge(challenge) {
			t.Fatal("noncanonical/overflow integer accepted", raw)
		}
	}
	// This exact canonical int64 value must not round through float64.
	raw := strings.Replace(envelope.CanonicalJSON, `"binding_generation":7`, `"binding_generation":9007199254740993`, 1)
	if e = decodeBLECanonical(raw, &challenge); e != nil || challenge.BindingGeneration != 9007199254740993 || !validBLEChallenge(challenge) {
		t.Fatal("exact integer rounded", challenge.BindingGeneration, e)
	}
	for _, n := range []any{float64(9007199254740992), float64(1.5), json.Number("9223372036854775808"), json.Number("7.0")} {
		if exactBLEGeneration(n) != 0 {
			t.Fatal("ambiguous source generation accepted", n)
		}
	}
}

func TestBLEServerConcurrentProofPublishesOneExactCertificate(t *testing.T) {
	svc, _, _, key := bleLifecycleFixture(t, "versioned")
	envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
	if e != nil {
		t.Fatal(e)
	}
	c, proof, sig := bleProof(t, envelope, key)
	var wg sync.WaitGroup
	results := make(chan BLEServerCertificate, 8)
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
			if e != nil {
				failures <- e
			} else {
				results <- v
			}
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for e := range failures {
		t.Fatal(e)
	}
	var first BLEServerCertificate
	for v := range results {
		if first.CertificatePEM == "" {
			first = v
		} else if first != v {
			t.Fatal("concurrent replay rotated issued certificate")
		}
	}
	if first.CertificatePEM == "" || len(svc.repo.Resources("ble_server_certificate", "tenant")) != 1 {
		t.Fatal("not exactly one consumed challenge result")
	}
}
func TestBLEServerCanonicalContractFixtures(t *testing.T) {
	svc, _, _, key := bleLifecycleFixture(t, "plain")
	envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
	if e != nil {
		t.Fatal(e)
	}
	c, proof, sig := bleProof(t, envelope, key)
	if dir := os.Getenv("BLE_LIFECYCLE_SYNTHETIC_FIXTURE_DIR"); dir != "" {
		if !filepath.IsAbs(dir) {
			t.Fatal("absolute disposable fixture dir required")
		}
		if e = os.MkdirAll(dir, 0700); e != nil {
			t.Fatal(e)
		}
		private, e := x509.MarshalPKCS8PrivateKey(key)
		if e != nil {
			t.Fatal(e)
		}
		public, e := x509.MarshalPKIXPublicKey(&key.PublicKey)
		if e != nil {
			t.Fatal(e)
		}
		context, _ := edgeCanonical(map[string]any{"device_id": c.DeviceID, "device_spki_sha256": c.DeviceSPKI, "tenant_id": c.TenantID, "location_id": c.LocationID, "register_id": c.RegisterID, "binding_sha256": c.BindingSHA256, "binding_generation": c.BindingGeneration, "now": c.IssuedAt, "root_key_id": envelope.KeyID})
		encoded, _ := json.Marshal(envelope)
		for name, data := range map[string][]byte{"challenge-envelope.json": encoded, "challenge.json": []byte(envelope.CanonicalJSON), "proof.json": []byte(proof), "proof-signature": []byte(sig), "trusted-context.json": context, "root-public.pem": []byte(svc.deviceCredentialIssuer.(*X509DeviceCredentialIssuer).caPEM), "device-public.pem": pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: public}), "synthetic-device-key.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})} {
			if e = os.WriteFile(filepath.Join(dir, name), data, 0600); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestBLEServerLifecycleDisabledAndNonDurableFailClosed(t *testing.T) {
	svc, _, _, _ := bleLifecycleFixture(t, "plain")
	issuer := svc.bleServerIssuer
	svc.bleServerIssuer = nil
	if _, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant"); e == nil {
		t.Fatal("default-disabled issuer bypassed")
	}
	svc.bleServerIssuer = issuer
	repo := svc.repo.(*MemoryRepository)
	repo.store = nil
	if _, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant"); e == nil {
		t.Fatal("non-durable authority published")
	}
}

func TestBLEServerCanonicalProofRejectsEvenValidKeySignature(t *testing.T) {
	for _, name := range []string{"space", "duplicate", "unknown", "fractional", "null"} {
		t.Run(name, func(t *testing.T) {
			svc, _, _, key := bleLifecycleFixture(t, "plain")
			envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
			if e != nil {
				t.Fatal(e)
			}
			c, proof, _ := bleProof(t, envelope, key)
			switch name {
			case "space":
				proof = " " + proof
			case "duplicate":
				proof = strings.Replace(proof, `"version":1`, `"version":1,"version":1`, 1)
			case "unknown":
				proof = strings.TrimSuffix(proof, "}") + `,"other":1}`
			case "fractional":
				proof = strings.Replace(proof, `"version":1`, `"version":1.0`, 1)
			case "null":
				proof = strings.Replace(proof, `"version":1`, `"version":null`, 1)
			}
			digest := sha256.Sum256([]byte(bleCertProofDomain + proof))
			r, s, e := ecdsa.Sign(rand.Reader, key, digest[:])
			if e != nil {
				t.Fatal(e)
			}
			sig := base64.RawURLEncoding.EncodeToString(append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...))
			if _, e = svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig); e == nil {
				t.Fatal("valid signature bypassed canonical contract")
			}
		})
	}
}
func TestBLEServerExactReplayLifetimeAndImmutableConflict(t *testing.T) {
	svc, _, _, key := bleLifecycleFixture(t, "plain")
	envelope, e := svc.NewBLEServerCertificateChallenge(bleFixtureDeviceID, "activation", "binding", "tenant")
	if e != nil {
		t.Fatal(e)
	}
	c, proof, sig := bleProof(t, envelope, key)
	first, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
	if e != nil {
		t.Fatal(e)
	}
	svc.bleServerIssuer.clock = func() time.Time { return time.Unix(c.ExpiresAt+1, 0) }
	again, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig)
	if e != nil || again != first {
		t.Fatal("durable exact replay rotated after challenge expiry", e)
	}
	repo := svc.repo.(*MemoryRepository)
	resource, e := repo.Resource("ble_server_certificate", c.ChallengeID)
	if e != nil {
		t.Fatal(e)
	}
	resource.Version++
	resource.Data["proof_sha256"] = strings.Repeat("0", 64)
	if e = repo.PutResource(resource); e == nil {
		t.Fatal("immutable consumption record replaced")
	}
	svc.bleServerIssuer.clock = func() time.Time { return time.Unix(c.CertificateNotAfter, 0) }
	if v, e := svc.IssueBLEServerCertificate(c.ChallengeID, "tenant", proof, sig); e == nil || v.CertificatePEM != "" {
		t.Fatal("expired certificate returned", e)
	}
}
