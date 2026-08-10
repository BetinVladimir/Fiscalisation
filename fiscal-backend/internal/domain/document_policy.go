package domain

import (
	"errors"
	"regexp"
	"strings"
)

// DocumentClass is selected by Fiscal Core. A POS may request a business
// action, but it cannot choose a legal document class or supply template code.
type DocumentClass string

const (
	DocumentFiscalReceipt  DocumentClass = "FISCAL_RECEIPT"
	DocumentFiscalReversal DocumentClass = "FISCAL_REVERSAL"
	DocumentServiceCashIn  DocumentClass = "SERVICE_CASH_IN"
	DocumentServiceCashOut DocumentClass = "SERVICE_CASH_OUT"
	DocumentOperational    DocumentClass = "OPERATIONAL_NON_FISCAL"
)

type DocumentPurpose string

const (
	PurposeCustomerSale DocumentPurpose = "CUSTOMER_SALE"
	PurposeReversal     DocumentPurpose = "CUSTOMER_REVERSAL"
	PurposeCashIn       DocumentPurpose = "CASH_IN"
	PurposeCashOut      DocumentPurpose = "CASH_OUT"
	PurposeInternal     DocumentPurpose = "INTERNAL_OPERATIONAL"
)

type ServerTemplate struct {
	ID, Version, Locale, Body string
	Class                     DocumentClass
}

var bannedNonFiscalWording = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(^|[^[:alpha:]])фискал(ен|на|но|ни)?([^[:alpha:]]|$)`),
	regexp.MustCompile(`(?i)(^|[^[:alpha:]])fiscal([[:space:]-]+receipt)?([^[:alpha:]]|$)`),
	regexp.MustCompile(`(?i)(^|[^[:alpha:]])касова[[:space:]]+бележка([^[:alpha:]]|$)`),
	regexp.MustCompile(`(?i)(^|[^[:alpha:]])фискален[[:space:]]+бон([^[:alpha:]]|$)`),
}

func DocumentClassFor(purpose DocumentPurpose) (DocumentClass, error) {
	switch purpose {
	case PurposeCustomerSale:
		return DocumentFiscalReceipt, nil
	case PurposeReversal:
		return DocumentFiscalReversal, nil
	case PurposeCashIn:
		return DocumentServiceCashIn, nil
	case PurposeCashOut:
		return DocumentServiceCashOut, nil
	case PurposeInternal:
		return DocumentOperational, nil
	default:
		return "", errors.New("unknown document purpose")
	}
}

func ValidateServerTemplate(template ServerTemplate) error {
	if strings.TrimSpace(template.ID) == "" || strings.TrimSpace(template.Version) == "" || template.Locale != "bg-BG" || strings.TrimSpace(template.Body) == "" {
		return errors.New("invalid server template identity")
	}
	if template.Class == DocumentOperational {
		for _, banned := range bannedNonFiscalWording {
			if banned.MatchString(template.Body) {
				return errors.New("non-fiscal template contains prohibited fiscal wording")
			}
		}
	}
	return nil
}

func ValidateCustomerDocumentClass(class DocumentClass) error {
	if class != DocumentFiscalReceipt && class != DocumentFiscalReversal {
		return errors.New("customer transaction requires a fiscal document")
	}
	return nil
}
