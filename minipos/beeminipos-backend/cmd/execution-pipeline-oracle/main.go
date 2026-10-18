// Disposable opt-in test host: production JWT, persisted source context,
// PostgreSQL encrypted credential and actual HTTP root issuer client.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fiscalisation/beeminipos-backend/internal/api"
	"fiscalisation/beeminipos-backend/internal/config"
	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type fileStore struct {
	mu   sync.Mutex
	path string
}

func (s *fileStore) Load() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := os.ReadFile(s.path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	return b, e
}
func (s *fileStore) Save(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := os.WriteFile(s.path+".next", b, 0600); e != nil {
		return e
	}
	return os.Rename(s.path+".next", s.path)
}
func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: execution-pipeline-oracle PRIVATE_DIR DISPOSABLE_POSTGRES_URL")
	}
	dir, dsn := os.Args[1], os.Args[2]
	const company = "30000000-0000-4000-8000-000000000001"
	const tenant = "10000000-0000-4000-8000-000000000001"
	const system = "20000000-0000-4000-8000-000000000001"
	const binding = "40000000-0000-4000-8000-000000000001"
	const credential = "40000000-0000-4000-8000-000000000002"
	const register = "50000000-0000-4000-8000-000000000001"
	const secret = "test-only-source-signing-secret-32bytes"
	const issuer = "https://pipeline.identity.test"
	const subject = "pipeline-employee"
	const rawCredential = "tenant_live_40000000-0000-4000-8000-000000000002.test-only-secret"
	svc, e := domain.NewPersistentService("http://127.0.0.1:1/public/v1", "2026-08-07", &fileStore{path: filepath.Join(dir, "source.snapshot.json")})
	must(e)
	employee, e := svc.CreateEmployee(domain.Employee{TenantID: company, FirstName: "Pipeline", LastName: "Cashier", OperatorCode: "A001", Roles: []string{"CASHIER"}})
	must(e)
	_, e = svc.BindEmployeeIdentity(company, employee.ID, subject, issuer)
	must(e)
	_, e = svc.CreateProduct(domain.Product{TenantID: company, SKU: "COFFEE", Name: "Coffee", Price: domain.Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B", Status: "ACTIVE"})
	must(e)
	_, e = svc.SaveConfiguration(company, 0, domain.Configuration{LocationName: "Pipeline", WorkstationName: "Browser", LocationID: "60000000-0000-4000-8000-000000000001", FiscalRegisterID: register, FiscalAdapterID: "70000000-0000-4000-8000-000000000001", BindingGeneration: 7, AdapterBaseURL: "http://127.0.0.1:5099/beeloy/local/v1"})
	must(e)
	shift, e := svc.OpenShiftForTenant(register, employee.ID, company)
	must(e)
	write(dir, "source.json", map[string]any{"employee_id": employee.ID, "register_id": register, "issuer": issuer, "subject": subject, "shift_id": shift.ID})
	var root struct {
		URL string `json:"url"`
	}
	deadline := time.Now().Add(time.Minute)
	for {
		b, e := os.ReadFile(filepath.Join(dir, "root-server.json"))
		if e == nil && json.Unmarshal(b, &root) == nil {
			break
		}
		if time.Now().After(deadline) {
			log.Fatal("root setup timeout")
		}
		time.Sleep(50 * time.Millisecond)
	}
	db, e := sql.Open("pgx", dsn)
	must(e)
	defer db.Close()
	_, e = db.Exec(`insert into fiscal.external_systems(id,code,display_name,status,webhook_url,webhook_signing_secret_ciphertext,created_by,updated_by)values($1,'PIPELINE','Pipeline','ACTIVE','https://webhook.test','\x00','test-admin','test-admin')`, system)
	must(e)
	_, e = db.Exec(`insert into fiscal.tenant_source_bindings(id,tenant_id,external_system_id,source_company_id,tax_country,tax_type,tax_normalized_value,status)values($1,$2,$3,$4,'BG','UIC','999999999','ACTIVE')`, binding, tenant, system, company)
	must(e)
	mac := hmac.New(sha256.New, []byte("abcdef0123456789abcdef0123456789"))
	mac.Write([]byte(rawCredential))
	_, e = db.Exec(`insert into fiscal.tenant_integration_credentials(credential_id,binding_id,secret_hash,scopes,status,expires_at)values($1,$2,$3,array['execution_authorizations:issue'],'ACTIVE',now()+interval '1 hour')`, credential, binding, mac.Sum(nil))
	must(e)
	_, e = db.Exec(`insert into minipos.organizations(id,name,fiscal_external_id,status)values($1,'Pipeline',$2,'ACTIVE')`, company, tenant)
	must(e)
	_, e = db.Exec(`insert into minipos.company_fiscal_bindings(id,company_id,external_system_id,source_company_id,fiscal_tenant_id,status)values($1,$2,$3,$4,$5,'ACTIVE')`, binding, company, system, company, tenant)
	must(e)
	aesKey := []byte("0123456789abcdef0123456789abcdef")
	block, e := aes.NewCipher(aesKey)
	must(e)
	aead, e := cipher.NewGCM(block)
	must(e)
	nonce := make([]byte, aead.NonceSize())
	_, e = rand.Read(nonce)
	must(e)
	encrypted := aead.Seal(nonce, nonce, []byte(rawCredential), nil)
	_, e = db.Exec(`insert into minipos.company_fiscal_credentials(binding_id,credential_id,credential_fingerprint,ciphertext,encryption_key_id,status)values($1,$2,'test',$3,'test-key','ACTIVE')`, binding, credential, encrypted)
	must(e)
	client, e := fiscalintegration.New(dsn, root.URL, "explicit-test-system-token", base64.StdEncoding.EncodeToString(aesKey), "test-key")
	must(e)
	defer client.Close()
	key, e := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(e)
	der, e := x509.MarshalECPrivateKey(key)
	must(e)
	claims := map[string]any{"sub": subject, "iss": issuer, "tenant_id": company, "roles": []string{"CASHIER"}, "scope": "fiscal.base", "exp": time.Now().Add(30 * time.Minute).Unix()}
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	body, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(body)
	mac = hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	jwt := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	c := config.Config{AuthHMACKey: secret, APIVersion: "2026-08-07", CORSAllowedOrigins: "*", LocalTokenSigningKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})), LocalTokenIssuer: "https://pipeline.minipos.test", LocalTokenSigningKID: "pipeline-local"}
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	must(e)
	write(dir, "source-server.json", map[string]string{"url": "http://" + listener.Addr().String(), "jwt": jwt})
	log.Fatal(http.Serve(listener, api.NewWithIntegrations(svc, c, nil, client)))
}
func write(dir, name string, v any) {
	b, e := json.Marshal(v)
	must(e)
	must(os.WriteFile(filepath.Join(dir, name), b, 0600))
}
func must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
