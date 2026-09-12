package domain

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

func TestComplianceExportsJSONCSVAndXLSX(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	registerID, _ := prepareBLERegister(t, s, "tenant-1")
	sale, err := s.CreateSale(CreateSale{TenantID: "tenant-1", ExternalID: "e1", RegisterID: registerID, OperatorID: "A001"})
	if err != nil {
		t.Fatal(err)
	}
	discount := Money{Amount: "0.20", Currency: "EUR"}
	sale, err = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, Discount: &discount, TaxGroup: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "payment-1", Type: "CASH", Amount: Money{Amount: "0.80", Currency: "EUR"}}, "tenant-1"); err != nil {
		t.Fatal(err)
	}
	for _, exportType := range []string{"SUPTO_18_1", "SUPTO_18_2", "SUPTO_18_3", "SUPTO_18_4", "SUPTO_18_5", "SUPTO_18_9"} {
		for _, format := range []string{"JSON", "CSV", "XLSX"} {
			op, err := s.CreateExport(ExportRequest{Type: exportType, From: time.Now().Add(-time.Hour), To: time.Now().Add(time.Hour), Format: format}, "tenant-1")
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
				archive, zipErr := zip.NewReader(bytes.NewReader(b), int64(len(b)))
				if zipErr != nil {
					t.Fatal("invalid xlsx zip", err)
				}
				for _, file := range archive.File {
					if file.Name == "xl/worksheets/sheet1.xml" {
						reader, _ := file.Open()
						b, _ = io.ReadAll(reader)
						_ = reader.Close()
					}
				}
			}
			evidence := []string{"system_sale_id"}
			if exportType == "SUPTO_18_9" {
				evidence = []string{"catalog", "PAYMENT_TYPE", "PRODUCT"}
			}
			if format == "JSON" {
				evidence = append(evidence, "0.20", "0.80", "tax_group", "CASH", "lines")
			}
			for _, evidence := range evidence {
				if !bytes.Contains(b, []byte(evidence)) {
					t.Fatalf("%s export lost detailed sale evidence %q: %s", format, evidence, b)
				}
			}
			if format == "JSON" && (!bytes.Contains(b, []byte(`"schema_id":"BG_`+exportType+`_V1"`)) || !bytes.Contains(b, []byte(`"columns"`))) {
				t.Fatalf("%s missing field-perfect schema metadata: %s", exportType, b)
			}
			if _, _, err = s.ExportArtifact(op.FiscalReference, "tenant-2"); err == nil {
				t.Fatal("cross tenant export leaked")
			}
		}
	}
}

func TestExportRejectsInvalidRangeAndFormat(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	if _, err := s.CreateExport(ExportRequest{Type: "SUPTO_18_1", From: time.Now(), To: time.Now().Add(-time.Hour), Format: "PDF"}, "tenant-1"); err == nil {
		t.Fatal("invalid export accepted")
	}
	boundary := time.Now().UTC()
	if _, err := s.CreateExport(ExportRequest{Type: "SUPTO_18_1", From: boundary, To: boundary, Format: "JSON"}, "tenant-1"); err == nil {
		t.Fatal("zero-width export interval accepted")
	}
}

func TestXLSXColumnNameBeyondZ(t *testing.T) {
	for index, want := range map[int]string{0: "A", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA"} {
		if got := xlsxColumnName(index); got != want {
			t.Fatalf("xlsxColumnName(%d) = %q, want %q", index, got, want)
		}
	}
}

