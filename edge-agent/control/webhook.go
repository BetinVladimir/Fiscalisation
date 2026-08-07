package control

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fiscalisation/edge-agent/journal"
)

// Handler consumes the narrow Fiscal control-plane surface needed by Edge.
// The raw body is authenticated before parsing and revocation is durable before
// success is returned, so a Fiscal retry is always safe.
type Handler struct {
	journal *journal.Journal
	key     []byte
	now     func() time.Time
}

func NewHandler(j *journal.Journal, key string) *Handler {
	return &Handler{journal: j, key: []byte(key), now: time.Now}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
		return
	}
	if r.URL.Path != "/control/v1/fiscal-webhooks" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil || !validSignature(body, r.Header.Get("BeeFiscal-Signature"), h.key, h.now().UTC()) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}
	var event struct {
		EventID    string `json:"event_id"`
		EventType  string `json:"event_type"`
		APIVersion string `json:"api_version"`
		ResourceID string `json:"resource_id"`
		Data       struct {
			SessionID string    `json:"ble_session_id"`
			ExpiresAt time.Time `json:"expires_at"`
		} `json:"data"`
	}
	if json.Unmarshal(body, &event) != nil || event.EventID == "" || event.EventType != "ble.session.revoked" || event.APIVersion != "2026-08-07" || event.ResourceID == "" || event.Data.SessionID != event.ResourceID || !event.Data.ExpiresAt.After(h.now().UTC()) {
		http.Error(w, "invalid event", http.StatusUnprocessableEntity)
		return
	}
	if err = h.journal.RevokeBLESession(event.Data.SessionID, h.now().UTC(), event.Data.ExpiresAt); err != nil {
		http.Error(w, "persistence failure", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validSignature(body []byte, encoded string, key []byte, now time.Time) bool {
	fields := map[string]string{}
	for _, part := range strings.Split(encoded, ",") {
		pair := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(pair) != 2 || fields[pair[0]] != "" {
			return false
		}
		fields[pair[0]] = pair[1]
	}
	seconds, err := strconv.ParseInt(fields["t"], 10, 64)
	if err != nil || fields["kid"] == "" || len(key) < 16 || now.Unix()-seconds < -300 || now.Unix()-seconds > 300 {
		return false
	}
	got, err := hex.DecodeString(fields["v1"])
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, key)
	_, _ = m.Write([]byte(fields["t"] + "."))
	_, _ = m.Write(body)
	return hmac.Equal(got, m.Sum(nil))
}
