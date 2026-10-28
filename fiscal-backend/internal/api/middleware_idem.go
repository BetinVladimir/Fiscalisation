package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"fiscalisation/fiscal-backend/internal/auth"
	"fiscalisation/fiscal-backend/internal/domain"
)

type fiscalIdempotencyOwnerKey struct{}

type fiscalCapturedResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func (w *fiscalCapturedResponse) Header() http.Header { return w.header }
func (w *fiscalCapturedResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *fiscalCapturedResponse) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(body)
}

func fiscalIdempotency(s *domain.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/public/v1") || (r.Method != http.MethodPost && r.Method != http.MethodPatch && r.Method != http.MethodDelete) {
			next.ServeHTTP(w, r)
			return
		}
		if n := len(r.Header.Get("Idempotency-Key")); n < 16 || n > 255 {
			problem(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			problem(w, http.StatusBadRequest, "INVALID_BODY")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		hash := fiscalIdempotencyFingerprint(r, body)
		old, owner, err := s.ClaimReplay(replayKey(r), hash)
		if err != nil {
			if strings.Contains(err.Error(), "mismatch") {
				problem(w, http.StatusConflict, "IDEMPOTENCY_MISMATCH")
			} else {
				problem(w, http.StatusServiceUnavailable, "IDEMPOTENCY_RESERVATION_FAILED")
			}
			return
		}
		if !owner {
			if old.Pending {
				applyFiscalReplayHeaders(w.Header(), old.Headers, old.ContentType)
				w.Header().Set("Retry-After", "1")
				if old.Status >= 500 && len(old.Body) > 0 {
					w.Header().Set("Idempotency-Replayed", "true")
					if w.Header().Get("Content-Type") == "" {
						w.Header().Set("Content-Type", "application/json")
					}
					w.WriteHeader(old.Status)
					_, _ = w.Write(old.Body)
				} else {
					problem(w, http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS")
				}
				return
			}
			w.Header().Set("Idempotency-Replayed", "true")
			applyFiscalReplayHeaders(w.Header(), old.Headers, old.ContentType)
			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", "application/json")
			}
			w.WriteHeader(old.Status)
			_, _ = w.Write(old.Body)
			return
		}
		cw := &fiscalCapturedResponse{header: make(http.Header)}
		r = r.WithContext(context.WithValue(r.Context(), fiscalIdempotencyOwnerKey{}, true))
		repositoryAvailability(s, next).ServeHTTP(cw, r)
		if cw.status == 0 {
			cw.status = http.StatusOK
		}
		if cw.status < 500 {
			if err = s.PutReplay(replayKey(r), domain.ReplayRecord{Hash: hash, Status: cw.status, Body: append([]byte(nil), cw.body.Bytes()...), ContentType: cw.header.Get("Content-Type"), Headers: fiscalReplayHeaders(cw.header)}); err != nil {
				problem(w, http.StatusInternalServerError, "IDEMPOTENCY_PERSIST_FAILED")
				return
			}
		} else {
			cw.header.Set("Retry-After", "1")
			if err = s.PutAmbiguousReplay(replayKey(r), domain.ReplayRecord{Hash: hash, Status: cw.status, Body: append([]byte(nil), cw.body.Bytes()...), ContentType: cw.header.Get("Content-Type"), Headers: fiscalReplayHeaders(cw.header), Pending: true}); err != nil {
				problem(w, http.StatusInternalServerError, "IDEMPOTENCY_PERSIST_FAILED")
				return
			}
		}
		for key, values := range cw.header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(cw.status)
		_, _ = w.Write(cw.body.Bytes())
	})
}

var fiscalReplayableHeader = map[string]bool{"Content-Type": true, "Etag": true, "Location": true, "Retry-After": true, "Cache-Control": true}

func fiscalReplayHeaders(source http.Header) map[string][]string {
	result := map[string][]string{}
	for key, values := range source {
		canonical := http.CanonicalHeaderKey(key)
		if fiscalReplayableHeader[canonical] {
			result[canonical] = append([]string(nil), values...)
		}
	}
	return result
}

func applyFiscalReplayHeaders(target http.Header, headers map[string][]string, legacyContentType string) {
	for key, values := range headers {
		canonical := http.CanonicalHeaderKey(key)
		if !fiscalReplayableHeader[canonical] {
			continue
		}
		for _, value := range values {
			target.Add(canonical, value)
		}
	}
	if target.Get("Content-Type") == "" && legacyContentType != "" {
		target.Set("Content-Type", legacyContentType)
	}
}

func fiscalIdempotencyFingerprint(r *http.Request, body []byte) string {
	h := sha256.New()
	issuer, subject := "", ""
	if claims, ok := auth.ClaimsFrom(r.Context()); ok {
		issuer, subject = claims.Issuer, claims.Subject
	}
	for _, value := range []string{r.URL.Query().Encode(), r.Header.Get("If-Match"), r.Header.Get("X-App-Instance-Id"), issuer, subject} {
		_, _ = fmt.Fprintf(h, "%d:", len(value))
		_, _ = h.Write([]byte(value))
	}
	_, _ = fmt.Fprintf(h, "%d:", len(body))
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func (h *Handler) replay(w http.ResponseWriter, r *http.Request, b []byte) bool {
	if owner, _ := r.Context().Value(fiscalIdempotencyOwnerKey{}).(bool); owner {
		return false
	}
	k := replayKey(r)
	v, ok := h.svc.Replay(k)
	if !ok {
		return false
	}
	if v.Hash != fiscalIdempotencyFingerprint(r, b) {
		problem(w, 409, "IDEMPOTENCY_MISMATCH")
		return true
	}
	w.Header().Set("Idempotency-Replayed", "true")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(v.Status)
	_, _ = w.Write(v.Body)
	return true
}

func (h *Handler) saveReplay(w http.ResponseWriter, r *http.Request, b []byte, status int, v any) {
	out, _ := json.Marshal(v)
	k := replayKey(r)
	if owner, _ := r.Context().Value(fiscalIdempotencyOwnerKey{}).(bool); !owner {
		if e := h.svc.PutReplay(k, domain.ReplayRecord{Hash: fiscalIdempotencyFingerprint(r, b), Status: status, Body: out, ContentType: "application/json"}); e != nil {
			problem(w, 500, "IDEMPOTENCY_PERSIST_FAILED")
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(out)
}

func (h *Handler) saveEmptyReplay(w http.ResponseWriter, r *http.Request, b []byte, status int) {
	k := replayKey(r)
	if owner, _ := r.Context().Value(fiscalIdempotencyOwnerKey{}).(bool); !owner {
		if e := h.svc.PutReplay(k, domain.ReplayRecord{Hash: fiscalIdempotencyFingerprint(r, b), Status: status, Body: []byte{}}); e != nil {
			problem(w, 500, "IDEMPOTENCY_PERSIST_FAILED")
			return
		}
	}
	w.WriteHeader(status)
}
