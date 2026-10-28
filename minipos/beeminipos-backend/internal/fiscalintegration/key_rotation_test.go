package fiscalintegration

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
)

func newAEAD(t *testing.T, key [32]byte) cipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(key[:])
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return aead
}

func sealToken(aead cipher.AEAD, token string) []byte {
	nonce := make([]byte, aead.NonceSize())
	return aead.Seal(nonce, nonce, []byte(token), nil)
}

// krDriver serves a credential row encrypted with a specific key ID.
type krDriver struct {
	keyID      string
	ciphertext []byte
	rewrapped  []byte // non-nil after re-wrap
	rewrapKeyID string
}

func (d *krDriver) Open(string) (driver.Conn, error) { return &krConn{d: d}, nil }

type krConnector struct{ d *krDriver }

func (c krConnector) Connect(context.Context) (driver.Conn, error) { return &krConn{d: c.d}, nil }
func (c krConnector) Driver() driver.Driver                        { return c.d }

type krConn struct{ d *krDriver }

func (c *krConn) Prepare(string) (driver.Stmt, error)  { return nil, errors.New("no prepare") }
func (c *krConn) Close() error                         { return nil }
func (c *krConn) Begin() (driver.Tx, error)            { return &ucTx{d: nil}, nil }

func (c *krConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	lower := strings.ToLower(query)
	if strings.Contains(lower, "company_fiscal_credentials") || strings.Contains(lower, "ciphertext") {
		return &krCredRows{d: c.d}, nil
	}
	return &ucEmptyRows{}, nil
}

func (c *krConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	lower := strings.ToLower(query)
	if strings.Contains(lower, "encryption_key_id") && strings.Contains(lower, "ciphertext") {
		// re-wrap update: capture new ciphertext and key ID
		for _, a := range args {
			if b, ok := a.Value.([]byte); ok {
				c.d.rewrapped = b
			}
			if s, ok := a.Value.(string); ok && len(s) < 50 {
				c.d.rewrapKeyID = s
			}
		}
	}
	return driver.RowsAffected(1), nil
}

type krCredRows struct {
	d    *krDriver
	done bool
}

func (r *krCredRows) Columns() []string { return []string{"ciphertext", "fiscal_tenant_id", "encryption_key_id"} }
func (r *krCredRows) Close() error      { return nil }
func (r *krCredRows) Next(v []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	v[0] = r.d.ciphertext
	v[1] = "tenant-x"
	v[2] = r.d.keyID
	return nil
}

func buildKeyring(keys map[string][32]byte) map[string]cipher.AEAD {
	m := make(map[string]cipher.AEAD, len(keys))
	for id, key := range keys {
		block, _ := aes.NewCipher(key[:])
		aead, _ := cipher.NewGCM(block)
		m[id] = aead
	}
	return m
}

func TestDecryptUsesStoredKeyID(t *testing.T) {
	var k1, k2 [32]byte
	k1[0] = 1
	k2[0] = 2
	aead1 := newAEAD(t, k1)
	ct := sealToken(aead1, "tenant_live_token.abc")

	d := &krDriver{keyID: "k1", ciphertext: ct}
	db := sql.OpenDB(krConnector{d})
	defer db.Close()
	aead2 := newAEAD(t, k2)
	c := &Client{
		db:          db,
		aead:        aead2, // active key is k2
		keyID:       "k2",
		keyring:     buildKeyring(map[string][32]byte{"k1": k1, "k2": k2}),
		activeKeyID: "k2",
	}
	token, _, err := c.fiscalReadCredential(context.Background(), "company-a")
	if err != nil {
		t.Fatal(err)
	}
	if token != "tenant_live_token.abc" {
		t.Fatalf("expected original token, got %q", token)
	}
}

func TestUnknownKeyIDFailsClosed(t *testing.T) {
	var k1 [32]byte
	k1[0] = 1
	aead1 := newAEAD(t, k1)
	ct := sealToken(aead1, "tenant_live_token.abc")

	d := &krDriver{keyID: "unknown-key", ciphertext: ct}
	db := sql.OpenDB(krConnector{d})
	defer db.Close()
	c := &Client{
		db:          db,
		aead:        aead1,
		keyID:       "k1",
		keyring:     buildKeyring(map[string][32]byte{"k1": k1}),
		activeKeyID: "k1",
	}
	_, _, err := c.fiscalReadCredential(context.Background(), "company-a")
	if err == nil {
		t.Fatal("expected error for unknown key ID")
	}
}

func TestRewrapOnRead(t *testing.T) {
	var k1, k2 [32]byte
	k1[0] = 1
	k2[0] = 2
	aead1 := newAEAD(t, k1)
	ct := sealToken(aead1, "tenant_live_token.abc")

	d := &krDriver{keyID: "k1", ciphertext: ct}
	db := sql.OpenDB(krConnector{d})
	defer db.Close()
	aead2 := newAEAD(t, k2)
	c := &Client{
		db:          db,
		aead:        aead2,
		keyID:       "k2",
		keyring:     buildKeyring(map[string][32]byte{"k1": k1, "k2": k2}),
		activeKeyID: "k2",
	}
	_, _, err := c.fiscalReadCredential(context.Background(), "company-a")
	if err != nil {
		t.Fatal(err)
	}
	if d.rewrapKeyID != "k2" {
		t.Fatalf("expected rewrap with k2, got %q", d.rewrapKeyID)
	}
	// Verify the rewrapped ciphertext decrypts with k2.
	if d.rewrapped == nil {
		t.Fatal("no re-wrap ciphertext captured")
	}
	nonce := d.rewrapped[:aead2.NonceSize()]
	plain, err := aead2.Open(nil, nonce, d.rewrapped[aead2.NonceSize():], nil)
	if err != nil || string(plain) != "tenant_live_token.abc" {
		t.Fatalf("re-wrapped ciphertext not decryptable: %v", err)
	}
}
