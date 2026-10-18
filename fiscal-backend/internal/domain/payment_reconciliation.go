package domain

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

const paymentReconciliationDomain = "BEELOY-PAYMENT-RECONCILIATION-V1\n"

type ReconciliationPaymentRow struct {
	PaymentID         string `json:"payment_id"`
	OperationID       string `json:"operation_id"`
	ReceiptID         string `json:"receipt_id"`
	State             string `json:"state"`
	AmountMinor       string `json:"amount_minor"`
	TerminalReference string `json:"terminal_reference"`
	RRN               string `json:"rrn"`
	AuthCode          string `json:"auth_code"`
	ReversalReference string `json:"reversal_reference"`
	ReversalRRN       string `json:"reversal_rrn"`
	ReversalAuthCode  string `json:"reversal_auth_code"`
}
type PaymentReconciliationGrant struct {
	Kind              string                   `json:"kind"`
	Version           int64                    `json:"version"`
	Scope             string                   `json:"scope"`
	ReconciliationID  string                   `json:"reconciliation_id"`
	TenantID          string                   `json:"tenant_id"`
	LocationID        string                   `json:"location_id"`
	RegisterID        string                   `json:"register_id"`
	EdgeDeviceID      string                   `json:"edge_device_id"`
	TerminalID        string                   `json:"terminal_id"`
	BindingSHA256     string                   `json:"binding_sha256"`
	BindingGeneration int64                    `json:"binding_generation"`
	ActorID           string                   `json:"actor_id"`
	EvidenceReference string                   `json:"evidence_reference"`
	EvidenceSHA256    string                   `json:"evidence_sha256"`
	NBF               int64                    `json:"nbf"`
	Exp               int64                    `json:"exp"`
	Expected          ReconciliationPaymentRow `json:"expected"`
	Observed          ReconciliationPaymentRow `json:"observed"`
}
type PaymentReconciliationRequest struct {
	OperationID       string `json:"operation_id"`
	PaymentID         string `json:"payment_id"`
	ObservedState     string `json:"observed_state"`
	TerminalReference string `json:"terminal_reference"`
	RRN               string `json:"rrn"`
	AuthCode          string `json:"auth_code"`
	ReversalReference string `json:"reversal_reference"`
	ReversalRRN       string `json:"reversal_rrn"`
	ReversalAuthCode  string `json:"reversal_auth_code"`
	EvidenceReference string `json:"evidence_reference"`
	EvidenceSHA256    string `json:"evidence_sha256"`
}
type PaymentReconciliationCommand struct {
	Command           string                         `json:"command"`
	ClientOperationID string                         `json:"client_operation_id"`
	Authorization     ExecutionAuthorizationEnvelope `json:"reconciliation_authorization"`
}
type paymentReconciliationSigner interface {
	SignPaymentReconciliation([]byte) (string, string, error)
	VerifyPaymentReconciliation(ExecutionAuthorizationEnvelope) bool
}

