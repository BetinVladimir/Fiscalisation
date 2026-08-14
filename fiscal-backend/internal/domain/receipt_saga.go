package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
)

type ReceiptSagaState string

const (
	ReceiptReserved             ReceiptSagaState = "RESERVED"
	ReceiptCardAuthorizing      ReceiptSagaState = "CARD_AUTHORIZING"
	ReceiptCardApproved         ReceiptSagaState = "CARD_APPROVED"
	ReceiptFiscalOpening        ReceiptSagaState = "FISCAL_OPENING"
	ReceiptFiscalOpen           ReceiptSagaState = "FISCAL_OPEN"
	ReceiptLinesRegistering     ReceiptSagaState = "LINES_REGISTERING"
	ReceiptPaymentsRegistering  ReceiptSagaState = "PAYMENTS_REGISTERING"
	ReceiptFiscalClosing        ReceiptSagaState = "FISCAL_CLOSING"
	ReceiptCommitted            ReceiptSagaState = "COMMITTED"
	ReceiptCompensationRequired ReceiptSagaState = "COMPENSATION_REQUIRED"
	ReceiptCardReversing        ReceiptSagaState = "CARD_REVERSING"
	ReceiptCompensated          ReceiptSagaState = "COMPENSATED"
	ReceiptRecoveryRequired     ReceiptSagaState = "RECOVERY_REQUIRED"
)

type ReceiptFinalizePlan struct {
	ClientOperationID string           `json:"client_operation_id"`
	ReceiptSessionID  string           `json:"receipt_session_id"`
	SaleID            string           `json:"sale_id"`
	UNP               string           `json:"unp"`
	Items             []SaleLine       `json:"items"`
	Payments          []PaymentRequest `json:"payments"`
	ExpectedTotal     Money            `json:"expected_total"`
}
type ReceiptSaga struct {
	Plan       ReceiptFinalizePlan `json:"plan"`
	PlanSHA256 string              `json:"plan_sha256"`
	State      ReceiptSagaState    `json:"state"`
	Version    int64               `json:"version"`
}

type ReceiptSagaStore interface {
	ReserveReceiptSaga(string, ReceiptSaga) (ReceiptSaga, bool, error)
	AdvanceReceiptSaga(string, string, ReceiptSagaState, ReceiptSagaState) (ReceiptSaga, error)
}

type MemoryReceiptSagaStore struct {
	mu   sync.Mutex
	rows map[string]ReceiptSaga
}

func NewMemoryReceiptSagaStore() *MemoryReceiptSagaStore {
	return &MemoryReceiptSagaStore{rows: map[string]ReceiptSaga{}}
}
func receiptSagaKey(tenant, operation string) string { return tenant + "\x00" + operation }
func ReceiptPlanHash(v ReceiptFinalizePlan) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func ValidateReceiptPlan(v ReceiptFinalizePlan) error {
	if !uuidPattern.MatchString(v.ClientOperationID) || !uuidPattern.MatchString(v.ReceiptSessionID) || v.SaleID == "" || v.UNP == "" || len(v.Items) == 0 || len(v.Payments) == 0 || v.ExpectedTotal.Currency != "EUR" {
		return errors.New("invalid receipt finalize plan")
	}
	var total int64
	for _, p := range v.Payments {
		if !uuidPattern.MatchString(p.PaymentID) || p.Amount.Currency != "EUR" {
			return errors.New("invalid payment plan")
		}
		cents, e := receiptMoneyCents(p.Amount.Amount)
		if e != nil || cents <= 0 {
			return errors.New("invalid payment amount")
		}
		total += cents
	}
	expected, e := receiptMoneyCents(v.ExpectedTotal.Amount)
	if e != nil || expected != total {
		return errors.New("payment total mismatch")
	}
	return nil
}
func receiptMoneyCents(v string) (int64, error) {
	parts := strings.Split(v, ".")
	if len(parts) != 2 || len(parts[1]) != 2 {
		return 0, errors.New("invalid money")
	}
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil {
		return 0, e
	}
	fraction, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil {
		return 0, e
	}
	return whole*100 + fraction, nil
}
func (s *MemoryReceiptSagaStore) ReserveReceiptSaga(tenant string, v ReceiptSaga) (ReceiptSaga, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tenant == "" || ValidateReceiptPlan(v.Plan) != nil {
		return ReceiptSaga{}, false, errors.New("invalid receipt saga")
	}
	v.PlanSHA256 = ReceiptPlanHash(v.Plan)
	k := receiptSagaKey(tenant, v.Plan.ClientOperationID)
	if old, ok := s.rows[k]; ok {
		if old.PlanSHA256 != v.PlanSHA256 {
			return ReceiptSaga{}, false, errors.New("IDEMPOTENCY_PAYLOAD_CONFLICT")
		}
		return old, false, nil
	}
	v.State = ReceiptReserved
	v.Version = 1
	s.rows[k] = v
	return v, true, nil
}
func (s *MemoryReceiptSagaStore) AdvanceReceiptSaga(tenant, operation string, from, to ReceiptSagaState) (ReceiptSaga, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := receiptSagaKey(tenant, operation)
	v, ok := s.rows[k]
	if !ok || v.State != from || !allowedReceiptTransition(from, to) {
		return ReceiptSaga{}, errors.New("invalid receipt saga transition")
	}
	v.State = to
	v.Version++
	s.rows[k] = v
	return v, nil
}
func allowedReceiptTransition(a, b ReceiptSagaState) bool {
	allowed := map[ReceiptSagaState][]ReceiptSagaState{ReceiptReserved: {ReceiptCardAuthorizing, ReceiptFiscalOpening, ReceiptCompensationRequired}, ReceiptCardAuthorizing: {ReceiptCardApproved, ReceiptCompensationRequired, ReceiptRecoveryRequired}, ReceiptCardApproved: {ReceiptFiscalOpening, ReceiptCompensationRequired}, ReceiptFiscalOpening: {ReceiptFiscalOpen, ReceiptCompensationRequired, ReceiptRecoveryRequired}, ReceiptFiscalOpen: {ReceiptLinesRegistering, ReceiptCompensationRequired}, ReceiptLinesRegistering: {ReceiptPaymentsRegistering, ReceiptCompensationRequired}, ReceiptPaymentsRegistering: {ReceiptFiscalClosing, ReceiptCompensationRequired}, ReceiptFiscalClosing: {ReceiptCommitted, ReceiptCompensationRequired, ReceiptRecoveryRequired}, ReceiptCompensationRequired: {ReceiptCardReversing, ReceiptCompensated, ReceiptRecoveryRequired}, ReceiptCardReversing: {ReceiptCompensated, ReceiptRecoveryRequired}}
	for _, v := range allowed[a] {
		if v == b {
			return true
		}
	}
	return false
}
