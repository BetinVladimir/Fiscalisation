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
	"math/big"
	"strings"
	"testing"
)

func deliveryFixture(t *testing.T) (*Service, ProvisioningDeliveryInput, *ecdsa.PrivateKey) {
	t.Helper()
	repo := NewMemoryRepository()
	svc := NewService(repo, NewSimulator(true))
	ca, key := testDeviceCA(t)
	issuer, e := NewX509DeviceCredentialIssuer(ca, key, "ssl://mqtt.test:8883", "", "01234567890123456789012345678901")
	if e != nil {
		t.Fatal(e)
	}
	svc.SetDeviceCredentialIssuer(issuer)
	device, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	enc := func(v *big.Int) string { return base64.RawURLEncoding.EncodeToString(v.FillBytes(make([]byte, 32))) }
	jwk := `{"crv":"P-256","kty":"EC","x":"` + enc(device.X) + `","y":"` + enc(device.Y) + `"}`
	der, _ := x509.MarshalPKIXPublicKey(&device.PublicKey)
	pub := base64.RawURLEncoding.EncodeToString(der)
	digest := sha256.Sum256([]byte(pub))
	kid := "esp32-p256-" + base64.RawURLEncoding.EncodeToString(digest[:12])
	payload := map[string]any{"schema_version": 3, "generation": 7, "tenant_id": "tenant", "location_id": "location", "register_id": "register", "edge_device_id": "device", "profile": "DATECS_DP150_BLUEPAD50", "operational_authority": map[string]any{"transaction_signing_kid": kid}, "fiscal_endpoint": map[string]any{"device_id": "fiscal"}}
	canonical, _ := json.Marshal(payload)
	h := sha256.Sum256(canonical)
	hash := hex.EncodeToString(h[:])
	sig, keyid, e := issuer.SignCompositeBinding(canonical)
	if e != nil {
		t.Fatal(e)
	}
	for _, record := range []ResourceRecord{
		{Kind: "device", ID: "device", TenantID: "tenant", Version: 1, Data: map[string]any{"status": "ACTIVE", "serial": "S3-1", "credential_id": "existing-credential", "transaction_signing_public_key": pub, "transaction_signing_kid": kid}},
		{Kind: "platform_device", ID: "factory", TenantID: platformDeviceTenant, Version: 1, Data: map[string]any{"state": "ASSIGNED", "tenant_id": "tenant", "serial": "S3-1", "device_public_key_jwk": jwk}},
		{Kind: "register", ID: "register", TenantID: "tenant", Version: 1, Data: map[string]any{"location_id": "location"}},
		{Kind: "issued_binding", ID: hash, TenantID: "tenant", Version: 1, Data: map[string]any{"canonical_json": string(canonical), "payload_sha256": hash, "key_id": keyid, "signature": sig}},
		{Kind: "composite_binding", ID: "binding", TenantID: "tenant", Version: 1, Data: map[string]any{"status": "PENDING", "adapter_device_id": "device", "fiscal_device_id": "fiscal", "register_id": "register", "profile": "DATECS_DP150_BLUEPAD50", "generation": 7, "expected_register_version": 1, "payload_sha256": hash}},
	} {
		if e = repo.PutResource(record); e != nil {
			t.Fatal(e)
		}
	}
	return svc, ProvisioningDeliveryInput{BindingID: "binding", SetupNonce: base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{3}, 32)), SSID: "test-network", WifiPassword: "private-wifi-password", FiscalPassword: "12345678"}, device
}
func TestProvisioningDeliveryUsesEnrolledKeyAndOpaqueResponse(t *testing.T) {
	s, in, _ := deliveryFixture(t)
	v, e := s.PrepareProvisioningDelivery("device", "tenant", in)
	if e != nil {
		t.Fatal(e)
	}
	packet, e := base64.StdEncoding.DecodeString(v["package_base64"].(string))
	if e != nil || !bytes.HasPrefix(packet, []byte("BFP2")) {
		t.Fatal("not encrypted package")
	}
	encoded, _ := json.Marshal(v)
	for _, secret := range []string{in.WifiPassword, in.FiscalPassword, "BEGIN CERTIFICATE", "command_hmac_key", "existing-credential"} {
		if bytes.Contains(encoded, []byte(secret)) || bytes.Contains(packet, []byte(secret)) {
			t.Fatal("plaintext secret leaked", secret)
		}
	}
	records := s.repo.Resources("provisioning_delivery", "tenant")
	if len(records) != 1 || records[0].Data["state"] != "ISSUED" {
		t.Fatal("missing audit subject")
	}
}
func TestProvisioningDeliveryRejectsTrustAndAssociationConflicts(t *testing.T) {
	for _, name := range []string{"foreign tenant", "unassigned registry", "foreign registry key", "changed device key", "wrong binding device", "revoked binding", "wrong register location", "malformed nonce", "short password", "wrong fiscal password"} {
		t.Run(name, func(t *testing.T) {
			s, in, _ := deliveryFixture(t)
			tenant := "tenant"
			change := func(kind, id string, f func(map[string]any)) {
				r, _ := s.repo.Resource(kind, id)
				f(r.Data)
				r.Version++
				if e := s.repo.PutResource(r); e != nil {
					t.Fatal(e)
				}
			}
			switch name {
			case "foreign tenant":
				tenant = "other"
			case "unassigned registry":
				change("platform_device", "factory", func(v map[string]any) { v["state"] = "SUSPENDED" })
			case "foreign registry key":
				change("platform_device", "factory", func(v map[string]any) { v["device_public_key_jwk"] = `{"kty":"EC"}` })
			case "changed device key":
				change("device", "device", func(v map[string]any) { v["transaction_signing_public_key"] = "foreign" })
			case "wrong binding device":
				change("composite_binding", "binding", func(v map[string]any) { v["adapter_device_id"] = "other" })
			case "revoked binding":
				change("composite_binding", "binding", func(v map[string]any) { v["status"] = "REVOKED" })
			case "wrong register location":
				change("register", "register", func(v map[string]any) { v["location_id"] = "foreign" })
			case "malformed nonce":
				in.SetupNonce = "bad"
			case "short password":
				in.WifiPassword = "bad"
			case "wrong fiscal password":
				in.FiscalPassword = "secret"
			}
			if _, e := s.PrepareProvisioningDelivery("device", tenant, in); e == nil {
				t.Fatal("invalid delivery accepted")
			}
			if len(s.repo.Resources("provisioning_delivery", "tenant")) != 0 {
				t.Fatal("invalid issued")
			}
		})
	}
}