func (i *X509DeviceCredentialIssuer) SignPaymentReconciliation(raw []byte) (string, string, error) {
	k, ok := i.signer.Public().(*ecdsa.PublicKey)
	if !ok || k.Curve.Params().Name != "P-256" {
		return "", "", errors.New("P256 issuer required")
	}
	d := sha256.Sum256(append([]byte(paymentReconciliationDomain), raw...))
	sig, e := i.signer.Sign(rand.Reader, d[:], crypto.SHA256)
	kid := sha256.Sum256(i.ca.RawSubjectPublicKeyInfo)
	return base64.RawURLEncoding.EncodeToString(sig), "binding-" + base64.RawURLEncoding.EncodeToString(kid[:8]), e
}
func (i *X509DeviceCredentialIssuer) VerifyPaymentReconciliation(v ExecutionAuthorizationEnvelope) bool {
	k, ok := i.ca.PublicKey.(*ecdsa.PublicKey)
	if !ok || k.Curve.Params().Name != "P-256" || v.Algorithm != "ES256_DER" {
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
	h := sha256.Sum256(raw)
	if hex.EncodeToString(h[:]) != v.SHA256 {
		return false
	}
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&decoded) != nil {
		return false
	}
	canonical, e := edgeCanonical(decoded)
	if e != nil || !bytes.Equal(canonical, raw) {
		return false
	}
	d := sha256.Sum256(append([]byte(paymentReconciliationDomain), raw...))
	return ecdsa.VerifyASN1(k, d[:], sig)
}
func reconciliationTransition(a, b ReconciliationPaymentRow) bool {
	if a.PaymentID == "" || a.OperationID == "" || a.ReceiptID == "" || a.PaymentID != b.PaymentID || a.OperationID != b.OperationID || a.ReceiptID != b.ReceiptID || a.AmountMinor != b.AmountMinor {
		return false
	}
	n, e := strconv.ParseInt(a.AmountMinor, 10, 64)
	if e != nil || n <= 0 || strconv.FormatInt(n, 10) != a.AmountMinor {
		return false
	}
	purchase := (a.State == "CARD_SENDING" || a.State == "CARD_UNKNOWN") && (b.State == "CARD_APPROVED" || b.State == "CARD_DECLINED" || b.State == "CARD_NOT_SENT")
	reversal := (a.State == "REVERSAL_SENDING" || a.State == "REVERSAL_UNKNOWN") && (b.State == "REVERSED" || b.State == "REVERSAL_FAILED")
	if !purchase && !reversal {
		return false
	}
	if (a.TerminalReference != "" && a.TerminalReference != b.TerminalReference) || (a.RRN != "" && a.RRN != b.RRN) || (a.AuthCode != "" && a.AuthCode != b.AuthCode) {
		return false
	}
	if b.State == "CARD_APPROVED" && (b.TerminalReference == "" || b.RRN == "" || b.AuthCode == "") {
		return false
	}
	if b.State == "REVERSED" && (b.ReversalReference == "" || b.ReversalRRN == "" || b.ReversalAuthCode == "") {
		return false
	}
	return true
}
func (s *Service) paymentReconciliationContext(operation, payment, tenant string) (EdgePendingCommand, ReconciliationPaymentRow, ResourceRecord, map[string]any, error) {
	fail := func() (EdgePendingCommand, ReconciliationPaymentRow, ResourceRecord, map[string]any, error) {
		return EdgePendingCommand{}, ReconciliationPaymentRow{}, ResourceRecord{}, nil, errors.New("unresolved authenticated payment context required")
	}
	p, e := s.repo.EdgePendingCommand(operation, tenant)
	if e != nil || p.TenantID != tenant || p.Acceptance == nil || p.Acceptance.Version != 2 || p.ResultPayload == nil || (p.ResultState != "UNKNOWN" && p.ResultState != "RECOVERY_REQUIRED") {
		return fail()
	}
	if _, e = s.verifyFinalizeAcceptance(p); e != nil {
		return fail()
	}
	issued, e := s.repo.Resource("issued_binding", p.Acceptance.BindingSHA256)
	if e != nil || issued.TenantID != tenant {
		return fail()
	}
	binding, ok := issued.Data["canonical_payload"].(map[string]any)
	if !ok || edgeDigest(binding) != p.Acceptance.BindingSHA256 || stringAny(binding, "tenant_id") != tenant || stringAny(binding, "edge_device_id") != p.DeviceID || stringAny(binding, "register_id") != p.RegisterID || intAny(binding, "generation") != p.Acceptance.Generation {
		return fail()
	}
	terminal, _ := binding["payment_endpoint"].(map[string]any)
	if stringAny(terminal, "device_id") == "" {
		return fail()
	}
	var rows []EdgeTenderEvidence
	b, _ := json.Marshal(p.ResultPayload["payment_evidence"])
	if json.Unmarshal(b, &rows) != nil {
		return fail()
	}
	var row ReconciliationPaymentRow
	for _, r := range rows {
		if r.PaymentID == payment {
			if row.PaymentID != "" || r.Type != "CARD" {
				return fail()
			}
			row = ReconciliationPaymentRow{r.PaymentID, p.OperationID, p.Acceptance.ReceiptSessionID, r.State, strconv.FormatInt(r.AmountMinor, 10), r.TerminalReference, r.RRN, r.AuthCode, r.ReversalReference, r.ReversalRRN, r.ReversalAuthCode}
		}
	}
	if row.PaymentID == "" {
		return fail()
	}
	return p, row, issued, binding, nil
}
func (s *Service) IssuePaymentReconciliation(in PaymentReconciliationRequest, tenant, actor string, administrator bool) (PaymentReconciliationCommand, error) {
	var empty PaymentReconciliationCommand
	if !administrator || tenant == "" || actor == "" || in.EvidenceReference == "" || len(in.EvidenceReference) > 1024 || !validSHA256(in.EvidenceSHA256) {
		return empty, errors.New("ADMIN identity and immutable acquirer evidence required")
	}
	p, expected, issued, binding, e := s.paymentReconciliationContext(in.OperationID, in.PaymentID, tenant)
	if e != nil {
		return empty, e
	}
	for _, existing := range s.repo.Resources("payment_reconciliation", tenant) {
		event, _ := existing.Data["event"].(map[string]any)
		body, _ := event["payload"].(map[string]any)
		if stringAny(body, "payment_id") == expected.PaymentID && stringAny(body, "old_state") == expected.State {
			return empty, errors.New("payment snapshot already reconciled; await new durable device snapshot")
		}
	}
	observed := expected
	observed.State = in.ObservedState
	for _, v := range []struct {
		dst *string
		src string
	}{{&observed.TerminalReference, in.TerminalReference}, {&observed.RRN, in.RRN}, {&observed.AuthCode, in.AuthCode}, {&observed.ReversalReference, in.ReversalReference}, {&observed.ReversalRRN, in.ReversalRRN}, {&observed.ReversalAuthCode, in.ReversalAuthCode}} {
		if v.src != "" {
			if len(v.src) > 128 {
				return empty, errors.New("reference too long")
			}
			*v.dst = v.src
		}
	}
	if !reconciliationTransition(expected, observed) {
		return empty, errors.New("invalid reconciliation decision")
	}
	id, e := newUUID()
	if e != nil {
		return empty, e
	}
	now := time.Now().UTC()
	terminal, _ := binding["payment_endpoint"].(map[string]any)
	grant := PaymentReconciliationGrant{"PAYMENT_RECONCILIATION", 1, "payment.reconcile", id, tenant, stringAny(binding, "location_id"), p.RegisterID, p.DeviceID, stringAny(terminal, "device_id"), p.Acceptance.BindingSHA256, p.Acceptance.Generation, actor, in.EvidenceReference, in.EvidenceSHA256, now.Unix(), now.Add(5 * time.Minute).Unix(), expected, observed}
	raw, e := edgeCanonical(grant)
	if e != nil {
		return empty, e
	}
	signer, ok := s.deviceCredentialIssuer.(paymentReconciliationSigner)
	if !ok {
		return empty, errors.New("reconciliation signer unavailable")
	}
	sig, kid, e := signer.SignPaymentReconciliation(raw)
	if e != nil {
		return empty, e
	}
	digest := sha256.Sum256(raw)
	envelope := ExecutionAuthorizationEnvelope{"ES256_DER", kid, base64.RawURLEncoding.EncodeToString(raw), hex.EncodeToString(digest[:]), sig}
	record := ResourceRecord{Kind: "payment_reconciliation_authorization", ID: id, TenantID: tenant, Version: 1, Data: map[string]any{"envelope": asMap(envelope), "issuance_pending_digest": edgeDigest(p), "issuance_operation_id": p.OperationID, "issuance_binding_id": issued.ID, "issuance_binding_version": issued.Version}, CreatedAt: now, UpdatedAt: now}
	if e = s.repo.PutResource(record); e != nil {
		return empty, e
	}
	return PaymentReconciliationCommand{"PAYMENT_RECONCILE", id, envelope}, nil
}