func TestAnnex29ExportHeadersAreFieldComplete(t *testing.T) {
	want := map[string][]string{
		"SUPTO_18_1": {"unp", "system_sale_id", "location_code", "location_name", "opened_date", "opened_time", "workstation_code", "operator_code", "net_amount", "discount_amount", "vat_amount", "amount_due", "invoice_number", "invoice_date", "completed_date", "completed_time", "customer_code", "customer_name"},
		"SUPTO_18_2": {"unp", "system_sale_id", "opened_date", "completed_date", "sale_total", "payment_date", "operator_code", "paid_amount", "payment_type", "fiscal_device_number"},
		"SUPTO_18_3": {"unp", "system_sale_id", "product_code", "product_name", "quantity", "unit_net_price", "discount_amount", "vat_rate", "vat_amount", "gross_amount", "period_from", "period_to"},
		"SUPTO_18_4": {"unp", "system_sale_id", "product_code", "product_name", "quantity", "unit_net_price", "discount_amount", "vat_rate", "vat_amount", "gross_amount", "completed_date", "completed_time", "reversed_date", "reversed_time", "fiscal_device_number", "operator_code"},
		"SUPTO_18_5": {"unp", "system_sale_id", "product_code", "product_name", "quantity", "unit_net_price", "discount_amount", "vat_rate", "vat_amount", "gross_amount", "opened_date", "opened_time", "cancelled_date", "cancelled_time", "operator_code"},
		"SUPTO_18_6": {"record_id", "delivery_date", "delivery_time", "operator_code", "supplier_code", "supplier_name", "invoice_number", "invoice_date", "net_amount", "discount_amount", "vat_amount", "gross_amount", "payment_type"},
		"SUPTO_18_7": {"record_id", "product_code", "product_name", "quantity", "unit_price", "discount_amount", "vat_amount", "gross_amount"},
		"SUPTO_18_8": {"product_code", "product_name", "opening_quantity", "opening_value", "debit_quantity", "debit_value", "credit_quantity", "credit_value", "closing_quantity", "closing_value"},
		"SUPTO_18_9": {"catalog", "code", "name", "configured_at", "changed_at", "deactivated_at", "details"},
	}
	for exportType, expected := range want {
		headers, _ := normativeExportTable(exportType, nil)
		if fmt.Sprint(headers) != fmt.Sprint(expected) {
			t.Fatalf("%s headers = %v, want %v", exportType, headers, expected)
		}
	}
}

func TestAnnex29TaxCalculationsCoverAllTaxGroups(t *testing.T) {
	for _, tc := range []struct{ group, net, vat string }{{"A", "120.00", "0.00"}, {"B", "100.00", "20.00"}, {"C", "100.00", "20.00"}, {"D", "110.09", "9.91"}} {
		line := SaleLine{LineID: "line", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "120.00", Currency: "EUR"}, TaxGroup: tc.group}
		net, _, _, vat, gross := exportLineAmounts(line)
		if net != tc.net || vat != tc.vat || gross != "120.00" {
			t.Fatalf("group %s: net=%s vat=%s gross=%s", tc.group, net, vat, gross)
		}
	}
}

