package domain

import (
	"errors"
	"reflect"
)

func (r *MemoryRepository) ReserveSaleFinalize(next Sale, expected int64, op Operation, command *ResourceRecord) (Sale, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.persistenceReadyLocked(); err != nil {
		return *new(Sale), err
	}
	old, ok := r.sales[next.ID]
	if !ok || old.TenantID != next.TenantID {
		return Sale{}, ErrNotFound
	}
	if old.Version != expected || old.State != "OPEN" || next.Version != expected || len(next.Payments) == 0 || !reflect.DeepEqual(old.Lines, next.Lines) || old.UNP != next.UNP || old.RegisterID != next.RegisterID || old.OperatorID != next.OperatorID || op.ID == "" || op.ClientOperationID != op.ID || op.ReceiptSessionID == "" || op.SaleID != next.ID || op.TenantID != next.TenantID || op.RegisterID != next.RegisterID || op.Type != "SALE_FINALIZE" || op.State != "EXECUTING" {
		return Sale{}, errors.New("receipt finalize state conflict")
	}
	if _, exists := r.operations[op.ID]; exists {
		return Sale{}, errors.New("receipt operation identity already reserved")
	}
	if next.TenantID != "" && next.FiscalDevice.DeviceID == "" {
		return Sale{}, errors.New("fiscal snapshot required")
	}
	if command != nil && (command.Kind != "device_command_outbox" || command.ID != op.ID || command.TenantID != next.TenantID || command.Version != 1 || command.Data == nil) {
		return Sale{}, errors.New("invalid prepared receipt command")
	}
	next.State = "PAYMENT_PENDING"
	next.FiscalOperationID = op.ID
	next.Version++
	next.UpdatedAt = op.CreatedAt
	r.sales[next.ID] = next
	r.operations[op.ID] = op
	if command != nil {
		r.resources[resourceKey(command.Kind, command.ID)] = *command
	}
	r.appendAuditLocked(next.TenantID, next.OperatorID, "PAYMENT_INTENT_CREATED", "sale", next.ID, next.UNP, asMap(old), asMap(next))
	if err := r.persistLocked(); err != nil {
		return Sale{}, err
	}
	return next, nil
}
