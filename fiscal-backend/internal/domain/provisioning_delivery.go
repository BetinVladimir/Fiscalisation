package domain

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fiscalisation/fiscal-backend/internal/provisioningseal"
	"fmt"
	"math/big"
	"strings"
	"time"
)

type ProvisioningDeliveryInput struct {
	BindingID      string `json:"binding_id"`
	SetupNonce     string `json:"setup_nonce"`
	SSID           string `json:"ssid"`
	WifiPassword   string `json:"wifi_password"`
	FiscalPassword string `json:"fiscal_password"`
}
type runtimeProvisioningIssuer interface {
	SealRuntimeProvisioning(provisioningseal.Request, string, string, string) ([]byte, error)
}

// PrepareProvisioningDelivery uses only the platform manufacturing registry key.
// No caller/BLE supplied key, certificate, HMAC or identity is trusted.
func (s *Service) PrepareProvisioningDelivery(deviceID, tenant string, in ProvisioningDeliveryInput) (map[string]any, error) {
	reject := func() (map[string]any, error) { return nil, errors.New("provisioning delivery rejected") }
	device, e := s.repo.Resource("device", deviceID)
	if e != nil || device.TenantID != tenant || stringField(device.Data, "status") != "ACTIVE" || stringField(device.Data, "credential_id") == "" {
		return reject()
	}
	var enrolled ResourceRecord
	for _, p := range s.repo.Resources("platform_device", platformDeviceTenant) {
		if stringField(p.Data, "serial") == stringField(device.Data, "serial") && stringField(p.Data, "tenant_id") == tenant && oneOf(stringField(p.Data, "state"), "ASSIGNED", "DEPLOYED") {
			if enrolled.ID != "" {
				return reject()
			}
			enrolled = p
		}
	}
	if enrolled.ID == "" {
		return reject()
	}
	_, public, _, e := activationPublicKeyJSON(stringField(enrolled.Data, "device_public_key_jwk"))
	if e != nil {
		return reject()
	}
	der, e := x509.MarshalPKIXPublicKey(public)
	if e != nil {
		return reject()
	}
	encoded := base64.RawURLEncoding.EncodeToString(der)
	kidHash := sha256.Sum256([]byte(encoded))
	deviceKID := "esp32-p256-" + base64.RawURLEncoding.EncodeToString(kidHash[:12])
	if stringField(device.Data, "transaction_signing_public_key") != encoded || stringField(device.Data, "transaction_signing_kid") != deviceKID {
		return reject()
	}
	binding, e := s.repo.Resource("composite_binding", in.BindingID)
	if e != nil || binding.TenantID != tenant || stringField(binding.Data, "adapter_device_id") != deviceID || !oneOf(stringField(binding.Data, "status"), "PENDING", "ACTIVE") {
		return reject()
	}
	hash := stringField(binding.Data, "payload_sha256")
	digest, e := hex.DecodeString(hash)
	if e != nil || len(digest) != 32 {
		return reject()
	}
	snapshot, e := s.repo.Resource("issued_binding", hash)
	if e != nil || snapshot.TenantID != tenant {
		return reject()
	}
	canonical := stringField(snapshot.Data, "canonical_json")
	actual := sha256.Sum256([]byte(canonical))
	if hex.EncodeToString(actual[:]) != hash {
		return reject()
	}
	var payload struct {
		Schema     int    `json:"schema_version"`
		Generation uint64 `json:"generation"`
		Tenant     string `json:"tenant_id"`
		Device     string `json:"edge_device_id"`
		Register   string `json:"register_id"`
		Location   string `json:"location_id"`
		Profile    string `json:"profile"`
		Fiscal     struct {
			ID string `json:"device_id"`
		} `json:"fiscal_endpoint"`
		Authority struct {
			KID string `json:"transaction_signing_kid"`
		} `json:"operational_authority"`
	}
	if json.Unmarshal([]byte(canonical), &payload) != nil || payload.Schema != 3 || payload.Tenant != tenant || payload.Device != deviceID || payload.Generation != uint64(int64Field(binding.Data, "generation")) || payload.Authority.KID != deviceKID || payload.Register != stringField(binding.Data, "register_id") || payload.Profile != stringField(binding.Data, "profile") || payload.Fiscal.ID != stringField(binding.Data, "fiscal_device_id") {
		return reject()
	}
	register, e := s.repo.Resource("register", payload.Register)
	if e != nil || register.TenantID != tenant || stringField(register.Data, "location_id") != payload.Location {
		return reject()
	}
	nonce, e := base64.RawURLEncoding.DecodeString(in.SetupNonce)
	if e != nil || len(nonce) != 32 || len(in.SSID) < 1 || len(in.SSID) > 32 || strings.ContainsRune(in.SSID, 0) || len(in.WifiPassword) < 8 || len(in.WifiPassword) > 63 || strings.ContainsRune(in.WifiPassword, 0) {
		return reject()
	}
	profile := stringField(binding.Data, "profile")
	digits := func(v string, n int) bool {
		if len(v) != n {
			return false
		}
		for _, c := range v {
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	}
	if !(profile == "DATECS_DP150_BLUEPAD50" && digits(in.FiscalPassword, 8) || profile == "DAISY_COMPACT_S01" && digits(in.FiscalPassword, 4)) {
		return reject()
	}
	issuer, ok := s.deviceCredentialIssuer.(runtimeProvisioningIssuer)
	if !ok {
		return reject()
	}
	request := provisioningseal.Request{EnrolledPublicKeyDER: der, DeviceKID: deviceKID, IssuerKID: stringField(snapshot.Data, "key_id"), Generation: payload.Generation}
	copy(request.SetupNonce[:], nonce)
	copy(request.BindingSHA256[:], digest)
	envelope := map[string]any{"canonical_json": canonical, "signature": stringField(snapshot.Data, "signature"), "key_id": request.IssuerKID}
	request.Plaintext, _ = json.Marshal(map[string]any{"schema_version": 1, "binding": envelope, "wifi": map[string]string{"ssid": in.SSID, "password": in.WifiPassword}, "fiscal_operator_password": in.FiscalPassword})
	packet, e := issuer.SealRuntimeProvisioning(request, deviceID, stringField(device.Data, "credential_id"), stringField(enrolled.Data, "device_public_key_jwk"))
	clear(request.Plaintext)
	if e != nil {
		return reject()
	}
	sum := sha256.Sum256(packet)
	id, e := newUUID()
	if e != nil {
		return nil, e
	}
	now := time.Now().UTC()
	data := map[string]any{"device_id": deviceID, "binding_id": in.BindingID, "binding_sha256": hash, "generation": int64(payload.Generation), "packet_sha256": hex.EncodeToString(sum[:]), "state": "ISSUED", "created_at": now}
	if e = s.repo.PutResource(ResourceRecord{Kind: "provisioning_delivery", ID: id, TenantID: tenant, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now}); e != nil {
		return nil, e
	}
	return map[string]any{"delivery_id": id, "package_base64": base64.StdEncoding.EncodeToString(packet), "package_sha256": data["packet_sha256"], "binding_sha256": hash, "generation": int64(payload.Generation), "state": "ISSUED"}, nil
}

func (i *X509DeviceCredentialIssuer) SealRuntimeProvisioning(r provisioningseal.Request, deviceID, credentialID, jwk string) ([]byte, error) {
	key, ok := i.signer.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("P256 provisioning signer required")
	}
	pub, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	caHash := sha256.Sum256(pub)
	if r.IssuerKID != "binding-"+base64.RawURLEncoding.EncodeToString(caHash[:8]) {
		return nil, errors.New("provisioning issuer mismatch")
	}
	cred, e := i.Issue(DeviceActivationRequest{DeviceInstanceID: deviceID, PublicKeyJWK: jwk})
	if e != nil {
		return nil, e
	}
	var body map[string]any
	if json.Unmarshal(r.Plaintext, &body) != nil {
		return nil, errors.New("invalid provisioning body")
	}
	body["runtime_secrets"] = map[string]string{"root_ca_pem": cred.CAChainPEM, "client_certificate_pem": cred.ClientCertificatePEM, "command_hmac_key": base64.RawURLEncoding.EncodeToString(DeriveDeviceTransportKey(i.transportMaster, "command", deviceID, credentialID)), "sync_ack_hmac_key": base64.RawURLEncoding.EncodeToString(DeriveDeviceTransportKey(i.transportMaster, "sync-ack", deviceID, credentialID))}
	r.Plaintext, e = json.Marshal(body)
	if e != nil {
		return nil, e
	}
	defer clear(r.Plaintext)
	return provisioningseal.Seal(r, key)
}