// The device journal proves application of a previously issued exact decision.
// It does not prove a fiscal document or close the original financial operation.
func (s *Service) paymentReconciliationEvent(tenant string, event DeviceEventEnvelope) (ResourceRecord, error) {
	reject := func() (ResourceRecord, error) {
		return ResourceRecord{}, errors.New("invalid payment reconciliation evidence")
	}
	id := stringAny(event.Payload, "reconciliation_id")
	issued, e := s.repo.Resource("payment_reconciliation_authorization", id)
	if e != nil || issued.TenantID != tenant {
		return reject()
	}
	var envelope ExecutionAuthorizationEnvelope
	b, _ := json.Marshal(issued.Data["envelope"])
	if json.Unmarshal(b, &envelope) != nil {
		return reject()
	}
	signer, ok := s.deviceCredentialIssuer.(paymentReconciliationSigner)
	if !ok || !signer.VerifyPaymentReconciliation(envelope) {
		return reject()
	}
	raw, e := base64.RawURLEncoding.DecodeString(envelope.Payload)
	if e != nil {
		return reject()
	}
	var g PaymentReconciliationGrant
	if json.Unmarshal(raw, &g) != nil {
		return reject()
	}
	at, e := time.Parse(time.RFC3339Nano, event.OccurredAt)
	if e != nil || at.Unix() < g.NBF || at.Unix() >= g.Exp || g.TenantID != tenant || event.DeviceID != g.EdgeDeviceID || event.OperationID != g.Expected.OperationID || event.EventID != g.ReconciliationID+"-reconciled" {
		return reject()
	}
	expected := map[string]any{"reconciliation_id": g.ReconciliationID, "terminal_id": g.TerminalID, "payment_id": g.Expected.PaymentID, "operation_id": g.Expected.OperationID, "actor_id": g.ActorID, "evidence_reference": g.EvidenceReference, "evidence_sha256": g.EvidenceSHA256, "authorization_sha256": envelope.SHA256, "old_state": g.Expected.State, "new_state": g.Observed.State, "amount_minor": g.Observed.AmountMinor, "terminal_reference": g.Observed.TerminalReference, "rrn": g.Observed.RRN, "auth_code": g.Observed.AuthCode, "reversal_reference": g.Observed.ReversalReference, "reversal_rrn": g.Observed.ReversalRRN, "reversal_auth_code": g.Observed.ReversalAuthCode}
	if edgeDigest(event.Payload) != edgeDigest(expected) || !reconciliationTransition(g.Expected, g.Observed) {
		return reject()
	}
	return ResourceRecord{Kind: "payment_reconciliation", ID: id, TenantID: tenant, Version: 1, Data: map[string]any{"grant_sha256": envelope.SHA256, "issued_resource_digest": edgeDigest(issued), "issuance_pending_digest": stringAny(issued.Data, "issuance_pending_digest"), "original_operation_id": g.Expected.OperationID, "event": asMap(event)}, CreatedAt: at, UpdatedAt: at}, nil
}
