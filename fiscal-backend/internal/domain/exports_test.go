package domain

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestComplianceExportsJSONCSVAndXLSX(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	registerID, _ := prepareBLERegister(t, s, "tenant-1")
	sale, _ := s.CreateSale(CreateSale{TenantID: "tenant-1", ExternalID: "e1", RegisterID: registerID, OperatorID: "A001"})
	_, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B"})
	for _, format := range []string{"JSON", "CSV", "XLSX"} {
		op, err := s.CreateExport(ExportRequest{Type: "SUPTO_18_1", From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour), Format: format}, "tenant-1")
		if err != nil || op.State != "FISCALIZED" {
			t.Fatal(format, op, err)
		}
		v, err := s.Export(op.FiscalReference, "tenant-1")
		if err != nil {
			t.Fatal(err)
		}
		if v["official_currency"] != "EUR" {
			t.Fatal(v)
		}
		b, media, err := s.ExportArtifact(op.FiscalReference, "tenant-1")
		if err != nil || len(b) == 0 || media == "" {
			t.Fatal(format, media, err)
		}
		if format == "XLSX" {
			if _, err = zip.NewReader(bytes.NewReader(b), int64(len(b))); err != nil {
				t.Fatal("invalid xlsx zip", err)
			}
		}
		if _, _, err = s.ExportArtifact(op.FiscalReference, "tenant-2"); err == nil {
			t.Fatal("cross tenant export leaked")
		}
	}
}

func TestExportRejectsInvalidRangeAndFormat(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	if _, err := s.CreateExport(ExportRequest{Type: "SUPTO_18_1", From: time.Now(), To: time.Now().Add(-time.Hour), Format: "PDF"}, "tenant-1"); err == nil {
		t.Fatal("invalid export accepted")
	}
}

