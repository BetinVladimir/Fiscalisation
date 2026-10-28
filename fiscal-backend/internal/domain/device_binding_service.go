package domain

import (
	"errors"
	"time"
)

// boundDriver revalidates the complete immutable route snapshot immediately
// before use. The configured driver is a transport dispatcher (MQTT/simulator),
// not a device choice; the selected device remains the snapshot carried in the
// command. Any rebind or identity mismatch is fenced without fallback.
func (s *Service) boundDriver(tenant, register string, expected FiscalDeviceSnapshot) (Driver, error) {
	d := s.driverFor(register)
	if d == nil {
		return nil, errors.New("device route unavailable")
	}
	if tenant == "" { // isolated unit/dev simulator path has no registry authority
		return d, nil
	}
	current, err := s.activeFiscalDeviceSnapshot(register, tenant, time.Now().UTC())
	if err != nil || expected.DeviceID == "" || current.DeviceID != expected.DeviceID ||
		current.BindingVersion != expected.BindingVersion || current.Serial != expected.Serial ||
		current.FiscalMemoryNumber != expected.FiscalMemoryNumber || current.Vendor != expected.Vendor ||
		current.Model != expected.Model {
		return nil, errors.New("stale or mismatched device route")
	}
	return d, nil
}

func (s *Service) registerHasActiveFiscalDevice(registerID, tenant string) bool {
	_, err := s.activeFiscalDeviceSnapshot(registerID, tenant, time.Now().UTC())
	return err == nil
}

func (s *Service) activeFiscalDeviceSnapshot(registerID, tenant string, now time.Time) (FiscalDeviceSnapshot, error) {
	register, err := s.repo.Resource("register", registerID)
	if err != nil || register.TenantID != tenant || stringField(register.Data, "status") != "ACTIVE" {
		return FiscalDeviceSnapshot{}, errors.New("register unavailable")
	}
	if !bindingIsActive(register.Data, "fiscal_device_active_from", now) {
		return FiscalDeviceSnapshot{}, errors.New("fiscal device binding inactive")
	}
	deviceID := stringField(register.Data, "fiscal_device_id")
	device, err := s.repo.Resource("device", deviceID)
	if err != nil || device.TenantID != tenant || stringField(device.Data, "status") != "ACTIVE" || !oneOf(stringField(device.Data, "kind"), "FISCAL_DEVICE", "SMART_DEVICE") {
		return FiscalDeviceSnapshot{}, errors.New("fiscal device unavailable")
	}
	adapterID := stringField(register.Data, "adapter_device_id")
	var generation int64
	if adapterID != "" {
		binding, bindingErr := s.repo.Resource("composite_binding", stringField(register.Data, "composite_binding_id"))
		if bindingErr != nil || binding.TenantID != tenant || stringField(binding.Data, "status") != "ACTIVE" ||
			stringField(binding.Data, "register_id") != registerID || stringField(binding.Data, "adapter_device_id") != adapterID ||
			stringField(binding.Data, "fiscal_device_id") != deviceID || int64Field(binding.Data, "generation") < 1 {
			return FiscalDeviceSnapshot{}, errors.New("composite binding unavailable")
		}
		adapter, adapterErr := s.repo.Resource("device", adapterID)
		if adapterErr != nil || adapter.TenantID != tenant || stringField(adapter.Data, "status") != "ACTIVE" {
			return FiscalDeviceSnapshot{}, errors.New("adapter unavailable")
		}
		generation = int64Field(binding.Data, "generation")
	}
	return FiscalDeviceSnapshot{AdapterDeviceID: adapterID, BindingGeneration: generation, DeviceID: device.ID, BindingVersion: register.Version, Serial: stringField(device.Data, "serial"), FiscalDeviceNumber: stringField(device.Data, "fiscal_device_number"), FiscalMemoryNumber: stringField(device.Data, "fiscal_memory_number"), Vendor: stringField(device.Data, "vendor"), Model: stringField(device.Data, "model"), Firmware: stringField(device.Data, "firmware")}, nil
}

func (s *Service) registerHasActivePaymentTerminal(registerID, tenant string) bool {
	register, err := s.repo.Resource("register", registerID)
	if err != nil || register.TenantID != tenant || stringField(register.Data, "status") != "ACTIVE" {
		return false
	}
	if !bindingIsActive(register.Data, "payment_terminal_active_from", time.Now().UTC()) {
		return false
	}
	terminalID := stringField(register.Data, "payment_terminal_id")
	terminal, err := s.repo.Resource("device", terminalID)
	if err != nil || terminal.TenantID != tenant || stringField(terminal.Data, "status") != "ACTIVE" {
		return false
	}
	return oneOf(stringField(terminal.Data, "kind"), "PAYMENT_TERMINAL", "SMART_DEVICE")
}

func bindingIsActive(data map[string]any, field string, now time.Time) bool {
	raw := stringField(data, field)
	if raw == "" { // Compatibility with bindings created before active_from persistence.
		return true
	}
	activeFrom, err := time.Parse(time.RFC3339, raw)
	return err == nil && !activeFrom.After(now)
}

func (s *Service) activeOperatorForTenant(code, tenant string, at time.Time) bool {
	for _, operator := range s.repo.Resources("operator", tenant) {
		if stringField(operator.Data, "code") != code {
			continue
		}
		activeFrom, err := time.Parse(time.RFC3339, stringField(operator.Data, "active_from"))
		if err != nil || activeFrom.After(at) {
			return false
		}
		activeTo := stringField(operator.Data, "active_to")
		if activeTo == "" {
			return true
		}
		until, err := time.Parse(time.RFC3339, activeTo)
		return err == nil && at.Before(until)
	}
	return false
}

