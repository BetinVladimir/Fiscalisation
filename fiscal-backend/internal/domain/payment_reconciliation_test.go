package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type reconciliationFailStore struct{ *testStore }

func (s *reconciliationFailStore) Save([]byte) error { return errors.New("disk unavailable") }

func newReconciliationFixture(t *testing.T) *offlineV2Fixture {
	return newReconciliationFixtureForTenant(t, "tenant-1")
}
func newReconciliationFixtureForTenant(t *testing.T, tenant string) *offlineV2Fixture {
	t.Helper()
	f := &offlineV2Fixture{t: t, store: &testStore{}, at: time.Now().UTC().Truncate(time.Second)}
	f.repo, _ = NewPersistentRepository(f.store)
	f.svc = NewService(f.repo, NewSimulator(true))
	f.svc.SetBLESigningKey("01234567890123456789012345678901")
	f.svc.SetRequireHardwareSyncSignatures(true)
	ca, key := testDeviceCA(t)
	f.issuer, _ = NewX509DeviceCredentialIssuer(ca, key, "ssl://mqtt.example:8883", "wss://mqtt.example/mqtt", "01234567890123456789012345678901")
	f.svc.SetDeviceCredentialIssuer(f.issuer)
	f.device, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	f.client, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	deviceDER, _ := x509.MarshalPKIXPublicKey(&f.device.PublicKey)
	clientDER, _ := x509.MarshalPKIXPublicKey(&f.client.PublicKey)
	put := func(kind, id string, data map[string]any) {
		t.Helper()
		if err := f.repo.PutResource(ResourceRecord{Kind: kind, ID: id, TenantID: tenant, Version: 1, Data: data, CreatedAt: f.at, UpdatedAt: f.at}); err != nil {
			t.Fatal(err)
		}
	}
	binding := map[string]any{"schema_version": 3, "tenant_id": tenant, "location_id": "location-1", "register_id": "register-1", "edge_device_id": "adapter-1", "generation": int64(7), "fiscal_endpoint": map[string]any{"device_id": "fiscal-1", "fiscal_memory_number": "12345678", "fiscal_device_number": "AB123456"}, "payment_endpoint": map[string]any{"device_id": "terminal-1"}, "operational_authority": map[string]any{"unp_prefix": "AB123456", "unp_range_start": int64(1), "unp_range_end": int64(1000)}}
	hash := edgeDigest(binding)
	put("device", "adapter-1", map[string]any{"kind": "SMART_DEVICE", "status": "ACTIVE", "serial": "adapter-serial", "credential_id": "credential-1", "transaction_signing_public_key": base64.RawURLEncoding.EncodeToString(deviceDER), "transaction_signing_kid": "device-kid"})
	put("device", "fiscal-1", map[string]any{"kind": "FISCAL_DEVICE", "status": "ACTIVE", "serial": "fiscal-serial", "fiscal_memory_number": "12345678", "fiscal_device_number": "AB123456"})
	put("register", "register-1", map[string]any{"status": "ACTIVE", "code": "R001", "location_id": "location-1", "fiscal_device_id": "fiscal-1", "adapter_device_id": "adapter-1", "composite_binding_id": "binding-1"})
	put("operator", "operator-1", map[string]any{"code": "A001", "actor_subject": "subject-1", "active_from": f.at.Add(-time.Hour).Format(time.RFC3339)})
	put("composite_binding", "binding-1", map[string]any{"status": "ACTIVE", "register_id": "register-1", "adapter_device_id": "adapter-1", "fiscal_device_id": "fiscal-1", "generation": int64(7), "payload_sha256": hash})
	put("issued_binding", hash, map[string]any{"canonical_payload": binding, "payload_sha256": hash})
	put("workstation_session", "session-1", map[string]any{"session_id": "session-1", "tenant_id": tenant, "workstation_id": "register-1", "operator_id": "operator-1", "operator_code": "A001", "app_instance_id": "app-1", "actor_subject": "subject-1", "expires_at": f.at.Add(time.Hour).Format(time.RFC3339), "created_at": f.at.Format(time.RFC3339)})
	var err error
	f.envelope, err = f.svc.IssueExecutionAuthorization("session-1", base64.RawURLEncoding.EncodeToString(clientDER), tenant, "subject-1")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(f.envelope.Payload)
	if json.Unmarshal(raw, &f.auth) != nil {
		t.Fatal("auth decode")
	}
	f.at = time.Unix(f.auth.NBF, 0).UTC()
	operation := "10000000-0000-4000-8000-000000000001"
	receipt := "10000000-0000-4000-8000-000000000002"
	surrogate := "offline-external-1"
	payload := map[string]any{"currency": "EUR", "server_sale_id": surrogate, "external_id": surrogate, "location_id": "location-1", "operator_id": "operator-1", "operator_code": "A001", "items": []any{map[string]any{"line_id": "30000000-0000-4000-8000-000000000001", "name": "Кафе ☕", "quantity": "1.000", "unit_price": map[string]any{"amount": "2.50", "currency": "EUR"}, "tax_group": "B"}}, "payments": []any{map[string]any{"payment_id": "20000000-0000-4000-8000-000000000001", "type": "CARD", "amount": map[string]any{"amount": "2.50", "currency": "EUR"}}}, "metadata": map[string]any{}}
	acceptance := EdgeAcceptanceEvidence{Version: 2, Ingress: "BLE", ReceiptSessionID: receipt, ClientSaleSurrogateID: &surrogate, PayloadSHA256: edgeDigest(payload), UNP: "AB123456-A001-0000011", BindingSHA256: hash, Generation: 7, FiscalDeviceID: "fiscal-1", Authority: f.auth.Authority, SessionEvidence: &EdgeSessionEvidence{AuthorizationID: f.auth.AuthorizationID, AuthorizationSHA256: f.envelope.SHA256}, ProofState: "AUTHORIZED"}
	acceptance.SessionEvidence.ClientProofSHA256 = edgeDigest(executionIntentProof(acceptance, operation))
	f.accepted = DeviceEventEnvelope{EventID: "received-" + operation, OperationID: operation, DeviceID: "adapter-1", EventType: "ACCEPTED", OccurredAt: f.at.Format(time.RFC3339Nano), Payload: map[string]any{"operation_sequence": int64(10), "unp_sequence": int64(11), "command": map[string]any{"operation_id": operation, "tenant_id": tenant, "register_id": "register-1", "device_id": "adapter-1", "command_type": "SALE_FINALIZE", "payload": payload}, "acceptance_evidence": asMap(acceptance)}}
	f.result = DeviceEventEnvelope{EventID: "result-" + operation, OperationID: operation, DeviceID: "adapter-1", EventType: "UNKNOWN", OccurredAt: f.at.Add(time.Second).Format(time.RFC3339Nano), Payload: map[string]any{"state": "UNKNOWN", "operation_id": operation, "receipt_session_id": receipt, "payload_sha256": acceptance.PayloadSHA256, "unp": acceptance.UNP, "fiscal_reference": "FD-123", "payment_evidence": []any{map[string]any{"payment_id": "20000000-0000-4000-8000-000000000001", "type": "CARD", "amount_minor": int64(250), "state": "CARD_UNKNOWN", "terminal_reference": "", "rrn": "", "auth_code": ""}}}}
	transcript, _ := edgeCanonical(executionIntentProof(acceptance, operation))
	digest := sha256.Sum256(append([]byte(executionIntentProofDomain), transcript...))
	proof, _ := ecdsa.SignASN1(rand.Reader, f.client, digest[:])
	f.command = map[string]any{"protocol_version": "1.0", "intent_id": operation, "client_operation_id": operation, "action": "SALE_FINALIZE", "command": "SALE_FINALIZE", "receipt_session_id": receipt, "server_sale_id": nil, "client_sale_surrogate_id": surrogate, "tenant_id": tenant, "location_id": "location-1", "register_id": "register-1", "edge_device_id": "adapter-1", "binding_generation": int64(7), "canonical_payload": payload, "payload_sha256": acceptance.PayloadSHA256, "execution_authorization": asMap(f.envelope), "client_proof": map[string]any{"algorithm": "ES256_DER", "signature": base64.RawURLEncoding.EncodeToString(proof)}}
	return f
}

