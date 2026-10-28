package domain

import (
	"errors"
	"testing"
)

func TestEReceiptUnsupported(t *testing.T) {
	svc := NewService(NewMemoryRepository(), &errorCodeDriver{code: "IRRELEVANT"})
	err := svc.SendEReceipt(EReceiptRequest{SaleID: "sale-1", TenantID: "t1", Email: "user@example.com"})
	if err == nil {
		t.Fatal("expected ERECEIPT_UNSUPPORTED error, got nil")
	}
	var eErr *EReceiptError
	if !errors.As(err, &eErr) {
		t.Fatalf("expected *EReceiptError, got %T: %v", err, err)
	}
	if eErr.Code != "ERECEIPT_UNSUPPORTED" {
		t.Fatalf("expected code ERECEIPT_UNSUPPORTED, got %q", eErr.Code)
	}
}
