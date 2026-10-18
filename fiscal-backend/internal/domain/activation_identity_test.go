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

type countedActivationIssuer struct{ calls int }

func (i *countedActivationIssuer) Issue(v DeviceActivationRequest) (DeviceCredential, error) {
	i.calls++
	return activationIssuer{}.Issue(v)
}
func TestS3ActivationRejectsUnprovenFiscalIdentityBeforeIssuer(t *testing.T) {
	for _, name := range []string{"board-serial-only", "foreign-tenant", "wrong-fm", "unknown-embedded", "foreign-factory-key", "missing-factory-assignment", "route-changed"} {
		t.Run(name, func(t *testing.T) {
			repo := NewMemoryRepository()
			svc := NewService(repo, NewSimulator(true))
			issuer := &countedActivationIssuer{}
			svc.SetDeviceCredentialIssuer(issuer)
			private, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			enc := func(n *big.Int) string { return base64.RawURLEncoding.EncodeToString(n.FillBytes(make([]byte, 32))) }
			jwk := map[string]any{"kty": "EC", "crv": "P-256", "x": enc(private.X), "y": enc(private.Y)}
			canonical, _, _, _ := activationPublicKey(jwk)
			v := DeviceActivationRequest{ID: "activation", DeviceInstanceID: "adapter", PublicKeyJWK: canonical, SecretHash: digestString("secret"), State: "CONFIRMED", ExpiresAt: time.Now().Add(time.Hour), Model: "EDGE_AGENT_S3", Serial: "BOARD001", FMIN: "12345678", ClaimedTenantID: "tenant", ClaimedLocationID: "location", ClaimedRegisterID: "register"}
			fiscal := ResourceRecord{Kind: "device", ID: "physical", TenantID: "tenant", Version: 1, Data: map[string]any{"kind": "FISCAL_DEVICE", "status": "ACTIVE", "vendor": "DATECS", "model": "DP-150 MX", "fiscal_device_number": "DT000001", "fiscal_memory_number": "12345678"}}
			register := ResourceRecord{Kind: "register", ID: "register", TenantID: "tenant", Version: 1, Data: map[string]any{"location_id": "location", "fiscal_device_id": "physical"}}
			factory := ResourceRecord{Kind: "platform_device", ID: "factory", TenantID: platformDeviceTenant, Version: 1, Data: map[string]any{"serial": "BOARD001", "tenant_id": "tenant", "state": "ASSIGNED", "device_public_key_jwk": canonical}}
			switch name {
			case "board-serial-only":
				delete(fiscal.Data, "fiscal_device_number")
				fiscal.Data["serial"] = "BOARD001"
			case "foreign-tenant":
				fiscal.TenantID = "other"
			case "wrong-fm":
				v.FMIN = "87654321"
			case "unknown-embedded":
				v.Model = "BLUECASH_50"
			case "foreign-factory-key":
				factory.Data["device_public_key_jwk"] = "{}"
			case "missing-factory-assignment":
				factory.Data["tenant_id"] = "other"
			case "route-changed":
				register.Data["fiscal_device_id"] = "adapter"
			}
			for _, r := range []ResourceRecord{fiscal, register, factory} {
				if e := repo.PutResource(r); e != nil {
					t.Fatal(e)
				}
			}
			repo.activationRequests[v.ID] = v
			signature, _ := signP1363(private, sha256Bytes("credential\nactivation\nnonce\n"+digestString("secret")))
			_, e := svc.IssueDeviceActivationCredential(v.ID, "secret", "nonce", base64.RawURLEncoding.EncodeToString(signature))
			if e == nil || !strings.Contains(e.Error(), "FISCAL_IDENTITY_PROOF_REQUIRED") || issuer.calls != 0 {
				t.Fatalf("unproven issuance e=%v issuer=%d", e, issuer.calls)
			}
			if len(repo.unp) != 0 {
				t.Fatal("unproven identity consumed/granted authority")
			}
		})
	}
}