func TestPeriodizedExportSplitsBGNAndEURWithoutBoundaryOverlap(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))
	boundary := bgEuroAdoption
	fixtures := []Sale{
		{ID: "sale-bgn", TenantID: "tenant-1", ExternalID: "bgn", State: "COMPLETED", CreatedAt: boundary.Add(-time.Nanosecond), UpdatedAt: boundary.Add(-time.Nanosecond)},
		{ID: "sale-eur", TenantID: "tenant-1", ExternalID: "eur", State: "COMPLETED", CreatedAt: boundary, UpdatedAt: boundary},
		{ID: "sale-other", TenantID: "tenant-2", ExternalID: "other", State: "COMPLETED", CreatedAt: boundary, UpdatedAt: boundary},
	}
	for _, sale := range fixtures {
		if err := repo.PutSale(sale); err != nil {
			t.Fatal(err)
		}
	}
	op, err := s.CreatePeriodizedExport(ExportRequest{Type: "SUPTO_18_1", From: boundary.Add(-time.Hour), To: boundary.Add(time.Hour), Format: "JSON"}, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.ExportPeriods(op.FiscalReference, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(v["periods"])
	var periods []struct {
		Currency string `json:"official_currency"`
		Artifact struct {
			ID     string `json:"artifact_id"`
			SHA256 string `json:"sha256"`
			Size   int    `json:"size"`
		} `json:"artifact"`
	}
	if json.Unmarshal(encoded, &periods) != nil || len(periods) != 2 || periods[0].Currency != "BGN" || periods[1].Currency != "EUR" {
		t.Fatalf("unexpected periods: %s", encoded)
	}
	for index, expected := range []string{"sale-bgn", "sale-eur"} {
		artifact, media, artifactErr := s.ExportPeriodArtifact(op.FiscalReference, periods[index].Artifact.ID, "tenant-1")
		if artifactErr != nil || media != "application/json" || !bytes.Contains(artifact, []byte(expected)) {
			t.Fatalf("period %d: media=%s body=%s err=%v", index, media, artifact, artifactErr)
		}
		digest := sha256.Sum256(artifact)
		if periods[index].Artifact.Size != len(artifact) || periods[index].Artifact.SHA256 != fmt.Sprintf("%x", digest[:]) {
			t.Fatalf("period %d manifest does not bind artifact", index)
		}
		unexpected := "sale-eur"
		if index == 1 {
			unexpected = "sale-bgn"
		}
		if bytes.Contains(artifact, []byte(unexpected)) || bytes.Contains(artifact, []byte("sale-other")) {
			t.Fatalf("period overlap/tenant leak: %s", artifact)
		}
	}
	if _, _, err = s.ExportPeriodArtifact(op.FiscalReference, periods[0].Artifact.ID, "tenant-2"); err == nil {
		t.Fatal("cross-tenant artifact leaked")
	}
}

func TestPeriodizedExportSingleCurrencyAndEmptyArtifactsRemainSelfDescribing(t *testing.T) {
	for _, fixture := range []struct {
		name, format, currency string
		from, to               time.Time
	}{
		{"historical CSV", "CSV", "BGN", bgEuroAdoption.Add(-2 * time.Hour), bgEuroAdoption.Add(-time.Hour)},
		{"current XLSX", "XLSX", "EUR", bgEuroAdoption, bgEuroAdoption.Add(time.Hour)},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			s := NewService(NewMemoryRepository(), NewSimulator(true))
			op, err := s.CreatePeriodizedExport(ExportRequest{Type: "SUPTO_18_1", From: fixture.from, To: fixture.to, Format: fixture.format}, "tenant-1")
			if err != nil {
				t.Fatal(err)
			}
			v, _ := s.ExportPeriods(op.FiscalReference, "tenant-1")
			encoded, _ := json.Marshal(v["periods"])
			if !strings.Contains(string(encoded), `"official_currency":"`+fixture.currency+`"`) {
				t.Fatalf("currency missing: %s", encoded)
			}
			var periods []struct {
				Artifact struct {
					ID string `json:"artifact_id"`
				} `json:"artifact"`
			}
			_ = json.Unmarshal(encoded, &periods)
			artifact, _, err := s.ExportPeriodArtifact(op.FiscalReference, periods[0].Artifact.ID, "tenant-1")
			if err != nil || len(artifact) == 0 {
				t.Fatalf("empty period artifact missing: %v", err)
			}
			if fixture.format == "CSV" && !bytes.Contains(artifact, []byte(fixture.currency)) {
				t.Fatalf("CSV metadata missing: %s", artifact)
			}
			if fixture.format == "XLSX" {
				if _, err = zip.NewReader(bytes.NewReader(artifact), int64(len(artifact))); err != nil {
					t.Fatal("invalid period XLSX", err)
				}
			}
		})
	}
	if _, err := NewService(NewMemoryRepository(), NewSimulator(true)).CreatePeriodizedExport(ExportRequest{Type: "SUPTO_18_1", From: bgEuroAdoption, To: bgEuroAdoption, Format: "JSON"}, "tenant-1"); err == nil {
		t.Fatal("empty half-open interval accepted")
	}
}

func TestPeriodizedExportManifestAndArtifactsSurviveRestart(t *testing.T) {
	store := &testStore{}
	repository, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(repository, NewSimulator(true))
	op, err := service.CreatePeriodizedExport(ExportRequest{Type: "SUPTO_18_1", From: bgEuroAdoption.Add(-time.Hour), To: bgEuroAdoption.Add(time.Hour), Format: "JSON"}, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	restartedRepository, err := NewPersistentRepository(store)
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewService(restartedRepository, NewSimulator(true))
	manifest, err := restarted.ExportPeriods(op.FiscalReference, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(manifest["periods"])
	var periods []struct {
		Artifact struct {
			ID string `json:"artifact_id"`
		} `json:"artifact"`
	}
	if json.Unmarshal(encoded, &periods) != nil || len(periods) != 2 {
		t.Fatalf("restart manifest: %s", encoded)
	}
	for _, period := range periods {
		if artifact, _, artifactErr := restarted.ExportPeriodArtifact(op.FiscalReference, period.Artifact.ID, "tenant-1"); artifactErr != nil || len(artifact) == 0 {
			t.Fatalf("restart artifact: %v", artifactErr)
		}
	}
}