func seedReconciliation(t *testing.T) *offlineV2Fixture {
	f := newReconciliationFixture(t)
	if _, e := f.svc.SyncBatchForTenant("tenant-1", f.batch(1, nil, f.accepted, f.result)); e != nil {
		t.Fatal(e)
	}
	return f
}
func reconciliationInput(f *offlineV2Fixture) PaymentReconciliationRequest {
	return PaymentReconciliationRequest{OperationID: f.accepted.OperationID, PaymentID: "20000000-0000-4000-8000-000000000001", ObservedState: "CARD_APPROVED", TerminalReference: "00000367", RRN: "RRN123", AuthCode: "AUTH01", EvidenceReference: "acquirer-case-1", EvidenceSHA256: strings.Repeat("b", 64)}
}
func TestPaymentReconciliationIssuance(t *testing.T) {
	for _, mode := range []string{"valid", "role", "tenant", "actor", "payment", "operation", "evidence", "decision", "approved-missing-proof", "immutable", "stale-source"} {
		t.Run(mode, func(t *testing.T) {
			f := seedReconciliation(t)
			in := reconciliationInput(f)
			tenant, actor, admin := "tenant-1", "admin-1", true
			switch mode {
			case "role":
				admin = false
			case "tenant":
				tenant = "foreign"
			case "actor":
				actor = ""
			case "payment":
				in.PaymentID = "foreign"
			case "operation":
				in.OperationID = "foreign"
			case "evidence":
				in.EvidenceSHA256 = "invalid"
			case "decision":
				in.ObservedState = "REVERSED"
			case "approved-missing-proof":
				in.AuthCode = ""
			}
			command, e := f.svc.IssuePaymentReconciliation(in, tenant, actor, admin)
			valid := mode == "valid" || mode == "immutable" || mode == "stale-source"
			if (e == nil) != valid {
				t.Fatalf("unexpected %v", e)
			}
			if !valid {
				return
			}
			if !f.issuer.VerifyPaymentReconciliation(command.Authorization) {
				t.Fatal("issuer signature")
			}
			raw, _ := base64.RawURLEncoding.DecodeString(command.Authorization.Payload)
			var grant PaymentReconciliationGrant
			if json.Unmarshal(raw, &grant) != nil || grant.Expected.State != "CARD_UNKNOWN" || grant.TerminalID != "terminal-1" || grant.Expected.AmountMinor != "250" || grant.ActorID != "admin-1" {
				t.Fatal(string(raw))
			}
			record, _ := f.repo.Resource("payment_reconciliation_authorization", command.ClientOperationID)
			if mode == "immutable" {
				record.Data["envelope"] = map[string]any{}
				if f.repo.PutResource(record) == nil {
					t.Fatal("authorization rewritten")
				}
			}
			if mode == "stale-source" {
				record.ID = "another-id"
				record.Data["issuance_pending_digest"] = strings.Repeat("0", 64)
				if f.repo.PutResource(record) == nil {
					t.Fatal("stale snapshot accepted")
				}
			}
			if mode == "valid" {
				if out := os.Getenv("PAYMENT_RECONCILIATION_FIXTURE_DIR"); out != "" {
					os.MkdirAll(out, 0700)
					der, _ := x509.MarshalPKIXPublicKey(f.issuer.ca.PublicKey)
					payload := map[string]any{"command": command, "grant": grant, "binding": f.repo.resources[resourceKey("issued_binding", grant.BindingSHA256)].Data["canonical_payload"], "trust_key_der_base64": base64.RawURLEncoding.EncodeToString(der), "now": grant.NBF}
					b, _ := json.Marshal(payload)
					if e := os.WriteFile(filepath.Join(out, "fixture.json"), b, 0600); e != nil {
						t.Fatal(e)
					}
				}
			}
		})
	}
}

