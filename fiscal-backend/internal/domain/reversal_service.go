package domain

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (s *Service) Reverse(saleID, reason string) (Operation, error) {
	return s.reverseForTenant(saleID, reason, "", "")
}
func (s *Service) ReverseForTenant(saleID, reason, tenant string) (Operation, error) {
	return s.reverseForTenant(saleID, reason, "", tenant)
}
func (s *Service) ReverseForTenantWithReference(saleID, reason, originalReference, tenant string) (Operation, error) {
	return s.reverseForTenant(saleID, reason, originalReference, tenant)
}
func (s *Service) reverseForTenant(saleID, reason, originalReference, tenant string) (Operation, error) {
	sale, e := s.saleForTenantMutation(saleID, tenant)
	if e != nil {
		return Operation{}, e
	}
	reason = strings.TrimSpace(reason)
	if sale.State != "COMPLETED" || reason == "" || len(reason) > 64 || strings.ContainsAny(reason, "\r\n\t") || sale.FiscalOperationID == "" {
		return Operation{}, errors.New("reversal not allowed")
	}
	original, e := s.repo.Operation(sale.FiscalOperationID)
	if e != nil || original.FiscalReference == "" || (originalReference != "" && originalReference != original.FiscalReference) {
		return Operation{}, errors.New("original fiscal reference mismatch")
	}
	now := time.Now().UTC()
	if !reversalAllowed(reason, original.CreatedAt, now) {
		return Operation{}, errors.New("reversal reason or deadline not allowed")
	}
	if sale.TenantID != "" && !s.registerHasActiveFiscalDevice(sale.RegisterID, sale.TenantID) {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	driver, routeErr := s.boundDriver(sale.TenantID, sale.RegisterID, sale.FiscalDevice)
	if routeErr != nil || driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	documentNumber, parseErr := strconv.ParseInt(original.FiscalReference, 10, 64)
	if parseErr != nil || documentNumber < 1 || documentNumber > 9999999 || !regexp.MustCompile(`^[0-9]{8}$`).MatchString(sale.FiscalDevice.FiscalMemoryNumber) {
		return Operation{}, errors.New("original fiscal document metadata unavailable")
	}
	if original.FiscalDocumentAt.IsZero() {
		return Operation{}, errors.New("ORIGINAL_DOCUMENT_TIME_UNAVAILABLE")
	}
	op := Operation{ID: newID("op"), TenantID: sale.TenantID, SaleID: saleID, RegisterID: sale.RegisterID, Type: "REVERSAL", State: "EXECUTING", Version: 1, OriginalFiscalReference: original.FiscalReference, ReasonCode: reason, OriginalDocumentNumber: documentNumber, OriginalDocumentAt: original.FiscalDocumentAt, Simulated: driverIsSimulated(driver), AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	if queued, ok := driver.(durableQueuedDriver); ok {
		command, prepareErr := queued.Prepare(op, sale, PaymentRequest{})
		if prepareErr != nil {
			return Operation{}, prepareErr
		}
		sale, e = s.repo.ReserveSaleReversalCommand(saleID, tenant, sale.Version, op, command)
		if e != nil {
			return Operation{}, e
		}
		if e = queued.Publish(command); e != nil {
			return op, nil
		}
		return op, nil
	}
	sale, e = s.repo.ReserveSaleReversal(saleID, tenant, sale.Version, op)
	if e != nil {
		return Operation{}, e
	}
	ref, code := driver.Execute(op, sale, PaymentRequest{})
	op.Version++
	op.UpdatedAt = time.Now().UTC()
	if code == "FISCAL_RESULT_UNKNOWN" {
		op.State, op.ErrorCode, op.AllowedActions = "UNKNOWN", code, []string{"RECONCILE"}
		sale.State = "UNKNOWN"
		sale.Version++
		sale.UpdatedAt = op.UpdatedAt
		return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
	}
	if code != "" {
		op.State, op.ErrorCode = "FAILED", code
		sale.State = "COMPLETED"
		sale.Version++
		sale.UpdatedAt = op.UpdatedAt
		return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
	}
	op.State, op.FiscalReference, op.FiscalDocumentAt = "FISCALIZED", ref, op.UpdatedAt
	// The locked Fiscal Sale contract represents a successfully stornoed sale as
	// CANCELLED; the REVERSAL operation and webhook retain explicit storno semantics.
	sale.State = "CANCELLED"
	sale.ReversedAt = &op.UpdatedAt
	sale.Version++
	sale.UpdatedAt = now
	return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
}
func (s *Service) FiscalOperation(register, typ, tenant string) (Operation, error) {
	op, e := s.reserveFiscalOperation(register, typ, tenant)
	if e != nil {
		return Operation{}, e
	}
	device, deviceErr := s.activeFiscalDeviceSnapshot(register, tenant, time.Now().UTC())
	if deviceErr == nil {
		if driver, routeErr := s.boundDriver(tenant, register, device); routeErr == nil {
			if queued, ok := driver.(durableQueuedDriver); ok {
				command, prepareErr := queued.Prepare(op, Sale{TenantID: tenant, RegisterID: register, FiscalDevice: device}, PaymentRequest{})
				if prepareErr != nil {
					return Operation{}, prepareErr
				}
				if putErr := s.repo.PutResource(command); putErr != nil {
					return Operation{}, putErr
				}
				if publishErr := queued.Publish(command); publishErr != nil {
					return op, nil
				}
				return op, nil
			}
		}
	}
	op = s.executeReservedFiscalOperation(op, register, tenant)
	return op, s.repo.CommitOperationEvent(op, fiscalCommandEvent(register, op))
}
func (s *Service) reserveFiscalOperation(register, typ, tenant string) (Operation, error) {
	if register == "" {
		return Operation{}, errors.New("register required")
	}
	allowed := []string{"CASH_IN", "CASH_OUT", "X", "Z", "KLEN", "FISCAL_MEMORY", "OPERATOR", "DEPARTMENT", "PLU", "PRINTER_TEST", "DEVICE_IDENTITY", "DEVICE_TIME"}
	if !contains(allowed, typ) {
		return Operation{}, errors.New("unsupported operation")
	}
	if tenant != "" && !s.registerHasActiveFiscalDevice(register, tenant) {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	device := FiscalDeviceSnapshot{}
	if tenant != "" {
		var err error
		device, err = s.activeFiscalDeviceSnapshot(register, tenant, time.Now().UTC())
		if err != nil {
			return Operation{}, errors.New("fiscal device unavailable")
		}
	}
	driver, routeErr := s.boundDriver(tenant, register, device)
	if routeErr != nil || driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	now := time.Now().UTC()
	op := Operation{ID: newID("op"), TenantID: tenant, RegisterID: register, Type: typ, State: "EXECUTING", Version: 1, Simulated: driverIsSimulated(driver), AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.PutOperation(op); err != nil {
		return Operation{}, err
	}
	return op, nil
}
func (s *Service) executeReservedFiscalOperation(op Operation, register, tenant string) Operation {
	ref, code := s.driverFor(register).Execute(op, Sale{RegisterID: register, TenantID: tenant}, PaymentRequest{})
	op.Version++
	op.UpdatedAt = time.Now().UTC()
	if code == "FISCAL_RESULT_UNKNOWN" {
		op.State, op.ErrorCode, op.AllowedActions = "UNKNOWN", code, []string{"RECONCILE"}
	} else if code != "" {
		op.State, op.ErrorCode = "FAILED", code
	} else {
		op.State, op.FiscalReference, op.FiscalDocumentAt = "FISCALIZED", ref, op.UpdatedAt
	}
	return op
}
func (s *Service) CancelSale(id string) (Operation, error) {
	return s.cancelSaleForTenant(id, "")
}
func (s *Service) CancelSaleForTenant(id, tenant string) (Operation, error) {
	return s.cancelSaleForTenant(id, tenant)
}
func (s *Service) cancelSaleForTenant(id, tenant string) (Operation, error) {
	v, e := s.saleForTenantMutation(id, tenant)
	if e != nil {
		return Operation{}, e
	}
	if (v.State != "DRAFT" && v.State != "OPEN") || len(v.Payments) != 0 {
		return Operation{}, errors.New("sale cannot be cancelled")
	}
	driver, _ := s.boundDriver(v.TenantID, v.RegisterID, FiscalDeviceSnapshot{})
	now := time.Now().UTC()
	op := Operation{ID: newID("op"), TenantID: v.TenantID, SaleID: v.ID, RegisterID: v.RegisterID, Type: "CANCEL_SALE", State: "CANCELLED", Version: 1, Simulated: driverIsSimulated(driver), AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	v.State, v.UpdatedAt, v.Version = "CANCELLED", now, v.Version+1
	v.CancelledAt = &now
	return op, s.repo.CommitSaleOperation(v, op)
}
func (s *Service) ReconcileOperation(id string) (Operation, error) {
	return s.reconcileOperationForTenant(id, "")
}
func (s *Service) ReconcileOperationForTenant(id, tenant string) (Operation, error) {
	return s.reconcileOperationForTenant(id, tenant)
}
func (s *Service) reconcileOperationForTenant(id, tenant string) (Operation, error) {
	op, e := s.operationForTenantMutation(id, tenant)
	if e != nil {
		return op, e
	}
	if op.State != "UNKNOWN" {
		return op, errors.New("operation does not require reconciliation")
	}
	op.State, op.Version, op.UpdatedAt = "RECONCILING", op.Version+1, time.Now().UTC()
	op.AllowedActions = []string{}
	return op, s.repo.CommitOperationEvent(op, reconciliationRequiredEvent(op))
}
