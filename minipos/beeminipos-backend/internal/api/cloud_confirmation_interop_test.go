package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fiscalisation/beeminipos-backend/internal/domain"
	"fiscalisation/beeminipos-backend/internal/fiscalintegration"
)

// Opt-in only in generic Go suites; the dedicated runner requires Go and Node
// and sets this flag, so its advertised vertical gate cannot silently skip.
func TestNativeCloudConfirmationActualHTTP(t *testing.T) {
	if os.Getenv("MINIPOS_CLOUD_CONFIRM_INTEROP") != "1" {
		t.Skip("run scripts/test-native-cloud-confirmation.sh for mandatory Go/Node HTTP interop")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required", err)
	}
	for _, state := range []string{"FISCALIZED", "FAILED", "CANCELLED"} {
		t.Run(state, func(t *testing.T) {
			payments := []domain.OrderPayment{{PaymentID: "private-payment-one", Type: "CASH", Amount: domain.Money{Amount: "1.00", Currency: "EUR"}}, {PaymentID: "private-payment-two", Type: "CARD", Amount: domain.Money{Amount: "1.50", Currency: "EUR"}, TerminalPolicy: "REQUIRED"}}
			h, token, calls := recoveryHandler(t, func(op *fiscalintegration.FiscalOperation) {
				op.State = state
				if state != "FISCALIZED" {
					op.FiscalReference = ""
				}
			}, nil, func(o *domain.Order) {
				o.Version = 4
				o.ShiftID = "00000000-0000-4000-8000-000000000024"
				o.RegisterID = "source-register"
				o.FiscalRegisterID = "register"
				o.ExecutionAuthorizationID = "private-execution-authorization"
				o.Payments = payments
				o.Lines = []domain.Line{}
				o.Total = domain.Money{Amount: "2.50", Currency: "EUR"}
				o.CreatedAt = time.Now().UTC()
				o.UpdatedAt = o.CreatedAt
			})
			server := httptest.NewServer(h)
			defer server.Close()
			// Extra provider/card fields are not part of the normalized immutable plan.
			expected := map[string]any{"sourceOrderID": recoveryOrderID, "shiftID": "00000000-0000-4000-8000-000000000024", "sourceRegisterID": "source-register", "operatorCode": "operator", "externalID": "external-order", "sourceVersion": 4, "operationID": recoveryOperationID, "receiptSessionID": recoverySessionID, "fiscalSaleID": "sale", "fiscalRegisterID": "register", "total": domain.Money{Amount: "2.50", Currency: "EUR"}, "payments": []map[string]any{{"payment_id": "private-payment-one", "type": "CASH", "amount": map[string]string{"amount": "1.00", "currency": "EUR"}, "card_pan": "4111111111111111"}, {"payment_id": "private-payment-two", "type": "CARD", "amount": map[string]string{"amount": "1.50", "currency": "EUR"}, "terminal_policy": "REQUIRED", "terminal_reference": "terminal-secret"}}}
			body, _ := json.Marshal(expected)
			script, err := filepath.Abs("../../../BeeMiniPOS/scripts/cloud-confirmation-interop.mts")
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(node, "--experimental-strip-types", script)
			kind := "FAILED"
			if state == "FISCALIZED" {
				kind = state
			}
			cmd.Env = append(os.Environ(), "MINIPOS_CONFIRM_EXPECTED="+string(body), "MINIPOS_CONFIRM_URL="+server.URL+"/public/v1/minipos", "MINIPOS_CONFIRM_TOKEN="+token, "MINIPOS_CONFIRM_KIND="+kind)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("actual HTTP → native helper failed: %v\n%s", err, out)
			}
			if !strings.Contains(string(out), "actual HTTP → native confirmation PASS") {
				t.Fatal("interop runner did not assert success", string(out))
			}
			if *calls != 1 {
				t.Fatal("confirmation must read once without dispatch", *calls)
			}
		})
	}
}

func TestPublicOrderConfirmationRedactsPrivateFields(t *testing.T) {
	o := domain.Order{FiscalRegisterID: "root-register", CheckoutOperationID: "operation", ExecutionAuthorizationID: "private-execution-authorization", Payments: []domain.OrderPayment{{PaymentID: "private-payment-one", Type: "CASH", Amount: domain.Money{Amount: "1.00", Currency: "EUR"}}}}
	result := publicOrder(o)
	for _, key := range []string{"payments", "fiscal_register_id", "execution_authorization_id"} {
		if _, ok := result[key]; ok {
			t.Fatal("private field", key)
		}
	}
	proof, ok := result["checkout_confirmation"].(map[string]any)
	if !ok || len(proof) != 3 || proof["root_register_id"] != "root-register" || len(proof["payments_sha256"].(string)) != 64 {
		t.Fatal("invalid safe proof", proof)
	}
	encoded, _ := json.Marshal(result)
	for _, secret := range []string{"private-payment-one", "private-execution-authorization"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("private value leaked")
		}
	}
	o.Payments = nil
	if _, ok := publicOrder(o)["checkout_confirmation"]; ok {
		t.Fatal("unprepared order cannot claim payment proof")
	}
}
