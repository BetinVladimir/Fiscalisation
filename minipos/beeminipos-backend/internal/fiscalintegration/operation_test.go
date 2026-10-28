package fiscalintegration

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type recoveryDriver struct {
	encrypted []byte
	company   string
	missing   bool
}

func (d *recoveryDriver) Open(string) (driver.Conn, error) { return recoveryConn{d}, nil }

type recoveryConnector struct{ d *recoveryDriver }

func (c recoveryConnector) Connect(context.Context) (driver.Conn, error) {
	return recoveryConn{c.d}, nil
}
func (c recoveryConnector) Driver() driver.Driver { return c.d }

type recoveryConn struct{ d *recoveryDriver }

func (c recoveryConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (c recoveryConn) Close() error              { return nil }
func (c recoveryConn) Begin() (driver.Tx, error) { return nil, errors.New("unexpected write") }
func (c recoveryConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !strings.Contains(query, "b.company_id=$1") || !strings.Contains(query, "c.status='ACTIVE'") || !strings.Contains(query, "b.status='ACTIVE'") {
		return nil, errors.New("credential query not scoped/active")
	}
	c.d.company = args[0].Value.(string)
	return &recoveryRows{data: c.d.encrypted, missing: c.d.missing}, nil
}

type recoveryRows struct {
	data          []byte
	missing, done bool
}

func (r *recoveryRows) Columns() []string {
	return []string{"ciphertext", "fiscal_tenant_id", "encryption_key_id"}
}
func (r *recoveryRows) Close() error { return nil }
func (r *recoveryRows) Next(v []driver.Value) error {
	if r.done || r.missing {
		return io.EOF
	}
	r.done = true
	v[0] = r.data
	v[1] = "fiscal-tenant-a"
	v[2] = "" // single-key mode: empty keyID falls through to c.decrypt
	return nil
}

type readTransport func(*http.Request) (*http.Response, error)

func (f readTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGetFiscalOperationUsesCompanyCredentialAndValidatesIdentity(t *testing.T) {
	for _, scenario := range []string{"success", "tenant", "uuid", "client", "missing", "cipher", "404", "401", "503", "cancelled"} {
		t.Run(scenario, func(t *testing.T) {
			block, _ := aes.NewCipher(make([]byte, 32))
			aead, _ := cipher.NewGCM(block)
			c := &Client{base: "https://fiscal.test", aead: aead}
			encrypted, _ := c.encrypt("company-secret")
			d := &recoveryDriver{encrypted: encrypted, missing: scenario == "missing"}
			if scenario == "cipher" {
				d.encrypted = []byte("bad")
			}
			db := sql.OpenDB(recoveryConnector{d})
			defer db.Close()
			c.db = db
			calls := 0
			c.http = &http.Client{Transport: readTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method != "GET" || r.URL.Path != "/public/v1/operations/op-one" || r.Header.Get("Authorization") != "Bearer company-secret" || r.Header.Get("X-Api-Version") != "2026-08-07" || r.Header.Get("Idempotency-Key") != "" {
					t.Errorf("unsafe request: %s %s", r.Method, r.URL.Path)
				}
				op := FiscalOperation{OperationID: "op-one", ClientOperationID: "op-one", TenantID: "fiscal-tenant-a"}
				status := 200
				switch scenario {
				case "tenant":
					op.TenantID = "foreign"
				case "uuid":
					op.OperationID = "other"
				case "client":
					op.ClientOperationID = "other"
				case "404":
					status = 404
				case "401":
					status = 401
				case "503":
					status = 503
				}
				body, _ := json.Marshal(op)
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
			})}
			ctx := context.Background()
			if scenario == "cancelled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			_, err := c.GetFiscalOperation(ctx, "company-a", "op-one")
			if scenario == "success" && err != nil {
				t.Fatal(err)
			}
			if scenario != "success" && err == nil {
				t.Fatal("accepted unproven lookup")
			}
			if scenario != "cancelled" && d.company != "company-a" {
				t.Fatal("company not used")
			}
			if (scenario == "missing" || scenario == "cipher" || scenario == "cancelled") && calls != 0 {
				t.Fatal("I/O without active credential")
			}
		})
	}
}

func TestGetFiscalSaleUsesTenantCredentialAndReadOnlyIdentity(t *testing.T) {
	for _, scenario := range []string{"success", "tenant", "id", "notfound", "timeout"} {
		t.Run(scenario, func(t *testing.T) {
			block, _ := aes.NewCipher(make([]byte, 32))
			aead, _ := cipher.NewGCM(block)
			c := &Client{base: "https://fiscal.test", aead: aead}
			encrypted, _ := c.encrypt("company-secret")
			d := &recoveryDriver{encrypted: encrypted}
			db := sql.OpenDB(recoveryConnector{d})
			defer db.Close()
			c.db = db
			c.http = &http.Client{Transport: readTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" || r.URL.Path != "/public/v1/sales/sale-one" || r.Header.Get("Authorization") != "Bearer company-secret" || r.Header.Get("Idempotency-Key") != "" {
					t.Fatal("unsafe request", r.Method, r.URL.Path)
				}
				if scenario == "timeout" {
					return nil, context.DeadlineExceeded
				}
				sale := FiscalSale{SaleID: "sale-one", TenantID: "fiscal-tenant-a"}
				status := 200
				if scenario == "tenant" {
					sale.TenantID = "foreign"
				}
				if scenario == "id" {
					sale.SaleID = "foreign"
				}
				if scenario == "notfound" {
					status = 404
				}
				body, _ := json.Marshal(sale)
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(body))), Header: http.Header{}}, nil
			})}
			_, err := c.GetFiscalSale(context.Background(), "company-a", "sale-one")
			if scenario == "success" && err != nil {
				t.Fatal(err)
			}
			if scenario != "success" && err == nil {
				t.Fatal("unvalidated sale accepted")
			}
			if d.company != "company-a" {
				t.Fatal("wrong credential scope")
			}
		})
	}
}
