package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"time"
)

type EdgeSessionEvidence struct {
	AuthorizationID     string `json:"authorization_id"`
	AuthorizationSHA256 string `json:"authorization_sha256"`
	ClientProofSHA256   string `json:"client_proof_sha256"`
}
type EdgeAcceptanceEvidence struct {
	UNPSource             string                  `json:"unp_source,omitempty"`
	Version               int64                   `json:"version"`
	Ingress               string                  `json:"ingress"`
	ReceiptSessionID      string                  `json:"receipt_session_id"`
	ServerSaleID          *string                 `json:"server_sale_id"`
	ClientSaleSurrogateID *string                 `json:"client_sale_surrogate_id"`
	PayloadSHA256         string                  `json:"payload_sha256"`
	UNP                   string                  `json:"unp"`
	BindingSHA256         string                  `json:"binding_sha256"`
	Generation            int64                   `json:"generation"`
	FiscalDeviceID        string                  `json:"fiscal_device_id"`
	Authority             ExecutionAuthorityRange `json:"authority"`
	SessionEvidence       *EdgeSessionEvidence    `json:"session_evidence"`
	ProofState            string                  `json:"proof_state"`
}
type EdgeTenderEvidence struct {
	PaymentID         string `json:"payment_id"`
	Type              string `json:"type"`
	AmountMinor       int64  `json:"amount_minor"`
	State             string `json:"state"`
	TerminalReference string `json:"terminal_reference"`
	RRN               string `json:"rrn"`
	AuthCode          string `json:"auth_code"`
	ReversalReference string `json:"reversal_reference"`
	ReversalRRN       string `json:"reversal_rrn"`
	ReversalAuthCode  string `json:"reversal_auth_code"`
}

