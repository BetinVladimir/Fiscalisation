package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
	if kind == "device" && stringField(data, "status") != "DRAFT" {
		return nil, errors.New("new device must start in DRAFT")
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
	storedData := cloneMap(data)
	if kind == "register" {
		normalizeRegisterBindingTimes(nil, storedData, now)
	}
	v := ResourceRecord{Kind: kind, TenantID: tenant, ID: id, Version: 1, Data: storedData, CreatedAt: now, UpdatedAt: now}
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
	if kind == "device" && !validDeviceTransition(stringField(v.Data, "status"), stringField(data, "status")) {
		return nil, errors.New("invalid device status transition")
	}
	if err = s.validateResourceReferences(kind, tenant, data); err != nil {
		return nil, err
	}
	if err = s.ensureUniqueResource(kind, tenant, data, id); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	storedData := cloneMap(data)
	if kind == "register" {
		normalizeRegisterBindingTimes(v.Data, storedData, now)
	}
	v.Data, v.Version, v.UpdatedAt = storedData, v.Version+1, now
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
	sort.Slice(v, func(i, j int) bool {
		if v[i].CreatedAt.Equal(v[j].CreatedAt) {
			return v[i].ID < v[j].ID
		}
		return v[i].CreatedAt.Before(v[j].CreatedAt)
	})
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
			if stringField(v, "status") == "ACTIVE" && (stringField(v, "approved_type_evidence_id") == "" || stringField(v, "service_contract_evidence_id") == "") {
				return errors.New("active PROD device evidence required")
			}
			if stringField(v, "status") == "ACTIVE" && oneOf(stringField(v, "kind"), "FISCAL_DEVICE", "SMART_DEVICE") && (stringField(v, "fiscal_device_number") == "" || stringField(v, "fiscal_memory_number") == "") {
				return errors.New("active PROD fiscal identity required")
			}
		}
	default:
		return errors.New("unsupported resource kind")
	}
	return nil
}

func validDeviceTransition(from, to string) bool {
	if from == to {
		return true
	}
	allowed := map[string][]string{
		"DRAFT":                      {"PENDING_SERVICE_ACTIVATION", "BLOCKED", "RETIRED"},
		"PENDING_SERVICE_ACTIVATION": {"ACTIVE", "BLOCKED", "RETIRED"},
		"ACTIVE":                     {"BLOCKED", "RETIRED"},
		"BLOCKED":                    {"ACTIVE", "RETIRED"},
	}
	return oneOf(to, allowed[from]...)
}

func (s *Service) validateResourceReferences(kind, tenant string, v map[string]any) error {
	if kind == "register" {
		if _, err := s.GetResource("location", stringField(v, "location_id"), tenant); err != nil {
			return errors.New("location not found")
		}
		for _, binding := range []struct {
			field string
			kinds []string
		}{{"fiscal_device_id", []string{"FISCAL_DEVICE", "SMART_DEVICE"}}, {"payment_terminal_id", []string{"PAYMENT_TERMINAL", "SMART_DEVICE"}}} {
			id := stringField(v, binding.field)
			if id == "" {
				continue
			}
			device, err := s.repo.Resource("device", id)
			if err != nil || device.TenantID != tenant || stringField(device.Data, "status") != "ACTIVE" || !oneOf(stringField(device.Data, "kind"), binding.kinds...) {
				return errors.New("invalid active register device reference")
			}
		}
	}
	return nil
}

