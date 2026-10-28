package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProvisioningRelayHTTPRejectsRolesAndUnknownFields(t *testing.T) {
	for _, path := range []string{"/public/v1/devices/device/provisioning-deliveries", "/public/v1/devices/device/provisioning-deliveries/by-proof:confirm"} {
		for _, role := range []string{"CASHIER", "ADMIN"} {
			r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"caller_supplied_public_key":"evil"}`))
			r.Header.Set("Authorization", "Bearer "+jwt("tenant-a", role))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-API-Version", "2026-08-07")
			r.Header.Set("Idempotency-Key", "test-provision-delivery-0001")
			w := httptest.NewRecorder()
			testHandler().ServeHTTP(w, r)
			expected := 400
			if role == "CASHIER" {
				expected = 403
			}
			if w.Code != expected {
				t.Fatalf("%s %s expected%d got%d %s", path, role, expected, w.Code, w.Body.String())
			}
		}
	}
}
