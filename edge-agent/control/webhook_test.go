package control

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fiscalisation/edge-agent/journal"
)

func TestRevocationWebhookIsAuthenticatedAndDurableAcrossRestart(t *testing.T) {
	path := t.TempDir() + "/edge.db"
	j, err := journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)
	key := "edge-control-secret-long-enough"
	h := NewHandler(j, key)
	h.now = func() time.Time { return now }
	body := []byte(fmt.Sprintf(`{"event_id":"event-1","event_type":"ble.session.revoked","api_version":"2026-08-07","resource_id":"session-1","data":{"ble_session_id":"session-1","expires_at":%q}}`, now.Add(time.Hour).Format(time.RFC3339)))
	req := httptest.NewRequest(http.MethodPost, "/control/v1/fiscal-webhooks", bytes.NewReader(body))
	req.Header.Set("BeeFiscal-Signature", sign(body, key, now))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent || !j.IsBLESessionRevoked("session-1", now) {
		t.Fatalf("revocation not committed: %d %s", w.Code, w.Body.String())
	}
	_ = j.Close()
	j, err = journal.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if !j.IsBLESessionRevoked("session-1", now) {
		t.Fatal("revocation lost after restart")
	}
}

func TestRevocationWebhookRejectsTamperingAndWrongType(t *testing.T) {
	j, _ := journal.Open(t.TempDir() + "/edge.db")
	defer j.Close()
	h := NewHandler(j, "edge-control-secret-long-enough")
	for _, body := range [][]byte{[]byte(`{}`), []byte(`{"event_id":"e","event_type":"fiscal.operation.succeeded"}`)} {
		req := httptest.NewRequest(http.MethodPost, "/control/v1/fiscal-webhooks", bytes.NewReader(body))
		req.Header.Set("BeeFiscal-Signature", sign(body, "edge-control-secret-long-enough", time.Now().UTC()))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code < 400 {
			t.Fatalf("invalid event accepted: %d", w.Code)
		}
	}
	bad := httptest.NewRequest(http.MethodPost, "/control/v1/fiscal-webhooks", bytes.NewReader([]byte(`{}`)))
	bad.Header.Set("BeeFiscal-Signature", "00")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, bad)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("tampering accepted: %d", w.Code)
	}
}

func sign(body []byte, key string, now time.Time) string {
	timestamp := fmt.Sprintf("%d", now.Unix())
	m := hmac.New(sha256.New, []byte(key))
	_, _ = m.Write([]byte(timestamp + "."))
	_, _ = m.Write(body)
	return "t=" + timestamp + ",kid=test,v1=" + hex.EncodeToString(m.Sum(nil))
}

func TestRevocationWebhookRejectsStaleSignature(t *testing.T) {
	j, _ := journal.Open(t.TempDir() + "/edge.db")
	defer j.Close()
	now := time.Now().UTC()
	h := NewHandler(j, "edge-control-secret-long-enough")
	h.now = func() time.Time { return now }
	body := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/control/v1/fiscal-webhooks", bytes.NewReader(body))
	req.Header.Set("BeeFiscal-Signature", sign(body, "edge-control-secret-long-enough", now.Add(-10*time.Minute)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("stale signature accepted: %d", w.Code)
	}
}
