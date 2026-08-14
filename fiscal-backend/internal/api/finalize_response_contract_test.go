package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fiscalisation/fiscal-backend/internal/domain"
)

func TestAggregateFinalizeOperationMatchesRuntimeResponseContract(t *testing.T) {
	if pathMatches("/sales/{sale_id}:finalize", "/sales:finalize") || !pathMatches("/sales/{sale_id}:finalize", "/sales/sale-e2e:finalize") {
		t.Fatal("path matcher must enforce non-empty embedded path parameters")
	}
	op := domain.Operation{
		ID: "00000000-0000-4000-8000-000000000101", TenantID: "tenant-e2e",
		ClientOperationID: "00000000-0000-4000-8000-000000000101",
		ReceiptSessionID:  "00000000-0000-4000-8000-000000000102",
		SaleID:            "sale-e2e", RegisterID: "register-e2e", Type: "SALE_FINALIZE",
		State: "FISCALIZED", Version: 2, FiscalReference: "FD-000001",
		Simulated: true, AllowedActions: []string{}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	raw, err := json.Marshal(op)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err = json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	contract, found, statusFound := successContract("POST", "/public/v1/sales/sale-e2e:finalize", 202)
	if !found || !statusFound {
		t.Fatalf("finalize response contract missing")
	}
	if err = validateResponseSchema(contract.Schema, value); err != nil {
		t.Fatalf("aggregate response violates contract: %v; payload=%s", err, raw)
	}
}

func TestEmbeddedActionRequestSelectsSpecificContract(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/public/v1/device-activation-requests/11111111-1111-4111-8111-111111111111:confirm", strings.NewReader(`{"user_code":"ABCD-EFGH-IJKL","location_id":"l1","register_id":"r1","roles":["FISCAL_DEVICE"]}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Api-Version", "2026-08-07")
	r.Header.Set("Idempotency-Key", "activation-confirm-0001")
	path := "/device-activation-requests/11111111-1111-4111-8111-111111111111:confirm"
	var selected *requestContract
	best := -1
	for index := range generatedRequestContracts {
		candidate := &generatedRequestContracts[index]
		if candidate.Method == r.Method && pathMatches(candidate.Path, path) && pathSpecificity(candidate.Path) > best {
			selected, best = candidate, pathSpecificity(candidate.Path)
		}
	}
	if selected == nil || selected.Operation != "confirmDeviceActivationRequest" {
		t.Fatalf("selected=%+v", selected)
	}
	if err := validateGeneratedRequestParameters(selected.Parameters, selected.Path, path, r.URL.Query(), r.Header); err != nil {
		t.Fatal(err)
	}
	if err := validateOpenAPIRequest(r, path); err != nil {
		t.Fatal(err)
	}
}

func TestAggregateFinalizeContractRejectsNullAllowedActions(t *testing.T) {
	contract, _, _ := successContract("POST", "/public/v1/sales/sale-e2e:finalize", 202)
	value := map[string]any{"operation_id": "op", "type": "SALE_FINALIZE", "state": "FISCALIZED", "simulated": true, "allowed_actions": nil, "created_at": time.Now().UTC().Format(time.RFC3339Nano), "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}
	if err := validateResponseSchema(contract.Schema, value); err == nil {
		t.Fatal("null allowed_actions must fail closed")
	}
}