func TestProvisioningInstallationRequiresDeviceProofBeforeRouteApply(t *testing.T) {
	s, in, key := deliveryFixture(t)
	issued, e := s.PrepareProvisioningDelivery("device", "tenant", in)
	if e != nil {
		t.Fatal(e)
	}
	device, _ := s.repo.Resource("device", "device")
	p := ProvisioningInstallProof{State: "INSTALLED", DeviceKID: stringField(device.Data, "transaction_signing_kid"), BindingSHA256: issued["binding_sha256"].(string), PackageSHA256: issued["package_sha256"].(string), Generation: 7}
	proof := "beefiscal-provisioning-installed-v1\n" + p.DeviceKID + "\n" + p.BindingSHA256 + "\n7\n" + p.PackageSHA256
	digest := sha256.Sum256([]byte(proof))
	r, v, e := ecdsa.Sign(rand.Reader, key, digest[:])
	if e != nil {
		t.Fatal(e)
	}
	half := new(big.Int).Rsh(new(big.Int).Set(key.Params().N), 1)
	if v.Cmp(half) > 0 {
		v.Sub(key.Params().N, v)
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	v.FillBytes(sig[32:])
	p.Signature = base64.RawURLEncoding.EncodeToString(sig)
	for _, alter := range []func(*ProvisioningInstallProof){func(p *ProvisioningInstallProof) { p.Signature = "mobile-says-installed" }, func(p *ProvisioningInstallProof) { p.PackageSHA256 = strings.Repeat("0", 64) }, func(p *ProvisioningInstallProof) { p.Generation++ }, func(p *ProvisioningInstallProof) { p.DeviceKID = "foreign" }} {
		wrong := p
		alter(&wrong)
		if _, e = s.ConfirmProvisioningInstallation("device", issued["delivery_id"].(string), "tenant", wrong); e == nil {
			t.Fatal("invalid proof accepted")
		}
		binding, _ := s.repo.Resource("composite_binding", "binding")
		if binding.Data["status"] != "PENDING" {
			t.Fatal("route activated before proof")
		}
	}
	for i := 0; i < 2; i++ {
		result, e := s.ConfirmProvisioningInstallation("device", issued["delivery_id"].(string), "tenant", p)
		if e != nil || result["state"] != "INSTALLED" {
			t.Fatal("verified proof or exact retry rejected", result, e)
		}
	}
	register, _ := s.repo.Resource("register", "register")
	if register.Data["composite_binding_id"] != "binding" || register.Version != 2 {
		t.Fatal("route missing or duplicate apply")
	}
}