func (s *Service) Connectivity(register, tenant string) (ConnectivityProbe, error) {
	now := time.Now().UTC()
	v := ConnectivityProbe{ProbeID: newID("probe"), TenantID: tenant, RegisterID: register, State: "SUCCEEDED", ObservedAt: now, RecommendedTransport: "REST", Hops: map[string]map[string]any{"public_api": {"state": "READY", "latency_ms": 0}, "fiscal_core": {"state": "READY", "latency_ms": 0}, "edge_runtime": {"state": "READY", "latency_ms": 0}, "driver": {"state": "READY", "latency_ms": 0}, "fiscal_device": {"state": "READY", "latency_ms": 0}}}
	registerRecord, registerErr := s.repo.Resource("register", register)
	deviceID := ""
	registryReady := registerErr == nil && registerRecord.TenantID == tenant && stringField(registerRecord.Data, "status") == "ACTIVE"
	if registryReady {
		deviceID = stringField(registerRecord.Data, "fiscal_device_id")
		registryReady = s.activeDeviceForTenant(deviceID, tenant)
	}
	if !registryReady {
		v.Hops["fiscal_core"] = map[string]any{"state": "BLOCKED", "latency_ms": 0}
	}
	if d := s.driverFor(register); !registryReady || d == nil || d.Probe() != nil {
		v.State, v.RecommendedTransport = "FAILED", "BLOCK"
		v.Hops["driver"] = map[string]any{"state": "UNAVAILABLE", "latency_ms": 0}
		v.Hops["fiscal_device"] = map[string]any{"state": "UNAVAILABLE", "latency_ms": 0, "device_id": deviceID}
	}
	return v, s.repo.PutConnectivityProbe(v)
}
func (s *Service) GetConnectivityProbe(id, tenant string) (ConnectivityProbe, error) {
	v, e := s.repo.ConnectivityProbe(id)
	if r, ok := s.repo.(interface {
		ConnectivityProbeForTenant(string, string) (ConnectivityProbe, error)
	}); ok {
		v, e = r.ConnectivityProbeForTenant(id, tenant)
	}
	if e != nil || (tenant != "" && v.TenantID != tenant) {
		return ConnectivityProbe{}, ErrNotFound
	}
	return v, nil
}

// ExpireDeviceCommand fail-closes an immutable command that could not be
// delivered while its signed authority was valid. It never creates a fresh
// envelope and never repeats a possible device side effect.
func (s *Service) ExpireDeviceCommand(operationID string) error {
	op, err := s.repo.Operation(operationID)
	if err != nil || op.State != "EXECUTING" {
		return err
	}
	now := time.Now().UTC()
	op.State = "UNKNOWN"
	op.ErrorCode = "DEVICE_COMMAND_EXPIRED_BEFORE_ACCEPTANCE"
	op.AllowedActions = []string{"RECONCILE"}
	op.Version++
	op.UpdatedAt = now
	if op.SaleID != "" {
		sale, saleErr := s.repo.Sale(op.SaleID)
		if saleErr != nil {
			return saleErr
		}
		sale.State = "UNKNOWN"
		sale.Version++
		sale.UpdatedAt = now
		return s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
	}
	return s.repo.CommitOperationEvent(op, fiscalCommandEvent(op.RegisterID, op))
}

func (s *Service) CompleteDeviceCommandForDevice(tenant, deviceID, id, status, fiscalReference, errorCode string) (Operation, error) {
	if tenant == "" || deviceID == "" || id == "" {
		return Operation{}, ErrNotFound
	}
	command, err := s.repo.Resource("device_command_outbox", id)
	if err != nil || command.Kind != "device_command_outbox" || command.ID != id || command.TenantID != tenant || stringField(command.Data, "device_id") != deviceID {
		return Operation{}, ErrNotFound
	}
	op, err := s.repo.Operation(id)
	if err != nil || op.ID != id || op.TenantID != tenant {
		return Operation{}, ErrNotFound
	}
	sale, err := s.repo.Sale(op.SaleID)
	if err != nil || sale.TenantID != tenant {
		return Operation{}, ErrNotFound
	}
	return s.CompleteDeviceCommand(id, status, fiscalReference, errorCode)
}

func (s *Service) CompleteDeviceCommand(id, status, fiscalReference, errorCode string) (Operation, error) {
	op, err := s.repo.Operation(id)
	if err != nil {
		return Operation{}, err
	}
	if op.State != "EXECUTING" {
		return op, nil
	}
	sale, err := s.repo.Sale(op.SaleID)
	if err != nil {
		return Operation{}, err
	}
	now := time.Now().UTC()
	op.Version++
	op.UpdatedAt = now
	if status == "SUCCEEDED" {
		if fiscalReference == "" {
			return Operation{}, errors.New("device result missing fiscal reference")
		}
		op.State, op.FiscalReference, op.ErrorCode = "FISCALIZED", fiscalReference, ""
		sale.State, sale.FiscalOperationID = "COMPLETED", op.ID
		for i := range sale.Payments {
			sale.Payments[i].FiscalReference = fiscalReference
		}
		op.FiscalDocumentAt = now
		sale.Version++
		sale.UpdatedAt = now
		return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
	}
	op.State, op.ErrorCode, op.AllowedActions = "UNKNOWN", errorCode, []string{"RECONCILE"}
	if op.ErrorCode == "" {
		op.ErrorCode = "DEVICE_COMMAND_FAILED"
	}
	sale.State = "UNKNOWN"
	sale.Version++
	sale.UpdatedAt = now
	return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
}
