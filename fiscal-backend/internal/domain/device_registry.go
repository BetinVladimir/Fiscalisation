package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const platformDeviceTenant = "__platform__"

var deviceTransitions = map[string]map[string]bool{
	"MANUFACTURED": {"ASSIGNED": true, "SUSPENDED": true, "RETIRED": true},
	"ASSIGNED":     {"MANUFACTURED": true, "DEPLOYED": true, "SUSPENDED": true, "RETIRED": true},
	"DEPLOYED":     {"ASSIGNED": true, "SUSPENDED": true, "RETIRED": true},
	"SUSPENDED":    {"MANUFACTURED": true, "ASSIGNED": true, "DEPLOYED": true, "RETIRED": true},
	"RETIRED":      {},
}

type ManufacturingDeviceInput struct {
	Serial                     string         `json:"serial"`
	DevicePublicKeyJWK         map[string]any `json:"device_public_key_jwk"`
	HardwareRevision           string         `json:"hardware_revision"`
	FirmwareVersion            string         `json:"firmware_version"`
	BootloaderVersion          string         `json:"bootloader_version,omitempty"`
	ManufacturingBatch         string         `json:"manufacturing_batch"`
	ManufacturingStationID     string         `json:"manufacturing_station_id"`
	FirmwareSHA256             string         `json:"firmware_sha256"`
	RegistrationEvidenceSHA256 string         `json:"registration_evidence_sha256"`
	Proof                      string         `json:"proof"`
}

func canonicalManufacturingProof(in ManufacturingDeviceInput, thumb string) []byte {
	v := strings.Join([]string{in.Serial, thumb, in.HardwareRevision, in.FirmwareVersion,
		in.ManufacturingBatch, in.ManufacturingStationID, in.FirmwareSHA256,
		in.RegistrationEvidenceSHA256}, "\n")
	return []byte(v)
}

func (s *Service) RegisterManufacturedDevice(in ManufacturingDeviceInput) (map[string]any, error) {
	if in.Serial == "" || in.HardwareRevision == "" || in.FirmwareVersion == "" ||
		in.ManufacturingBatch == "" || in.ManufacturingStationID == "" ||
		len(in.FirmwareSHA256) != 64 || len(in.RegistrationEvidenceSHA256) != 64 {
		return nil, errors.New("invalid manufacturing identity")
	}
	jwk, key, thumb, err := activationPublicKey(in.DevicePublicKeyJWK)
	if err != nil {
		return nil, err
	}
	proof, err := base64.RawURLEncoding.DecodeString(in.Proof)
	proofDigest := sha256.Sum256(canonicalManufacturingProof(in, thumb))
	if err != nil || !verifyP1363(key, proofDigest[:], proof) {
		return nil, errors.New("invalid device proof")
	}
	for _, existing := range s.repo.Resources("platform_device", platformDeviceTenant) {
		serial := stringField(existing.Data, "serial")
		existingThumb := stringField(existing.Data, "device_key_thumbprint")
		if serial == in.Serial && existingThumb == thumb {
			return publicResource(existing), nil
		}
		if serial == in.Serial || existingThumb == thumb {
			return nil, errors.New("serial or key conflict")
		}
	}
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	data := map[string]any{
		"serial": in.Serial, "device_public_key_jwk": jwk, "device_key_thumbprint": thumb,
		"hardware_revision": in.HardwareRevision, "firmware_version": in.FirmwareVersion,
		"bootloader_version": in.BootloaderVersion, "manufacturing_batch": in.ManufacturingBatch,
		"manufacturing_station_id": in.ManufacturingStationID, "manufactured_at": now,
		"firmware_sha256":              strings.ToLower(in.FirmwareSHA256),
		"registration_evidence_sha256": strings.ToLower(in.RegistrationEvidenceSHA256),
		"state":                        "MANUFACTURED", "binding_version": int64(0),
	}
	r := ResourceRecord{Kind: "platform_device", TenantID: platformDeviceTenant, ID: id,
		Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}
	if err = s.repo.PutResource(r); err != nil {
		return nil, err
	}
	return publicResource(r), nil
}

func (s *Service) PlatformDevices(state, tenant, serial string) []map[string]any {
	items := []map[string]any{}
	for _, r := range s.repo.Resources("platform_device", platformDeviceTenant) {
		if state != "" && stringField(r.Data, "state") != state {
			continue
		}
		if tenant != "" && stringField(r.Data, "tenant_id") != tenant {
			continue
		}
		if serial != "" && !strings.Contains(strings.ToLower(stringField(r.Data, "serial")), strings.ToLower(serial)) {
			continue
		}
		items = append(items, publicResource(r))
	}
	sort.Slice(items, func(i, j int) bool { return stringField(items[i], "serial") < stringField(items[j], "serial") })
	return items
}

