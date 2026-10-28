package domain

import (
	"errors"
	"time"
)

// MutateOrderLineForTenant retains source-order provenance before trusted fiscal
// preparation. The exact version, tenant and editable state are checked under
// the same lock as the mutation; a frozen/prepared order cannot be changed.
func (s *Service) MutateOrderLineForTenant(id, lineID string, expected int64, replacement *Line, tenant string) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[id]
	if !ok || tenant == "" || order.TenantID != tenant {
		return Order{}, errors.New("not found")
	}
	if expected < 1 || order.Version != expected {
		return Order{}, errors.New("version conflict")
	}
	if order.State != "DRAFT" && order.State != "OPEN" {
		return Order{}, errors.New("not editable")
	}
	if order.FiscalSaleID != "" || order.FiscalOperationID != "" || order.ExecutionAuthorizationID != "" {
		return Order{}, errors.New("prepared order immutable")
	}
	index := -1
	for i, line := range order.Lines {
		if line.LineID == lineID {
			if index >= 0 {
				return Order{}, errors.New("duplicate line identity")
			}
			index = i
		}
	}
	if index < 0 {
		return Order{}, errors.New("line not found")
	}
	next := append([]Line(nil), order.Lines...)
	if replacement == nil {
		next = append(next[:index], next[index+1:]...)
	} else {
		line := *replacement
		if line.LineID != lineID || !validUUID(line.LineID) || line.Name == "" || !validMoney(line.UnitPrice) || !validDiscount(line.Discount) || !validTax(line.TaxGroup) || !validQuantity(line.Quantity) {
			return Order{}, errors.New("invalid line")
		}
		if line.ProductID != "" {
			product, exists := s.products[line.ProductID]
			if !exists || product.TenantID != tenant {
				return Order{}, errors.New("product not found")
			}
		}
		next[index] = line
	}
	total, err := orderTotal(next)
	if err != nil {
		return Order{}, err
	}
	previous := order
	order.Lines = next
	order.Total = total
	order.Version++
	order.UpdatedAt = time.Now().UTC()
	if len(next) == 0 {
		order.State = "DRAFT"
	} else {
		order.State = "OPEN"
	}
	s.orders[id] = order
	if err = s.persistLocked(); err != nil {
		s.orders[id] = previous
		return Order{}, err
	}
	return order, nil
}
