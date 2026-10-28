package api

import (
	"encoding/json"
	appversion "fiscalisation/fiscal-backend/internal/version"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInstalledVersionsInventory(t *testing.T) {
	h := testHandler()

	req := httptest.NewRequest(http.MethodGet, "/admin/v1/installed-versions", nil)
	req.Header.Set("Authorization", "Bearer "+jwt("tenant-a", "PLATFORM_ADMIN"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("cannot decode response: %v", err)
	}

	if body["backend"] == "" {
		t.Error("installed-versions response missing 'backend' field")
	}
	if body["backend"] != appversion.Backend {
		t.Errorf("backend version mismatch: got %q, want %q", body["backend"], appversion.Backend)
	}
	if body["apk_minimum"] == "" {
		t.Error("installed-versions response missing 'apk_minimum' field")
	}
	if body["firmware_s3_minimum"] == "" {
		t.Error("installed-versions response missing 'firmware_s3_minimum' field")
	}
}

func TestInstalledVersionsMethodNotAllowed(t *testing.T) {
	h := testHandler()

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/admin/v1/installed-versions", nil)
		req.Header.Set("Authorization", "Bearer "+jwt("tenant-a", "PLATFORM_ADMIN"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, rec.Code)
		}
	}
}
