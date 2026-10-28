package domain

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func delegatedReconciliationFixture(t *testing.T) (*offlineV2Fixture, DelegatedReconciliationPrincipal, DelegatedReconciliationRequest) {
	f := seedReconciliation(t)
	p := DelegatedReconciliationPrincipal{"tenant-1", "source-system", "credential", true}
	actor, _ := DelegatedActorSubject(p.SystemID, "https://identity.example", "admin-subject")
	for _, kind := range []string{"operator", "register"} {
		id, source := "operator-1", "employee-1"
		if kind == "register" {
			id, source = "register-1", "source-register"
		}
		r, _ := f.repo.Resource(kind, id)
		r.Data["external_system_id"] = p.SystemID
		r.Data["external_source_id"] = source
		if kind == "operator" {
			r.Data["actor_subject"] = actor
		}
		r.Version++
		if e := f.repo.PutResource(r); e != nil {
			t.Fatal(e)
		}
	}
	der, _ := x509.MarshalPKIXPublicKey(&f.client.PublicKey)
	session := DelegatedExecutionRequest{SourceOperatorID: "employee-1", SourceRegisterID: "source-register", IdentityIssuer: "https://identity.example", IdentitySubject: "admin-subject", SourceSessionID: "source-session", AppInstanceID: "app-1", SourceExpiresAt: time.Now().Add(90 * time.Second).Truncate(time.Second), ClientSigningPublicKeyDERBase64: base64.RawURLEncoding.EncodeToString(der)}
	if _, e := f.svc.IssueDelegatedExecutionAuthorization(DelegatedExecutionPrincipal{p.TenantID, p.SystemID, p.CredentialID, true}, session, "sale-transport-session"); e != nil {
		t.Fatal(e)
	}
	return f, p, DelegatedReconciliationRequest{session.SourceOperatorID, session.SourceRegisterID, session.IdentityIssuer, session.IdentitySubject, session.SourceSessionID, session.AppInstanceID, session.SourceExpiresAt, reconciliationInput(f)}
}
func bindReconciliationAdmin(t *testing.T, f *offlineV2Fixture, p DelegatedReconciliationPrincipal, in DelegatedReconciliationRequest, enabled bool, version int64) {
	t.Helper()
	if e := f.svc.BindReconciliationAdministrator(ReconciliationAdministratorBinding{"operator-1", p.SystemID, in.SourceOperatorID, in.IdentityIssuer, in.IdentitySubject, enabled, version}, p.TenantID, "root-admin"); e != nil {
		t.Fatal(e)
	}
}
func TestDelegatedReconciliationExplicitAdminCapabilityAndScope(t *testing.T) {
	f, p, in := delegatedReconciliationFixture(t)
	if _, e := f.svc.IssueDelegatedPaymentReconciliation(p, in, "decision"); e == nil {
		t.Fatal("sale identity mapping alone granted ADMIN")
	}
	bindReconciliationAdmin(t, f, p, in, true, 0)
	for _, alter := range []func(*DelegatedReconciliationPrincipal, *DelegatedReconciliationRequest){func(p *DelegatedReconciliationPrincipal, i *DelegatedReconciliationRequest) { p.CanAuthorize = false }, func(p *DelegatedReconciliationPrincipal, i *DelegatedReconciliationRequest) { p.TenantID = "foreign" }, func(p *DelegatedReconciliationPrincipal, i *DelegatedReconciliationRequest) { p.SystemID = "foreign" }, func(p *DelegatedReconciliationPrincipal, i *DelegatedReconciliationRequest) {
		i.IdentityIssuer = "https://foreign.example"
	}, func(p *DelegatedReconciliationPrincipal, i *DelegatedReconciliationRequest) {
		i.AppInstanceID = "other"
	}, func(p *DelegatedReconciliationPrincipal, i *DelegatedReconciliationRequest) {
		i.SourceSessionID = "other"
	}, func(p *DelegatedReconciliationPrincipal, i *DelegatedReconciliationRequest) {
		i.SourceExpiresAt = time.Now().Add(-time.Second)
	}} {
		cp, ci := p, in
		alter(&cp, &ci)
		if _, e := f.svc.IssueDelegatedPaymentReconciliation(cp, ci, "bad"); e == nil {
			t.Fatal("invalid delegation accepted")
		}
	}
	if _, _, _, _, err := f.svc.paymentReconciliationContext(in.Decision.OperationID, in.Decision.PaymentID, p.TenantID); err != nil {
		t.Fatalf("fixture original context broken: %v", err)
	}
	first, e := f.svc.IssueDelegatedPaymentReconciliation(p, in, "decision")
	if e != nil {
		t.Fatal(e)
	}
	if output := os.Getenv("RECONCILIATION_UI_FIXTURE"); output != "" {
		raw, _ := base64.RawURLEncoding.DecodeString(first.Authorization.Payload)
		var grant PaymentReconciliationGrant
		json.Unmarshal(raw, &grant)
		der, _ := x509.MarshalPKIXPublicKey(f.issuer.ca.PublicKey)
		fixture := map[string]any{"command": first, "grant": grant, "decision": in.Decision, "now": grant.NBF, "trust_key_der_base64": base64.RawURLEncoding.EncodeToString(der), "scope": map[string]any{"tenant_id": p.TenantID, "actor_id": grant.ActorID, "register_id": grant.RegisterID, "edge_device_id": grant.EdgeDeviceID, "binding_sha256": grant.BindingSHA256, "binding_generation": grant.BindingGeneration, "source_session_id": in.SourceSessionID, "app_instance_id": in.AppInstanceID}}
		body, _ := json.Marshal(fixture)
		os.MkdirAll(filepath.Dir(output), 0700)
		if e := os.WriteFile(output, body, 0600); e != nil {
			t.Fatal(e)
		}
	}
	again, e := f.svc.IssueDelegatedPaymentReconciliation(p, in, "decision")
	if e != nil || first != again {
		t.Fatal("changed exact replay", e)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(first.Authorization.Payload)
	var g PaymentReconciliationGrant
	json.Unmarshal(raw, &g)
	if g.Exp > in.SourceExpiresAt.Unix() || g.ActorID == in.IdentitySubject || g.RegisterID != "register-1" || !f.issuer.VerifyPaymentReconciliation(first.Authorization) {
		t.Fatal("authority scope/signature", g)
	}
	changed := in
	changed.Decision.EvidenceReference = "changed"
	if _, e = f.svc.IssueDelegatedPaymentReconciliation(p, changed, "decision"); e == nil {
		t.Fatal("changed decision replay")
	}
	bindReconciliationAdmin(t, f, p, in, false, 1)
	if _, e = f.svc.IssueDelegatedPaymentReconciliation(p, in, "decision"); e == nil {
		t.Fatal("revoked admin replayed grant")
	}
}
func TestDelegatedReconciliationAtomicGrantReplayFailure(t *testing.T) {
	f, p, in := delegatedReconciliationFixture(t)
	bindReconciliationAdmin(t, f, p, in, true, 0)
	f.repo.store = &reconciliationFailStore{f.store}
	if _, e := f.svc.IssueDelegatedPaymentReconciliation(p, in, "disk-failure"); e == nil {
		t.Fatal("persist failure accepted")
	}
	if len(f.repo.Resources("payment_reconciliation_authorization", p.TenantID)) != 0 {
		t.Fatal("orphan grant survived")
	}
	f.repo.store = f.store
	if _, e := f.svc.IssueDelegatedPaymentReconciliation(p, in, "disk-failure"); e != nil {
		t.Fatal("failed commit poisoned retry", e)
	}
}

type changingReconciliationSigner struct {
	*X509DeviceCredentialIssuer
	change func()
}

func (s *changingReconciliationSigner) SignPaymentReconciliation(raw []byte) (string, string, error) {
	s.change()
	return s.X509DeviceCredentialIssuer.SignPaymentReconciliation(raw)
}
func TestDelegatedReconciliationRevocationWhileSigningFailsClosed(t *testing.T) {
	f, p, in := delegatedReconciliationFixture(t)
	bindReconciliationAdmin(t, f, p, in, true, 0)
	f.svc.SetDeviceCredentialIssuer(&changingReconciliationSigner{f.issuer, func() { bindReconciliationAdmin(t, f, p, in, false, 1) }})
	if _, e := f.svc.IssueDelegatedPaymentReconciliation(p, in, "race"); e == nil {
		t.Fatal("issued after administrator revoke")
	}
	if len(f.repo.Resources("payment_reconciliation_authorization", p.TenantID)) != 0 {
		t.Fatal("stale authority committed")
	}
}