func normalizeRegisterBindingTimes(previous, next map[string]any, now time.Time) {
	for _, binding := range []struct{ id, active string }{{"fiscal_device_id", "fiscal_device_active_from"}, {"payment_terminal_id", "payment_terminal_active_from"}} {
		id := stringField(next, binding.id)
		if id == "" {
			delete(next, binding.active)
			continue
		}
		if previous != nil && id == stringField(previous, binding.id) && stringField(previous, binding.active) != "" {
			next[binding.active] = stringField(previous, binding.active)
			continue
		}
		next[binding.active] = now.UTC().Format(time.RFC3339Nano)
	}
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
	if stringField(register.Data, "status") != "ACTIVE" {
		return nil, errors.New("register is not active")
	}
	if stringField(device.Data, "status") != "ACTIVE" {
		return nil, errors.New("device is not active")
	}
	if role == "FISCAL_DEVICE" && stringField(device.Data, "kind") != "FISCAL_DEVICE" && stringField(device.Data, "kind") != "SMART_DEVICE" {
		return nil, errors.New("not a fiscal device")
	}
	if role == "OPTIONAL_PAYMENT_TERMINAL" && stringField(device.Data, "kind") != "PAYMENT_TERMINAL" && stringField(device.Data, "kind") != "SMART_DEVICE" {
		return nil, errors.New("not a payment terminal")
	}
	field := "fiscal_device_id"
	activeField := "fiscal_device_active_from"
	if role == "OPTIONAL_PAYMENT_TERMINAL" {
		field = "payment_terminal_id"
		activeField = "payment_terminal_active_from"
	}
	if activeFrom == "" {
		activeFrom = time.Now().UTC().Format(time.RFC3339Nano)
	}
	parsedActiveFrom, err := time.Parse(time.RFC3339, activeFrom)
	if err != nil {
		return nil, errors.New("invalid binding active_from")
	}
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	register.Data[field] = deviceID
	register.Data[activeField] = parsedActiveFrom.UTC().Format(time.RFC3339Nano)
	register.Version++
	register.UpdatedAt = time.Now().UTC()
	if err = s.repo.PutResource(register); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "register_id": registerID, "device_id": deviceID, "role": role, "version": int64(1), "active_from": parsedActiveFrom.UTC().Format(time.RFC3339Nano)}, nil
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
	device, err := s.repo.Resource("device", deviceID)
	if err != nil || device.TenantID != tenant {
		return nil, ErrNotFound
	}
	if !oneOf(stringField(device.Data, "status"), "DRAFT", "PENDING_SERVICE_ACTIVATION") {
		return nil, errors.New("device is not provisionable")
	}
	if stringField(device.Data, "environment") == "PROD" && stringField(device.Data, "approved_type_evidence_id") == "" {
		return nil, errors.New("approved type evidence required")
	}
	if _, err := s.GetResource("device", deviceID, tenant); err != nil {
		return nil, err
	}
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	data := map[string]any{"session_id": id, "device_id": deviceID, "expires_at": now.Add(15 * time.Minute), "state": "CREATED", "bootstrap_uri": "beefiscal://provision/" + id}
	if err = s.repo.PutResource(ResourceRecord{Kind: "provisioning_session", TenantID: tenant, ID: id, Version: 1, Data: cloneMap(data), CreatedAt: now, UpdatedAt: now}); err != nil {
		return nil, err
	}
	return data, nil
}

// DeviceActivationToken is a short-lived, audience-restricted JWT transported
// by an authenticated administrator over a physical-presence BLE session. The
// tenant is the organization authority and location_id is explicit; neither
// value may be supplied by the smart device itself.
func (s *Service) DeviceActivationToken(deviceID, locationID, appInstanceID, tenant string) (map[string]any, error) {
	if len(s.bleSigningKey) < 16 || locationID == "" || appInstanceID == "" {
		return nil, errors.New("activation signing unavailable or request invalid")
	}
	device, err := s.repo.Resource("device", deviceID)
	if err != nil || device.TenantID != tenant {
		return nil, ErrNotFound
	}
	location, err := s.repo.Resource("location", locationID)
	if err != nil || location.TenantID != tenant {
		return nil, ErrNotFound
	}
	if !oneOf(stringField(device.Data, "kind"), "SMART_DEVICE", "FISCAL_DEVICE") || !oneOf(stringField(device.Data, "status"), "DRAFT", "PENDING_SERVICE_ACTIVATION") {
		return nil, errors.New("device is not activation eligible")
	}
	now := time.Now().UTC()
	expires := now.Add(5 * time.Minute)
	jti, err := newUUID()
	if err != nil {
		return nil, err
	}
	header, _ := json.Marshal(map[string]any{"alg": "HS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iss": "beefiscal", "aud": "beefiscal-bluecash-activation", "sub": deviceID,
		"jti": jti, "iat": now.Unix(), "nbf": now.Add(-5 * time.Second).Unix(), "exp": expires.Unix(),
		"organization_id": tenant, "location_id": locationID, "device_id": deviceID,
		"app_instance_id": appInstanceID, "scope": "smart-device.activate",
	})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, s.bleSigningKey)
	_, _ = mac.Write([]byte(unsigned))
	token := unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return map[string]any{"activation_token": token, "token_type": "Bearer", "expires_at": expires, "organization_id": tenant, "location_id": locationID, "device_id": deviceID, "app_instance_id": appInstanceID}, nil
}
func (s *Service) DeviceDiagnostics(deviceID, tenant string) (map[string]any, error) {
	if _, err := s.GetResource("device", deviceID, tenant); err != nil {
		return nil, err
	}
	ready := s.driver != nil && s.driver.Probe() == nil
	return map[string]any{"device_id": deviceID, "observed_at": time.Now().UTC(), "metrics": map[string]any{"transport": map[string]any{"cloud_route": "READY"}, "fiscal_device": map[string]any{"reachable": ready}}, "redactions_applied": true}, nil
}
