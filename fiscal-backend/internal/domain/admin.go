package domain

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func publicResource(v ResourceRecord) map[string]any {
	out := make(map[string]any, len(v.Data)+4)
	for k, x := range v.Data {
		out[k] = x
	}
	out["id"], out["version"], out["created_at"], out["updated_at"] = v.ID, v.Version, v.CreatedAt, v.UpdatedAt
	return out
}

func (s *Service) CreateResource(kind, tenant string, data map[string]any) (map[string]any, error) {
	if tenant == "" {
		return nil, errors.New("tenant required")
	}
	if err := validateResource(kind, data); err != nil {
		return nil, err
	}
	if err := s.validateResourceReferences(kind, tenant, data); err != nil {
		return nil, err
	}
	if err := s.ensureUniqueResource(kind, tenant, data, ""); err != nil {
		return nil, err
	}
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	v := ResourceRecord{Kind: kind, TenantID: tenant, ID: id, Version: 1, Data: cloneMap(data), CreatedAt: now, UpdatedAt: now}
	if err = s.repo.PutResource(v); err != nil {
		return nil, err
	}
	return publicResource(v), nil
}

func (s *Service) UpdateResource(kind, id, tenant string, expected int64, data map[string]any) (map[string]any, error) {
	v, err := s.repo.Resource(kind, id)
	if err != nil || v.TenantID != tenant {
		return nil, ErrNotFound
	}
	if expected < 1 || v.Version != expected {
		return nil, errors.New("version conflict")
	}
	if err = validateResource(kind, data); err != nil {
		return nil, err
	}
	if err = s.validateResourceReferences(kind, tenant, data); err != nil {
		return nil, err
	}
	if err = s.ensureUniqueResource(kind, tenant, data, id); err != nil {
		return nil, err
	}
	v.Data, v.Version, v.UpdatedAt = cloneMap(data), v.Version+1, time.Now().UTC()
	if err = s.repo.PutResource(v); err != nil {
		return nil, err
	}
	return publicResource(v), nil
}

func (s *Service) GetResource(kind, id, tenant string) (map[string]any, error) {
	v, err := s.repo.Resource(kind, id)
	if err != nil || (tenant != "" && v.TenantID != tenant) {
		return nil, ErrNotFound
	}
	return publicResource(v), nil
}

func (s *Service) ListResources(kind, tenant string) []map[string]any {
	v := s.repo.Resources(kind, tenant)
	sort.Slice(v, func(i, j int) bool { return v[i].CreatedAt.Before(v[j].CreatedAt) })
	out := make([]map[string]any, 0, len(v))
	for _, x := range v {
		out = append(out, publicResource(x))
	}
	return out
}

func (s *Service) Organization(tenant string) (map[string]any, error) {
	v := s.ListResources("organization", tenant)
	if len(v) == 0 {
		return nil, ErrNotFound
	}
	return v[0], nil
}

func (s *Service) UpsertOrganization(tenant string, expected int64, data map[string]any) (map[string]any, error) {
	v := s.repo.Resources("organization", tenant)
	if len(v) == 0 {
		if expected != 0 && expected != 1 {
			return nil, errors.New("version conflict")
		}
		return s.CreateResource("organization", tenant, data)
	}
	return s.UpdateResource("organization", v[0].ID, tenant, expected, data)
}

func cloneMap(v map[string]any) map[string]any {
	out := make(map[string]any, len(v))
	for k, x := range v {
		out[k] = x
	}
	return out
}
func stringField(v map[string]any, k string) string {
	x, _ := v[k].(string)
	return strings.TrimSpace(x)
}
func oneOf(v string, allowed ...string) bool {
	for _, x := range allowed {
		if v == x {
			return true
		}
	}
	return false
}

func validateResource(kind string, v map[string]any) error {
	switch kind {
	case "organization":
		if stringField(v, "legal_name") == "" || stringField(v, "eik") == "" || stringField(v, "country") != "BG" || !oneOf(stringField(v, "status"), "ACTIVE", "SUSPENDED") {
			return errors.New("invalid organization")
		}
	case "location":
		if stringField(v, "code") == "" || stringField(v, "name") == "" || stringField(v, "address") == "" || !oneOf(stringField(v, "status"), "ACTIVE", "INACTIVE") {
			return errors.New("invalid location")
		}
	case "register":
		if stringField(v, "location_id") == "" || stringField(v, "code") == "" || !oneOf(stringField(v, "status"), "ACTIVE", "BLOCKED", "INACTIVE") {
			return errors.New("invalid register")
		}
	case "operator":
		code := stringField(v, "code")
		if len(code) != 4 || stringField(v, "first_name") == "" || stringField(v, "last_name") == "" || stringField(v, "active_from") == "" {
			return errors.New("invalid operator")
		}
		for _, c := range code {
			if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
				return errors.New("invalid operator code")
			}
		}
		roles, ok := v["roles"].([]any)
		if !ok || len(roles) == 0 {
			return errors.New("operator roles required")
		}
		for _, role := range roles {
			x, ok := role.(string)
			if !ok || !oneOf(x, "CASHIER", "SUPERVISOR", "ADMIN", "AUDITOR", "SERVICE") {
				return errors.New("invalid operator role")
			}
		}
	case "device":
		if !oneOf(stringField(v, "kind"), "FISCAL_DEVICE", "PAYMENT_TERMINAL", "EDGE", "SMART_DEVICE") || stringField(v, "vendor") == "" || stringField(v, "model") == "" || stringField(v, "serial") == "" || !oneOf(stringField(v, "status"), "DRAFT", "PENDING_SERVICE_ACTIVATION", "ACTIVE", "BLOCKED", "RETIRED") || !oneOf(stringField(v, "environment"), "DEV", "PROD") {
			return errors.New("invalid device")
		}
		if stringField(v, "environment") == "PROD" {
			if simulated, _ := v["simulated"].(bool); simulated {
				return errors.New("simulated PROD device forbidden")
			}
		}
	default:
		return errors.New("unsupported resource kind")
	}
	return nil
}

