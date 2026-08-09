package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
)

func TestCanonicalExportArtifactsDownloadThroughPublicAPI(t *testing.T) {
	const authKey = "01234567890123456789012345678901"
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true)), config.Config{APIVersion: "2026-08-07", AuthHMACKey: authKey})
	call := func(method, path, tenant, key, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+jwt(tenant, "SUPERVISOR"))
		r.Header.Set("X-Api-Version", "2026-08-07")
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	formats := []struct{ format, media, extension string }{
		{"JSON", "application/json", ".json\""},
		{"CSV", "text/csv", ".csv\""},
		{"XLSX", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx\""},
	}
	for index, fixture := range formats {
		body := `{"type":"SUPTO_18_1","from":"2026-01-01T00:00:00Z","to":"2026-01-02T00:00:00Z","format":"` + fixture.format + `"}`
		created := call(http.MethodPost, "/public/v1/exports", "tenant-a", "canonical-export-create-000"+string(rune('1'+index)), body)
		if created.Code != http.StatusAccepted {
			t.Fatalf("%s create: %d %s", fixture.format, created.Code, created.Body.String())
		}
		var operation struct {
			ExportID string `json:"fiscal_reference"`
		}
		if json.Unmarshal(created.Body.Bytes(), &operation) != nil || operation.ExportID == "" {
			t.Fatalf("%s operation: %s", fixture.format, created.Body.String())
		}
		manifestResponse := call(http.MethodGet, "/public/v1/exports/"+operation.ExportID, "tenant-a", "", "")
		var manifest struct {
			Artifact struct {
				ID string `json:"artifact_id"`
			} `json:"artifact"`
		}
		if manifestResponse.Code != http.StatusOK || json.Unmarshal(manifestResponse.Body.Bytes(), &manifest) != nil || manifest.Artifact.ID == "" {
			t.Fatalf("%s manifest: %d %s", fixture.format, manifestResponse.Code, manifestResponse.Body.String())
		}
		path := "/public/v1/exports/" + operation.ExportID + "/artifacts/" + manifest.Artifact.ID
		artifact := call(http.MethodGet, path, "tenant-a", "", "")
		if artifact.Code != http.StatusOK || artifact.Header().Get("Content-Type") != fixture.media || !bytes.Contains([]byte(artifact.Header().Get("Content-Disposition")), []byte(fixture.extension)) || artifact.Body.Len() == 0 {
			t.Fatalf("%s download: %d %s %s", fixture.format, artifact.Code, artifact.Header(), artifact.Body.String())
		}
		if fixture.format == "XLSX" {
			if _, err := zip.NewReader(bytes.NewReader(artifact.Body.Bytes()), int64(artifact.Body.Len())); err != nil {
				t.Fatalf("invalid public XLSX: %v", err)
			}
		}
		if foreign := call(http.MethodGet, path, "tenant-b", "", ""); foreign.Code != http.StatusNotFound {
			t.Fatalf("%s cross-tenant artifact: %d", fixture.format, foreign.Code)
		}
		wrongPath := "/public/v1/exports/" + operation.ExportID + "/artifacts/00000000-0000-4000-8000-000000000999"
		if wrong := call(http.MethodGet, wrongPath, "tenant-a", "", ""); wrong.Code != http.StatusNotFound {
			t.Fatalf("%s mismatched artifact id accepted: %d", fixture.format, wrong.Code)
		}
	}
}

