package domain

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestDelegatedExecutionRequiresNamespacedAdministratorMapping(t *testing.T) {
	f := newOfflineV2Fixture(t)
	der, _ := x509.MarshalPKIXPublicKey(&f.client.PublicKey)
	in := DelegatedExecutionRequest{SourceOperatorID: "employee-1", SourceRegisterID: "source-register-1", IdentityIssuer: "https://identity.example", IdentitySubject: "subject-1", SourceSessionID: "source-session", AppInstanceID: "app-1", SourceExpiresAt: time.Now().Add(time.Hour), ClientSigningPublicKeyDERBase64: base64.RawURLEncoding.EncodeToString(der)}
	if _, err := f.svc.IssueDelegatedExecutionAuthorization(DelegatedExecutionPrincipal{TenantID: "tenant-1", SystemID: "source-system", CredentialID: "credential", CanIssue: true}, in, "idempotency"); err == nil {
		t.Fatal("issued without explicit namespaced mapping")
	}
}

func delegatedFixture(t *testing.T) (*offlineV2Fixture, DelegatedExecutionPrincipal, DelegatedExecutionRequest) {
	f := newOfflineV2Fixture(t)
	der, _ := x509.MarshalPKIXPublicKey(&f.client.PublicKey)
	p := DelegatedExecutionPrincipal{TenantID: "tenant-1", SystemID: "source-system", CredentialID: "credential", CanIssue: true}
	in := DelegatedExecutionRequest{SourceOperatorID: "employee-1", SourceRegisterID: "source-register-1", IdentityIssuer: "https://identity.example", IdentitySubject: "subject-1", SourceSessionID: "source-session", AppInstanceID: "app-1", SourceExpiresAt: time.Now().Add(time.Hour).Truncate(time.Second), ClientSigningPublicKeyDERBase64: base64.RawURLEncoding.EncodeToString(der)}
	actor, _ := DelegatedActorSubject(p.SystemID, in.IdentityIssuer, in.IdentitySubject)
	for _, kind := range []string{"operator", "register"} {
		id := "operator-1"
		source := in.SourceOperatorID
		if kind == "register" {
			id = "register-1"
			source = in.SourceRegisterID
		}
		r, _ := f.repo.Resource(kind, id)
		r.Data["external_system_id"] = p.SystemID
		r.Data["external_source_id"] = source
		if kind == "operator" {
			r.Data["actor_subject"] = actor
		}
		r.Version++
		if err := f.repo.PutResource(r); err != nil {
			t.Fatal(err)
		}
	}
	return f, p, in
}
func TestDelegatedExecutionScopeReplayAndNamespace(t *testing.T) {
	f, p, in := delegatedFixture(t)
	for _, alter := range []func(*DelegatedExecutionPrincipal, *DelegatedExecutionRequest){
		func(p *DelegatedExecutionPrincipal, in *DelegatedExecutionRequest) { p.CanIssue = false },
		func(p *DelegatedExecutionPrincipal, in *DelegatedExecutionRequest) { p.TenantID = "foreign" },
		func(p *DelegatedExecutionPrincipal, in *DelegatedExecutionRequest) { p.SystemID = "foreign" },
		func(p *DelegatedExecutionPrincipal, in *DelegatedExecutionRequest) {
			in.IdentityIssuer = "https://foreign.example"
		},
		func(p *DelegatedExecutionPrincipal, in *DelegatedExecutionRequest) {
			in.SourceExpiresAt = time.Now().Add(-time.Second)
		},
	} {
		cp, ci := p, in
		alter(&cp, &ci)
		if _, err := f.svc.IssueDelegatedExecutionAuthorization(cp, ci, "same"); err == nil {
			t.Fatal("issued for invalid scope")
		}
	}
	before := len(f.repo.Resources("execution_authorization", p.TenantID))
	first, err := f.svc.IssueDelegatedExecutionAuthorization(p, in, "same")
	if err != nil {
		t.Fatal(err)
	}
	retry, err := f.svc.IssueDelegatedExecutionAuthorization(p, in, "same")
	if err != nil || first.Authorization != retry.Authorization {
		t.Fatal("exact replay changed", err)
	}
	if len(f.repo.Resources("execution_authorization", p.TenantID)) != before+1 {
		t.Fatal("duplicate authorization")
	}
	in.ClientSigningPublicKeyDERBase64 = "changed"
	if _, err = f.svc.IssueDelegatedExecutionAuthorization(p, in, "same"); err == nil {
		t.Fatal("changed request replayed")
	}
	if first.WorkstationSession.OperatorID != "operator-1" || first.WorkstationSession.ActorSubject == "subject-1" || first.WorkstationSession.ExpiresAt.After(in.SourceExpiresAt) {
		t.Fatal("untrusted actor or expiry")
	}
}
func TestDelegatedExecutionIssuanceFailureLeavesNoPendingClaim(t *testing.T) {
	f, p, in := delegatedFixture(t)
	key := in.ClientSigningPublicKeyDERBase64
	in.ClientSigningPublicKeyDERBase64 = "bad"
	if _, err := f.svc.IssueDelegatedExecutionAuthorization(p, in, "recoverable"); err == nil {
		t.Fatal("invalid key accepted")
	}
	in.ClientSigningPublicKeyDERBase64 = key
	if _, err := f.svc.IssueDelegatedExecutionAuthorization(p, in, "recoverable"); err != nil {
		t.Fatal("failed issuance poisoned replay", err)
	}
}

type delegatedFaultStore struct {
	*testStore
	fail bool
}

func (s *delegatedFaultStore) Save(body []byte) error {
	if s.fail {
		return errors.New("injected delegated commit failure")
	}
	return s.testStore.Save(body)
}
func TestDelegatedExecutionAtomicFailureRestartAndConcurrentReplay(t *testing.T) {
	f, p, in := delegatedFixture(t)
	key := in.ClientSigningPublicKeyDERBase64
	in.ClientSigningPublicKeyDERBase64 = "bad"
	if _, err := f.svc.IssueDelegatedExecutionAuthorization(p, in, "prime-session"); err == nil {
		t.Fatal("bad key")
	}
	in.ClientSigningPublicKeyDERBase64 = key
	fault := &delegatedFaultStore{testStore: f.store, fail: true}
	f.repo.store = fault
	before := len(f.repo.Resources("execution_authorization", p.TenantID))
	if _, err := f.svc.IssueDelegatedExecutionAuthorization(p, in, "atomic"); err == nil {
		t.Fatal("save failure ignored")
	}
	if len(f.repo.Resources("execution_authorization", p.TenantID)) != before {
		t.Fatal("failed authorization leaked")
	}
	restarted, err := NewPersistentRepository(f.store)
	if err != nil {
		t.Fatal(err)
	}
	if len(restarted.Resources("execution_authorization", p.TenantID)) != before {
		t.Fatal("partial authorization durable")
	}
	fault.fail = false
	const count = 8
	var wg sync.WaitGroup
	results := make(chan DelegatedExecutionResponse, count)
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := f.svc.IssueDelegatedExecutionAuthorization(p, in, "atomic")
			results <- out
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first ExecutionAuthorizationEnvelope
	for out := range results {
		if first.Payload == "" {
			first = out.Authorization
		}
		if out.Authorization != first {
			t.Fatal("concurrent replay minted different response")
		}
	}
	if len(f.repo.Resources("execution_authorization", p.TenantID)) != before+1 {
		t.Fatal("concurrent issuance duplicates")
	}
}
