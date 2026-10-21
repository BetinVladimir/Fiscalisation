package domain

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type offlineV2Fixture struct {
	t        *testing.T
	repo     *MemoryRepository
	svc      *Service
	store    *testStore
	issuer   *X509DeviceCredentialIssuer
	device   *ecdsa.PrivateKey
	client   *ecdsa.PrivateKey
	envelope ExecutionAuthorizationEnvelope
	auth     ExecutionAuthorization
	accepted DeviceEventEnvelope
	result   DeviceEventEnvelope
	command  map[string]any
	at       time.Time
}

func newOfflineV2Fixture(t *testing.T) *offlineV2Fixture {
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
		if err := f.repo.PutResource(ResourceRecord{Kind: kind, ID: id, TenantID: "tenant-1", Version: 1, Data: data, CreatedAt: f.at, UpdatedAt: f.at}); err != nil {
			t.Fatal(err)
		}
	}
	binding := map[string]any{"schema_version": 3, "tenant_id": "tenant-1", "location_id": "location-1", "register_id": "register-1", "edge_device_id": "adapter-1", "generation": int64(7), "fiscal_endpoint": map[string]any{"device_id": "fiscal-1", "fiscal_memory_number": "12345678", "fiscal_device_number": "AB123456"}, "operational_authority": map[string]any{"unp_prefix": "AB123456", "unp_range_start": int64(1), "unp_range_end": int64(1000)}}
	hash := edgeDigest(binding)
	put("device", "adapter-1", map[string]any{"kind": "SMART_DEVICE", "status": "ACTIVE", "serial": "adapter-serial", "credential_id": "credential-1", "transaction_signing_public_key": base64.RawURLEncoding.EncodeToString(deviceDER), "transaction_signing_kid": "device-kid"})
	put("device", "fiscal-1", map[string]any{"kind": "FISCAL_DEVICE", "status": "ACTIVE", "serial": "fiscal-serial", "fiscal_memory_number": "12345678", "fiscal_device_number": "AB123456"})
	put("register", "register-1", map[string]any{"status": "ACTIVE", "code": "R001", "location_id": "location-1", "fiscal_device_id": "fiscal-1", "adapter_device_id": "adapter-1", "composite_binding_id": "binding-1"})
	put("operator", "operator-1", map[string]any{"code": "A001", "actor_subject": "subject-1", "active_from": f.at.Add(-time.Hour).Format(time.RFC3339)})
	put("composite_binding", "binding-1", map[string]any{"status": "ACTIVE", "register_id": "register-1", "adapter_device_id": "adapter-1", "fiscal_device_id": "fiscal-1", "generation": int64(7), "payload_sha256": hash})
	put("issued_binding", hash, map[string]any{"canonical_payload": binding, "payload_sha256": hash})
	put("workstation_session", "session-1", map[string]any{"session_id": "session-1", "tenant_id": "tenant-1", "workstation_id": "register-1", "operator_id": "operator-1", "operator_code": "A001", "app_instance_id": "app-1", "actor_subject": "subject-1", "expires_at": f.at.Add(time.Hour).Format(time.RFC3339), "created_at": f.at.Format(time.RFC3339)})
	var err error
	f.envelope, err = f.svc.IssueExecutionAuthorization("session-1", base64.RawURLEncoding.EncodeToString(clientDER), "tenant-1", "subject-1")
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
	payload := map[string]any{"currency": "EUR", "server_sale_id": surrogate, "external_id": surrogate, "location_id": "location-1", "operator_id": "operator-1", "operator_code": "A001", "items": []any{map[string]any{"line_id": "30000000-0000-4000-8000-000000000001", "name": "Кафе ☕", "quantity": "1.000", "unit_price": map[string]any{"amount": "2.50", "currency": "EUR"}, "tax_group": "B"}}, "payments": []any{map[string]any{"payment_id": "20000000-0000-4000-8000-000000000001", "type": "CASH", "amount": map[string]any{"amount": "2.50", "currency": "EUR"}}}, "metadata": map[string]any{}}
	acceptance := EdgeAcceptanceEvidence{Version: 2, Ingress: "BLE", ReceiptSessionID: receipt, ClientSaleSurrogateID: &surrogate, PayloadSHA256: edgeDigest(payload), UNP: "AB123456-A001-0000011", BindingSHA256: hash, Generation: 7, FiscalDeviceID: "fiscal-1", Authority: f.auth.Authority, SessionEvidence: &EdgeSessionEvidence{AuthorizationID: f.auth.AuthorizationID, AuthorizationSHA256: f.envelope.SHA256}, ProofState: "AUTHORIZED"}
	acceptance.SessionEvidence.ClientProofSHA256 = edgeDigest(executionIntentProof(acceptance, operation))
	f.accepted = DeviceEventEnvelope{EventID: "received-" + operation, OperationID: operation, DeviceID: "adapter-1", EventType: "ACCEPTED", OccurredAt: f.at.Format(time.RFC3339Nano), Payload: map[string]any{"operation_sequence": int64(10), "unp_sequence": int64(11), "command": map[string]any{"operation_id": operation, "tenant_id": "tenant-1", "register_id": "register-1", "device_id": "adapter-1", "command_type": "SALE_FINALIZE", "payload": payload}, "acceptance_evidence": asMap(acceptance)}}
	f.result = DeviceEventEnvelope{EventID: "result-" + operation, OperationID: operation, DeviceID: "adapter-1", EventType: "FISCALIZED", OccurredAt: f.at.Add(time.Second).Format(time.RFC3339Nano), Payload: map[string]any{"state": "FISCALIZED", "operation_id": operation, "receipt_session_id": receipt, "payload_sha256": acceptance.PayloadSHA256, "unp": acceptance.UNP, "fiscal_reference": "FD-123", "payment_evidence": []any{map[string]any{"payment_id": "20000000-0000-4000-8000-000000000001", "type": "CASH", "amount_minor": int64(250), "state": "FISCALIZED_CASH", "terminal_reference": "", "rrn": "", "auth_code": ""}}}}
	transcript, _ := edgeCanonical(executionIntentProof(acceptance, operation))
	digest := sha256.Sum256(append([]byte(executionIntentProofDomain), transcript...))
	proof, _ := ecdsa.SignASN1(rand.Reader, f.client, digest[:])
	f.command = map[string]any{"protocol_version": "1.0", "intent_id": operation, "client_operation_id": operation, "action": "SALE_FINALIZE", "command": "SALE_FINALIZE", "receipt_session_id": receipt, "server_sale_id": nil, "client_sale_surrogate_id": surrogate, "tenant_id": "tenant-1", "location_id": "location-1", "register_id": "register-1", "edge_device_id": "adapter-1", "binding_generation": int64(7), "canonical_payload": payload, "payload_sha256": acceptance.PayloadSHA256, "execution_authorization": asMap(f.envelope), "client_proof": map[string]any{"algorithm": "ES256_DER", "signature": base64.RawURLEncoding.EncodeToString(proof)}}
	return f
}
func (f *offlineV2Fixture) batch(first int64, previous *string, events ...DeviceEventEnvelope) EdgeSyncBatch {
	f.t.Helper()
	prior := ""
	if previous != nil {
		prior = *previous
	}
	for i := range events {
		events[i].JournalSeq = first + int64(i)
		if prior != "" {
			v := prior
			events[i].PrevHash = &v
		} else {
			events[i].PrevHash = nil
		}
		events[i].EventHash = DeviceEventHash(events[i])
		sig := f.sign(events[i].EventHash)
		events[i].Signature = &sig
		prior = events[i].EventHash
	}
	b := EdgeSyncBatch{EdgeID: "adapter-1", SchemaVersion: "2026-08-07", FirstSeq: first, LastSeq: first + int64(len(events)) - 1, PreviousAcknowledgedHash: previous, Events: events}
	b.BatchSHA256 = EdgeBatchHash(b)
	b.Signature = f.sign(b.BatchSHA256)
	return b
}
func (f *offlineV2Fixture) sign(hash string) string {
	raw, _ := hex.DecodeString(hash)
	digest := sha256.Sum256(raw)
	sig, _ := signP1363(f.device, digest[:])
	return "device-kid:" + base64.RawURLEncoding.EncodeToString(sig)
}
func TestOfflineFinalizeV2MissingSaleCreatesAfterSignedRestart(t *testing.T) {
	f := newOfflineV2Fixture(t)
	batch := f.batch(1, nil, f.accepted)
	ack, err := f.svc.SyncBatchForTenant("tenant-1", batch)
	if err != nil {
		t.Fatal(err)
	}
	f.repo, _ = NewPersistentRepository(f.store)
	f.svc = NewService(f.repo, NewSimulator(true))
	f.svc.SetBLESigningKey("01234567890123456789012345678901")
	f.svc.SetRequireHardwareSyncSignatures(true)
	f.svc.SetDeviceCredentialIssuer(f.issuer)
	prev := ack.CommittedEventHash
	result := f.batch(2, &prev, f.result)
	ack, err = f.svc.SyncBatchForTenant("tenant-1", result)
	if err != nil {
		t.Fatal(err)
	}
	sales := f.repo.Sales("tenant-1")
	if len(sales) != 1 || sales[0].State != "COMPLETED" || sales[0].UNP != "AB123456-A001-0000011" || sales[0].ReceiptArtifactID == "" {
		t.Fatalf("invalid projected sale %#v", sales)
	}
	pending, e := f.repo.EdgePendingCommand(f.accepted.OperationID, "tenant-1")
	if e != nil || pending.ResultState != "FISCALIZED" || pending.ResultDigest == "" {
		t.Fatal("accepted evidence lost", e)
	}
	if _, err = f.svc.SyncBatchForTenant("tenant-1", result); err != nil {
		t.Fatal(err)
	}
	if len(f.repo.PendingOutbox(time.Now().Add(time.Hour))) != 1 {
		t.Fatal("duplicate webhook")
	}
}
func TestExecutionAuthorizationActualRootSignatureAndFirmwareFixture(t *testing.T) {
	f := newOfflineV2Fixture(t)
	raw, _ := base64.RawURLEncoding.DecodeString(f.envelope.Payload)
	sig, _ := base64.RawURLEncoding.DecodeString(f.envelope.Signature)
	digest := sha256.Sum256(append([]byte(executionAuthorizationDomain), raw...))
	if !ecdsa.VerifyASN1(f.issuer.ca.PublicKey.(*ecdsa.PublicKey), digest[:], sig) {
		t.Fatal("actual root signature invalid")
	}
	if out := os.Getenv("EXECUTION_AUTH_FIXTURE_DIR"); out != "" {
		if err := os.MkdirAll(out, 0700); err != nil {
			t.Fatal(err)
		}
		trust, _ := x509.MarshalPKIXPublicKey(f.issuer.ca.PublicKey)
		fixture := map[string]any{"binding": f.repo.resources[resourceKey("issued_binding", f.auth.BindingSHA256)].Data["canonical_payload"], "authorization": f.envelope, "command": f.command, "trust_key_der_base64": base64.RawURLEncoding.EncodeToString(trust), "now": f.at.Unix(), "acceptance_event": f.accepted, "result_event": f.result}
		b, _ := json.MarshalIndent(fixture, "", "  ")
		if err := os.WriteFile(filepath.Join(out, "execution-authorization-fixture.json"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func (f *offlineV2Fixture) acceptance() *EdgeAcceptanceEvidence {
	var a EdgeAcceptanceEvidence
	b, _ := json.Marshal(f.accepted.Payload["acceptance_evidence"])
	if json.Unmarshal(b, &a) != nil {
		f.t.Fatal("acceptance decode")
	}
	return &a
}
func (f *offlineV2Fixture) setAcceptance(a *EdgeAcceptanceEvidence) {
	f.accepted.Payload["acceptance_evidence"] = asMap(a)
}
func (f *offlineV2Fixture) payload() map[string]any {
	return f.accepted.Payload["command"].(map[string]any)["payload"].(map[string]any)
}
func (f *offlineV2Fixture) reseal() {
	a := f.acceptance()
	a.PayloadSHA256 = edgeDigest(f.payload())
	a.SessionEvidence.ClientProofSHA256 = edgeDigest(executionIntentProof(*a, f.accepted.OperationID))
	f.setAcceptance(a)
	f.result.Payload["payload_sha256"] = a.PayloadSHA256
	f.result.Payload["receipt_session_id"] = a.ReceiptSessionID
	f.result.Payload["unp"] = a.UNP
}
func (f *offlineV2Fixture) sync(events ...DeviceEventEnvelope) (SyncAck, error) {
	return f.svc.SyncBatchForTenant("tenant-1", f.batch(1, nil, events...))
}
func TestOfflineFinalizeV2RejectsIncompleteOrForeignAuthority(t *testing.T) {
	for _, name := range []string{"legacy", "open", "missing-auth", "foreign-tenant", "wrong-generation", "wrong-fiscal", "wrong-unp", "wrong-range", "wrong-digest", "wrong-pop", "wrong-operator-id", "wrong-operator-code", "wrong-location", "wrong-surrogate", "before-validity", "revoked-before", "forged-auth-signature"} {
		t.Run(name, func(t *testing.T) {
			f := newOfflineV2Fixture(t)
			a := f.acceptance()
			switch name {
			case "legacy":
				delete(f.accepted.Payload, "acceptance_evidence")
			case "open":
				a.ProofState = "OPEN_AUTHORIZATION"
			case "missing-auth":
				a.SessionEvidence.AuthorizationID = "missing"
			case "foreign-tenant":
				f.accepted.Payload["command"].(map[string]any)["tenant_id"] = "foreign"
			case "wrong-generation":
				a.Generation++
			case "wrong-fiscal":
				a.FiscalDeviceID = "foreign"
			case "wrong-unp":
				a.UNP = "CD123456-A001-0000011"
			case "wrong-range":
				a.Authority.RangeEnd++
			case "wrong-digest":
				a.PayloadSHA256 = strings64("a")
			case "wrong-pop":
				a.SessionEvidence.ClientProofSHA256 = strings64("b")
			case "wrong-operator-id":
				f.payload()["operator_id"] = "foreign"
				f.reseal()
				a = f.acceptance()
			case "wrong-operator-code":
				f.payload()["operator_code"] = "B002"
				f.reseal()
				a = f.acceptance()
			case "wrong-location":
				f.payload()["location_id"] = "foreign"
				f.reseal()
				a = f.acceptance()
			case "wrong-surrogate":
				v := "foreign"
				a.ClientSaleSurrogateID = &v
			case "before-validity":
				f.accepted.OccurredAt = f.at.Add(-time.Hour).Format(time.RFC3339)
			case "revoked-before":
				if err := f.repo.PutResource(ResourceRecord{Kind: "execution_authorization_revocation", ID: f.auth.AuthorizationID, TenantID: "tenant-1", Version: 1, Data: map[string]any{"effective_at": f.at.Format(time.RFC3339)}}); err != nil {
					t.Fatal(err)
				}
			case "forged-auth-signature":
				r := f.repo.resources[resourceKey("execution_authorization", f.auth.AuthorizationID)]
				r.Data["envelope"].(map[string]any)["signature"] = "forged"
				f.repo.resources[resourceKey(r.Kind, r.ID)] = r
			}
			if name != "legacy" {
				f.setAcceptance(a)
			}
			if _, err := f.sync(f.accepted); err == nil {
				t.Fatal("invalid authority accepted")
			}
			if _, ok := f.repo.LastSyncAck("tenant-1", "adapter-1"); ok {
				t.Fatal("invalid authority ACKed")
			}
			if len(f.repo.Sales("tenant-1")) != 0 {
				t.Fatal("invalid authority materialized")
			}
		})
	}
}
func TestOfflineFinalizeV2PlanAndResultGuards(t *testing.T) {
	for _, name := range []string{"duplicate-line", "duplicate-payment", "total-mismatch", "currency", "discount-currency", "result-receipt", "result-device", "result-digest", "missing-payment-evidence", "duplicate-payment-evidence", "amount-evidence", "missing-fiscal-reference", "card-not-approved", "failed-after-approved"} {
		t.Run(name, func(t *testing.T) {
			f := newOfflineV2Fixture(t)
			payload := f.payload()
			switch name {
			case "duplicate-line":
				lines := payload["items"].([]any)
				payload["items"] = append(lines, lines[0])
			case "duplicate-payment":
				p := payload["payments"].([]any)
				payload["payments"] = append(p, p[0])
			case "total-mismatch":
				payload["payments"].([]any)[0].(map[string]any)["amount"].(map[string]any)["amount"] = "2.00"
			case "currency":
				payload["currency"] = "USD"
			case "discount-currency":
				payload["items"].([]any)[0].(map[string]any)["discount"] = map[string]any{"amount": "0.10", "currency": "USD"}
			case "result-receipt":
				f.result.Payload["receipt_session_id"] = "foreign"
			case "result-device":
				f.result.DeviceID = "foreign"
			case "result-digest":
				f.result.Payload["payload_sha256"] = strings64("a")
			case "missing-payment-evidence":
				delete(f.result.Payload, "payment_evidence")
			case "duplicate-payment-evidence":
				p := f.result.Payload["payment_evidence"].([]any)
				f.result.Payload["payment_evidence"] = append(p, p[0])
			case "amount-evidence":
				f.result.Payload["payment_evidence"].([]any)[0].(map[string]any)["amount_minor"] = 251
			case "missing-fiscal-reference":
				delete(f.result.Payload, "fiscal_reference")
			case "card-not-approved", "failed-after-approved":
				payload["payments"].([]any)[0].(map[string]any)["type"] = "CARD"
				p := f.result.Payload["payment_evidence"].([]any)[0].(map[string]any)
				p["type"] = "CARD"
				p["state"] = "CARD_UNKNOWN"
				if name == "failed-after-approved" {
					f.result.EventType = "FAILED"
					p["state"] = "CARD_APPROVED"
				}
			}
			if name == "duplicate-line" || name == "duplicate-payment" || name == "total-mismatch" || name == "currency" || name == "discount-currency" || name == "card-not-approved" || name == "failed-after-approved" {
				f.reseal()
			}
			if _, err := f.sync(f.accepted, f.result); err == nil {
				t.Fatal("invalid plan/result accepted")
			}
			if len(f.repo.Sales("tenant-1")) != 0 {
				t.Fatal("partial invalid sale")
			}
			if _, ok := f.repo.LastSyncAck("tenant-1", "adapter-1"); ok {
				t.Fatal("invalid result ACKed")
			}
		})
	}
}
func TestOfflineFinalizeV2UnknownFinalAndLaterDuplicate(t *testing.T) {
	f := newOfflineV2Fixture(t)
	unknown := f.result
	unknown.Payload = edgeDeepMap(f.result.Payload)
	unknown.EventType = "UNKNOWN"
	unknown.Payload["state"] = "UNKNOWN"
	unknown.Payload["fiscal_reference"] = ""
	unknown.Payload["payment_evidence"].([]any)[0].(map[string]any)["state"] = "RESERVED"
	ack, err := f.sync(f.accepted, unknown)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.repo.Sales("tenant-1"); len(got) != 1 || got[0].State != "UNKNOWN" {
		t.Fatal(got)
	}
	prev := ack.CommittedEventHash
	ack, err = f.svc.SyncBatchForTenant("tenant-1", f.batch(3, &prev, f.result))
	if err != nil {
		t.Fatal(err)
	}
	sale := f.repo.Sales("tenant-1")[0]
	if sale.State != "COMPLETED" {
		t.Fatal(sale)
	}
	audits := len(f.repo.AuditEvents("tenant-1"))
	hooks := len(f.repo.PendingOutbox(time.Now().Add(time.Hour)))
	prev = ack.CommittedEventHash
	if _, err = f.svc.SyncBatchForTenant("tenant-1", f.batch(4, &prev, f.result)); err != nil {
		t.Fatal(err)
	}
	after := f.repo.Sales("tenant-1")[0]
	if after.Version != sale.Version || len(f.repo.AuditEvents("tenant-1")) != audits || len(f.repo.PendingOutbox(time.Now().Add(time.Hour))) != hooks {
		t.Fatal("later duplicate caused side effects")
	}
}
func TestOfflineFinalizeV2HistoricalAuthorizationAfterRebind(t *testing.T) {
	f := newOfflineV2Fixture(t)
	ack, err := f.sync(f.accepted)
	if err != nil {
		t.Fatal(err)
	}
	register, _ := f.repo.Resource("register", "register-1")
	register.Data["fiscal_device_id"] = "new-device"
	register.Data["adapter_device_id"] = "new-adapter"
	register.Version++
	if err = f.repo.PutResource(register); err != nil {
		t.Fatal(err)
	}
	prev := ack.CommittedEventHash
	if _, err = f.svc.SyncBatchForTenant("tenant-1", f.batch(2, &prev, f.result)); err != nil {
		t.Fatal("historical accepted route improperly rerouted", err)
	}
}
func TestExecutionAuthorizationIdentityAndImmutableGuards(t *testing.T) {
	f := newOfflineV2Fixture(t)
	der, _ := x509.MarshalPKIXPublicKey(&f.client.PublicKey)
	key := base64.RawURLEncoding.EncodeToString(der)
	for _, actor := range []string{"", "foreign"} {
		if _, err := f.svc.IssueExecutionAuthorization("session-1", key, "tenant-1", actor); err == nil {
			t.Fatal("foreign actor issued authority")
		}
	}
	if _, err := f.svc.IssueExecutionAuthorization("session-1", key, "foreign", "subject-1"); err == nil {
		t.Fatal("foreign tenant issued authority")
	}
	record, _ := f.repo.Resource("execution_authorization", f.auth.AuthorizationID)
	record.Version++
	record.Data["injected"] = "change"
	if f.repo.PutResource(record) == nil {
		t.Fatal("issued authorization mutated")
	}
	binding, _ := f.repo.Resource("issued_binding", f.auth.BindingSHA256)
	binding.Version++
	binding.Data["injected"] = "change"
	if f.repo.PutResource(binding) == nil {
		t.Fatal("issued binding mutated")
	}
}

type edgeFaultStore struct {
	*testStore
	saveFail, loadFail bool
}

func (s *edgeFaultStore) Save(b []byte) error {
	if s.saveFail {
		return errors.New("injected sync commit failure")
	}
	return s.testStore.Save(b)
}
func (s *edgeFaultStore) Load() ([]byte, error) {
	if s.loadFail {
		return nil, errors.New("injected reload failure")
	}
	return s.testStore.Load()
}
func TestOfflineFinalizeV2CommitFailureRollsBackMemoryAndPersistence(t *testing.T) {
	for _, loadFailure := range []bool{false, true} {
		t.Run(fmt.Sprint(loadFailure), func(t *testing.T) {
			f := newOfflineV2Fixture(t)
			ack, err := f.sync(f.accepted)
			if err != nil {
				t.Fatal(err)
			}
			beforeAudit := len(f.repo.AuditEvents("tenant-1"))
			fault := &edgeFaultStore{f.store, true, loadFailure}
			f.repo.store = fault
			previous := ack.CommittedEventHash
			batch := f.batch(2, &previous, f.result)
			if _, err = f.svc.SyncBatchForTenant("tenant-1", batch); err == nil {
				t.Fatal("failed commit ACKed")
			}
			if loadFailure {
				if _, ok := f.repo.LastSyncAck("tenant-1", "adapter-1"); ok {
					t.Fatal("uncertain ACK exposed as authoritative")
				}
				if len(f.repo.Sales("tenant-1")) != 0 || len(f.repo.AuditEvents("tenant-1")) != 0 || len(f.repo.PendingOutbox(time.Now().Add(time.Hour))) != 0 {
					t.Fatal("unavailable repository exposed projections")
				}
				if _, err := f.repo.EdgePendingCommand(f.accepted.OperationID, "tenant-1"); !errors.Is(err, ErrRepositoryUnavailable) {
					t.Fatal("uncertain pending state exposed", err)
				}
				f.repo.mu.Lock()
				badSnapshot := f.repo.syncAcks["tenant-1\nadapter-1"].CommittedThroughSeq != 1 || len(f.repo.audit) != beforeAudit || len(f.repo.sales) != 0 || f.repo.edgePending[f.accepted.OperationID].ResultDigest != ""
				f.repo.mu.Unlock()
				if badSnapshot {
					t.Fatal("confirmed snapshot was not restored")
				}
			} else {
				if got, _ := f.repo.LastSyncAck("tenant-1", "adapter-1"); got.CommittedThroughSeq != 1 {
					t.Fatal("dirty ACK survived")
				}
				if len(f.repo.Sales("tenant-1")) != 0 || len(f.repo.AuditEvents("tenant-1")) != beforeAudit || len(f.repo.PendingOutbox(time.Now().Add(time.Hour))) != 0 {
					t.Fatal("dirty projection survived")
				}
				pending, _ := f.repo.EdgePendingCommand(f.accepted.OperationID, "tenant-1")
				if pending.ResultDigest != "" {
					t.Fatal("dirty result survived")
				}
			}
			reopened, _ := NewPersistentRepository(f.store)
			if len(reopened.Sales("tenant-1")) != 0 {
				t.Fatal("partial persisted sale")
			}
			fault.saveFail = false
			fault.loadFail = false
			if loadFailure {
				if _, err = f.svc.SyncBatchForTenant("tenant-1", batch); !errors.Is(err, ErrRepositoryUnavailable) {
					t.Fatal("recovery retry fence absent", err)
				}
			}
			if _, err = f.svc.SyncBatchForTenant("tenant-1", batch); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type edgeCommitBarrier struct {
	Repository
	arrived chan struct{}
	release chan struct{}
}

func (r *edgeCommitBarrier) CommitEdgeSync(tenant string, ack SyncAck, sales []Sale, operations []Operation, artifacts map[string][]byte, outbox []OutboxItem, pending []EdgePendingCommand, completed []string, expected EdgeSyncExpectation) error {
	r.arrived <- struct{}{}
	<-r.release
	return r.Repository.CommitEdgeSync(tenant, ack, sales, operations, artifacts, outbox, pending, completed, expected)
}
func TestOfflineFinalizeV2ConcurrentCommitCAS(t *testing.T) {
	f := newOfflineV2Fixture(t)
	barrier := &edgeCommitBarrier{f.repo, make(chan struct{}, 2), make(chan struct{})}
	f.svc.repo = barrier
	batch := f.batch(1, nil, f.accepted, f.result)
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := f.svc.SyncBatchForTenant("tenant-1", batch); results <- err }()
	}
	<-barrier.arrived
	<-barrier.arrived
	close(barrier.release)
	errorsCount := 0
	for i := 0; i < 2; i++ {
		if <-results != nil {
			errorsCount++
		}
	}
	if errorsCount != 1 {
		t.Fatalf("expected one CAS rejection, got %d", errorsCount)
	}
	if len(f.repo.Sales("tenant-1")) != 1 || len(f.repo.PendingOutbox(time.Now().Add(time.Hour))) != 1 {
		t.Fatal("concurrent duplicated projection")
	}
}
func TestOfflineFinalizeV2RejectsReusedReceiptUNPAndTender(t *testing.T) {
	for _, reuse := range []string{"receipt", "unp", "payment"} {
		t.Run(reuse, func(t *testing.T) {
			f := newOfflineV2Fixture(t)
			if _, err := f.sync(f.accepted); err != nil {
				t.Fatal(err)
			}
			next := f.accepted
			next.Payload = edgeDeepMap(f.accepted.Payload)
			next.OperationID = "10000000-0000-4000-8000-000000000009"
			next.EventID = "received-" + next.OperationID
			command := next.Payload["command"].(map[string]any)
			command["operation_id"] = next.OperationID
			a, _ := parseAcceptanceEvidence(next)
			if reuse != "receipt" {
				a.ReceiptSessionID = "10000000-0000-4000-8000-000000000008"
			}
			if reuse != "unp" {
				a.UNP = "AB123456-A001-0000012"
				next.Payload["unp_sequence"] = int64(12)
			}
			surrogate := "different-surrogate"
			a.ClientSaleSurrogateID = &surrogate
			p := command["payload"].(map[string]any)
			p["external_id"] = surrogate
			p["server_sale_id"] = surrogate
			if reuse != "payment" {
				p["payments"].([]any)[0].(map[string]any)["payment_id"] = "20000000-0000-4000-8000-000000000009"
			}
			a.PayloadSHA256 = edgeDigest(p)
			a.SessionEvidence.ClientProofSHA256 = edgeDigest(executionIntentProof(*a, next.OperationID))
			next.Payload["acceptance_evidence"] = asMap(a)
			ack, _ := f.repo.LastSyncAck("tenant-1", "adapter-1")
			prev := ack.CommittedEventHash
			if _, err := f.svc.SyncBatchForTenant("tenant-1", f.batch(2, &prev, next)); err == nil {
				t.Fatal("reused " + reuse + " accepted")
			}
			if current, _ := f.repo.LastSyncAck("tenant-1", "adapter-1"); current.CommittedThroughSeq != 1 {
				t.Fatal("conflict advanced cursor")
			}
		})
	}
}

// Run against the separately compiled actual firmware IntentProcessor/SQLite
// producer. The oracle is exported by actual Go issuance, never a mock signer.
func TestOfflineFinalizeV2ActualFirmwareConsumerInterop(t *testing.T) {
	dir := os.Getenv("EXECUTION_AUTH_ACTUAL_DIR")
	if dir == "" {
		t.Skip("set EXECUTION_AUTH_ACTUAL_DIR to actual firmware oracle directory")
	}
	f := newOfflineV2Fixture(t)
	var oracle struct {
		Authorization ExecutionAuthorizationEnvelope `json:"authorization"`
		Trust         string                         `json:"trust_key_der_base64"`
		Now           int64                          `json:"now"`
	}
	b, err := os.ReadFile(filepath.Join(dir, "oracle.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &oracle); err != nil {
		t.Fatal(err)
	}
	der, err := base64.RawURLEncoding.DecodeString(oracle.Trust)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		t.Fatal(err)
	}
	f.issuer.ca.PublicKey = key
	f.issuer.ca.RawSubjectPublicKeyInfo = der
	f.envelope = oracle.Authorization
	raw, _ := base64.RawURLEncoding.DecodeString(f.envelope.Payload)
	if err = json.Unmarshal(raw, &f.auth); err != nil {
		t.Fatal(err)
	}
	f.repo.resources[resourceKey("execution_authorization", f.auth.AuthorizationID)] = ResourceRecord{ID: f.auth.AuthorizationID, Kind: "execution_authorization", TenantID: f.auth.TenantID, Version: 1, Data: map[string]any{"envelope": asMap(f.envelope)}}
	var produced struct {
		Accepted map[string]any `json:"accepted"`
		Result   map[string]any `json:"result"`
	}
	b, err = os.ReadFile(filepath.Join(dir, "actual-events.json"))
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.UseNumber()
	if err = dec.Decode(&produced); err != nil {
		t.Fatal(err)
	}
	f.accepted.Payload = produced.Accepted
	f.accepted.OccurredAt = time.Unix(oracle.Now, 0).UTC().Format(time.RFC3339Nano)
	f.result.Payload = produced.Result
	f.result.OccurredAt = time.Unix(oracle.Now+1, 0).UTC().Format(time.RFC3339Nano)
	if _, err = f.sync(f.accepted, f.result); err != nil {
		t.Fatal(err)
	}
	sales := f.repo.Sales("tenant-1")
	if len(sales) != 1 || sales[0].State != "COMPLETED" || sales[0].UNP != "AB123456-A001-0000001" {
		t.Fatalf("actual firmware projection: %#v", sales)
	}
}
