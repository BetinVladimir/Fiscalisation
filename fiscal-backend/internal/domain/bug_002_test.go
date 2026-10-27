package domain

import (
	"encoding/json"
	"os"
	"testing"
)

func TestValidMoney(t *testing.T) {
	type entry struct {
		Name     string `json:"name"`
		Currency string `json:"currency"`
		Amount   string `json:"amount"`
		Want     bool   `json:"want"`
	}
	data, err := os.ReadFile("../../../docs/MVP1/fixes/golden_money_validation.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var cases []entry
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			got := validMoney(Money{Currency: c.Currency, Amount: c.Amount})
			if got != c.Want {
				t.Errorf("validMoney({%q, %q}) = %v, want %v", c.Currency, c.Amount, got, c.Want)
			}
		})
	}
}
