package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMoneyRoundingGolden runs all 34 vectors from rounding_golden.json
// against lineTotalCents / discountedLineTotalCents.
// Vectors without discount test LineTotal directly; vectors with discount
// use discountedLineTotalCents to apply the fractional discount.
// The test prints "ROUND OK <N>" on success.
func TestMoneyRoundingGolden(t *testing.T) {
	root := findRepoRoot(t)
	goldenPath := filepath.Join(root, "docs", "MVP1", "fixtures", "rounding_golden.json")
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("cannot read rounding_golden.json: %v", err)
	}
	var vectors []struct {
		Name        string `json:"name"`
		UnitCents   int64  `json:"unit_cents"`
		QtyMilli    int64  `json:"qty_milli"`
		DiscountCts int64  `json:"discount_cents"`
		Expect      int64  `json:"expect_line_cents"`
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}

	passed := 0
	for _, v := range vectors {
		v := v
		t.Run("golden_"+v.Name, func(t *testing.T) {
			var got int64
			if v.DiscountCts == 0 {
				c, e := LineTotal(Cents(v.UnitCents), v.QtyMilli)
				if e != nil {
					t.Fatalf("LineTotal error: %v", e)
				}
				got = int64(c)
			} else {
				// Build a SaleLine with discount and use discountedLineTotalCents.
				qty := fmt.Sprintf("%d.%03d", v.QtyMilli/1000, v.QtyMilli%1000)
				unit := FormatMoney(Cents(v.UnitCents))
				disc := FormatMoney(Cents(v.DiscountCts))
				line := SaleLine{
					Quantity:  qty,
					UnitPrice: Money{Amount: unit, Currency: "EUR"},
					Discount:  &Money{Amount: disc, Currency: "EUR"},
				}
				c, e := discountedLineTotalCents(line)
				if e != nil {
					t.Fatalf("discountedLineTotalCents error: %v", e)
				}
				got = c
			}
			if got != v.Expect {
				t.Errorf("vector %s: got %d, want %d", v.Name, got, v.Expect)
			} else {
				passed++
			}
		})
	}
	if passed == len(vectors) {
		t.Logf("ROUND OK %d", len(vectors))
	}
}
