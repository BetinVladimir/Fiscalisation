package domain

import (
	"fmt"
	"testing"
	"time"
)

func TestSplitPaymentAndExactTotal(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	sale, e := s.CreateSale(CreateSale{ExternalID: "split", RegisterID: "r1", OperatorID: "A001"})
	if e != nil {
		t.Fatal(e)
	}
	sale, e = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "2.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if e != nil {
		t.Fatal(e)
	}
	op, e := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.00", Currency: "EUR"}})
	if e != nil || op.State != "PAYMENT_ACCEPTED" {
		t.Fatalf("%+v %v", op, e)
	}
	op, e = s.Pay(sale.ID, PaymentRequest{PaymentID: "p2", Type: "CASH", Amount: Money{Amount: "3.00", Currency: "EUR"}})
	if e != nil || op.State != "FISCALIZED" {
		t.Fatalf("%+v %v", op, e)
	}
	got, _ := s.GetSale(sale.ID)
	if got.State != "COMPLETED" || len(got.Payments) != 2 {
		t.Fatalf("%+v", got)
	}
}
func TestOverpaymentAndInvalidQuantityRejected(t *testing.T) {
	s := NewService(NewMemoryRepository(), NewSimulator(true))
	sale, _ := s.CreateSale(CreateSale{ExternalID: "over", RegisterID: "r1", OperatorID: "A001"})
	if _, e := s.AddLine(sale.ID, SaleLine{LineID: "bad", Name: "Item", Quantity: "0.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}); e == nil {
		t.Fatal("zero quantity")
	}
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "ok", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	if _, e := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.51", Currency: "EUR"}}); e == nil {
		t.Fatal("overpayment")
	}
}

func TestUnknownOutcomeRequiresReconciliationAndNoBlindRetry(t *testing.T) {
	driver := NewSimulator(true)
	driver.SetOutcomeUnknown(true)
	s := NewService(NewMemoryRepository(), driver)
	sale, _ := s.CreateSale(CreateSale{ExternalID: "unknown", RegisterID: "r1", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"})
	op, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}})
	if err != nil || op.State != "UNKNOWN" {
		t.Fatal(op, err)
	}
	if _, err = s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "2.50", Currency: "EUR"}}); err == nil {
		t.Fatal("unknown operation was blindly retried")
	}
	got, _ := s.GetSale(sale.ID)
	if got.State != "UNKNOWN" {
		t.Fatal(got)
	}
}

func TestLostFiscalDeviceBlocksBeforeOperation(t *testing.T) {
	r := NewMemoryRepository()
	s := NewService(r, NewSimulator(false))
	sale, _ := s.CreateSale(CreateSale{ExternalID: "offline", RegisterID: "r1", OperatorID: "A001"})
	sale, _ = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "1.00", Currency: "EUR"}, TaxGroup: "B"})
	if _, err := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: Money{Amount: "1.00", Currency: "EUR"}}); err == nil {
		t.Fatal("lost device accepted payment")
	}
	if len(r.Operations()) != 0 {
		t.Fatal("operation created before device reachability was established")
	}
}

