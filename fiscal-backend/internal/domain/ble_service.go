package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

func (s *Service) SetBLESigningKey(v string) { s.bleSigningKey = []byte(v) }

func (s *Service) BLESession(register, operator, app, tenant, actorSubject, clientPublicKey string) (map[string]any, error) {
	decodedPublicKey, publicKeyErr := base64.RawURLEncoding.DecodeString(clientPublicKey)
	if register == "" || operator == "" || app == "" || actorSubject == "" || publicKeyErr != nil || len(decodedPublicKey) != 32 || len(s.bleSigningKey) < 16 {
		return nil, errors.New("BLE session unavailable")
	}
	now := time.Now().UTC()
	device, deviceErr := s.activeFiscalDeviceSnapshot(register, tenant, now)
	registerResource, registerErr := s.repo.Resource("register", register)
	locationID := ""
	if registerErr == nil && registerResource.TenantID == tenant {
		locationID = stringField(registerResource.Data, "location_id")
	}
	if deviceErr != nil || locationID == "" || !s.activeOperatorResourceForTenant(operator, tenant, now) {
		return nil, errors.New("BLE session unavailable")
	}
	nonceBytes := make([]byte, 16)
	if _, e := rand.Read(nonceBytes); e != nil {
		return nil, e
	}
	v := BLESessionRecord{SessionID: newID("ble"), TenantID: tenant, LocationID: locationID, RegisterID: register, OperatorID: operator, AppInstanceID: app, ActorSubject: actorSubject, ClientPublicKey: clientPublicKey, DeviceID: register + "-edge", FiscalDeviceID: device.DeviceID, Scopes: []string{"fiscal.execute", "fiscal.read"}, FencingToken: now.UnixNano(), ExpiresAt: now.Add(8 * time.Hour), Nonce: base64.RawURLEncoding.EncodeToString(nonceBytes)}
	if e := s.repo.PutBLESession(v); e != nil {
		return nil, e
	}
	return s.bleResponse(v)
}
func (s *Service) RefreshBLE(id, tenant, actorSubject string) (map[string]any, error) {
	v, e := s.bleSessionForTenant(id, tenant)
	now := time.Now().UTC()
	currentDevice, deviceErr := s.activeFiscalDeviceSnapshot(v.RegisterID, v.TenantID, now)
	currentRegister, registerErr := s.repo.Resource("register", v.RegisterID)
	if e != nil || actorSubject == "" || v.ActorSubject == "" || v.ActorSubject != actorSubject || v.Revoked || (tenant != "" && v.TenantID != tenant) || !now.Before(v.ExpiresAt) || deviceErr != nil || currentDevice.DeviceID != v.FiscalDeviceID || registerErr != nil || currentRegister.TenantID != v.TenantID || stringField(currentRegister.Data, "location_id") != v.LocationID || !s.activeOperatorResourceForTenant(v.OperatorID, v.TenantID, now) {
		return nil, errors.New("BLE session inactive")
	}
	nonceBytes := make([]byte, 16)
	if _, e = rand.Read(nonceBytes); e != nil {
		return nil, e
	}
	nextToken := now.UnixNano()
	if nextToken <= v.FencingToken {
		nextToken = v.FencingToken + 1
	}
	v.FencingToken = nextToken
	v.Nonce = base64.RawURLEncoding.EncodeToString(nonceBytes)
	v.ExpiresAt = now.Add(8 * time.Hour)
	if e = s.repo.PutBLESession(v); e != nil {
		return nil, e
	}
	return s.bleResponse(v)
}
func (s *Service) RevokeBLE(id, tenant, actorSubject string) error {
	v, e := s.bleSessionForTenant(id, tenant)
	if e != nil || actorSubject == "" || v.ActorSubject == "" || v.ActorSubject != actorSubject || (tenant != "" && v.TenantID != tenant) {
		return ErrNotFound
	}
	v.Revoked = true
	now := time.Now().UTC()
	event := WebhookEvent{EventID: "event-ble-revoke-" + v.SessionID, EventType: "ble.session.revoked", APIVersion: "2026-08-07", TenantID: v.TenantID, ResourceID: v.SessionID, ResourceVersion: v.FencingToken, OccurredAt: now, Data: map[string]any{"ble_session_id": v.SessionID, "edge_id": v.DeviceID, "expires_at": v.ExpiresAt}}
	return s.repo.CommitBLESessionEvent(v, OutboxItem{ID: event.EventID, Event: event, NextAttempt: now})
}
func (s *Service) bleSessionForTenant(id, tenant string) (BLESessionRecord, error) {
	if r, ok := s.repo.(interface {
		BLESessionForTenant(string, string) (BLESessionRecord, error)
	}); ok {
		return r.BLESessionForTenant(id, tenant)
	}
	v, err := s.repo.BLESession(id)
	if err != nil || (tenant != "" && v.TenantID != tenant) {
		return BLESessionRecord{}, ErrNotFound
	}
	return v, nil
}
func (s *Service) bleResponse(v BLESessionRecord) (map[string]any, error) {
	ticket := struct {
		SessionID, TenantID, LocationID, RegisterID, DeviceID, FiscalDeviceID, AppInstanceID, ActorSubject, ClientPublicKey string
		Scopes                                                                                                              []string
		FencingToken                                                                                                        int64
		ExpiresAt                                                                                                           time.Time
		Nonce                                                                                                               string
	}{v.SessionID, v.TenantID, v.LocationID, v.RegisterID, v.DeviceID, v.FiscalDeviceID, v.AppInstanceID, v.ActorSubject, v.ClientPublicKey, v.Scopes, v.FencingToken, v.ExpiresAt, v.Nonce}
	b, e := json.Marshal(ticket)
	if e != nil {
		return nil, e
	}
	ticketKey := s.bleSigningKey
	if device, deviceErr := s.repo.Resource("device", v.FiscalDeviceID); deviceErr == nil {
		if credentialID, _ := device.Data["credential_id"].(string); credentialID != "" {
			ticketKey = DeriveDeviceTransportKey(s.bleSigningKey, "ble-ticket", v.FiscalDeviceID, credentialID)
		}
	}
	m := hmac.New(sha256.New, ticketKey)
	m.Write(b)
	wrapped, e := json.Marshal(struct {
		Payload   string `json:"payload"`
		Signature string `json:"signature"`
	}{base64.RawURLEncoding.EncodeToString(b), base64.RawURLEncoding.EncodeToString(m.Sum(nil))})
	if e != nil {
		return nil, e
	}
	raw := base64.RawURLEncoding.EncodeToString(wrapped)
	return map[string]any{
		"ble_session_id": v.SessionID, "tenant_id": v.TenantID, "location_id": v.LocationID, "register_id": v.RegisterID,
		"edge_id": v.DeviceID, "device_id": v.FiscalDeviceID, "binding_version": v.FencingToken,
		"operator_id": v.OperatorID, "app_instance_id": v.AppInstanceID,
		"protocol_version": "2026-08-07", "expires_at": v.ExpiresAt, "scopes": v.Scopes,
		"security_mode":               "OPEN_MVP",
		"service_uuid":                "7b6f0000-7c6d-4c7a-9e4f-424545464953",
		"command_characteristic_uuid": "7b6f0002-7c6d-4c7a-9e4f-424545464953",
		"event_characteristic_uuid":   "7b6f0003-7c6d-4c7a-9e4f-424545464953",
		"advertising_identity":        v.DeviceID,
		"signed_session_ticket":       raw,
	}, nil
}
