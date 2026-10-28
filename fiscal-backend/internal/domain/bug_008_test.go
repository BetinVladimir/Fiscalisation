package domain

import (
	"testing"
	"time"
)

// TestReverseUsesFiscalDocumentTimestamp verifies BUG-008:
// the reversal command must use FiscalDocumentAt (the fiscal unit's timestamp),
// not UpdatedAt which changes on reconcile.
func TestReverseUsesFiscalDocumentTimestamp(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, newFakeDriver())

	now := time.Now().UTC()
	fiscalDocumentAt := now.Add(-2 * time.Hour)
	updatedAt := now.Add(-1 * time.Hour) // later — simulates a reconcile bump

	repo.operations["op-orig"] = Operation{
		ID:               "op-orig",
		TenantID:         "",
		RegisterID:       "FD00000001",
		Type:             "SALE_FINALIZE",
		State:            "FISCALIZED",
		Version:          1,
		FiscalReference:  "0000001",
		FiscalDocumentAt: fiscalDocumentAt,
		AllowedActions:   []string{},
		CreatedAt:        now.Add(-3 * time.Hour),
		UpdatedAt:        updatedAt,
	}
	repo.sales["sale-1"] = Sale{
		ID:                "sale-1",
		TenantID:          "",
		RegisterID:        "FD00000001",
		State:             "COMPLETED",
		Version:           1,
		FiscalOperationID: "op-orig",
		FiscalDevice:      FiscalDeviceSnapshot{FiscalMemoryNumber: "12345678"},
		CreatedAt:         now.Add(-3 * time.Hour),
		UpdatedAt:         updatedAt,
	}

	reversal, err := s.ReverseForTenant("sale-1", "CUSTOMER_RETURN", "")
	if err != nil {
		t.Fatalf("ReverseForTenant: %v", err)
	}
	if !reversal.OriginalDocumentAt.Equal(fiscalDocumentAt) {
		t.Errorf("OriginalDocumentAt = %v, want FiscalDocumentAt %v (must not be UpdatedAt %v)",
			reversal.OriginalDocumentAt, fiscalDocumentAt, updatedAt)
	}
}

// TestReverseFailsWhenFiscalDocumentAtMissing verifies that a reversal is
// rejected if the original operation has no FiscalDocumentAt recorded.
func TestReverseFailsWhenFiscalDocumentAtMissing(t *testing.T) {
	repo := NewMemoryRepository()
	s := NewService(repo, newFakeDriver())

	now := time.Now().UTC()

	repo.operations["op-no-ts"] = Operation{
		ID:              "op-no-ts",
		TenantID:        "",
		RegisterID:      "FD00000001",
		Type:            "SALE_FINALIZE",
		State:           "FISCALIZED",
		Version:         1,
		FiscalReference: "0000001",
		// FiscalDocumentAt intentionally zero
		AllowedActions: []string{},
		CreatedAt:      now.Add(-3 * time.Hour),
		UpdatedAt:      now.Add(-2 * time.Hour),
	}
	repo.sales["sale-2"] = Sale{
		ID:                "sale-2",
		TenantID:          "",
		RegisterID:        "FD00000001",
		State:             "COMPLETED",
		Version:           1,
		FiscalOperationID: "op-no-ts",
		FiscalDevice:      FiscalDeviceSnapshot{FiscalMemoryNumber: "12345678"},
		CreatedAt:         now.Add(-3 * time.Hour),
		UpdatedAt:         now.Add(-2 * time.Hour),
	}

	_, err := s.ReverseForTenant("sale-2", "CUSTOMER_RETURN", "")
	if err == nil || err.Error() != "ORIGINAL_DOCUMENT_TIME_UNAVAILABLE" {
		t.Errorf("want error ORIGINAL_DOCUMENT_TIME_UNAVAILABLE, got %v", err)
	}
}