type ProvisioningInstallProof struct {
	State         string `json:"state"`
	DeviceKID     string `json:"device_kid"`
	BindingSHA256 string `json:"binding_sha256"`
	PackageSHA256 string `json:"package_sha256"`
	Generation    int64  `json:"generation"`
	Signature     string `json:"signature"`
}

// ConfirmProvisioningInstallation authenticates device proof relayed by mobile;
// an ATT write success or a mobile INSTALLED flag never activates a binding.
func (s *Service) ConfirmProvisioningInstallation(deviceID, deliveryID, tenant string, p ProvisioningInstallProof) (map[string]any, error) {
	reject := func() (map[string]any, error) { return nil, errors.New("provisioning installation proof rejected") }
	if deliveryID == "by-proof" {
		matched := ""
		for _, candidate := range s.repo.Resources("provisioning_delivery", tenant) {
			if stringField(candidate.Data, "device_id") == deviceID && stringField(candidate.Data, "packet_sha256") == p.PackageSHA256 && stringField(candidate.Data, "binding_sha256") == p.BindingSHA256 && int64Field(candidate.Data, "generation") == p.Generation {
				if matched != "" {
					return reject()
				}
				matched = candidate.ID
			}
		}
		if matched == "" {
			return reject()
		}
		deliveryID = matched
	}
	delivery, e := s.repo.Resource("provisioning_delivery", deliveryID)
	if e != nil || delivery.TenantID != tenant || stringField(delivery.Data, "device_id") != deviceID || p.State != "INSTALLED" || p.BindingSHA256 != stringField(delivery.Data, "binding_sha256") || p.PackageSHA256 != stringField(delivery.Data, "packet_sha256") || p.Generation != int64Field(delivery.Data, "generation") {
		return reject()
	}
	device, e := s.repo.Resource("device", deviceID)
	if e != nil || device.TenantID != tenant || stringField(device.Data, "status") != "ACTIVE" || stringField(device.Data, "transaction_signing_kid") != p.DeviceKID {
		return reject()
	}
	var public *ecdsa.PublicKey
	for _, enrolled := range s.repo.Resources("platform_device", platformDeviceTenant) {
		if stringField(enrolled.Data, "serial") == stringField(device.Data, "serial") && stringField(enrolled.Data, "tenant_id") == tenant && oneOf(stringField(enrolled.Data, "state"), "ASSIGNED", "DEPLOYED") {
			if public != nil {
				return reject()
			}
			_, public, _, e = activationPublicKeyJSON(stringField(enrolled.Data, "device_public_key_jwk"))
			if e != nil {
				return reject()
			}
		}
	}
	if public == nil {
		return reject()
	}
	der, _ := x509.MarshalPKIXPublicKey(public)
	if base64.RawURLEncoding.EncodeToString(der) != stringField(device.Data, "transaction_signing_public_key") {
		return reject()
	}
	signature, e := base64.RawURLEncoding.DecodeString(p.Signature)
	proof := "beefiscal-provisioning-installed-v1\n" + p.DeviceKID + "\n" + p.BindingSHA256 + "\n" + fmt.Sprint(p.Generation) + "\n" + p.PackageSHA256
	if e != nil || len(signature) != 64 || !verifyP1363(public, sha256Bytes(proof), signature) {
		return reject()
	}
	half := new(big.Int).Rsh(new(big.Int).Set(public.Params().N), 1)
	if new(big.Int).SetBytes(signature[32:]).Cmp(half) > 0 {
		return reject()
	}
	bindingID := stringField(delivery.Data, "binding_id")
	binding, e := s.repo.Resource("composite_binding", bindingID)
	if e != nil || binding.TenantID != tenant || stringField(binding.Data, "adapter_device_id") != deviceID || stringField(binding.Data, "payload_sha256") != p.BindingSHA256 || int64Field(binding.Data, "generation") != p.Generation {
		return reject()
	}
	registerID := stringField(binding.Data, "register_id")
	if stringField(binding.Data, "status") == "PENDING" {
		if _, e = s.ApplyCompositeBinding(registerID, bindingID, tenant, deviceID, p.Generation); e != nil {
			binding, e = s.repo.Resource("composite_binding", bindingID)
			if e != nil || stringField(binding.Data, "status") != "ACTIVE" {
				return reject()
			}
		}
	}
	binding, e = s.repo.Resource("composite_binding", bindingID)
	register, re := s.repo.Resource("register", registerID)
	if e != nil || re != nil || register.TenantID != tenant || stringField(binding.Data, "status") != "ACTIVE" || stringField(register.Data, "composite_binding_id") != bindingID || stringField(register.Data, "adapter_device_id") != deviceID {
		return reject()
	}
	if stringField(delivery.Data, "state") != "INSTALLED" {
		delivery.Data["state"] = "INSTALLED"
		delivery.Data["installed_proof"] = asMap(p)
		delivery.Data["installed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		delivery.Version++
		delivery.UpdatedAt = time.Now().UTC()
		if e = s.repo.PutResource(delivery); e != nil {
			return nil, e
		}
	}
	return map[string]any{"state": "INSTALLED", "binding_id": bindingID, "binding_sha256": p.BindingSHA256, "generation": p.Generation}, nil
}