func (s *Service) validateResourceReferences(kind, tenant string, v map[string]any) error {
	if kind == "register" {
		if _, err := s.GetResource("location", stringField(v, "location_id"), tenant); err != nil {
			return errors.New("location not found")
		}
	}
	return nil
}
func (s *Service) ensureUniqueResource(kind, tenant string, data map[string]any, except string) error {
	key := map[string]string{"location": "code", "register": "code", "operator": "code", "device": "serial"}[kind]
	if key == "" {
		return nil
	}
	for _, x := range s.repo.Resources(kind, tenant) {
		if x.ID != except && strings.EqualFold(stringField(x.Data, key), stringField(data, key)) {
			return errors.New("duplicate " + key)
		}
	}
	return nil
}

func (s *Service) BindRegister(registerID, tenant, deviceID, role, activeFrom string) (map[string]any, error) {
	register, err := s.repo.Resource("register", registerID)
	if err != nil || register.TenantID != tenant {
		return nil, ErrNotFound
	}
	device, err := s.repo.Resource("device", deviceID)
	if err != nil || device.TenantID != tenant {
		return nil, errors.New("device not found")
	}
	if !oneOf(role, "FISCAL_DEVICE", "OPTIONAL_PAYMENT_TERMINAL") {
		return nil, errors.New("invalid binding role")
	}
	if role == "FISCAL_DEVICE" && stringField(device.Data, "kind") != "FISCAL_DEVICE" && stringField(device.Data, "kind") != "SMART_DEVICE" {
		return nil, errors.New("not a fiscal device")
	}
	if role == "OPTIONAL_PAYMENT_TERMINAL" && stringField(device.Data, "kind") != "PAYMENT_TERMINAL" && stringField(device.Data, "kind") != "SMART_DEVICE" {
		return nil, errors.New("not a payment terminal")
	}
	field := "fiscal_device_id"
	if role == "OPTIONAL_PAYMENT_TERMINAL" {
		field = "payment_terminal_id"
	}
	register.Data[field] = deviceID
	register.Version++
	register.UpdatedAt = time.Now().UTC()
	if err = s.repo.PutResource(register); err != nil {
		return nil, err
	}
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	if activeFrom == "" {
		activeFrom = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return map[string]any{"id": id, "register_id": registerID, "device_id": deviceID, "role": role, "version": int64(1), "active_from": activeFrom}, nil
}

func (s *Service) DeviceCapabilities(deviceID, tenant string) (map[string]any, error) {
	v, err := s.repo.Resource("device", deviceID)
	if err != nil || v.TenantID != tenant {
		return nil, ErrNotFound
	}
	evidence := "DOCUMENTED"
	if simulated, _ := v.Data["simulated"].(bool); simulated {
		evidence = "STUB"
	}
	caps := map[string]string{"fiscal.sale": "SUPPORTED", "fiscal.reversal": "SUPPORTED", "report.x": "SUPPORTED", "report.z": "SUPPORTED", "report.klen": "SUPPORTED", "report.fiscal_memory": "SUPPORTED"}
	if stringField(v.Data, "kind") == "PAYMENT_TERMINAL" {
		caps = map[string]string{"payment.card": "UNVERIFIED"}
	}
	return map[string]any{"device_id": deviceID, "driver_version": "mvp-0.1.0", "protocol_version": "2026-08-07", "capabilities": caps, "evidence_state": evidence}, nil
}
func (s *Service) ProvisioningSession(deviceID, tenant string) (map[string]any, error) {
	if _, err := s.GetResource("device", deviceID, tenant); err != nil {
		return nil, err
	}
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	return map[string]any{"session_id": id, "device_id": deviceID, "expires_at": time.Now().UTC().Add(15 * time.Minute), "state": "CREATED", "bootstrap_uri": "beefiscal://provision/" + id}, nil
}
func (s *Service) DeviceDiagnostics(deviceID, tenant string) (map[string]any, error) {
	if _, err := s.GetResource("device", deviceID, tenant); err != nil {
		return nil, err
	}
	ready := s.driver != nil && s.driver.Probe() == nil
	return map[string]any{"device_id": deviceID, "observed_at": time.Now().UTC(), "metrics": map[string]any{"transport": map[string]any{"cloud_route": "READY"}, "fiscal_device": map[string]any{"reachable": ready}}, "redactions_applied": true}, nil
}
