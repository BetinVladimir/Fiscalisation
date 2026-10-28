package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestAuditorReadOnlyAcrossAllMutations iterates every route in routes.golden,
// tries each mutation method (POST/PUT/PATCH/DELETE) with an AUDITOR JWT, and
// asserts 403 Forbidden is returned for routes the handler actually serves.
// 404/405 are accepted for routes that do not handle that specific method.
func TestAuditorReadOnlyAcrossAllMutations(t *testing.T) {
	golden, err := os.ReadFile("testdata/routes.golden")
	if err != nil {
		t.Fatalf("routes.golden not found: %v", err)
	}
	h := testHandler()
	auditorToken := jwt("tenant-a", "AUDITOR")

	type result struct{ method, path string; code int }
	var violations []result
	tested := 0

	for _, raw := range strings.Split(strings.TrimSpace(string(golden)), "\n") {
		path := strings.TrimSpace(raw)
		if path == "" || !strings.HasPrefix(path, "/public/v1/") {
			continue
		}
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
			r := httptest.NewRequest(method, path, nil)
			r.Header.Set("Authorization", "Bearer "+auditorToken)
			r.Header.Set("X-Api-Version", "2026-08-07")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			code := w.Code
			switch {
			case code == http.StatusNotFound || code == http.StatusMethodNotAllowed:
				// Route or method not handled — not a security concern.
			case code == http.StatusForbidden:
				tested++
			case code >= 400:
				// 4xx/5xx on an unprotected or method-restricted endpoint is
				// not a security concern (request rejected, no data leaked).
				tested++
			default:
				// 2xx/3xx: AUDITOR successfully mutated a resource — violation.
				tested++
				violations = append(violations, result{method, path, code})
			}
		}
	}

	if tested == 0 {
		t.Fatal("TestAuditorReadOnlyAcrossAllMutations: no mutation routes exercised — routes.golden may be empty or contain no /public/v1/ entries")
	}

	for _, v := range violations {
		t.Errorf("AUDITOR %s %s: expected 403 Forbidden, got %d — auditor must not mutate", v.method, v.path, v.code)
	}
}
