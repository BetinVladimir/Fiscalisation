package domain

// This opt-in integration setup exports only freshly generated TEST authority
// into a private temporary directory; no production credentials are consumed.
import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestExecutionHTTPPipelineFixture(t *testing.T) {
	dir := os.Getenv("EXECUTION_PIPELINE_DIR")
	if dir == "" {
		t.Skip("opt-in HTTP pipeline setup")
	}
	var source struct {
		Employee string `json:"employee_id"`
		Register string `json:"register_id"`
		Issuer   string `json:"issuer"`
		Subject  string `json:"subject"`
	}
	raw, e := os.ReadFile(filepath.Join(dir, "source.json"))
	if e != nil || json.Unmarshal(raw, &source) != nil {
		t.Fatal(e)
	}
	const tenant = "10000000-0000-4000-8000-000000000001"
	const system = "20000000-0000-4000-8000-000000000001"
	f := newOfflineV2Fixture(t)
	snapshot := bytes.ReplaceAll(f.store.b, []byte(`"tenant-1"`), []byte(`"`+tenant+`"`))
	store := &testStore{b: snapshot}
	repo, e := NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	// Tenant changes the immutable issued binding hash; recalculate before publishing setup.
	old := f.auth.BindingSHA256
	issued := repo.resources[resourceKey("issued_binding", old)]
	binding := issued.Data["canonical_payload"].(map[string]any)
	hash := edgeDigest(binding)
	snapshot = bytes.ReplaceAll(snapshot, []byte(old), []byte(hash))
	store = &testStore{b: snapshot}
	repo, e = NewPersistentRepository(store)
	if e != nil {
		t.Fatal(e)
	}
	svc := NewService(repo, NewSimulator(true))
	svc.SetDeviceCredentialIssuer(f.issuer)
	svc.SetBLESigningKey("test-only-readiness-secret-32bytes")
	actor, _ := DelegatedActorSubject(system, source.Issuer, source.Subject)
	for _, kind := range []string{"operator", "register"} {
		id, external := "operator-1", source.Employee
		if kind == "register" {
			id, external = "register-1", source.Register
		}
		r, _ := repo.Resource(kind, id)
		r.Data["external_system_id"] = system
		r.Data["external_source_id"] = external
		if kind == "operator" {
			r.Data["actor_subject"] = actor
		}
		r.Version++
		if e = repo.PutResource(r); e != nil {
			t.Fatal(e)
		}
	}
	if _, _, e = repo.ReserveFiscalUNPRange(tenant, "AB123456", "12345678", 1000); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.SyncWorkstationClock("register-1", tenant); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.RefreshReadiness("register-1", tenant); e != nil {
		t.Fatal(e)
	}
	key, e := x509.MarshalPKCS8PrivateKey(f.issuer.signer)
	if e != nil {
		t.Fatal(e)
	}
	trust, _ := x509.MarshalPKIXPublicKey(f.issuer.ca.PublicKey)
	for name, body := range map[string][]byte{"root.snapshot.json": store.b, "root-ca.pem": []byte(f.issuer.caPEM), "root-key.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), "trust.der": trust} {
		if e = os.WriteFile(filepath.Join(dir, name), body, 0600); e != nil {
			t.Fatal(e)
		}
	}
	issued, _ = repo.Resource("issued_binding", hash)
	setup, _ := json.Marshal(map[string]any{"binding": issued.Data["canonical_payload"], "binding_sha256": hash, "tenant": tenant, "system": system})
	if e = os.WriteFile(filepath.Join(dir, "root-setup.json"), setup, 0600); e != nil {
		t.Fatal(e)
	}
}