func TestPaymentReconciliationSignedDeviceConsumer(t *testing.T) {
	for _, mode := range []string{"valid", "amount", "reference", "authorization", "actor", "foreign-device", "expired", "wrong-operation", "repeat", "store-failure", "source-changed", "duplicate-batch"} {
		t.Run(mode, func(t *testing.T) {
			f := seedReconciliation(t)
			command, e := f.svc.IssuePaymentReconciliation(reconciliationInput(f), "tenant-1", "admin-1", true)
			if e != nil {
				t.Fatal(e)
			}
			raw, _ := base64.RawURLEncoding.DecodeString(command.Authorization.Payload)
			var g PaymentReconciliationGrant
			json.Unmarshal(raw, &g)
			payload := map[string]any{"reconciliation_id": g.ReconciliationID, "terminal_id": g.TerminalID, "payment_id": g.Expected.PaymentID, "operation_id": g.Expected.OperationID, "actor_id": g.ActorID, "evidence_reference": g.EvidenceReference, "evidence_sha256": g.EvidenceSHA256, "authorization_sha256": command.Authorization.SHA256, "old_state": g.Expected.State, "new_state": g.Observed.State, "amount_minor": g.Observed.AmountMinor, "terminal_reference": g.Observed.TerminalReference, "rrn": g.Observed.RRN, "auth_code": g.Observed.AuthCode, "reversal_reference": g.Observed.ReversalReference, "reversal_rrn": g.Observed.ReversalRRN, "reversal_auth_code": g.Observed.ReversalAuthCode}
			event := DeviceEventEnvelope{EventID: g.ReconciliationID + "-reconciled", OperationID: g.Expected.OperationID, DeviceID: g.EdgeDeviceID, EventType: "PAYMENT_RECONCILED", OccurredAt: time.Unix(g.NBF, 0).UTC().Format(time.RFC3339Nano), Payload: payload}
			switch mode {
			case "amount":
				payload["amount_minor"] = "251"
			case "reference":
				payload["rrn"] = "different"
			case "authorization":
				payload["authorization_sha256"] = strings.Repeat("a", 64)
			case "actor":
				payload["actor_id"] = "different"
			case "foreign-device":
				event.DeviceID = "different"
			case "expired":
				event.OccurredAt = time.Unix(g.Exp, 0).UTC().Format(time.RFC3339Nano)
			case "wrong-operation":
				event.OperationID = "different"
			}
			ack, _ := f.repo.LastSyncAck("tenant-1", "adapter-1")
			batch := f.batch(ack.CommittedThroughSeq+1, &ack.CommittedEventHash, event)
			if mode == "store-failure" {
				f.repo.store = &reconciliationFailStore{f.store}
			}
			if mode == "source-changed" {
				original := f.repo.edgePending[g.Expected.OperationID]
				original.ResultDigest = "changed"
				f.repo.edgePending[g.Expected.OperationID] = original
			}
			if mode == "duplicate-batch" {
				batch = f.batch(ack.CommittedThroughSeq+1, &ack.CommittedEventHash, event, event)
			}
			_, e = f.svc.SyncBatchForTenant("tenant-1", batch)
			valid := mode == "valid" || mode == "repeat"
			if (e == nil) != valid {
				t.Fatalf("consumer error=%v", e)
			}
			if !valid {
				if _, err := f.repo.Resource("payment_reconciliation", g.ReconciliationID); err == nil {
					t.Fatal("failed transaction leaked reconciliation audit")
				}
				after, _ := f.repo.LastSyncAck("tenant-1", "adapter-1")
				if after.CommittedThroughSeq != ack.CommittedThroughSeq {
					t.Fatal("failed transaction advanced cursor")
				}
			}
			if valid {
				record, e := f.repo.Resource("payment_reconciliation", g.ReconciliationID)
				if e != nil || record.TenantID != "tenant-1" {
					t.Fatal("missing audit resource", e)
				}
				original, _ := f.repo.EdgePendingCommand(g.Expected.OperationID, "tenant-1")
				if original.ResultState != "UNKNOWN" {
					t.Fatal("administrative bank decision fabricated fiscal result")
				}
				if mode == "repeat" {
					ack, _ = f.repo.LastSyncAck("tenant-1", "adapter-1")
					if _, e = f.svc.SyncBatchForTenant("tenant-1", f.batch(ack.CommittedThroughSeq+1, &ack.CommittedEventHash, event)); e == nil {
						t.Fatal("replayed grant accepted in a new sequence")
					}
				}
			}
		})
	}
}