func TestAnnex29ReversalExportKeepsBothLifecycleTimes(t *testing.T) {
	completed := "2026-09-11T10:00:00Z"
	reversed := "2026-09-11T11:30:00Z"
	row := exportRow{UNP: "DY000001-A001-0000001", SaleID: "sale-1", State: "CANCELLED", FiscalOperationID: "op-sale", CompletedAt: completed, ReversedAt: reversed, OperatorCode: "A001", FiscalDevice: FiscalDeviceSnapshot{FiscalDeviceNumber: "DY000001"}, Lines: []SaleLine{{LineID: "line-1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "12.00", Currency: "EUR"}, TaxGroup: "B"}}}
	_, records := normativeExportTable("SUPTO_18_4", []exportRow{row})
	if len(records) != 1 || records[0][10] != "2026-09-11" || records[0][11] != "10:00:00" || records[0][12] != "2026-09-11" || records[0][13] != "11:30:00" {
		t.Fatalf("completion/reversal timestamps collapsed: %#v", records)
	}
}

func TestComplianceExportUsesImmutableLocationAndDeviceFilters(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	registerID, deviceID := prepareBLERegister(t, s, "tenant-filter")
	sale, err := s.CreateSale(CreateSale{TenantID: "tenant-filter", ExternalID: "filter-sale", RegisterID: registerID, OperatorID: "A001"})
	if err != nil || sale.LocationID == "" {
		t.Fatalf("sale-time location was not captured: %+v %v", sale, err)
	}
	sale, err = s.AddLineForTenant(sale.ID, SaleLine{LineID: "line-filter", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B"}, sale.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "pay-filter", Type: "CASH", Amount: Money{Amount: "1.00", Currency: "EUR"}}, sale.TenantID); err != nil {
		t.Fatal(err)
	}
	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)
	for name, request := range map[string]ExportRequest{
		"matching":       {Type: "SUPTO_18_1", From: from, To: to, LocationID: sale.LocationID, DeviceID: deviceID, Format: "JSON"},
		"wrong-location": {Type: "SUPTO_18_1", From: from, To: to, LocationID: "00000000-0000-4000-8000-000000000001", Format: "JSON"},
		"wrong-device":   {Type: "SUPTO_18_1", From: from, To: to, DeviceID: "00000000-0000-4000-8000-000000000002", Format: "JSON"},
	} {
		op, exportErr := s.CreateExport(request, sale.TenantID)
		if exportErr != nil {
			t.Fatal(name, exportErr)
		}
		artifact, _, exportErr := s.ExportArtifact(op.FiscalReference, sale.TenantID)
		var document struct {
			Rows []exportRow `json:"rows"`
		}
		if exportErr != nil || json.Unmarshal(artifact, &document) != nil {
			t.Fatal(name, exportErr, string(artifact))
		}
		want := 0
		if name == "matching" {
			want = 1
		}
		if len(document.Rows) != want {
			t.Fatalf("%s filter returned %d rows, want %d: %s", name, len(document.Rows), want, artifact)
		}
	}
}

func TestExportPublicationIsAtomicOnFirstPersistenceFailure(t *testing.T) {
	repo, err := NewPersistentRepository(&failingStore{err: errors.New("disk unavailable")})
	if err != nil {
		t.Fatal(err)
	}
	s := NewService(repo, NewSimulator(true))
	now := time.Now().UTC()
	if _, err = s.CreatePeriodizedExport(ExportRequest{Type: "SUPTO_18_1", From: now.Add(-time.Hour), To: now.Add(time.Hour), Format: "JSON"}, "tenant-1"); err == nil {
		t.Fatal("injected export publication failure was ignored")
	}
	if len(repo.Resources("export_periods", "tenant-1")) != 0 || len(repo.Operations()) != 0 || len(repo.artifacts) != 0 || len(repo.audit) != 0 {
		t.Fatalf("failed atomic export leaked partial state: resources=%+v operations=%+v artifacts=%d audit=%d", repo.Resources("export_periods", "tenant-1"), repo.Operations(), len(repo.artifacts), len(repo.audit))
	}
}

func TestCanonicalExportIntervalIsHalfOpen(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))
	from := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	for _, sale := range []Sale{
		{ID: "sale-at-from", TenantID: "tenant-1", ExternalID: "included-from", State: "COMPLETED", CreatedAt: from, UpdatedAt: from},
		{ID: "sale-before-to", TenantID: "tenant-1", ExternalID: "included-before-to", State: "COMPLETED", CreatedAt: to.Add(-time.Nanosecond), UpdatedAt: to},
		{ID: "sale-at-to", TenantID: "tenant-1", ExternalID: "excluded-at-to", State: "COMPLETED", CreatedAt: to, UpdatedAt: to},
	} {
		if err := repo.PutSale(sale); err != nil {
			t.Fatal(err)
		}
	}
	op, err := s.CreateExport(ExportRequest{Type: "SUPTO_18_1", From: from, To: to, Format: "JSON"}, "tenant-1")
	if err != nil {
		t.Fatal(err)
	}
	artifact, _, err := s.ExportArtifact(op.FiscalReference, "tenant-1")
	if err != nil || !bytes.Contains(artifact, []byte("included-from")) || !bytes.Contains(artifact, []byte("included-before-to")) || bytes.Contains(artifact, []byte("excluded-at-to")) || !bytes.Contains(artifact, []byte(`"interval_semantics":"[from,to)"`)) {
		t.Fatalf("canonical interval artifact invalid: %v %s", err, artifact)
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

func TestPeriodizedExportRejectsCurrencyRelabeling(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulator(true))
	sale := Sale{
		ID: "legacy-wrong-currency", TenantID: "tenant-1", ExternalID: "legacy",
		State: "COMPLETED", CreatedAt: bgEuroAdoption.Add(-time.Hour), UpdatedAt: bgEuroAdoption.Add(-time.Hour),
		Lines: []SaleLine{{LineID: "line-1", Name: "Legacy", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B"}},
	}
	if err := repo.PutSale(sale); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreatePeriodizedExport(ExportRequest{Type: "SUPTO_18_3", From: bgEuroAdoption.Add(-2 * time.Hour), To: bgEuroAdoption, Format: "JSON"}, "tenant-1"); err == nil {
		t.Fatal("historical EUR amounts were silently relabeled BGN")
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
