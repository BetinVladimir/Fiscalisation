package domain

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// FreezeExecutionOrder snapshots server-persisted lines and validates the chosen
// tenders before any downstream preparation. Retries cannot change the plan.
func (s *Service) FreezeExecutionOrder(id, shiftID, employee, tenant string, payments []map[string]any) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[id]
	if !ok || order.TenantID != tenant || order.ShiftID != shiftID {
		return Order{}, errors.New("caller order not found")
	}
	shift, ok := s.shifts[shiftID]
	if !ok || shift.TenantID != tenant || shift.EmployeeID != employee || shift.State != "OPEN" {
		return Order{}, errors.New("caller-owned open shift required")
	}
	if order.State != "OPEN" && order.State != "FISCAL_PENDING" {
		return Order{}, errors.New("order not payable")
	}
	if err := validateCheckoutPayments(order.Total, payments); err != nil {
		return Order{}, err
	}
	normalized := normalizedOrderPayments(payments)
	body, _ := json.Marshal(struct {
		Lines    []Line
		Payments []OrderPayment
		Total    Money
	}{order.Lines, normalized, order.Total})
	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	key := "execution-plan:" + tenant + ":" + id
	if old, exists := s.checkoutHashes[key]; exists {
		if old != digest {
			return Order{}, errors.New("immutable execution receipt plan conflict")
		}
		return order, nil
	}
	if order.State != "OPEN" {
		return Order{}, errors.New("order has another pending checkout")
	}
	previous := order
	order.Payments = normalized
	order.State = "FISCAL_PENDING"
	order.Version++
	order.UpdatedAt = time.Now().UTC()
	s.orders[id] = order
	s.checkoutHashes[key] = digest
	if err := s.persistLocked(); err != nil {
		s.orders[id] = previous
		delete(s.checkoutHashes, key)
		return Order{}, err
	}
	return order, nil
}
func (s *Service) AttachPreparedFiscalExecution(id, tenant, sale, operation, receipt, fiscalRegister, authorizationID string) (Order, error) {
	if fiscalRegister == "" || authorizationID == "" || sale == "" || !validUUID(operation) || !validUUID(receipt) {
		return Order{}, errors.New("trusted fiscal preparation identity required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[id]
	if !ok || order.TenantID != tenant || order.State != "FISCAL_PENDING" || len(order.Payments) == 0 {
		return Order{}, errors.New("frozen source order required")
	}
	if order.FiscalSaleID != "" && (order.FiscalSaleID != sale || order.FiscalOperationID != operation || order.ReceiptSessionID != receipt) {
		return Order{}, errors.New("prepared fiscal identity conflict")
	}
	if order.FiscalSaleID == sale && order.ExecutionAuthorizationID == authorizationID {
		return order, nil
	}
	previous := order
	order.FiscalRegisterID = fiscalRegister
	order.ExecutionAuthorizationID = authorizationID
	order.FiscalSaleID = sale
	order.FiscalOperationID = operation
	order.CheckoutOperationID = operation
	order.ReceiptSessionID = receipt
	order.Version++
	order.UpdatedAt = time.Now().UTC()
	s.orders[id] = order
	if err := s.persistLocked(); err != nil {
		s.orders[id] = previous
		return Order{}, err
	}
	return order, nil
}

func FiscalRegisterForOrder(order Order) string {
	if order.FiscalRegisterID != "" {
		return order.FiscalRegisterID
	}
	return order.RegisterID
}