func TestPeriodizedExportPublicContractAndTenantArtifactBoundary(t *testing.T) {
	const authKey = "01234567890123456789012345678901"
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true)), config.Config{APIVersion: "2026-08-07", AuthHMACKey: authKey})
	call := func(method, path, tenant, key, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		r.Header.Set("Authorization", "Bearer "+jwt(tenant, "SUPERVISOR"))
		r.Header.Set("X-Api-Version", "2026-08-07")
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	body := `{"type":"SUPTO_18_1","from":"2025-12-31T20:00:00Z","to":"2026-01-01T02:00:00Z","format":"JSON"}`
	created := call(http.MethodPost, "/public/v1/exports/periodized", "tenant-a", "periodized-export-create-0001", body)
	if created.Code != http.StatusAccepted {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var operation struct {
		ExportID string `json:"fiscal_reference"`
	}
	if json.Unmarshal(created.Body.Bytes(), &operation) != nil || operation.ExportID == "" {
		t.Fatalf("operation response: %s", created.Body.String())
	}
	replayed := call(http.MethodPost, "/public/v1/exports/periodized", "tenant-a", "periodized-export-create-0001", body)
	if replayed.Code != http.StatusAccepted || replayed.Header().Get("Idempotency-Replayed") != "true" || replayed.Body.String() != created.Body.String() {
		t.Fatalf("replay: %d %s", replayed.Code, replayed.Body.String())
	}
	mismatch := call(http.MethodPost, "/public/v1/exports/periodized", "tenant-a", "periodized-export-create-0001", `{"type":"SUPTO_18_2","from":"2025-12-31T20:00:00Z","to":"2026-01-01T02:00:00Z","format":"JSON"}`)
	if mismatch.Code != http.StatusConflict {
		t.Fatalf("idempotency mismatch: %d %s", mismatch.Code, mismatch.Body.String())
	}
	periodsResponse := call(http.MethodGet, "/public/v1/exports/"+operation.ExportID+"/periods", "tenant-a", "", "")
	if periodsResponse.Code != http.StatusOK {
		t.Fatalf("periods: %d %s", periodsResponse.Code, periodsResponse.Body.String())
	}
	var periodized struct {
		Periods []struct {
			Currency string `json:"official_currency"`
			Artifact struct {
				ID string `json:"artifact_id"`
			} `json:"artifact"`
		} `json:"periods"`
	}
	if json.Unmarshal(periodsResponse.Body.Bytes(), &periodized) != nil || len(periodized.Periods) != 2 || periodized.Periods[0].Currency != "BGN" || periodized.Periods[1].Currency != "EUR" {
		t.Fatalf("period manifest: %s", periodsResponse.Body.String())
	}
	artifactPath := "/public/v1/exports/" + operation.ExportID + "/artifacts/" + periodized.Periods[0].Artifact.ID
	artifact := call(http.MethodGet, artifactPath, "tenant-a", "", "")
	if artifact.Code != http.StatusOK || artifact.Header().Get("Content-Type") != "application/json" || !bytes.Contains(artifact.Body.Bytes(), []byte(`"official_currency":"BGN"`)) {
		t.Fatalf("artifact: %d %s %s", artifact.Code, artifact.Header().Get("Content-Type"), artifact.Body.String())
	}
	foreign := call(http.MethodGet, artifactPath, "tenant-b", "", "")
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant artifact: %d %s", foreign.Code, foreign.Body.String())
	}
	foreignPeriods := call(http.MethodGet, "/public/v1/exports/"+operation.ExportID+"/periods", "tenant-b", "", "")
	if foreignPeriods.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant periods: %d %s", foreignPeriods.Code, foreignPeriods.Body.String())
	}
}

func TestPeriodizedExportRejectsInvalidIntervalAndCashierRole(t *testing.T) {
	const authKey = "01234567890123456789012345678901"
	h := NewHandler(domain.NewService(domain.NewMemoryRepository(), domain.NewSimulator(true)), config.Config{APIVersion: "2026-08-07", AuthHMACKey: authKey})
	call := func(role, body, key string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/public/v1/exports/periodized", bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+jwt("tenant-a", role))
		r.Header.Set("X-Api-Version", "2026-08-07")
		r.Header.Set("Idempotency-Key", key)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	invalid := call("SUPERVISOR", `{"type":"SUPTO_18_1","from":"2026-01-01T00:00:00Z","to":"2026-01-01T00:00:00Z","format":"JSON"}`, "periodized-invalid-interval-01")
	if invalid.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid interval: %d %s", invalid.Code, invalid.Body.String())
	}
	denied := call("CASHIER", `{"type":"SUPTO_18_1","from":"2026-01-01T00:00:00Z","to":"2026-01-02T00:00:00Z","format":"JSON"}`, "periodized-cashier-denied-01")
	if denied.Code != http.StatusForbidden {
		t.Fatalf("cashier export accepted: %d %s", denied.Code, denied.Body.String())
	}
}
