package domain

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"
)

const executionAuthorizationDomain = "BEELOY-EXECUTION-AUTHORIZATION-V1\n"
const executionIntentProofDomain = "BEELOY-EXECUTION-INTENT-PROOF-V1\n"

type ExecutionAuthorityRange struct {
	BindingSHA256 string `json:"binding_sha256"`
	RangeStart    int64  `json:"range_start"`
	RangeEnd      int64  `json:"range_end"`
	UNPPrefix     string `json:"unp_prefix"`
}
type ExecutionUNPAuthority struct {
	Source           string `json:"source"`
	ServerSaleID     string `json:"server_sale_id"`
	UNP              string `json:"unp"`
	PayloadSHA256    string `json:"payload_sha256"`
	OperationID      string `json:"operation_id"`
	ReceiptSessionID string `json:"receipt_session_id"`
}
type ExecutionAuthorization struct {
	Kind                            string                  `json:"kind"`
	Version                         int64                   `json:"version"`
	AuthorizationID                 string                  `json:"authorization_id"`
	WorkstationSessionID            string                  `json:"workstation_session_id"`
	TenantID                        string                  `json:"tenant_id"`
	LocationID                      string                  `json:"location_id"`
	RegisterID                      string                  `json:"register_id"`
	EdgeDeviceID                    string                  `json:"edge_device_id"`
	FiscalDeviceID                  string                  `json:"fiscal_device_id"`
	FiscalMemoryNumber              string                  `json:"fiscal_memory_number"`
	FiscalDeviceNumber              string                  `json:"fiscal_device_number"`
	BindingSHA256                   string                  `json:"binding_sha256"`
	BindingGeneration               int64                   `json:"binding_generation"`
	OperatorID                      string                  `json:"operator_id"`
	OperatorCode                    string                  `json:"operator_code"`
	ActorSubject                    string                  `json:"actor_subject"`
	AppInstanceID                   string                  `json:"app_instance_id"`
	ClientSigningPublicKeyDERBase64 string                  `json:"client_signing_public_key_der_base64"`
	Scope                           string                  `json:"scope"`
	AllowedCommands                 []string                `json:"allowed_commands"`
	NBF                             int64                   `json:"nbf"`
	Exp                             int64                   `json:"exp"`
	Authority                       ExecutionAuthorityRange `json:"authority"`
	UNPAuthority                    *ExecutionUNPAuthority  `json:"unp_authority,omitempty"`
}
type ExecutionAuthorizationEnvelope struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id"`
	Payload   string `json:"payload"`
	SHA256    string `json:"sha256"`
	Signature string `json:"signature"`
}
type ExecutionAuthorizationSigner interface {
	SignExecutionAuthorization([]byte) (string, string, error)
}

