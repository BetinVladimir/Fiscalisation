package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBGUNPV1GoldenVectors(t *testing.T) {
	path := filepath.Join("..", "..", "..", "contracts", "bg-unp-v1-golden.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var vectors struct {
		Valid []struct {
			FiscalDeviceNumber string `json:"fiscal_device_number"`
			OperatorCode       string `json:"operator_code"`
			Sequence           int64  `json:"sequence"`
			Value              string `json:"value"`
		} `json:"valid"`
		Invalid []string `json:"invalid"`
	}
	if err = json.Unmarshal(b, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, vector := range vectors.Valid {
		u, err := NewBGUNP(vector.FiscalDeviceNumber, vector.OperatorCode, vector.Sequence)
		if err != nil || u.String() != vector.Value {
			t.Fatalf("format %#v: %q %v", vector, u.String(), err)
		}
		parsed, err := ParseBGUNP(vector.Value)
		if err != nil || parsed != u {
			t.Fatalf("parse %q: %#v %v", vector.Value, parsed, err)
		}
		id := u.RegulatoryIdentifier()
		if id.Scheme != BGUNPV1 || id.CountryCode != "BG" || id.ProfileVersion == "" || id.Value != vector.Value {
			t.Fatalf("identifier: %#v", id)
		}
	}
	for _, value := range vectors.Invalid {
		if parsed, err := ParseBGUNP(value); err == nil {
			t.Fatalf("invalid vector accepted: %q => %#v", value, parsed)
		}
	}
}

func TestBGUNPBoundsAndProfile(t *testing.T) {
	for _, sequence := range []int64{-1, 0, BGUNPSequenceMax + 1} {
		if _, err := NewBGUNP("AB123456", "A001", sequence); err == nil {
			t.Fatalf("sequence accepted: %d", sequence)
		}
	}
	profile := DefaultBGFiscalProfile()
	if profile.CountryCode != "BG" || profile.Currency != "EUR" || profile.IdentifierScheme != BGUNPV1 {
		t.Fatalf("profile drift: %#v", profile)
	}
}