func (s *Service) PlatformDevice(id string) (map[string]any, error) {
	r, err := s.repo.Resource("platform_device", id)
	if err != nil || r.TenantID != platformDeviceTenant {
		return nil, ErrNotFound
	}
	return publicResource(r), nil
}

func (s *Service) TransitionPlatformDevice(id, target, tenant, reason, actor string, expected int64) (map[string]any, error) {
	r, err := s.repo.Resource("platform_device", id)
	if err != nil || r.TenantID != platformDeviceTenant {
		return nil, ErrNotFound
	}
	if expected != r.Version {
		return nil, errors.New("version conflict")
	}
	current := stringField(r.Data, "state")
	if !deviceTransitions[current][target] {
		return nil, errors.New("invalid device transition")
	}
	if actor == "" {
		return nil, errors.New("actor required")
	}
	if target == "ASSIGNED" && tenant == "" {
		return nil, errors.New("tenant required")
	}
	if target == "RETIRED" && reason == "" {
		return nil, errors.New("retirement reason required")
	}
	if target == "MANUFACTURED" {
		delete(r.Data, "tenant_id")
	} else if tenant != "" {
		r.Data["tenant_id"] = tenant
	}
	r.Data["state"] = target
	r.Data["last_transition_reason"] = reason
	r.Data["binding_version"] = int64Field(r.Data, "binding_version") + 1
	r.Version++
	r.UpdatedAt = time.Now().UTC()
	if err = s.repo.PutResource(r); err != nil {
		return nil, err
	}
	return publicResource(r), nil
}

func int64Field(v map[string]any, key string) int64 {
	switch n := v[key].(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

type DeviceCapabilityRequest struct {
	InstallationID string         `json:"installation_id"`
	PublicKeyJWK   map[string]any `json:"public_key_jwk"`
	Permissions    []string       `json:"permissions"`
	TTLSeconds     int64          `json:"ttl_seconds"`
}

func (s *Service) IssueDeviceCapability(deviceID, tenant, subject string, in DeviceCapabilityRequest) (map[string]any, error) {
	device, err := s.repo.Resource("platform_device", deviceID)
	if err != nil || stringField(device.Data, "tenant_id") != tenant || stringField(device.Data, "state") == "SUSPENDED" || stringField(device.Data, "state") == "RETIRED" {
		return nil, ErrNotFound
	}
	if subject == "" || in.InstallationID == "" || in.TTLSeconds < 60 || in.TTLSeconds > 86400 {
		return nil, errors.New("invalid capability")
	}
	jwk, _, thumb, err := activationPublicKey(in.PublicKeyJWK)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{"wifi.set": true, "store.bind": true, "config.read": true, "config.write": true, "device.reboot": true, "diagnostics.read": true, "transaction.command": true, "transaction.status": true, "sync.request": true}
	seen := map[string]bool{}
	permissions := []string{}
	for _, p := range in.Permissions {
		if !allowed[p] || seen[p] {
			return nil, errors.New("invalid permission")
		}
		seen[p] = true
		permissions = append(permissions, p)
	}
	if len(permissions) == 0 {
		return nil, errors.New("permission required")
	}
	sort.Strings(permissions)
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	payload := map[string]any{"type": "device_capability", "version": 2, "capability_id": id, "device_id": deviceID, "tenant_id": tenant, "subject": subject, "installation_id": in.InstallationID, "public_key_jwk": jwk, "public_key_thumbprint": thumb, "permissions": permissions, "binding_version": int64Field(device.Data, "binding_version"), "issued_at": now.Unix(), "not_before": now.Unix(), "expires_at": now.Add(time.Duration(in.TTLSeconds) * time.Second).Unix()}
	canonical, _ := json.Marshal(payload)
	digest := sha256.Sum256(canonical)
	// DEV implementation signs with an ephemeral backend key. Production must replace this provider with KMS.
	private, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	signature, _ := signP1363(private, digest[:])
	pub := private.PublicKey
	payload["backend_public_key_jwk"] = map[string]any{"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(pub.X.FillBytes(make([]byte, 32))), "y": base64.RawURLEncoding.EncodeToString(pub.Y.FillBytes(make([]byte, 32)))}
	payload["signature"] = base64.RawURLEncoding.EncodeToString(signature)
	return payload, nil
}