func (i *X509DeviceCredentialIssuer) SignExecutionAuthorization(canonical []byte) (string, string, error) {
	key, ok := i.signer.Public().(*ecdsa.PublicKey)
	if !ok || key.Curve.Params().Name != "P-256" {
		return "", "", errors.New("execution authorization requires P256 root")
	}
	digest := sha256.Sum256(append([]byte(executionAuthorizationDomain), canonical...))
	signature, err := i.signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		return "", "", err
	}
	kid := sha256.Sum256(i.ca.RawSubjectPublicKeyInfo)
	return base64.RawURLEncoding.EncodeToString(signature), "binding-" + base64.RawURLEncoding.EncodeToString(kid[:8]), nil
}
func edgeCanonical(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err = decoder.Decode(&m); err != nil {
		return nil, err
	}
	return json.Marshal(m)
}
func edgeDigest(v any) string {
	b, err := edgeCanonical(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func validSHA256(v string) bool {
	b, e := hex.DecodeString(v)
	return e == nil && len(b) == 32 && hex.EncodeToString(b) == v
}
func executionAuthorizationFromResource(r ResourceRecord) (ExecutionAuthorization, ExecutionAuthorizationEnvelope, error) {
	var a ExecutionAuthorization
	var envelope ExecutionAuthorizationEnvelope
	b, err := json.Marshal(r.Data["envelope"])
	if err != nil || json.Unmarshal(b, &envelope) != nil {
		return a, envelope, errors.New("authorization envelope unavailable")
	}
	raw, err := base64.RawURLEncoding.DecodeString(envelope.Payload)
	if err != nil || json.Unmarshal(raw, &a) != nil {
		return a, envelope, errors.New("authorization payload unavailable")
	}
	canonical, err := edgeCanonical(a)
	sum := sha256.Sum256(raw)
	if err != nil || string(raw) != string(canonical) || envelope.Algorithm != "ES256_DER" || envelope.KeyID == "" || envelope.Signature == "" || hex.EncodeToString(sum[:]) != envelope.SHA256 || a.Kind != "EXECUTION_AUTHORIZATION" || a.Version != 1 || a.AuthorizationID != r.ID || a.TenantID != r.TenantID || a.NBF <= 0 || a.Exp <= a.NBF || a.Exp-a.NBF > 900 {
		return a, envelope, errors.New("invalid immutable authorization")
	}
	return a, envelope, nil
}

// IssueExecutionAuthorization accepts only a persisted caller-owned session and a
// signing key. Tenant, operator, routes and sequence authority are never caller maps.
func (s *Service) IssueExecutionAuthorization(sessionID, clientKey, tenant, actor string, serverSaleIDs ...string) (ExecutionAuthorizationEnvelope, error) {
	var empty ExecutionAuthorizationEnvelope
	now := time.Now().UTC()
	sessionResource, err := s.repo.Resource("workstation_session", sessionID)
	if err != nil || sessionResource.TenantID != tenant || tenant == "" || actor == "" || stringField(sessionResource.Data, "actor_subject") != actor {
		return empty, errors.New("execution session unavailable")
	}
	session, err := s.WorkstationSession(sessionID, stringField(sessionResource.Data, "workstation_id"), tenant)
	if err != nil {
		return empty, err
	}
	operator, err := s.repo.Resource("operator", session.OperatorID)
	if err != nil || operator.TenantID != tenant || stringField(operator.Data, "actor_subject") != actor || stringField(operator.Data, "code") != session.OperatorCode {
		return empty, errors.New("operator authenticated identity mapping required")
	}
	der, err := base64.RawURLEncoding.DecodeString(clientKey)
	if err != nil {
		return empty, errors.New("invalid client signing key")
	}
	parsed, err := x509.ParsePKIXPublicKey(der)
	client, ok := parsed.(*ecdsa.PublicKey)
	if err != nil || !ok || client.Curve.Params().Name != "P-256" {
		return empty, errors.New("client signing key must be P256")
	}
	register, err := s.repo.Resource("register", session.WorkstationID)
	if err != nil || register.TenantID != tenant {
		return empty, errors.New("register unavailable")
	}
	binding, err := s.repo.Resource("composite_binding", stringField(register.Data, "composite_binding_id"))
	if err != nil || binding.TenantID != tenant || stringField(binding.Data, "status") != "ACTIVE" {
		return empty, errors.New("active issued binding required")
	}
	current, routeErr := s.activeFiscalDeviceSnapshot(session.WorkstationID, tenant, now)
	if routeErr != nil || current.AdapterDeviceID != stringField(binding.Data, "adapter_device_id") || current.DeviceID != stringField(binding.Data, "fiscal_device_id") || stringField(binding.Data, "register_id") != session.WorkstationID {
		return empty, errors.New("current issued route unavailable")
	}
	hash := stringField(binding.Data, "payload_sha256")
	if hash == "" {
		hash = stringField(binding.Data, "binding_sha256")
	}
	issued, err := s.repo.Resource("issued_binding", hash)
	if err != nil || issued.TenantID != tenant || !validSHA256(hash) {
		return empty, errors.New("immutable issued binding required")
	}
	payload, ok := issued.Data["canonical_payload"].(map[string]any)
	if !ok || edgeDigest(payload) != hash {
		return empty, errors.New("issued binding digest mismatch")
	}
	fiscal, _ := payload["fiscal_endpoint"].(map[string]any)
	authority, _ := payload["operational_authority"].(map[string]any)
	fmin := stringAny(fiscal, "fiscal_memory_number")
	serial := stringAny(fiscal, "fiscal_device_number")
	generation := intAny(payload, "generation")
	if stringAny(payload, "tenant_id") != tenant || stringAny(payload, "register_id") != session.WorkstationID || stringAny(payload, "location_id") == "" || stringAny(payload, "edge_device_id") != stringField(binding.Data, "adapter_device_id") || stringAny(fiscal, "device_id") != stringField(binding.Data, "fiscal_device_id") || generation < 1 || generation != int64Field(binding.Data, "generation") || fmin == "" || !bgFMINPattern.MatchString(serial) || stringAny(authority, "unp_prefix") != serial || intAny(authority, "unp_range_start") < 1 || intAny(authority, "unp_range_end") < intAny(authority, "unp_range_start") || intAny(authority, "unp_range_end") > BGUNPSequenceMax {
		return empty, errors.New("issued binding authority invalid")
	}
	adapterResource, adapterErr := s.repo.Resource("device", stringAny(payload, "edge_device_id"))
	fiscalResource, fiscalErr := s.repo.Resource("device", stringAny(fiscal, "device_id"))
	if adapterErr != nil || fiscalErr != nil || adapterResource.TenantID != tenant || fiscalResource.TenantID != tenant || stringField(fiscalResource.Data, "fiscal_device_number") != serial || stringField(fiscalResource.Data, "fiscal_memory_number") != fmin {
		return empty, errors.New("issued fiscal identity does not match current enrollment")
	}
	id, err := newUUID()
	if err != nil {
		return empty, err
	}
	exp := now.Add(15 * time.Minute)
	if session.ExpiresAt.Before(exp) {
		exp = session.ExpiresAt
	}
	a := ExecutionAuthorization{Kind: "EXECUTION_AUTHORIZATION", Version: 1, AuthorizationID: id, WorkstationSessionID: sessionID, TenantID: tenant, LocationID: stringAny(payload, "location_id"), RegisterID: session.WorkstationID, EdgeDeviceID: stringAny(payload, "edge_device_id"), FiscalDeviceID: stringAny(fiscal, "device_id"), FiscalMemoryNumber: fmin, FiscalDeviceNumber: serial, BindingSHA256: hash, BindingGeneration: generation, OperatorID: session.OperatorID, OperatorCode: session.OperatorCode, ActorSubject: actor, AppInstanceID: session.AppInstanceID, ClientSigningPublicKeyDERBase64: clientKey, Scope: "fiscal.execute", AllowedCommands: []string{"SALE_FINALIZE"}, NBF: now.Unix(), Exp: exp.Unix(), Authority: ExecutionAuthorityRange{hash, intAny(authority, "unp_range_start"), intAny(authority, "unp_range_end"), serial}}
	var onlineSale Sale
	var onlineOperation Operation
	if len(serverSaleIDs) > 1 {
		return empty, errors.New("one server sale required")
	}
	if len(serverSaleIDs) == 1 && serverSaleIDs[0] != "" {
		onlineSale, err = s.repo.Sale(serverSaleIDs[0])
		if err != nil || onlineSale.TenantID != tenant || onlineSale.RegisterID != a.RegisterID || onlineSale.LocationID != a.LocationID || onlineSale.State != "PAYMENT_PENDING" || onlineSale.OperatorCode != a.OperatorCode || (onlineSale.OperatorID != a.OperatorID && onlineSale.OperatorID != a.OperatorCode) || onlineSale.FiscalDevice.DeviceID != a.FiscalDeviceID || onlineSale.FiscalDevice.AdapterDeviceID != a.EdgeDeviceID || onlineSale.FiscalDevice.BindingGeneration != a.BindingGeneration || onlineSale.FiscalDevice.FiscalDeviceNumber != serial || onlineSale.FiscalDevice.FiscalMemoryNumber != fmin {
			return empty, errors.New("server sale authority mismatch")
		}
		parsed, e := ParseBGUNP(onlineSale.UNP)
		if e != nil || parsed.FiscalDeviceNumber != serial || parsed.OperatorCode != a.OperatorCode || (parsed.Sequence >= a.Authority.RangeStart && parsed.Sequence <= a.Authority.RangeEnd) {
			return empty, errors.New("server UNP conflicts with offline authority")
		}
		op, e := s.repo.Operation(onlineSale.FiscalOperationID)
		onlineOperation = op
		if e != nil || op.TenantID != tenant || op.SaleID != onlineSale.ID || op.Type != "SALE_FINALIZE" || op.State != "EXECUTING" || op.ID == "" || op.ReceiptSessionID == "" {
			return empty, errors.New("server pending finalization required")
		}
		payload, e := BuildFinalizePayload(onlineSale, a.OperatorID, a.OperatorCode)
		if e != nil {
			return empty, e
		}
		a.UNPAuthority = &ExecutionUNPAuthority{Source: "SERVER_ASSIGNED", ServerSaleID: onlineSale.ID, UNP: onlineSale.UNP, PayloadSHA256: edgeDigest(payload), OperationID: op.ID, ReceiptSessionID: op.ReceiptSessionID}
	}
	canonical, err := edgeCanonical(a)
	if err != nil {
		return empty, err
	}
	signer, ok := s.deviceCredentialIssuer.(ExecutionAuthorizationSigner)
	if !ok {
		return empty, errors.New("execution authorization signer unavailable")
	}
	signature, kid, err := signer.SignExecutionAuthorization(canonical)
	if err != nil {
		return empty, err
	}
	sum := sha256.Sum256(canonical)
	envelope := ExecutionAuthorizationEnvelope{"ES256_DER", kid, base64.RawURLEncoding.EncodeToString(canonical), hex.EncodeToString(sum[:]), signature}
	record := ResourceRecord{Kind: "execution_authorization", ID: id, TenantID: tenant, Version: 1, Data: map[string]any{"envelope": asMap(envelope), "issued_binding_sha256": hash, "issuance_sources": map[string]any{resourceKey("workstation_session", sessionID): sessionResource.Version, resourceKey("operator", operator.ID): operator.Version, resourceKey("register", register.ID): register.Version, resourceKey("composite_binding", binding.ID): binding.Version, resourceKey("issued_binding", issued.ID): issued.Version, resourceKey("device", adapterResource.ID): adapterResource.Version, resourceKey("device", fiscalResource.ID): fiscalResource.Version}}, CreatedAt: now, UpdatedAt: now}
	if a.UNPAuthority != nil {
		record.Data["issuance_sale_id"] = onlineSale.ID
		record.Data["issuance_sale_version"] = onlineSale.Version
		record.Data["issuance_operation_id"] = a.UNPAuthority.OperationID
		record.Data["issuance_operation_version"] = onlineOperation.Version
	}
	if err = s.repo.PutResource(record); err != nil {
		return empty, err
	}
	return envelope, nil
}

type ExecutionAuthorizationVerifier interface {
	VerifyExecutionAuthorization(ExecutionAuthorizationEnvelope) bool
}

func (i *X509DeviceCredentialIssuer) VerifyExecutionAuthorization(v ExecutionAuthorizationEnvelope) bool {
	key, ok := i.ca.PublicKey.(*ecdsa.PublicKey)
	if !ok || key.Curve.Params().Name != "P-256" || v.Algorithm != "ES256_DER" {
		return false
	}
	kid := sha256.Sum256(i.ca.RawSubjectPublicKeyInfo)
	if v.KeyID != "binding-"+base64.RawURLEncoding.EncodeToString(kid[:8]) {
		return false
	}
	raw, e := base64.RawURLEncoding.DecodeString(v.Payload)
	if e != nil {
		return false
	}
	sig, e := base64.RawURLEncoding.DecodeString(v.Signature)
	if e != nil {
		return false
	}
	digest := sha256.Sum256(append([]byte(executionAuthorizationDomain), raw...))
	return ecdsa.VerifyASN1(key, digest[:], sig)
}

// Identity mapping is an explicit administrator action, not a session/body claim.
func (s *Service) BindOperatorIdentity(id, subject, tenant, administrator string, expected int64) error {
	if id == "" || subject == "" || tenant == "" || administrator == "" || expected < 1 {
		return errors.New("invalid operator identity mapping")
	}
	writer, ok := s.repo.(interface {
		BindOperatorIdentity(string, string, string, string, int64) error
	})
	if !ok {
		return errors.New("atomic identity mapping unavailable")
	}
	return writer.BindOperatorIdentity(id, subject, tenant, administrator, expected)
}
