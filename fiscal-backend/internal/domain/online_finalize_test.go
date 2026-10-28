package domain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func onlineFixture(t *testing.T) *offlineV2Fixture {
	f := newOfflineV2Fixture(t)
	var plan offlineSalePayload
	b, _ := json.Marshal(f.payload())
	_ = json.Unmarshal(b, &plan)
	sale := Sale{ID: "online-sale", TenantID: "tenant-1", ExternalID: plan.ExternalID, LocationID: "location-1", RegisterID: "register-1", OperatorID: "A001", OperatorCode: "A001", UNP: "AB123456-A001-0005000", State: "PAYMENT_PENDING", Version: 3, Lines: plan.Items, Payments: []PaymentRecord{{PaymentID: plan.Payments[0].PaymentID, Type: "CASH", Amount: plan.Payments[0].Amount}}, FiscalOperationID: f.accepted.OperationID, FiscalDevice: FiscalDeviceSnapshot{AdapterDeviceID: "adapter-1", BindingGeneration: 7, DeviceID: "fiscal-1", FiscalDeviceNumber: "AB123456", FiscalMemoryNumber: "12345678"}, CreatedAt: f.at, UpdatedAt: f.at}
	if err := f.repo.PutSale(sale); err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: f.accepted.OperationID, TenantID: sale.TenantID, SaleID: sale.ID, Type: "SALE_FINALIZE", State: "EXECUTING", Version: 1, ReceiptSessionID: f.acceptance().ReceiptSessionID, CreatedAt: f.at, UpdatedAt: f.at}
	if err := f.repo.PutOperation(op); err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&f.client.PublicKey)
	var err error
	f.envelope, err = f.svc.IssueExecutionAuthorization("session-1", base64.RawURLEncoding.EncodeToString(der), "tenant-1", "subject-1", sale.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(f.envelope.Payload)
	_ = json.Unmarshal(raw, &f.auth)
	payload, err := BuildFinalizePayload(sale, f.auth.OperatorID, f.auth.OperatorCode)
	if err != nil {
		t.Fatal(err)
	}
	f.accepted.Payload["command"].(map[string]any)["payload"] = payload
	f.accepted.Payload["unp_sequence"] = int64(0)
	a := f.acceptance()
	id := sale.ID
	a.ServerSaleID = &id
	a.UNP = sale.UNP
	a.SessionEvidence.AuthorizationID = f.auth.AuthorizationID
	a.SessionEvidence.AuthorizationSHA256 = f.envelope.SHA256
	f.setAcceptance(a)
	f.reseal()
	f.command["server_sale_id"] = sale.ID
	f.command["canonical_payload"] = payload
	f.command["payload_sha256"] = f.acceptance().PayloadSHA256
	f.command["execution_authorization"] = asMap(f.envelope)
	transcript, _ := edgeCanonical(executionIntentProof(*f.acceptance(), f.accepted.OperationID))
	digest := sha256.Sum256(append([]byte(executionIntentProofDomain), transcript...))
	sig, _ := ecdsa.SignASN1(rand.Reader, f.client, digest[:])
	f.command["client_proof"] = map[string]any{"algorithm": "ES256_DER", "signature": base64.RawURLEncoding.EncodeToString(sig)}
	return f
}
func TestOnlineFinalizeAuthoritativeMerge(t *testing.T) {
	f := onlineFixture(t)
	if _, err := f.sync(f.accepted, f.result); err != nil {
		t.Fatal(err)
	}
	sale, _ := f.repo.Sale("online-sale")
	if sale.State != "COMPLETED" || sale.Version != 4 || sale.UNP != "AB123456-A001-0005000" {
		t.Fatal(sale)
	}
}
func TestOnlineFinalizeRejectsClientChangedAuthority(t *testing.T) {
	for _, name := range []string{"line", "tender", "operation", "receipt", "unp", "source", "sequence"} {
		t.Run(name, func(t *testing.T) {
			f := onlineFixture(t)
			switch name {
			case "line":
				f.payload()["items"].([]any)[0].(map[string]any)["name"] = "changed"
			case "tender":
				f.payload()["payments"].([]any)[0].(map[string]any)["type"] = "CARD"
			case "operation":
				f.accepted.OperationID = "10000000-0000-4000-8000-000000000099"
				f.accepted.Payload["command"].(map[string]any)["operation_id"] = f.accepted.OperationID
			case "receipt":
				a := f.acceptance()
				a.ReceiptSessionID = "10000000-0000-4000-8000-000000000098"
				f.setAcceptance(a)
			case "unp":
				a := f.acceptance()
				a.UNP = "AB123456-A001-0005001"
				f.setAcceptance(a)
				f.payload()["unp"] = a.UNP
			case "source":
				f.payload()["unp_source"] = "LEASED"
			case "sequence":
				f.accepted.Payload["unp_sequence"] = int64(5000)
			}
			f.reseal()
			if _, err := f.sync(f.accepted); err == nil {
				t.Fatal("changed authority accepted")
			}
			sale, _ := f.repo.Sale("online-sale")
			if sale.State != "PAYMENT_PENDING" {
				t.Fatal("dirty sale")
			}
		})
	}
}
func TestOnlineFinalizeExportsActualIssuerFixture(t *testing.T) {
	dir := os.Getenv("EXECUTION_AUTH_ONLINE_FIXTURE_DIR")
	if dir == "" {
		t.Skip("optional actual firmware fixture export")
	}
	f := onlineFixture(t)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(f.issuer.ca.PublicKey)
	fixture := map[string]any{"authorization": f.envelope, "command": f.command, "binding": f.repo.resources[resourceKey("issued_binding", f.auth.BindingSHA256)].Data["canonical_payload"], "trust_key_der_base64": base64.RawURLEncoding.EncodeToString(der), "now": f.at.Unix(), "acceptance_event": f.accepted, "result_event": f.result}
	b, _ := json.MarshalIndent(fixture, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "oracle.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestOnlineFinalizeActualFirmwareConsumerInterop(t *testing.T) {
	oraclePath := os.Getenv("EXECUTION_AUTH_ONLINE_ACTUAL_ORACLE")
	eventsPath := os.Getenv("EXECUTION_AUTH_ONLINE_ACTUAL_EVENTS")
	if oraclePath == "" || eventsPath == "" {
		t.Skip("provide actual firmware frozen oracle/events")
	}
	f := onlineFixture(t)
	var oracle struct {
		Authorization ExecutionAuthorizationEnvelope `json:"authorization"`
		Trust         string                         `json:"trust_key_der_base64"`
		Now           int64                          `json:"now"`
	}
	b, err := os.ReadFile(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &oracle); err != nil {
		t.Fatal(err)
	}
	der, _ := base64.RawURLEncoding.DecodeString(oracle.Trust)
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
	b, err = os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
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
	sale, _ := f.repo.Sale("online-sale")
	if sale.State != "COMPLETED" || sale.UNP != "AB123456-A001-0005000" {
		t.Fatal(sale)
	}
}
func mqttOnlineFixture(t *testing.T) *offlineV2Fixture {
	f := onlineFixture(t)
	a := f.acceptance()
	a.Ingress = "MQTT"
	a.ProofState = "SERVER_COMMAND"
	a.SessionEvidence = nil
	a.ClientSaleSurrogateID = nil
	a.UNPSource = "SERVER_ASSIGNED"
	f.setAcceptance(a)
	body, _ := edgeCanonical(map[string]any{"operation_id": f.accepted.OperationID, "receipt_session_id": a.ReceiptSessionID, "tenant_id": "tenant-1", "register_id": "register-1", "device_id": "adapter-1", "command_type": "SALE_FINALIZE", "fencing_token": int64(7), "payload": f.payload(), "payload_sha256": a.PayloadSHA256, "issued_at": f.at.Add(-time.Second).Format(time.RFC3339Nano), "expires_at": f.at.Add(time.Minute).Format(time.RFC3339Nano)})
	if err := f.repo.PutResource(ResourceRecord{ID: f.accepted.OperationID, Kind: "device_command_outbox", TenantID: "tenant-1", Version: 1, Data: map[string]any{"device_id": "adapter-1", "topic": "tenants/tenant-1/devices/adapter-1/commands", "body": string(body)}}); err != nil {
		t.Fatal(err)
	}
	return f
}
func TestOnlineFinalizeMQTTUsesImmutablePreparedAuthority(t *testing.T) {
	f := mqttOnlineFixture(t)
	if _, err := f.sync(f.accepted, f.result); err != nil {
		t.Fatal(err)
	}
	sale, _ := f.repo.Sale("online-sale")
	if sale.State != "COMPLETED" {
		t.Fatal(sale)
	}
	for _, field := range []string{"body", "device_id", "topic"} {
		record, _ := f.repo.Resource("device_command_outbox", f.accepted.OperationID)
		record.Data[field] = "changed"
		record.Version++
		if err := f.repo.PutResource(record); err == nil {
			t.Fatal("prepared authority mutable", field)
		}
	}
}
func TestOnlineFinalizeMQTTRejectsForeignOrChangedCommand(t *testing.T) {
	for _, name := range []string{"line", "hash", "receipt", "sale", "binding", "sequence", "surrogate"} {
		t.Run(name, func(t *testing.T) {
			f := mqttOnlineFixture(t)
			a := f.acceptance()
			switch name {
			case "line":
				f.payload()["items"].([]any)[0].(map[string]any)["name"] = "wrong"
				a.PayloadSHA256 = edgeDigest(f.payload())
			case "hash":
				a.PayloadSHA256 = string(make([]byte, 64))
			case "receipt":
				a.ReceiptSessionID = "other"
			case "sale":
				id := "foreign"
				a.ServerSaleID = &id
			case "binding":
				a.Generation++
			case "sequence":
				f.accepted.Payload["unp_sequence"] = int64(5000)
			case "surrogate":
				id := "unexpected"
				a.ClientSaleSurrogateID = &id
			}
			f.setAcceptance(a)
			if _, err := f.sync(f.accepted); err == nil {
				t.Fatal("wrong MQTT context accepted")
			}
		})
	}
}

type mutatingAuthorizationSigner struct {
	*X509DeviceCredentialIssuer
	mutate func()
}

func (s *mutatingAuthorizationSigner) SignExecutionAuthorization(b []byte) (string, string, error) {
	s.mutate()
	return s.X509DeviceCredentialIssuer.SignExecutionAuthorization(b)
}
func TestOnlineFinalizeIssuanceRequiresTrustedPendingContext(t *testing.T) {
	for _, name := range []string{"foreign-tenant", "wrong-register", "wrong-operator", "wrong-device", "wrong-state", "lease-overlap", "missing-operation", "wrong-operation-type", "missing-receipt"} {
		t.Run(name, func(t *testing.T) {
			f := onlineFixture(t)
			sale, _ := f.repo.Sale("online-sale")
			op, _ := f.repo.Operation(f.accepted.OperationID)
			switch name {
			case "foreign-tenant":
				sale.TenantID = "foreign"
			case "wrong-register":
				sale.RegisterID = "other"
			case "wrong-operator":
				sale.OperatorCode = "B002"
			case "wrong-device":
				sale.FiscalDevice.DeviceID = "other"
			case "wrong-state":
				sale.State = "COMPLETED"
			case "lease-overlap":
				sale.UNP = "AB123456-A001-0000010"
			case "missing-operation":
				sale.FiscalOperationID = "missing"
			case "wrong-operation-type":
				op.Type = "REVERSAL"
			case "missing-receipt":
				op.ReceiptSessionID = ""
			}
			sale.Version++
			if err := f.repo.PutSale(sale); err != nil {
				t.Fatal(err)
			}
			op.Version++
			if err := f.repo.PutOperation(op); err != nil {
				t.Fatal(err)
			}
			der, _ := x509.MarshalPKIXPublicKey(&f.client.PublicKey)
			if _, err := f.svc.IssueExecutionAuthorization("session-1", base64.RawURLEncoding.EncodeToString(der), "tenant-1", "subject-1", "online-sale"); err == nil {
				t.Fatal("untrusted pending context issued")
			}
		})
	}
}
func TestOnlineFinalizeIssuanceConcurrentChangeFailsClosed(t *testing.T) {
	for _, which := range []string{"sale", "operation"} {
		t.Run(which, func(t *testing.T) {
			f := onlineFixture(t)
			before := len(f.repo.Resources("execution_authorization", "tenant-1"))
			f.svc.SetDeviceCredentialIssuer(&mutatingAuthorizationSigner{f.issuer, func() {
				if which == "sale" {
					sale, _ := f.repo.Sale("online-sale")
					sale.Version++
					sale.State = "COMPLETED"
					if err := f.repo.PutSale(sale); err != nil {
						t.Fatal(err)
					}
				} else {
					op, _ := f.repo.Operation(f.accepted.OperationID)
					op.Version++
					op.State = "FAILED"
					if err := f.repo.PutOperation(op); err != nil {
						t.Fatal(err)
					}
				}
			}})
			der, _ := x509.MarshalPKIXPublicKey(&f.client.PublicKey)
			if _, err := f.svc.IssueExecutionAuthorization("session-1", base64.RawURLEncoding.EncodeToString(der), "tenant-1", "subject-1", "online-sale"); err == nil {
				t.Fatal("stale authorization issued")
			}
			if len(f.repo.Resources("execution_authorization", "tenant-1")) != before {
				t.Fatal("stale authorization persisted")
			}
		})
	}
}

func TestOnlineFinalizeActualMQTTPreparedFirmwareConsumer(t *testing.T) {
	fixturePath := os.Getenv("EXECUTION_AUTH_MQTT_ACTUAL_ORACLE")
	eventsPath := os.Getenv("EXECUTION_AUTH_MQTT_ACTUAL_EVENTS")
	if fixturePath == "" || eventsPath == "" {
		t.Skip("provide actual Prepare and firmware artifacts")
	}
	f := mqttOnlineFixture(t)
	var fixture struct {
		Prepared ResourceRecord `json:"prepared_command"`
		Command  struct {
			IssuedAt time.Time `json:"issued_at"`
		} `json:"command"`
	}
	b, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &fixture); err != nil {
		t.Fatal(err)
	}
	f.repo.resources[resourceKey("device_command_outbox", f.accepted.OperationID)] = fixture.Prepared
	var events struct {
		Accepted map[string]any `json:"accepted"`
		Result   map[string]any `json:"result"`
	}
	b, err = os.ReadFile(eventsPath)
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err = d.Decode(&events); err != nil {
		t.Fatal(err)
	}
	f.accepted.Payload = events.Accepted
	f.result.Payload = events.Result
	f.accepted.OccurredAt = fixture.Command.IssuedAt.Add(time.Millisecond).Format(time.RFC3339Nano)
	f.result.OccurredAt = fixture.Command.IssuedAt.Add(time.Second).Format(time.RFC3339Nano)
	if _, err = f.sync(f.accepted, f.result); err != nil {
		t.Fatal(err)
	}
	sale, _ := f.repo.Sale("online-sale")
	if sale.State != "COMPLETED" || sale.UNP != "AB123456-A001-0005000" {
		t.Fatal(sale)
	}
}
func TestOnlineFinalizeMQTTValidityIntervalIsExact(t *testing.T) {
	for _, name := range []string{"before-issued", "at-issued", "before-expiry", "at-expiry", "after-expiry", "missing-issued", "malformed-expiry", "reversed"} {
		t.Run(name, func(t *testing.T) {
			f := mqttOnlineFixture(t)
			record, _ := f.repo.Resource("device_command_outbox", f.accepted.OperationID)
			var cmd map[string]any
			_ = json.Unmarshal([]byte(stringAny(record.Data, "body")), &cmd)
			issued, _ := time.Parse(time.RFC3339Nano, stringAny(cmd, "issued_at"))
			expires, _ := time.Parse(time.RFC3339Nano, stringAny(cmd, "expires_at"))
			valid := false
			switch name {
			case "before-issued":
				f.accepted.OccurredAt = issued.Add(-time.Nanosecond).Format(time.RFC3339Nano)
			case "at-issued":
				f.accepted.OccurredAt = issued.Format(time.RFC3339Nano)
				valid = true
			case "before-expiry":
				f.accepted.OccurredAt = expires.Add(-time.Nanosecond).Format(time.RFC3339Nano)
				valid = true
			case "at-expiry":
				f.accepted.OccurredAt = expires.Format(time.RFC3339Nano)
			case "after-expiry":
				f.accepted.OccurredAt = expires.Add(time.Nanosecond).Format(time.RFC3339Nano)
			case "missing-issued":
				delete(cmd, "issued_at")
			case "malformed-expiry":
				cmd["expires_at"] = "not-time"
			case "reversed":
				cmd["expires_at"] = issued.Add(-time.Second).Format(time.RFC3339Nano)
			}
			body, _ := json.Marshal(cmd)
			record.Data["body"] = string(body)
			f.repo.resources[resourceKey(record.Kind, record.ID)] = record
			_, err := f.sync(f.accepted)
			if valid && err != nil {
				t.Fatal(err)
			}
			if !valid && err == nil {
				t.Fatal("invalid command interval accepted")
			}
		})
	}
}
