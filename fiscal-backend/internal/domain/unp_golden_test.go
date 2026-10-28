package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type unpGoldenFile struct {
	SchemaVersion  int    `json:"schema_version"`
	Scheme         string `json:"scheme"`
	ProfileVersion string `json:"profile_version"`
	Valid          []struct {
		FiscalDeviceNumber string `json:"fiscal_device_number"`
		OperatorCode       string `json:"operator_code"`
		Sequence           int64  `json:"sequence"`
		Value              string `json:"value"`
	} `json:"valid"`
	Invalid []string `json:"invalid"`
}

func loadUNPGolden(t *testing.T) unpGoldenFile {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	// Walk up to the repo root (fiscal-backend/internal/domain → ../../../ = Fiscalisation)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..")
	golden := filepath.Join(root, "contracts", "bg-unp-v1-golden.json")
	data, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("cannot read golden file %s: %v", golden, err)
	}
	var g unpGoldenFile
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatalf("cannot parse golden file: %v", err)
	}
	return g
}

func TestUNPGolden(t *testing.T) {
	g := loadUNPGolden(t)
	if len(g.Valid) == 0 {
		t.Fatal("golden file has no valid entries")
	}
	if len(g.Invalid) == 0 {
		t.Fatal("golden file has no invalid entries")
	}
	for _, v := range g.Valid {
		t.Run("valid/"+v.Value, func(t *testing.T) {
			parsed, err := ParseBGUNP(v.Value)
			if err != nil {
				t.Errorf("ParseBGUNP(%q) returned error: %v", v.Value, err)
				return
			}
			if parsed.FiscalDeviceNumber != v.FiscalDeviceNumber {
				t.Errorf("FiscalDeviceNumber: got %q, want %q", parsed.FiscalDeviceNumber, v.FiscalDeviceNumber)
			}
			if parsed.OperatorCode != v.OperatorCode {
				t.Errorf("OperatorCode: got %q, want %q", parsed.OperatorCode, v.OperatorCode)
			}
			if parsed.Sequence != v.Sequence {
				t.Errorf("Sequence: got %d, want %d", parsed.Sequence, v.Sequence)
			}
			if got := parsed.String(); got != v.Value {
				t.Errorf("String(): got %q, want %q", got, v.Value)
			}
		})
	}
	for _, inv := range g.Invalid {
		inv := inv
		t.Run("invalid/"+inv, func(t *testing.T) {
			_, err := ParseBGUNP(inv)
			if err == nil {
				t.Errorf("ParseBGUNP(%q) should have returned error but did not", inv)
			}
		})
	}
}
