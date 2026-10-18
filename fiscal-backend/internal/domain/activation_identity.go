package domain

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"strings"
)

var ErrFiscalIdentityProofRequired = errors.New("FISCAL_IDENTITY_PROOF_REQUIRED: S3 requires an existing verified physical fiscal route; embedded profiles require a vendor identity proof contract")

// Board identity is never fiscal identity. Only the explicit S3 adapter profile
// can currently derive authority from a separate, previously activated resource.
func (s *Service) activationFiscalIdentity(v DeviceActivationRequest) (ResourceRecord, error) {
	if v.Model != "EDGE_AGENT_S3" {
		return ResourceRecord{}, ErrFiscalIdentityProofRequired
	}
	register, e := s.repo.Resource("register", v.ClaimedRegisterID)
	if e != nil || register.TenantID != v.ClaimedTenantID || stringField(register.Data, "location_id") != v.ClaimedLocationID {
		return ResourceRecord{}, ErrFiscalIdentityProofRequired
	}
	fiscal, e := s.repo.Resource("device", stringField(register.Data, "fiscal_device_id"))
	if e != nil || fiscal.ID == v.DeviceInstanceID || fiscal.TenantID != v.ClaimedTenantID || stringField(fiscal.Data, "status") != "ACTIVE" || stringField(fiscal.Data, "kind") != "FISCAL_DEVICE" || !bgFMINPattern.MatchString(stringField(fiscal.Data, "fiscal_device_number")) || !bgFMINPattern.MatchString(stringField(fiscal.Data, "fiscal_memory_number")) || stringField(fiscal.Data, "fiscal_memory_number") != v.FMIN {
		return ResourceRecord{}, ErrFiscalIdentityProofRequired
	}
	vendor, model := strings.ToUpper(stringField(fiscal.Data, "vendor")), strings.ToUpper(stringField(fiscal.Data, "model"))
	if !(vendor == "DATECS" && model == "DP-150 MX" || vendor == "DAISY" && model == "COMPACT S 01") {
		return ResourceRecord{}, ErrFiscalIdentityProofRequired
	}
	_, requestKey, _, e := activationPublicKeyJSON(v.PublicKeyJWK)
	if e != nil {
		return ResourceRecord{}, ErrFiscalIdentityProofRequired
	}
	requestDER, e := x509.MarshalPKIXPublicKey(requestKey)
	if e != nil {
		return ResourceRecord{}, ErrFiscalIdentityProofRequired
	}
	matches := 0
	for _, record := range s.repo.Resources("platform_device", platformDeviceTenant) {
		if stringField(record.Data, "serial") != v.Serial || stringField(record.Data, "tenant_id") != v.ClaimedTenantID || !oneOf(stringField(record.Data, "state"), "ASSIGNED", "DEPLOYED") {
			continue
		}
		_, key, _, err := activationPublicKeyJSON(stringField(record.Data, "device_public_key_jwk"))
		if err != nil {
			return ResourceRecord{}, ErrFiscalIdentityProofRequired
		}
		der, err := x509.MarshalPKIXPublicKey(key)
		if err != nil || base64.RawURLEncoding.EncodeToString(der) != base64.RawURLEncoding.EncodeToString(requestDER) {
			return ResourceRecord{}, ErrFiscalIdentityProofRequired
		}
		matches++
	}
	if matches != 1 {
		return ResourceRecord{}, ErrFiscalIdentityProofRequired
	}
	return fiscal, nil
}