func edgeAcceptanceDigest(p EdgePendingCommand) string {
	p.ResultPayload = nil
	p.ResultDigest = ""
	p.ResultState = ""
	p.ProjectedSaleID = ""
	return edgeDigest(p)
}
func executionIntentProof(a EdgeAcceptanceEvidence, operation string) map[string]any {
	return map[string]any{"kind": "EXECUTION_INTENT_PROOF", "version": 1, "authorization_sha256": a.SessionEvidence.AuthorizationSHA256, "operation_id": operation, "receipt_session_id": a.ReceiptSessionID, "payload_sha256": a.PayloadSHA256, "server_sale_id": a.ServerSaleID, "client_sale_surrogate_id": a.ClientSaleSurrogateID, "binding_sha256": a.BindingSHA256, "binding_generation": a.Generation, "command_type": "SALE_FINALIZE"}
}
func (s *Service) verifyFinalizeAcceptance(p EdgePendingCommand) (ExecutionAuthorization, error) {
	var empty ExecutionAuthorization
	a := p.Acceptance
	if a != nil && a.Ingress == "MQTT" {
		return s.verifyServerFinalizeAcceptance(p)
	}
	if a == nil || a.Version != 2 || a.ProofState != "AUTHORIZED" || (a.Ingress != "BLE" && a.Ingress != "HTTP") || a.SessionEvidence == nil || a.ReceiptSessionID == "" || a.ClientSaleSurrogateID == nil || *a.ClientSaleSurrogateID == "" || !validSHA256(a.PayloadSHA256) || edgeDigest(p.Payload) != a.PayloadSHA256 || !validSHA256(a.BindingSHA256) || a.Generation < 1 {
		return empty, errors.New("offline finalize acceptance authority incomplete; quarantine required")
	}
	if !s.requireHardwareSyncSignatures {
		return empty, errors.New("v2 finalize requires hardware signatures")
	}
	r, err := s.repo.Resource("execution_authorization", a.SessionEvidence.AuthorizationID)
	if err != nil || r.TenantID != p.TenantID {
		return empty, errors.New("execution authorization unavailable")
	}
	auth, envelope, err := executionAuthorizationFromResource(r)
	if verifier, ok := s.deviceCredentialIssuer.(ExecutionAuthorizationVerifier); !ok || !verifier.VerifyExecutionAuthorization(envelope) {
		return empty, errors.New("execution authorization signature unavailable/invalid")
	}
	if err != nil {
		return empty, err
	}
	if envelope.SHA256 != a.SessionEvidence.AuthorizationSHA256 || edgeDigest(executionIntentProof(*a, p.OperationID)) != a.SessionEvidence.ClientProofSHA256 || auth.TenantID != p.TenantID || auth.RegisterID != p.RegisterID || auth.EdgeDeviceID != p.DeviceID || auth.FiscalDeviceID != a.FiscalDeviceID || auth.BindingSHA256 != a.BindingSHA256 || auth.BindingGeneration != a.Generation || auth.Authority != a.Authority || auth.Scope != "fiscal.execute" || !reflect.DeepEqual(auth.AllowedCommands, []string{"SALE_FINALIZE"}) || p.AcceptedAt.Unix() < auth.NBF || p.AcceptedAt.Unix() >= auth.Exp || stringAny(p.Payload, "location_id") != auth.LocationID || stringAny(p.Payload, "operator_id") != auth.OperatorID || stringAny(p.Payload, "operator_code") != auth.OperatorCode || stringAny(p.Payload, "external_id") != *a.ClientSaleSurrogateID {
		return empty, errors.New("finalize authorization context mismatch")
	}
	if revoked, e := s.repo.Resource("execution_authorization_revocation", auth.AuthorizationID); e == nil {
		at, e := time.Parse(time.RFC3339Nano, stringField(revoked.Data, "effective_at"))
		if revoked.TenantID != p.TenantID || e != nil || !p.AcceptedAt.Before(at) {
			return empty, errors.New("execution authorization revoked at acceptance")
		}
	}
	issued, e := s.repo.Resource("issued_binding", a.BindingSHA256)
	if e != nil || issued.TenantID != p.TenantID || edgeDigest(issued.Data["canonical_payload"]) != a.BindingSHA256 {
		return empty, errors.New("historical issued binding unavailable")
	}
	if auth.UNPAuthority != nil {
		u := auth.UNPAuthority
		parsed, e := ParseBGUNP(u.UNP)
		if e != nil || u.Source != "SERVER_ASSIGNED" || a.ServerSaleID == nil || *a.ServerSaleID != u.ServerSaleID || a.UNP != u.UNP || a.PayloadSHA256 != u.PayloadSHA256 || p.OperationID != u.OperationID || a.ReceiptSessionID != u.ReceiptSessionID || p.UNPSequence != 0 || stringAny(p.Payload, "unp_source") != "SERVER_ASSIGNED" || stringAny(p.Payload, "unp") != u.UNP || parsed.FiscalDeviceNumber != auth.FiscalDeviceNumber || parsed.OperatorCode != auth.OperatorCode || (parsed.Sequence >= a.Authority.RangeStart && parsed.Sequence <= a.Authority.RangeEnd) {
			return empty, errors.New("server assigned UNP authorization mismatch")
		}
	} else {
		if a.ServerSaleID != nil && *a.ServerSaleID != "" {
			return empty, errors.New("server sale requires assigned authority")
		}
		if stringAny(p.Payload, "unp") != "" || stringAny(p.Payload, "unp_source") != "" {
			return empty, errors.New("offline caller UNP forbidden")
		}
		unp, e := NewBGUNP(auth.FiscalDeviceNumber, auth.OperatorCode, p.UNPSequence)
		if e != nil || a.UNP != unp.String() || p.UNPSequence < a.Authority.RangeStart || p.UNPSequence > a.Authority.RangeEnd {
			return empty, errors.New("allocated UNP authority mismatch")
		}
	}
	if a.Authority.UNPPrefix != auth.FiscalDeviceNumber || a.Authority.BindingSHA256 != a.BindingSHA256 {
		return empty, errors.New("UNP authority binding mismatch")
	}
	return auth, nil
}
func validateOfflineFinalizePlan(p EdgePendingCommand, auth ExecutionAuthorization) (offlineSalePayload, int64, error) {
	var plan offlineSalePayload
	b, err := json.Marshal(p.Payload)
	if err != nil || json.Unmarshal(b, &plan) != nil || plan.Currency != "EUR" || len(plan.Items) == 0 || len(plan.Payments) == 0 {
		return plan, 0, errors.New("invalid offline finalization plan")
	}
	lines := map[string]bool{}
	for _, line := range plan.Items {
		if line.LineID == "" || lines[line.LineID] || line.Name == "" || line.UnitPrice.Currency != "EUR" || !validMoney(line.UnitPrice) || !validDiscount(line.Discount) || !validQuantity(line.Quantity) || !validTax(line.TaxGroup) {
			return plan, 0, errors.New("invalid offline finalization line")
		}
		lines[line.LineID] = true
		if line.Discount != nil && line.Discount.Currency != "EUR" {
			return plan, 0, errors.New("discount currency mismatch")
		}
	}
	total, err := saleTotal(Sale{Lines: plan.Items})
	if err != nil || total <= 0 {
		return plan, 0, errors.New("invalid offline finalization total")
	}
	paid := int64(0)
	payments := map[string]bool{}
	for _, payment := range plan.Payments {
		amount, e := parseFixed(payment.Amount.Amount, 2)
		if payment.PaymentID == "" || payments[payment.PaymentID] || payment.Amount.Currency != "EUR" || !validMoney(payment.Amount) || e != nil || amount <= 0 || !supportedPaymentType(payment.Type) || amount > math.MaxInt64-paid {
			return plan, 0, errors.New("invalid offline tender")
		}
		payments[payment.PaymentID] = true
		paid += amount
	}
	if paid != total || plan.OperatorID != auth.OperatorID || plan.OperatorCode != auth.OperatorCode {
		return plan, 0, errors.New("offline finalization total/operator mismatch")
	}
	return plan, total, nil
}
func (s *Service) materializeOfflineFinalizeV2(p EdgePendingCommand, event DeviceEventEnvelope, op Operation, finishedAt time.Time, materialized map[string]Sale) (Sale, Operation, string, []byte, OutboxItem, error) {
	fail := func(err error) (Sale, Operation, string, []byte, OutboxItem, error) {
		return Sale{}, Operation{}, "", nil, OutboxItem{}, err
	}
	auth, err := s.verifyFinalizeAcceptance(p)
	if err != nil {
		return fail(err)
	}
	plan, total, err := validateOfflineFinalizePlan(p, auth)
	if err != nil {
		return fail(err)
	}
	for _, line := range plan.Items {
		if !s.policy.AllowsTaxGroup(line.TaxGroup, p.AcceptedAt) {
			return fail(errors.New("offline tax group unavailable"))
		}
	}
	a := p.Acceptance
	if event.DeviceID != p.DeviceID || stringAny(event.Payload, "operation_id") != p.OperationID || stringAny(event.Payload, "receipt_session_id") != a.ReceiptSessionID || stringAny(event.Payload, "payload_sha256") != a.PayloadSHA256 || stringAny(event.Payload, "unp") != a.UNP {
		return fail(errors.New("offline result context mismatch"))
	}
	var evidence []EdgeTenderEvidence
	b, _ := json.Marshal(event.Payload["payment_evidence"])
	if json.Unmarshal(b, &evidence) != nil || len(evidence) != len(plan.Payments) {
		return fail(errors.New("complete per-tender evidence required"))
	}
	byID := map[string]EdgeTenderEvidence{}
	for _, tender := range evidence {
		if tender.PaymentID == "" || byID[tender.PaymentID].PaymentID != "" {
			return fail(errors.New("duplicate tender evidence"))
		}
		byID[tender.PaymentID] = tender
	}
	for _, payment := range plan.Payments {
		amount, _ := parseFixed(payment.Amount.Amount, 2)
		tender, ok := byID[payment.PaymentID]
		if !ok || tender.Type != payment.Type || tender.AmountMinor != amount {
			return fail(errors.New("tender evidence mismatch"))
		}
		switch event.EventType {
		case "FISCALIZED":
			if payment.Type == "CARD" && (tender.State != "CARD_APPROVED" || tender.TerminalReference == "") {
				return fail(errors.New("approved CARD evidence required"))
			}
			if payment.Type == "CASH" && tender.State != "FISCALIZED_CASH" {
				return fail(errors.New("fiscalized cash evidence required"))
			}
		case "COMPENSATED":
			if payment.Type == "CARD" && tender.State != "REVERSED" && tender.State != "CARD_DECLINED" && tender.State != "CARD_NOT_SENT" && tender.State != "RESERVED" {
				return fail(errors.New("compensated tender proof incomplete"))
			}
			if tender.State == "REVERSED" && (tender.RRN == "" || tender.ReversalReference == "") {
				return fail(errors.New("reversal evidence required"))
			}
		case "FAILED":
			if payment.Type == "CARD" && tender.State != "CARD_DECLINED" && tender.State != "CARD_NOT_SENT" && tender.State != "RESERVED" {
				return fail(errors.New("FAILED after monetary effects is not proven"))
			}
		case "UNKNOWN", "RECOVERY_REQUIRED":
		default:
			return fail(errors.New("unsupported offline result state"))
		}
	}
	saleID := "edge-sale-" + edgeDigest(map[string]any{"tenant_id": p.TenantID, "receipt_session_id": a.ReceiptSessionID})
	if a.ServerSaleID != nil && *a.ServerSaleID != "" {
		saleID = *a.ServerSaleID
	}
	existing, found := materialized[saleID]
	if !found {
		existing, err = s.repo.Sale(saleID)
		found = err == nil
	}
	sale := Sale{ID: saleID, TenantID: p.TenantID, ExternalID: plan.ExternalID, LocationID: auth.LocationID, RegisterID: p.RegisterID, OperatorID: auth.OperatorCode, OperatorCode: auth.OperatorCode, UNP: a.UNP, State: "UNKNOWN", Version: 1, Lines: plan.Items, Payments: []PaymentRecord{}, FiscalOperationID: p.OperationID, FiscalDevice: FiscalDeviceSnapshot{AdapterDeviceID: p.DeviceID, BindingGeneration: a.Generation, DeviceID: a.FiscalDeviceID, FiscalMemoryNumber: auth.FiscalMemoryNumber, FiscalDeviceNumber: auth.FiscalDeviceNumber}, RegulatoryIdentifiers: []RegulatoryIdentifier{{Type: "SALE", Scheme: BGUNPV1, Value: a.UNP, CountryCode: "BG", ProfileVersion: BGProfileVersion}}, CreatedAt: p.AcceptedAt, UpdatedAt: finishedAt}
	if found {
		if existing.TenantID != p.TenantID || existing.RegisterID != p.RegisterID || existing.LocationID != auth.LocationID || existing.ExternalID != plan.ExternalID || existing.OperatorID != auth.OperatorCode || existing.UNP != a.UNP || !reflect.DeepEqual(existing.Lines, plan.Items) || (existing.FiscalOperationID != "" && existing.FiscalOperationID != p.OperationID) || (existing.State != "PAYMENT_PENDING" && existing.State != "UNKNOWN" && existing.State != "COMPLETED" && existing.State != "CANCELLED") {
			return fail(errors.New("offline existing sale immutable context mismatch"))
		}
		if existing.FiscalDevice.DeviceID != a.FiscalDeviceID || existing.FiscalDevice.AdapterDeviceID != p.DeviceID || existing.FiscalDevice.BindingGeneration != a.Generation {
			return fail(errors.New("offline sale fiscal snapshot mismatch"))
		}
		if len(existing.Payments) != len(plan.Payments) {
			return fail(errors.New("existing finalization tender inventory mismatch"))
		}
		for i, payment := range plan.Payments {
			saved := existing.Payments[i]
			if saved.PaymentID != payment.PaymentID || saved.Type != payment.Type || saved.Amount != payment.Amount {
				return fail(errors.New("existing finalization tender mismatch"))
			}
		}
		sale.Version = existing.Version + 1
		sale.CreatedAt = existing.CreatedAt
	}
	if a.ServerSaleID != nil && *a.ServerSaleID != "" && !found {
		return fail(errors.New("explicit server sale missing"))
	}
	ref := stringAny(event.Payload, "fiscal_reference")
	op.SaleID = saleID
	switch event.EventType {
	case "FISCALIZED":
		if ref == "" {
			return fail(errors.New("physical fiscal reference required"))
		}
		sale.State = "COMPLETED"
		sale.CompletedAt = &finishedAt
	case "COMPENSATED":
		sale.State = "CANCELLED"
		sale.CancelledAt = &finishedAt
		op.State = "COMPENSATED"
	case "FAILED":
		sale.State = "CANCELLED"
		sale.CancelledAt = &finishedAt
	default:
		op.State = "RECOVERY_REQUIRED"
		op.AllowedActions = []string{"RECONCILE"}
		op.ErrorCode = stringAny(event.Payload, "error_code")
	}
	for _, payment := range plan.Payments {
		sale.Payments = append(sale.Payments, PaymentRecord{PaymentID: payment.PaymentID, Type: payment.Type, Amount: payment.Amount, FiscalReference: ref, CreatedAt: finishedAt})
	}
	artifactID := ""
	var artifact []byte
	if sale.State == "COMPLETED" {
		artifactID = "edge-receipt-" + edgeDigest(map[string]any{"tenant": p.TenantID, "operation": p.OperationID, "receipt": a.ReceiptSessionID})
		sale.ReceiptArtifactID = artifactID
		artifact, err = json.Marshal(map[string]any{"sale_id": saleID, "operation_id": p.OperationID, "receipt_session_id": a.ReceiptSessionID, "unp": a.UNP, "fiscal_reference": ref, "issued_at": finishedAt, "total": Money{Amount: formatFixed(total), Currency: "EUR"}, "fiscal_device": sale.FiscalDevice, "lines": sale.Lines, "payments": sale.Payments, "payment_evidence": evidence, "acceptance_evidence": a})
		if err != nil {
			return fail(err)
		}
	}
	if found && (existing.State == "COMPLETED" || existing.State == "CANCELLED") {
		if p.ResultDigest != edgeDigest(event.Payload) {
			return fail(errors.New("terminal offline result conflict"))
		}
		return Sale{}, Operation{}, "", nil, OutboxItem{}, nil
	}
	hook := edgeSaleHook(sale, op, event.EventType, finishedAt)
	return sale, op, artifactID, artifact, hook, nil
}
func parseAcceptanceEvidence(event DeviceEventEnvelope) (*EdgeAcceptanceEvidence, error) {
	raw, ok := event.Payload["acceptance_evidence"]
	if !ok {
		return nil, nil
	}
	b, e := json.Marshal(raw)
	if e != nil {
		return nil, e
	}
	var a EdgeAcceptanceEvidence
	if e = json.Unmarshal(b, &a); e != nil {
		return nil, fmt.Errorf("invalid acceptance evidence: %w", e)
	}
	return &a, nil
}
