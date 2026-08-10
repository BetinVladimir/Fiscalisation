package domain

import "testing"

func TestCustomerSaleCanNeverBecomeServiceBon(t *testing.T) {
	class, err := DocumentClassFor(PurposeCustomerSale)
	if err != nil || class != DocumentFiscalReceipt {
		t.Fatalf("customer sale class = %q, %v", class, err)
	}
	for _, forbidden := range []DocumentClass{DocumentServiceCashIn, DocumentServiceCashOut, DocumentOperational} {
		if ValidateCustomerDocumentClass(forbidden) == nil {
			t.Fatalf("customer sale accepted forbidden document class %q", forbidden)
		}
	}
}

func TestNonFiscalTemplateRejectsFiscalWording(t *testing.T) {
	for _, body := range []string{"Фискален бон № 1", "ФИСКАЛНА КАСОВА БЕЛЕЖКА", "Fiscal receipt"} {
		template := ServerTemplate{ID: "operational-note", Version: "1", Locale: "bg-BG", Class: DocumentOperational, Body: body}
		if ValidateServerTemplate(template) == nil {
			t.Fatalf("prohibited wording accepted: %q", body)
		}
	}
	if err := ValidateServerTemplate(ServerTemplate{ID: "kitchen-ticket", Version: "1", Locale: "bg-BG", Class: DocumentOperational, Body: "Кухненска поръчка — не е платежен документ"}); err != nil {
		t.Fatal(err)
	}
}

func TestDocumentPurposeMappingIsClosed(t *testing.T) {
	if _, err := DocumentClassFor("POS_SELECTED_CLASS"); err == nil {
		t.Fatal("unknown or POS-selected document class accepted")
	}
}
