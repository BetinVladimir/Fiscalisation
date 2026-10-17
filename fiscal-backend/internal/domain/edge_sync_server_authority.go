package domain

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// MQTT authority is the durable, already prepared server command, not a new
// authorization inferred from current register routes or a device assertion.
func (s *Service) verifyServerFinalizeAcceptance(p EdgePendingCommand) (ExecutionAuthorization, error) {
	var empty ExecutionAuthorization
	a := p.Acceptance
	if a == nil || a.Version != 2 || a.Ingress != "MQTT" || a.ProofState != "SERVER_COMMAND" || a.SessionEvidence != nil || a.ServerSaleID == nil || *a.ServerSaleID == "" || a.ClientSaleSurrogateID != nil || p.UNPSequence != 0 || !s.requireHardwareSyncSignatures || a.UNPSource != "SERVER_ASSIGNED" {
		return empty, errors.New("server acceptance authority incomplete")
	}
	record, err := s.repo.Resource("device_command_outbox", p.OperationID)
	if err != nil || record.TenantID != p.TenantID || stringAny(record.Data, "device_id") != p.DeviceID {
		return empty, errors.New("prepared server command unavailable")
	}
	var cmd struct {
		OperationID      string         `json:"operation_id"`
		ReceiptSessionID string         `json:"receipt_session_id"`
		TenantID         string         `json:"tenant_id"`
		RegisterID       string         `json:"register_id"`
		DeviceID         string         `json:"device_id"`
		Type             string         `json:"command_type"`
		Generation       int64          `json:"fencing_token"`
		PayloadSHA256    string         `json:"payload_sha256"`
		Payload          map[string]any `json:"payload"`
		IssuedAt         time.Time      `json:"issued_at"`
		ExpiresAt        time.Time      `json:"expires_at"`
	}
	decoder := json.NewDecoder(strings.NewReader(stringAny(record.Data, "body")))
	decoder.UseNumber()
	if decoder.Decode(&cmd) != nil {
		return empty, errors.New("prepared command malformed")
	}
	if cmd.IssuedAt.IsZero() || cmd.ExpiresAt.IsZero() || !cmd.ExpiresAt.After(cmd.IssuedAt) || cmd.OperationID != p.OperationID || cmd.ReceiptSessionID != a.ReceiptSessionID || cmd.TenantID != p.TenantID || cmd.RegisterID != p.RegisterID || cmd.DeviceID != p.DeviceID || cmd.Type != "SALE_FINALIZE" || cmd.Generation != a.Generation || cmd.PayloadSHA256 != a.PayloadSHA256 || edgeDigest(cmd.Payload) != a.PayloadSHA256 || edgeDigest(p.Payload) != a.PayloadSHA256 || p.AcceptedAt.Before(cmd.IssuedAt) || !p.AcceptedAt.Before(cmd.ExpiresAt) || stringAny(cmd.Payload, "server_sale_id") != *a.ServerSaleID || stringAny(cmd.Payload, "unp_source") != "SERVER_ASSIGNED" || stringAny(cmd.Payload, "unp") != a.UNP {
		return empty, errors.New("prepared command context mismatch")
	}
	sale, err := s.repo.Sale(*a.ServerSaleID)
	if err != nil || sale.TenantID != p.TenantID || sale.RegisterID != p.RegisterID || sale.FiscalOperationID != p.OperationID || sale.UNP != a.UNP || sale.FiscalDevice.DeviceID != a.FiscalDeviceID || sale.FiscalDevice.AdapterDeviceID != p.DeviceID || sale.FiscalDevice.BindingGeneration != a.Generation {
		return empty, errors.New("prepared server sale mismatch")
	}
	issued, err := s.repo.Resource("issued_binding", a.BindingSHA256)
	if err != nil || issued.TenantID != p.TenantID || edgeDigest(issued.Data["canonical_payload"]) != a.BindingSHA256 {
		return empty, errors.New("historical server binding unavailable")
	}
	binding, _ := issued.Data["canonical_payload"].(map[string]any)
	fiscal, _ := binding["fiscal_endpoint"].(map[string]any)
	authority, _ := binding["operational_authority"].(map[string]any)
	expected := ExecutionAuthorityRange{a.BindingSHA256, intAny(authority, "unp_range_start"), intAny(authority, "unp_range_end"), stringAny(authority, "unp_prefix")}
	parsed, e := ParseBGUNP(a.UNP)
	if e != nil || stringAny(binding, "tenant_id") != p.TenantID || stringAny(binding, "register_id") != p.RegisterID || stringAny(binding, "edge_device_id") != p.DeviceID || intAny(binding, "generation") != a.Generation || stringAny(fiscal, "device_id") != a.FiscalDeviceID || stringAny(fiscal, "fiscal_device_number") != parsed.FiscalDeviceNumber || stringAny(fiscal, "fiscal_memory_number") != sale.FiscalDevice.FiscalMemoryNumber || sale.FiscalDevice.FiscalDeviceNumber != parsed.FiscalDeviceNumber || parsed.OperatorCode != sale.OperatorCode || stringAny(p.Payload, "operator_code") != sale.OperatorCode || expected != a.Authority || expected.UNPPrefix != parsed.FiscalDeviceNumber || expected.RangeStart < 1 || expected.RangeEnd < expected.RangeStart || (parsed.Sequence >= expected.RangeStart && parsed.Sequence <= expected.RangeEnd) {
		return empty, errors.New("server assigned UNP historical authority mismatch")
	}
	return ExecutionAuthorization{TenantID: p.TenantID, LocationID: sale.LocationID, RegisterID: p.RegisterID, EdgeDeviceID: p.DeviceID, FiscalDeviceID: a.FiscalDeviceID, FiscalDeviceNumber: parsed.FiscalDeviceNumber, FiscalMemoryNumber: sale.FiscalDevice.FiscalMemoryNumber, OperatorID: stringAny(cmd.Payload, "operator_id"), OperatorCode: parsed.OperatorCode, BindingSHA256: a.BindingSHA256, BindingGeneration: a.Generation, Authority: expected, UNPAuthority: &ExecutionUNPAuthority{Source: "SERVER_ASSIGNED", ServerSaleID: sale.ID, UNP: sale.UNP, PayloadSHA256: a.PayloadSHA256, OperationID: p.OperationID, ReceiptSessionID: a.ReceiptSessionID}}, nil
}