func TestCardPaymentRequiresActiveRegisterTerminal(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, NewSimulatorWithCardTerminal(true, true))
	registerID, _ := prepareBLERegister(t, s, "tenant-card")
	newSale := func(externalID string) Sale {
		sale, err := s.CreateSale(CreateSale{TenantID: "tenant-card", ExternalID: externalID, RegisterID: registerID, OperatorID: "A001"})
		if err != nil {
			t.Fatal(err)
		}
		sale, err = s.AddLineExpectedForTenant(sale.ID, sale.Version, SaleLine{LineID: externalID + "-line", Name: "Item", Quantity: "1.000", UnitPrice: Money{Amount: "2.50", Currency: "EUR"}, TaxGroup: "B"}, "tenant-card")
		if err != nil {
			t.Fatal(err)
		}
		return sale
	}
	sale := newSale("card-without-terminal")
	if _, err := s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "card-1", Type: "CARD", TerminalPolicy: "AUTO_IF_AVAILABLE", Amount: Money{Amount: "2.50", Currency: "EUR"}}, "tenant-card"); err == nil {
		t.Fatal("card payment accepted without an active bound terminal")
	}
	if len(repo.Operations()) != 0 {
		t.Fatal("operation created before payment-terminal registry gate")
	}
	terminal, err := s.CreateResource("device", "tenant-card", map[string]any{"kind": "PAYMENT_TERMINAL", "vendor": "Simulator", "model": "CARD-STUB", "serial": "CARD-001", "status": "DRAFT", "environment": "DEV", "simulated": true})
	if err != nil {
		t.Fatal(err)
	}
	terminal = activateTestDevice(t, s, "tenant-card", terminal)
	if _, err = s.BindRegister(registerID, "tenant-card", terminal["id"].(string), "OPTIONAL_PAYMENT_TERMINAL", time.Now().UTC().Add(time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	sale = newSale("card-with-future-terminal")
	if _, err = s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "card-future", Type: "CARD", TerminalPolicy: "AUTO_IF_AVAILABLE", Amount: Money{Amount: "2.50", Currency: "EUR"}}, "tenant-card"); err == nil {
		t.Fatal("future-dated payment-terminal binding was treated as active")
	}
	if _, err = s.BindRegister(registerID, "tenant-card", terminal["id"].(string), "OPTIONAL_PAYMENT_TERMINAL", ""); err != nil {
		t.Fatal(err)
	}
	sale = newSale("card-with-terminal")
	op, err := s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "card-2", Type: "CARD", TerminalPolicy: "AUTO_IF_AVAILABLE", Amount: Money{Amount: "2.50", Currency: "EUR"}}, "tenant-card")
	if err != nil || op.State != "FISCALIZED" {
		t.Fatalf("active bound terminal did not enable card payment: %+v %v", op, err)
	}
	sale = newSale("card-none-policy")
	if _, err = s.PayForTenant(sale.ID, PaymentRequest{PaymentID: "card-3", Type: "CARD", TerminalPolicy: "NONE", Amount: Money{Amount: "2.50", Currency: "EUR"}}, "tenant-card"); err == nil {
		t.Fatal("CARD with terminal_policy=NONE was accepted")
	}
}

func TestEURTotalAndSplitPaymentPropertyMatrix(t *testing.T) {
	for i := 1; i <= 120; i++ {
		quantity := i%5 + 1
		unitCents := (i*37)%997 + 1
		totalCents := quantity * unitCents
		firstCents := (i * 13) % (totalCents + 1)
		secondCents := totalCents - firstCents
		money := func(cents int) Money {
			return Money{Amount: fmt.Sprintf("%d.%02d", cents/100, cents%100), Currency: "EUR"}
		}
		s := NewService(NewMemoryRepository(), NewSimulator(true))
		sale, err := s.CreateSale(CreateSale{ExternalID: fmt.Sprintf("property-%d", i), RegisterID: "r1", OperatorID: "A001"})
		if err != nil {
			t.Fatal(err)
		}
		sale, err = s.AddLine(sale.ID, SaleLine{LineID: "l1", Name: "Property item", Quantity: fmt.Sprintf("%d.000", quantity), UnitPrice: money(unitCents), TaxGroup: "B"})
		if err != nil {
			t.Fatalf("case %d line: %v", i, err)
		}
		if firstCents > 0 {
			op, payErr := s.Pay(sale.ID, PaymentRequest{PaymentID: "p1", Type: "CASH", Amount: money(firstCents)})
			if payErr != nil || (secondCents > 0 && op.State != "PAYMENT_ACCEPTED") {
				t.Fatalf("case %d first split: %+v %v", i, op, payErr)
			}
		}
		if secondCents > 0 {
			op, payErr := s.Pay(sale.ID, PaymentRequest{PaymentID: "p2", Type: "CASH", Amount: money(secondCents)})
			if payErr != nil || op.State != "FISCALIZED" {
				t.Fatalf("case %d final split: %+v %v", i, op, payErr)
			}
		}
		completed, err := s.GetSale(sale.ID)
		if err != nil || completed.State != "COMPLETED" {
			t.Fatalf("case %d did not complete exactly: %+v %v", i, completed, err)
		}
	}
}
