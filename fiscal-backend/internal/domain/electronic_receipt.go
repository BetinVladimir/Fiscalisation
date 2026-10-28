package domain

// EReceiptRequest holds the parameters for an electronic receipt delivery request.
type EReceiptRequest struct {
	SaleID   string
	TenantID string
	Email    string
}

// EReceiptError is returned for any e-receipt request; electronic receipts are
// permanently disabled by owner decision — no delivery implementation exists.
type EReceiptError struct {
	Code string
}

func (e *EReceiptError) Error() string { return e.Code }

// SendEReceipt rejects every e-receipt delivery attempt with UNSUPPORTED.
// Electronic receipts are disabled by owner decision; delivery must not be implemented.
func (s *Service) SendEReceipt(_ EReceiptRequest) error {
	return &EReceiptError{Code: "ERECEIPT_UNSUPPORTED"}
}
