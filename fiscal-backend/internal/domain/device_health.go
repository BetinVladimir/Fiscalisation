package domain

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

type DeviceEndpointHealth struct {
	Role            string     `json:"role"`
	Configured      bool       `json:"configured"`
	Reachable       bool       `json:"reachable"`
	State           string     `json:"state"`
	Vendor          string     `json:"vendor,omitempty"`
	Model           string     `json:"model,omitempty"`
	Serial          string     `json:"serial,omitempty"`
	ProtocolID      string     `json:"protocol_id,omitempty"`
	ProtocolVersion string     `json:"protocol_version,omitempty"`
	DriverID        string     `json:"driver_id,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	LastSuccessAt   *time.Time `json:"last_success_at,omitempty"`
}
type DeviceHealthStatus struct {
	SchemaVersion     int                    `json:"schema_version"`
	AdapterDeviceID   string                 `json:"adapter_device_id"`
	RegisterID        string                 `json:"register_id"`
	BootID            string                 `json:"boot_id"`
	Sequence          int64                  `json:"sequence"`
	BindingGeneration int64                  `json:"binding_generation"`
	FirmwareVersion   string                 `json:"firmware_version"`
	AdapterState      string                 `json:"adapter_state"`
	Endpoints         []DeviceEndpointHealth `json:"endpoints"`
	ObservedAt        string                 `json:"observed_at"`
}

func validHealthState(v string) bool {
	return oneOf(v, "READY", "DEGRADED", "OFFLINE", "MISCONFIGURED", "STALE")
}
func (s *Service) UpsertDeviceHealth(tenant, adapter string, status DeviceHealthStatus) (map[string]any, error) {
	observedAt, parseErr := time.Parse(time.RFC3339, status.ObservedAt)
	if parseErr != nil {
		if unix, err := strconv.ParseInt(status.ObservedAt, 10, 64); err == nil && unix > 0 {
			observedAt, parseErr = time.Unix(unix, 0).UTC(), nil
		}
	}
	if tenant == "" || adapter == "" || status.SchemaVersion != 1 || status.AdapterDeviceID != adapter || status.RegisterID == "" || status.BootID == "" || status.Sequence < 1 || status.BindingGeneration < 1 || parseErr != nil || !validHealthState(status.AdapterState) || len(status.Endpoints) == 0 {
		return nil, errors.New("invalid device health")
	}
	if _, err := s.GetResource("device", adapter, tenant); err != nil {
		return nil, err
	}
	for _, endpoint := range status.Endpoints {
		if !oneOf(endpoint.Role, "ADAPTER", "FISCAL_DEVICE", "PAYMENT_TERMINAL") || !validHealthState(endpoint.State) {
			return nil, errors.New("invalid endpoint health")
		}
	}
	now := time.Now().UTC()
	data := asMap(status)
	data["observed_at"] = observedAt
	data["received_at"] = now
	current, err := s.repo.Resource("device_health", adapter)
	if err == nil {
		if current.TenantID != tenant {
			return nil, ErrNotFound
		}
		oldBoot := stringField(current.Data, "boot_id")
		oldSequence := int64Field(current.Data, "sequence")
		if oldBoot == status.BootID && status.Sequence <= oldSequence {
			return publicResource(current), nil
		}
		current.Data = data
		current.Version++
		current.UpdatedAt = now
		if err = s.repo.PutResource(current); err != nil {
			return nil, err
		}
	} else {
		current = ResourceRecord{Kind: "device_health", TenantID: tenant, ID: adapter, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}
		if err = s.repo.PutResource(current); err != nil {
			return nil, err
		}
	}
	transitionID, uuidErr := newUUID()
	if uuidErr == nil {
		_ = s.repo.PutResource(ResourceRecord{Kind: "device_health_transition", TenantID: tenant, ID: transitionID, Version: 1, Data: map[string]any{"adapter_device_id": adapter, "state": status.AdapterState, "boot_id": status.BootID, "sequence": status.Sequence, "observed_at": observedAt}, CreatedAt: now, UpdatedAt: now})
	}
	return publicResource(current), nil
}
func (s *Service) DeviceHealth(adapter, tenant string) (map[string]any, error) {
	v, err := s.repo.Resource("device_health", adapter)
	if err != nil || v.TenantID != tenant {
		return nil, ErrNotFound
	}
	out := publicResource(v)
	received, _ := v.Data["received_at"].(time.Time)
	if received.IsZero() {
		if raw := stringField(v.Data, "received_at"); raw != "" {
			received, _ = time.Parse(time.RFC3339Nano, raw)
		}
	}
	age := time.Since(received)
	out["age_seconds"] = int64(age.Seconds())
	if age > 45*time.Second {
		out["effective_state"] = "STALE"
	} else {
		out["effective_state"] = strings.ToUpper(stringField(v.Data, "adapter_state"))
	}
	return out, nil
}
func (s *Service) DeviceActivity(adapter, tenant string) []map[string]any {
	all := s.ListResources("device_health_transition", tenant)
	out := make([]map[string]any, 0)
	for _, v := range all {
		if stringField(v, "adapter_device_id") == adapter {
			out = append(out, v)
		}
	}
	return out
}
