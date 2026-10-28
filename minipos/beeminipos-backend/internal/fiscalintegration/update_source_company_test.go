package fiscalintegration

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// ucDriver provides a DB fake for UpdateSourceCompanyID tests.
// It serves both the credential query and the binding SELECT FOR UPDATE.
type ucDriver struct {
	// binding SELECT FOR UPDATE returns a row only when version matches storedVersion.
	storedVersion int64
	commitErr     error
	fiscalCalls   int
	repairQueued  bool
}

func (d *ucDriver) Open(string) (driver.Conn, error) { return &ucConn{d: d}, nil }

type ucConnector struct{ d *ucDriver }

func (c ucConnector) Connect(context.Context) (driver.Conn, error) { return &ucConn{d: c.d}, nil }
func (c ucConnector) Driver() driver.Driver                        { return c.d }

type ucTx struct{ d *ucDriver }

func (t *ucTx) Commit() error {
	if t.d.commitErr != nil {
		return t.d.commitErr
	}
	return nil
}
func (t *ucTx) Rollback() error { return nil }

type ucConn struct{ d *ucDriver }

func (c *ucConn) Prepare(string) (driver.Stmt, error)  { return nil, errors.New("no prepare") }
func (c *ucConn) Close() error                          { return nil }
func (c *ucConn) Begin() (driver.Tx, error)             { return &ucTx{d: c.d}, nil }

func (c *ucConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	lower := strings.ToLower(query)
	// fiscalReadCredential: 3-column query (ciphertext, fiscal_tenant_id, encryption_key_id)
	if strings.Contains(lower, "fiscal_tenant_id") {
		return &ucCredRows{}, nil
	}
	// tenantCredential: 2-column query (ciphertext, binding_id)
	if strings.Contains(lower, "company_fiscal_credentials") {
		return &ucTenantCredRows{}, nil
	}
	// Binding version lock
	if strings.Contains(lower, "company_fiscal_bindings") && strings.Contains(lower, "for update") {
		if len(args) >= 2 {
			expectedVersion := args[1].Value.(int64)
			if expectedVersion == c.d.storedVersion {
				return &ucBindRow{}, nil
			}
		}
		return &ucEmptyRows{}, nil
	}
	return &ucEmptyRows{}, nil
}

func (c *ucConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	lower := strings.ToLower(query)
	if strings.Contains(lower, "binding_repair_queue") {
		c.d.repairQueued = true
	}
	return driver.RowsAffected(1), nil
}

// ucCredRows serves a fake (ciphertext, fiscal_tenant_id, encryption_key_id) credential row.
type ucCredRows struct{ done bool }

func (r *ucCredRows) Columns() []string {
	return []string{"ciphertext", "fiscal_tenant_id", "encryption_key_id"}
}
func (r *ucCredRows) Close() error { return nil }
func (r *ucCredRows) Next(v []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	block, _ := aes.NewCipher(make([]byte, 32))
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	ct := aead.Seal(nonce, nonce, []byte("tenant_live_tok.sig"), nil)
	v[0] = ct
	v[1] = "fiscal-tenant-uc"
	v[2] = "" // single-key mode
	return nil
}

// ucTenantCredRows serves a fake (ciphertext, binding_id) row for tenantCredential (2-col).
type ucTenantCredRows struct{ done bool }

func (r *ucTenantCredRows) Columns() []string { return []string{"ciphertext", "b"} }
func (r *ucTenantCredRows) Close() error      { return nil }
func (r *ucTenantCredRows) Next(v []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	block, _ := aes.NewCipher(make([]byte, 32))
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	ct := aead.Seal(nonce, nonce, []byte("tenant_live_tok.sig"), nil)
	v[0] = ct
	v[1] = "binding-uc-001"
	return nil
}

type ucBindRow struct{ done bool }

func (r *ucBindRow) Columns() []string { return []string{"id", "b"} }
func (r *ucBindRow) Close() error      { return nil }
func (r *ucBindRow) Next(v []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	v[0] = "binding-001"
	v[1] = []byte(`{}`)
	return nil
}

type ucEmptyRows struct{}

func (r *ucEmptyRows) Columns() []string                  { return nil }
func (r *ucEmptyRows) Close() error                       { return nil }
func (r *ucEmptyRows) Next([]driver.Value) error          { return io.EOF }

func ucClient(t *testing.T, d *ucDriver, fiscalStatus int) *Client {
	t.Helper()
	block, _ := aes.NewCipher(make([]byte, 32))
	aead, _ := cipher.NewGCM(block)
	db := sql.OpenDB(ucConnector{d})
	t.Cleanup(func() { db.Close() })
	rt := denialRoundTrip(func(*http.Request) (*http.Response, error) {
		d.fiscalCalls++
		if fiscalStatus == 0 {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: fiscalStatus, Body: io.NopCloser(strings.NewReader(`{"error":"denied"}`)), Header: make(http.Header)}, nil
	})
	return &Client{
		db:   db,
		base: "https://fiscal.example",
		aead: aead,
		http: &http.Client{Transport: rt},
	}
}

func TestUpdateSourceCompanyIDVersionConflictDoesNotCallFiscal(t *testing.T) {
	d := &ucDriver{storedVersion: 3}
	c := ucClient(t, d, 200)
	// expected=2, stored=3 → version conflict → must NOT call Fiscal
	err := c.UpdateSourceCompanyID(context.Background(), "company-x", "next-id", 2, "idem-1", "SERVICE", "actor", "", "reason")
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if d.fiscalCalls != 0 {
		t.Fatalf("Fiscal called %d times before local lock confirmed version", d.fiscalCalls)
	}
}

func TestUpdateSourceCompanyIDCommitFailureQueuesRepair(t *testing.T) {
	d := &ucDriver{storedVersion: 2, commitErr: errors.New("pg: commit timeout")}
	c := ucClient(t, d, 200)
	_ = c.UpdateSourceCompanyID(context.Background(), "company-x", "next-id", 2, "idem-1", "SERVICE", "actor", "", "reason")
	if !d.repairQueued {
		t.Fatal("expected repair queue entry on commit failure")
	}
}
