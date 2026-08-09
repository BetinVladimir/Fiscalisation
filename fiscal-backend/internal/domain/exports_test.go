package domain

import (
	"archive/zip"
	"bytes"
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
