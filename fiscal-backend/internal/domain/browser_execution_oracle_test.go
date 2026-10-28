package domain

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"
)

// The browser owns the client private key. This oracle issues only root authority;
// it cannot manufacture the client's PoP and exports no client private material.
func TestBrowserExecutionAuthorizationFixture(t *testing.T) {
	output := os.Getenv("BROWSER_AUTHORITY_FIXTURE")
	public := os.Getenv("BROWSER_CLIENT_SPKI")
	if output == "" || public == "" {
		t.Skip("browser interoperability oracle")
	}
	f := newOfflineV2Fixture(t)
	serverSale := ""
	if os.Getenv("BROWSER_AUTHORITY_ONLINE") == "1" {
		f = onlineFixture(t)
		serverSale = "online-sale"
		if path := os.Getenv("BROWSER_AUTHORITY_SOURCE_PLAN"); path != "" {
			var plan struct {
				Items    []SaleLine       `json:"items"`
				Payments []PaymentRequest `json:"payments"`
			}
			body, e := os.ReadFile(path)
			if e != nil || json.Unmarshal(body, &plan) != nil {
				t.Fatal("source plan", e)
			}
			sale, _ := f.repo.Sale(serverSale)
			sale.Lines = plan.Items
			sale.Payments = nil
			for _, p := range plan.Payments {
				sale.Payments = append(sale.Payments, PaymentRecord{PaymentID: p.PaymentID, Type: p.Type, Amount: p.Amount})
			}
			sale.Version++
			if e = f.repo.PutSale(sale); e != nil {
				t.Fatal(e)
			}
		}
	}
	if app := os.Getenv("BROWSER_APP_INSTANCE"); app != "" {
		session, _ := f.repo.Resource("workstation_session", "session-1")
		session.Data["app_instance_id"] = app
		session.Version++
		if err := f.repo.PutResource(session); err != nil {
			t.Fatal(err)
		}
	}
	envelope, err := f.svc.IssueExecutionAuthorization("session-1", public, "tenant-1", "subject-1", serverSale)
	if err != nil {
		t.Fatal(err)
	}
	var claims ExecutionAuthorization
	raw, _ := base64.RawURLEncoding.DecodeString(envelope.Payload)
	if json.Unmarshal(raw, &claims) != nil {
		t.Fatal("claims")
	}
	if serverSale != "" {
		sale, _ := f.repo.Sale(serverSale)
		payload, e := BuildFinalizePayload(sale, claims.OperatorID, claims.OperatorCode)
		if e != nil {
			t.Fatal(e)
		}
		f.command["canonical_payload"] = payload
		f.command["payload_sha256"] = edgeDigest(payload)
	}
	f.command["execution_authorization"] = asMap(envelope)
	delete(f.command, "client_proof")
	trust, _ := x509.MarshalPKIXPublicKey(f.issuer.ca.PublicKey)
	fixture := map[string]any{"binding": f.repo.resources[resourceKey("issued_binding", f.auth.BindingSHA256)].Data["canonical_payload"], "authorization": envelope, "command": f.command, "trust_key_der_base64": base64.RawURLEncoding.EncodeToString(trust), "now": claims.NBF, "acceptance_event": f.accepted}
	wire, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, wire, 0600); err != nil {
		t.Fatal(err)
	}
}
