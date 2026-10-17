package domain

import (
	"errors"
	"time"
)

type CheckoutLookup struct {
	OperationID, ReceiptSessionID, SaleID, RegisterID, State, FiscalReference string
	Version                                                                   int64
}

// The handler validates company/Fiscal-tenant and sale ownership using GETs.
// Recheck the local checkpoint under lock before binding or projecting proof.
func (s *Service) ApplyValidatedCheckoutLookup(tenant string, expected Order, proof CheckoutLookup) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.orders[expected.ID]
	if !ok || tenant == "" || current.TenantID != tenant || current.CheckoutOperationID != proof.OperationID || current.ReceiptSessionID != proof.ReceiptSessionID || current.RegisterID != proof.RegisterID || proof.SaleID == "" || proof.Version < 1 || (current.FiscalSaleID != "" && current.FiscalSaleID != proof.SaleID) {
		return Order{}, errors.New("checkout lookup association conflict")
	}
	if current.Version != expected.Version {
		return Order{}, errors.New("order changed during lookup")
	}
	if proof.State == "FISCALIZED" && proof.FiscalReference == "" {
		return Order{}, errors.New("checkout proof missing reference")
	}
	if current.State == "REVERSED" {
		return current, nil
	}
	if current.State == "COMPLETED" && current.ReceiptReference != "" {
		if proof.State == "FISCALIZED" && current.ReceiptReference != proof.FiscalReference {
			return Order{}, errors.New("conflicting checkout proof")
		}
		return current, nil
	}
	if proof.Version < current.FiscalVersion {
		return Order{}, errors.New("stale fiscal operation proof")
	}
	if proof.Version == current.FiscalVersion && current.FiscalVersion > 0 && proof.State == "FISCALIZED" && current.State != "COMPLETED" {
		return Order{}, errors.New("conflicting same-version checkout proof")
	}
	if proof.Version == current.FiscalVersion && current.FiscalSaleID == proof.SaleID {
		return current, nil
	}
	current.FiscalSaleID = proof.SaleID
	current.FiscalOperationID = proof.OperationID
	if proof.State == "FISCALIZED" {
		current.State = "COMPLETED"
		current.ReceiptReference = proof.FiscalReference
	}
	current.FiscalVersion = proof.Version
	current.Version++
	current.UpdatedAt = time.Now().UTC()
	s.orders[current.ID] = current
	if err := s.persistLocked(); err != nil {
		return Order{}, err
	}
	return current, nil
}
