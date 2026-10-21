package api

import (
	"context"
	"encoding/json"
	"fiscalisation/fiscal-backend/internal/domain"
	"net/http"
	"time"
)

// Gate authoritative business requests before an errorless repository read could
// turn a temporarily unavailable authority into a false not-found response.
func repositoryAvailability(s *domain.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.PersistenceReady() != nil {
			repositoryUnavailable(w)
			return
		}
		epoch, ok := r.Context().Value(repositoryEpochKey{}).(uint64)
		if ok && epoch != s.PersistenceEpoch() {
			repositoryUnavailable(w)
			return
		}
		if !ok {
			epoch = s.PersistenceEpoch()
			r = r.WithContext(context.WithValue(r.Context(), repositoryEpochKey{}, epoch))
		}
		next.ServeHTTP(&repositoryAvailabilityWriter{ResponseWriter: w, service: s, epoch: epoch}, r)
	})
}

type repositoryEpochKey struct{}

func repositoryUnavailable(w http.ResponseWriter) {
	for _, key := range []string{"Etag", "Location", "Content-Length", "Content-Encoding"} {
		w.Header().Del(key)
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "urn:beefiscal:error:persistence-recovery-required", "title": "PERSISTENCE_RECOVERY_REQUIRED", "status": 503, "detail": "Authoritative persistence recovery required; retry the request.", "code": "PERSISTENCE_RECOVERY_REQUIRED", "retryable": true, "trace_id": time.Now().UTC().Format("20060102150405.000000000")})
}

type repositoryAvailabilityWriter struct {
	http.ResponseWriter
	service          *domain.Service
	written, blocked bool
	epoch            uint64
}

func (w *repositoryAvailabilityWriter) WriteHeader(status int) {
	if w.written {
		return
	}
	w.written = true
	if w.service.PersistenceReady() != nil || w.service.PersistenceEpoch() != w.epoch {
		w.blocked = true
		repositoryUnavailable(w.ResponseWriter)
		return
	}
	w.ResponseWriter.WriteHeader(status)
}
func (w *repositoryAvailabilityWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	if w.blocked {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}
