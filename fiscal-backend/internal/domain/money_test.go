package domain

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// TestParseMoney validates ParseMoney per ARCH-004.2 acceptance criteria
// and BUG-001 test vectors.
func TestParseMoney(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		// Valid cases
		{"0.00", 0, false},
		{"0.01", 1, false},
		{"12.34", 1234, false},
		{"99999.99", 9999999, false},
		{"1.50", 150, false},
		{"0.50", 50, false},
		{"1.00", 100, false},
		{"1.50", 150, false},
		{"01.00", 100, false}, // leading zero – allowed

		// Invalid: three decimal digits → "12.345" should error
		{"12.345", 0, true},
		{"1.5", 0, true},
		{"1.500", 0, true},

		// Invalid: signed (BUG-001)
		{"-1.50", 0, true},
		{"-0.50", 0, true},
		{"+1.00", 0, true},

		// Invalid: malformed
		{"1.-5", 0, true},
		{"", 0, true},
		{"1e2.00", 0, true},
		{" 1.00", 0, true},
		{"1.000", 0, true},
		{".00", 0, true},
		{"1.", 0, true},
		{".", 0, true},

		// Invalid: non-numeric
		{"abc", 0, true},
		{"12.3a", 0, true},
		{"ab.cd", 0, true},

		// Invalid: integer part overflow (>7 digits)
		{"12345678.90", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseMoney(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("ParseMoney(%q) error = %v, wantErr = %v", tc.input, err, tc.wantErr)
				return
			}
			if !tc.wantErr && got != Cents(tc.want) {
				t.Errorf("ParseMoney(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// FuzzParseMoney checks roundtrip consistency: FormatMoney(ParseMoney(s)) == s.
func FuzzParseMoney(f *testing.F) {
	// Add some seed corpus
	for _, seed := range []string{"0.00", "0.01", "12.34", "1.50", "99999.99", "1.00"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) == 0 || len(input) > 20 {
			return
		}
		cents, err := ParseMoney(input)
		if err != nil {
			// Invalid input is ok for fuzzing
			return
		}
		// Round-trip check
		formatted := FormatMoney(cents)
		if formatted != input {
			t.Errorf("round-trip failed: ParseMoney(%q)→%d→FormatMoney→%q (expected %q)", input, cents, formatted, input)
			return
		}
		// Second parse should also succeed
		_, err2 := ParseMoney(formatted)
		if err2 != nil {
			t.Errorf("second ParseMoney failed on formatted output: %v", err2)
		}
	})
}

// TestLineTotal validates LineTotal per ARCH-004.2 acceptance criteria.
func TestLineTotal(t *testing.T) {
	tests := []struct {
		name     string
		price    Cents
		qtyMilli int64
		want     int64
		wantErr  bool
	}{
		{"332 * 5000 = 1660", 332, 5000, 1660, false},
		{"half-up: 999 * 333/1000 = 333", 999, 333, 333, false},
		{"zero price", 0, 100, 0, false},
		{"qty_milli=1, price=1 → 0", 1, 1, 0, false},
		{"qty_milli=500, price=1 → 1 (half-rounds up)", 1, 500, 1, false},
		{"qty_milli=499, price=1 → 0 (half-rounds down)", 1, 499, 0, false},
		{"qty_milli=501, price=1 → 1 (501 rounds to 1)", 1, 501, 1, false},
		{"qty_milli=1000, price=100 → 100", 100, 1000, 100, false},
		{"qty_milli=3000, price=100 → 300", 100, 3000, 300, false},

		// Overflow cases — values chosen so p*q actually overflows int64
		{"overflow int64", Cents(math.MaxInt64), 100, 0, true},
		{"overflow: qty nearly maxint", Cents(100), math.MaxInt64/100 + 1, 0, true},
		{"overflow: price*2 > maxint", Cents(math.MaxInt64/2 + 1), 2, 0, true},
		{"negative price", -1, 100, 0, true},
		{"zero qty", 100, 0, 0, true},
		{"negative qty", 100, -1, 0, true},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s(qty_milli=%d)", tc.name, tc.qtyMilli), func(t *testing.T) {
			got, err := LineTotal(tc.price, tc.qtyMilli)
			if (err != nil) != tc.wantErr {
				t.Errorf("LineTotal(%d, %d) error = %v, wantErr = %v", tc.price, tc.qtyMilli, err, tc.wantErr)
				return
			}
			if !tc.wantErr && got != Cents(tc.want) {
				t.Errorf("LineTotal(%d, %d) = %d, want %d", tc.price, tc.qtyMilli, got, tc.want)
			}
		})
	}
}

// TestFormatMoney validates FormatMoney per acceptance criteria.
func TestFormatMoney(t *testing.T) {
	tests := []struct {
		cents  int64
		output string
	}{
		{0, "0.00"},
		{1, "0.01"},
		{50, "0.50"},
		{100, "1.00"},
		{1234, "12.34"},
		{123456, "1234.56"},
		{9999999, "99999.99"},
		{-1, "-0.01"},
		{-50, "-0.50"},
		{-100, "-1.00"},
		{-1234, "-12.34"},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("cents_%d", tc.cents), func(t *testing.T) {
			got := FormatMoney(Cents(tc.cents))
			if got != tc.output {
				t.Errorf("FormatMoney(%d) = %q, want %q", tc.cents, got, tc.output)
			}
		})
	}
}

// TestLineTotalFromGoldenFile loads rounding_golden.json and validates every
// vector via LineTotal. This is the golden integration for ARCH-004.1.
func TestLineTotalFromGoldenFile(t *testing.T) {
	// Locate rounding_golden.json. Search current dir and known paths.
	root := findRepoRoot(t)
	goldenPath := filepath.Join(root, "docs", "MVP1", "fixtures", "rounding_golden.json")

	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("cannot read rounding_golden.json: %v", err)
	}

	var vectors []struct {
		Name        string `json:"name"`
		UnitCents   int    `json:"unit_cents"`
		QtyMilli    int64  `json:"qty_milli"`
		DiscountCts int    `json:"discount_cents"`
		Expect      int    `json:"expect_line_cents"`
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}

	for _, v := range vectors {
		name := fmt.Sprintf("golden_%s", v.Name)
		t.Run(name, func(t *testing.T) {
			var got Cents
			if v.DiscountCts == 0 {
				c, err := LineTotal(Cents(v.UnitCents), v.QtyMilli)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				got = c
			} else {
				qty := fmt.Sprintf("%d.%03d", v.QtyMilli/1000, v.QtyMilli%1000)
				unit := FormatMoney(Cents(v.UnitCents))
				disc := FormatMoney(Cents(v.DiscountCts))
				line := SaleLine{
					Quantity:  qty,
					UnitPrice: Money{Amount: unit, Currency: "EUR"},
					Discount:  &Money{Amount: disc, Currency: "EUR"},
				}
				c, err := discountedLineTotalCents(line)
				if err != nil {
					t.Fatalf("discountedLineTotalCents error: %v", err)
				}
				got = Cents(c)
			}
			want := Cents(v.Expect)
			if got != want {
				t.Errorf("LineTotal(%d, %d) = %d, want %d", v.UnitCents, v.QtyMilli, got, want)
			}
		})
	}
}

func findRepoRoot(t *testing.T) string {
	// Walk up to find the topmost go.mod (indicates repo root).
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("cannot get working dir")
	}
	var found string
	for depth := 0; depth < 20; depth++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			found = dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if found == "" {
		t.Fatal("go.mod not found walking up")
	}
	return found
}