func TestPaymentReconciliationActualFirmwareInterop(t *testing.T) {
	binary := os.Getenv("PAYMENT_RECONCILIATION_HOST_BINARY")
	if binary == "" {
		t.Skip("host runtime binary supplied by reconciliation_host runner")
	}
	f := seedReconciliation(t)
	command, e := f.svc.IssuePaymentReconciliation(reconciliationInput(f), "tenant-1", "admin-1", true)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(command.Authorization.Payload)
	var grant PaymentReconciliationGrant
	json.Unmarshal(raw, &grant)
	d := t.TempDir()
	der, _ := x509.MarshalPKIXPublicKey(f.issuer.ca.PublicKey)
	fixture := map[string]any{"command": command, "grant": grant, "trust_key_der_base64": base64.RawURLEncoding.EncodeToString(der)}
	b, _ := json.Marshal(fixture)
	fixturePath := filepath.Join(d, "fixture.json")
	os.WriteFile(fixturePath, b, 0600)
	keyPath := filepath.Join(d, "trust.pem")
	os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0600)
	eventPath := filepath.Join(d, "event.json")
	for _, mode := range []string{"valid", "signature", "tenant", "terminal", "binding", "expired", "future", "alias", "row", "admin-fault", "outbox-fault"} {
		cmd := exec.Command(binary, fixturePath, keyPath, filepath.Join(d, mode+".sqlite"), mode, eventPath)
		if b, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("actual firmware %s: %s %v", mode, b, e)
		}
	}

	// Boundary vectors use real root signatures and actual firmware parser.
	for _, actor := range []string{strings.Repeat("a", 136), strings.Repeat("я", 128), strings.Repeat("я", 128) + "x"} {
		changedGrant := grant
		changedGrant.ActorID = actor
		canonical, _ := edgeCanonical(changedGrant)
		sig, kid, _ := f.issuer.SignPaymentReconciliation(canonical)
		h := sha256.Sum256(canonical)
		changed := command
		changed.Authorization = ExecutionAuthorizationEnvelope{"ES256_DER", kid, base64.RawURLEncoding.EncodeToString(canonical), fmt.Sprintf("%x", h), sig}
		fixture["grant"], fixture["command"] = changedGrant, changed
		data, _ := json.Marshal(fixture)
		os.WriteFile(fixturePath, data, 0600)
		mode := "valid"
		if len(actor) > 256 {
			mode = "signed-invalid"
		}
		cmd := exec.Command(binary, fixturePath, keyPath, filepath.Join(d, fmt.Sprintf("actor-%d.sqlite", len(actor))), mode, filepath.Join(d, "actor-event.json"))
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("actor bytes%d: %s %v", len(actor), output, err)
		}
	}
	fixture["grant"], fixture["command"] = grant, command
	// Correctly signed malformed capabilities must still fail before any I/O.
	originalGrant := grant
	for _, mode := range []string{"missing-bank-proof", "wrong-row-amount", "illegal-transition", "oversized-evidence", "expired-signed"} {
		grant = originalGrant
		switch mode {
		case "missing-bank-proof":
			grant.Observed.AuthCode = ""
		case "wrong-row-amount":
			grant.Observed.AmountMinor = "251"
		case "illegal-transition":
			grant.Observed.State = "REVERSED"
		case "oversized-evidence":
			grant.EvidenceReference = strings.Repeat("x", 1025)
		case "expired-signed":
			grant.Exp = grant.NBF
		}
		canonical, _ := edgeCanonical(grant)
		sig, kid, _ := f.issuer.SignPaymentReconciliation(canonical)
		h := sha256.Sum256(canonical)
		changed := command
		changed.Authorization = ExecutionAuthorizationEnvelope{"ES256_DER", kid, base64.RawURLEncoding.EncodeToString(canonical), fmt.Sprintf("%x", h), sig}
		fixture["grant"] = grant
		fixture["command"] = changed
		bytes, _ := json.Marshal(fixture)
		os.WriteFile(fixturePath, bytes, 0600)
		cmd := exec.Command(binary, fixturePath, keyPath, filepath.Join(d, mode+".sqlite"), "signed-invalid", eventPath)
		if bytes, e := cmd.CombinedOutput(); e != nil {
			t.Fatalf("signed invalid %s: %s %v", mode, bytes, e)
		}
	}
	grant = originalGrant
	b, e = os.ReadFile(eventPath)
	if e != nil {
		t.Fatal(e)
	}
	var payload map[string]any
	if json.Unmarshal(b, &payload) != nil {
		t.Fatal("firmware event JSON")
	}
	event := DeviceEventEnvelope{EventID: grant.ReconciliationID + "-reconciled", OperationID: grant.Expected.OperationID, DeviceID: grant.EdgeDeviceID, EventType: "PAYMENT_RECONCILED", OccurredAt: time.Unix(grant.NBF, 0).UTC().Format(time.RFC3339Nano), Payload: payload}
	ack, _ := f.repo.LastSyncAck("tenant-1", "adapter-1")
	if _, e = f.svc.SyncBatchForTenant("tenant-1", f.batch(ack.CommittedThroughSeq+1, &ack.CommittedEventHash, event)); e != nil {
		t.Fatal("actual C++ produced evidence rejected", e)
	}
	if _, e = f.repo.Resource("payment_reconciliation", grant.ReconciliationID); e != nil {
		t.Fatal(e)
	}
}
