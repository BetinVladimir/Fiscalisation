package mqttclient

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fiscalisation/fiscal-backend/internal/domain"
	"fmt"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type fakeSync struct {
	tenant string
	batch  domain.EdgeSyncBatch
	err    error
}

func (f *fakeSync) ActivateDeviceCredential(id, credential, nonce, signature string) (domain.DeviceActivationRequest, error) {
	return domain.DeviceActivationRequest{ID: id, DeviceInstanceID: "device-1", State: "ACTIVE", BindingVersion: 3, ClaimedTenantID: "tenant-1", ClaimedLocationID: "location-1", ClaimedRegisterID: "register-1", ClaimedRoles: []string{"FISCAL_DEVICE"}}, f.err
}
func (f *fakeSync) SignDeviceActivationAcknowledgement(unsigned []byte) (string, error) {
	return base64.RawURLEncoding.EncodeToString(append([]byte("test-signature:"), unsigned...)), f.err
}

type publishToken struct{ err error }

func (t publishToken) Wait() bool                     { return true }
func (t publishToken) WaitTimeout(time.Duration) bool { return true }
func (t publishToken) Error() error                   { return t.err }
func (t publishToken) Done() <-chan struct{}          { ch := make(chan struct{}); close(ch); return ch }

type captureClient struct {
	mqtt.Client
	topic string
	body  []byte
}

func (c *captureClient) IsConnected() bool { return true }
func (c *captureClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	c.topic = topic
	c.body = append([]byte(nil), payload.([]byte)...)
	return publishToken{}
}

func TestBridgeCommandUsesRecursivelyCanonicalSignedPayload(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	client := &captureClient{}
	bridge := NewBridge(key)
	bridge.bind(client)
	sale := domain.Sale{ID: "sale-1", TenantID: "tenant-1", ExternalID: "external-1", LocationID: "loc-1", RegisterID: "reg-1", OperatorID: "operator-1", UNP: "DY000600-OP01-0000001", Lines: []domain.SaleLine{{LineID: "line-1", Name: "Coffee", Quantity: "1.000", UnitPrice: domain.Money{Amount: "2.65", Currency: "EUR"}, TaxGroup: "B"}}, FiscalDevice: domain.FiscalDeviceSnapshot{DeviceID: "device-1", BindingVersion: 7}}
	payment := domain.PaymentRequest{PaymentID: "pay-1", Type: "CARD", Amount: domain.Money{Amount: "2.65", Currency: "EUR"}}
	if err := bridge.Queue(domain.Operation{ID: "op-1", Type: "FISCAL_SALE"}, sale, payment); err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(client.body, &root); err != nil {
		t.Fatal(err)
	}
	signature := root["signature"].(string)
	delete(root, "signature")
	unsigned, _ := json.Marshal(root)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(unsigned)
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if signature != expected {
		t.Fatal("command signature is not canonical")
	}
	payload, _ := json.Marshal(root["payload"])
	sum := sha256.Sum256(payload)
	if root["payload_sha256"] != fmt.Sprintf("%x", sum) {
		t.Fatal("payload digest mismatch")
	}
}

func TestBridgeBuildsDatecsReversalPayload(t *testing.T) {
	bridge := NewBridge("0123456789abcdef0123456789abcdef")
	sale := domain.Sale{ID: "sale-1", TenantID: "tenant-1", ExternalID: "external-1", LocationID: "loc-1", RegisterID: "reg-1", OperatorID: "A001", UNP: "DY000600-OP01-0000001", FiscalOperationID: "original-op", Lines: []domain.SaleLine{{LineID: "line-1", Name: "Coffee", Quantity: "1.000", UnitPrice: domain.Money{Amount: "2.65", Currency: "EUR"}, TaxGroup: "B"}}, Payments: []domain.PaymentRecord{{PaymentID: "pay-1", Type: "CARD", Amount: domain.Money{Amount: "2.65", Currency: "EUR"}}}, FiscalDevice: domain.FiscalDeviceSnapshot{DeviceID: "device-1", BindingVersion: 7, FiscalMemoryNumber: "12345678"}}
	op := domain.Operation{ID: "reversal-op", Type: "REVERSAL", ReasonCode: "CUSTOMER_RETURN", OriginalDocumentNumber: 269, OriginalDocumentAt: time.Date(2026, 8, 11, 9, 30, 45, 0, time.UTC)}
	command, err := bridge.Prepare(op, sale, domain.PaymentRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err = json.Unmarshal([]byte(command.Data["body"].(string)), &envelope); err != nil {
		t.Fatal(err)
	}
	payload := envelope["payload"].(map[string]any)
	document := payload["original_document"].(map[string]any)
	if envelope["command_type"] != "REVERSAL" || payload["original_operation_id"] != "original-op" || document["document_number"] != float64(269) || document["document_datetime"] != "11-08-26 12:30:45" || document["fiscal_memory_number"] != "12345678" {
		t.Fatalf("invalid reversal envelope: %#v", envelope)
	}
}

func TestBridgeFlushesDurableExecutingCommandAfterReconnect(t *testing.T) {
	repo := domain.NewMemoryRepository()
	now := time.Now().UTC()
	op := domain.Operation{ID: "op-reconnect", TenantID: "tenant-1", Type: "FISCAL_SALE", State: "EXECUTING", Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := repo.PutOperation(op); err != nil {
		t.Fatal(err)
	}
	command := domain.ResourceRecord{Kind: "device_command_outbox", TenantID: "tenant-1", ID: op.ID, Version: 1, Data: map[string]any{"topic": "tenants/tenant-1/devices/device-1/commands", "body": "{\"operation_id\":\"op-reconnect\"}", "expires_at": now.Add(time.Hour).Format(time.RFC3339Nano)}, CreatedAt: now, UpdatedAt: now}
	if err := repo.PutResource(command); err != nil {
		t.Fatal(err)
	}
	client := &captureClient{}
	bridge := NewBridge("0123456789abcdef0123456789abcdef", repo)
	bridge.bind(client)
	if err := bridge.FlushOutbox(); err != nil {
		t.Fatal(err)
	}
	if client.topic != command.Data["topic"] || string(client.body) != command.Data["body"] {
		t.Fatalf("durable command not republished: %q %s", client.topic, client.body)
	}
}

func TestBridgePublishesCompositeBindingOnlyToAdapterControlTopic(t *testing.T) {
	client := &captureClient{}
	bridge := NewBridge("0123456789abcdef0123456789abcdef")
	bridge.bind(client)
	body := []byte("BFPE-signed-envelope")
	if err := bridge.PublishCompositeBinding("tenant-1", "adapter-1", body); err != nil {
		t.Fatal(err)
	}
	if client.topic != "tenants/tenant-1/devices/adapter-1/bindings" || string(client.body) != string(body) {
		t.Fatal(client.topic, string(client.body))
	}
}

type bindingSync struct {
	fakeSync
	tenant, adapter, register string
	generation                int64
}

func (b *bindingSync) ApplyCompositeBindingByAdapter(tenant, adapter, register string, generation int64) (map[string]any, error) {
	b.tenant, b.adapter, b.register, b.generation = tenant, adapter, register, generation
	return map[string]any{"status": "ACTIVE", "applied_generation": generation}, nil
}
func TestProcessorConsumesAuthenticatedAdapterBindingAck(t *testing.T) {
	svc := &bindingSync{}
	processor := NewProcessor(svc)
	topic, body, err := processor.Process("tenants/tenant-1/devices/adapter-1/bindings/acks", []byte(`{"adapter_device_id":"adapter-1","register_id":"register-1","generation":7}`))
	if err != nil || topic != "tenants/tenant-1/devices/adapter-1/bindings/acks/confirmed" || svc.generation != 7 || svc.tenant != "tenant-1" {
		t.Fatal(topic, string(body), svc, err)
	}
}

func (f *fakeSync) SyncBatchForTenant(t string, b domain.EdgeSyncBatch) (domain.SyncAck, error) {
	f.tenant = t
	f.batch = b
	return domain.SyncAck{AckID: "ack-1", EdgeID: b.EdgeID, CommittedThroughSeq: b.LastSeq}, f.err
}

func TestProcessorBindsTopicAndPublishesBusinessAck(t *testing.T) {
	f := &fakeSync{}
	p := NewProcessor(f)
	body, _ := json.Marshal(domain.EdgeSyncBatch{EdgeID: "device-1", SchemaVersion: "2026-08-07", FirstSeq: 1, LastSeq: 1})
	topic, ack, err := p.Process("tenants/tenant-1/devices/device-1/sync/batches/batch-1", body)
	if err != nil || topic != "tenants/tenant-1/devices/device-1/sync/acks/batch-1" || f.tenant != "tenant-1" || len(ack) == 0 {
		t.Fatalf("unexpected result %q %s %v", topic, ack, err)
	}
}
func TestProcessorRejectsCrossDeviceAndUnknownTopics(t *testing.T) {
	p := NewProcessor(&fakeSync{})
	body, _ := json.Marshal(domain.EdgeSyncBatch{EdgeID: "other"})
	if _, _, err := p.Process("tenants/t/devices/d/sync/batches/b", body); err == nil {
		t.Fatal("cross-device batch accepted")
	}
	if _, _, err := p.Process("tenants/t/devices/d/events", body); err == nil {
		t.Fatal("unknown topic accepted")
	}
}
func TestProcessorBindsActivationToDeviceTopicAndSignsAck(t *testing.T) {
	p := NewProcessor(&fakeSync{}, "01234567890123456789012345678901")
	payload := []byte(`{"activation_request_id":"request-1","credential_id":"credential-1","nonce":"nonce-1","signature":"proof"}`)
	topic, body, err := p.Process("beefiscal/v1/devices/device-1/activation", payload)
	if err != nil || topic != "beefiscal/v1/devices/device-1/activation/ack" {
		t.Fatal(topic, string(body), err)
	}
	var ack map[string]any
	if json.Unmarshal(body, &ack) != nil || ack["state"] != "ACTIVE" || ack["signature"] == "" {
		t.Fatal(string(body))
	}
	if _, _, err = p.Process("beefiscal/v1/devices/other-device/activation", payload); err == nil {
		t.Fatal("cross-device activation accepted")
	}
}

func TestBridgeUsesInternalSaleIDForEdgeMaterialization(t *testing.T) {
	bridge := NewBridge("0123456789abcdef0123456789abcdef")
	for _, kind := range []string{"FISCAL_SALE", "SALE_FINALIZE"} {
		sale := domain.Sale{ID: "internal-sale", ExternalID: "merchant-order", TenantID: "tenant", FiscalDevice: domain.FiscalDeviceSnapshot{DeviceID: "device", BindingVersion: 1}}
		command, err := bridge.Prepare(domain.Operation{ID: "operation", Type: kind}, sale, domain.PaymentRequest{})
		if err != nil {
			t.Fatal(err)
		}
		var envelope map[string]any
		if err = json.Unmarshal([]byte(command.Data["body"].(string)), &envelope); err != nil {
			t.Fatal(err)
		}
		payload := envelope["payload"].(map[string]any)
		if payload["server_sale_id"] != sale.ID || payload["external_id"] != sale.ExternalID {
			t.Fatalf("%s misidentifies sale: %#v", kind, payload)
		}
	}
}

func TestBridgeRoutesImmutableCompositeSnapshotToAdapter(t *testing.T) {
	repo := domain.NewMemoryRepository()
	repo.PutResource(domain.ResourceRecord{Kind: "device", ID: "adapter", Version: 1, TenantID: "tenant", Data: map[string]any{"credential_id": "adapter-credential"}})
	bridge := NewBridge("0123456789abcdef0123456789abcdef", repo)
	sale := domain.Sale{ID: "sale", TenantID: "tenant", RegisterID: "register", FiscalDevice: domain.FiscalDeviceSnapshot{DeviceID: "fiscal", AdapterDeviceID: "adapter", BindingVersion: 99, BindingGeneration: 7}}
	command, err := bridge.Prepare(domain.Operation{ID: "operation", Type: "SALE_FINALIZE"}, sale, domain.PaymentRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if command.Data["topic"] != "tenants/tenant/devices/adapter/commands" || command.Data["device_id"] != "adapter" {
		t.Fatalf("wrong adapter route: %#v", command.Data)
	}
	var envelope map[string]any
	json.Unmarshal([]byte(command.Data["body"].(string)), &envelope)
	if envelope["device_id"] != "adapter" || envelope["fencing_token"] != float64(7) {
		t.Fatalf("wrong adapter authority: %#v", envelope)
	}
	signature := envelope["signature"]
	delete(envelope, "signature")
	unsigned, _ := json.Marshal(envelope)
	key := domain.DeriveDeviceTransportKey([]byte("0123456789abcdef0123456789abcdef"), "command", "adapter", "adapter-credential")
	mac := hmac.New(sha256.New, key)
	mac.Write(unsigned)
	if signature != base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) {
		t.Fatal("command signed with wrong endpoint credential")
	}
}

func outcomeFixture(t *testing.T) (*domain.MemoryRepository, *domain.Service, *Processor) {
	t.Helper()
	repo := domain.NewMemoryRepository()
	now := time.Now().UTC()
	sale := domain.Sale{ID: "scope-sale", TenantID: "tenant-a", RegisterID: "register", State: "PAYMENT_PENDING", Version: 1, CreatedAt: now, UpdatedAt: now}
	op := domain.Operation{ID: "scope-command", TenantID: "tenant-a", SaleID: sale.ID, State: "EXECUTING", Type: "SALE_FINALIZE", Version: 1, CreatedAt: now, UpdatedAt: now}
	for _, err := range []error{repo.PutSale(sale), repo.PutOperation(op), repo.PutResource(domain.ResourceRecord{Kind: "device_command_outbox", ID: op.ID, TenantID: "tenant-a", Version: 1, Data: map[string]any{"device_id": "old-adapter", "topic": "tenants/tenant-a/devices/old-adapter/commands"}})} {
		if err != nil {
			t.Fatal(err)
		}
	}
	svc := domain.NewService(repo, nil)
	return repo, svc, NewProcessor(svc)
}
func outcomeTopic(tenant, device string) string {
	return "beeloy/v1/tenants/" + tenant + "/devices/" + device + "/commands/scope-command/result"
}

var outcomeBody = []byte(`{"command_id":"scope-command","status":"SUCCEEDED","result":{"fiscal_reference":"receipt-1"}}`)

func TestProcessorDeviceResultsRequireImmutableScope(t *testing.T) {
	for _, tc := range []struct {
		name, tenant, device string
		mutate               func(*domain.MemoryRepository)
	}{
		{"foreign-tenant", "tenant-b", "old-adapter", nil},
		{"foreign-device", "tenant-a", "new-adapter", nil},
		{"missing-device-authority", "tenant-a", "old-adapter", func(r *domain.MemoryRepository) {
			c, _ := r.Resource("device_command_outbox", "scope-command")
			c.Kind = "unrelated" /* a different command ID cannot authorize this operation */
			c.ID = "different-command"
			r.PutResource(c)
			original, _ := r.Resource("device_command_outbox", "scope-command")
			original.Data = map[string]any{}
			original.Version++
			if err := r.PutResource(original); err != nil {
				panic(err)
			}
		}},
		{"outbox-foreign-tenant", "tenant-a", "old-adapter", func(r *domain.MemoryRepository) {
			c, _ := r.Resource("device_command_outbox", "scope-command")
			c.TenantID = "tenant-b"
			c.Version++
			if err := r.PutResource(c); err != nil {
				panic(err)
			}
		}},
		{"operation-foreign-tenant", "tenant-a", "old-adapter", func(r *domain.MemoryRepository) {
			o, _ := r.Operation("scope-command")
			o.TenantID = "tenant-b"
			r.PutOperation(o)
		}},
		{"sale-foreign-tenant", "tenant-a", "old-adapter", func(r *domain.MemoryRepository) { s, _ := r.Sale("scope-sale"); s.TenantID = "tenant-b"; r.PutSale(s) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, _, p := outcomeFixture(t)
			if tc.mutate != nil {
				tc.mutate(r)
			}
			beforeOp, _ := r.Operation("scope-command")
			beforeSale, _ := r.Sale("scope-sale")
			beforeEvents := len(r.PendingOutbox(time.Now().UTC()))
			if _, _, err := p.Process(outcomeTopic(tc.tenant, tc.device), outcomeBody); err == nil {
				t.Fatal("unscoped device result accepted")
			}
			afterOp, _ := r.Operation("scope-command")
			afterSale, _ := r.Sale("scope-sale")
			if beforeOp.Version != afterOp.Version || beforeOp.State != afterOp.State || beforeSale.Version != afterSale.Version || beforeSale.State != afterSale.State || len(r.PendingOutbox(time.Now().UTC())) != beforeEvents {
				t.Fatal("rejected result changed domain state/evidence")
			}
		})
	}
}
func TestProcessorDeviceResultUsesHistoricalOutboxAfterRegisterRebind(t *testing.T) {
	r, _, p := outcomeFixture(t)
	r.PutResource(domain.ResourceRecord{Kind: "register", ID: "register", Version: 1, TenantID: "tenant-a", Data: map[string]any{"adapter_device_id": "new-adapter"}})
	if _, _, err := p.Process(outcomeTopic("tenant-a", "old-adapter"), outcomeBody); err != nil {
		t.Fatal(err)
	}
	before, _ := r.Operation("scope-command")
	beforeSale, _ := r.Sale("scope-sale")
	events := len(r.PendingOutbox(time.Now().UTC()))
	if before.State != "FISCALIZED" {
		t.Fatal(before)
	}
	if _, _, err := p.Process(outcomeTopic("tenant-a", "old-adapter"), outcomeBody); err != nil {
		t.Fatal(err)
	}
	after, _ := r.Operation("scope-command")
	afterSale, _ := r.Sale("scope-sale")
	if after.Version != before.Version || afterSale.Version != beforeSale.Version || len(r.PendingOutbox(time.Now().UTC())) != events {
		t.Fatal("replay changed state")
	}
	for _, topic := range []string{outcomeTopic("tenant-b", "old-adapter"), outcomeTopic("tenant-a", "new-adapter")} {
		if _, _, err := p.Process(topic, outcomeBody); err == nil {
			t.Fatal("terminal early-return bypassed scope validation")
		}
	}
}

func TestProcessorRejectsMissingCommandOutbox(t *testing.T) {
	source, _, _ := outcomeFixture(t)
	r := domain.NewMemoryRepository()
	op, _ := source.Operation("scope-command")
	sale, _ := source.Sale("scope-sale")
	if err := r.PutOperation(op); err != nil {
		t.Fatal(err)
	}
	if err := r.PutSale(sale); err != nil {
		t.Fatal(err)
	}
	p := NewProcessor(domain.NewService(r, nil))
	if _, _, err := p.Process(outcomeTopic("tenant-a", "old-adapter"), outcomeBody); err == nil {
		t.Fatal("outcome without durable command accepted")
	}
	after, _ := r.Operation(op.ID)
	if after.State != op.State || after.Version != op.Version || len(r.PendingOutbox(time.Now().UTC())) != 0 {
		t.Fatal("missing outbox result changed state")
	}
}
func TestBridgeRejectsMissingOrAmbiguousTransportAuthorityBeforePublish(t *testing.T) {
	for _, tc := range []struct {
		name, adapter, kind, tenant, credential string
		generation                              int64
		repo, device                            bool
		valid                                   bool
	}{
		{name: "composite-no-generation", adapter: "adapter", repo: true, device: true, tenant: "tenant", credential: "cred"},
		{name: "composite-no-repository", adapter: "adapter", generation: 7},
		{name: "composite-missing-device", adapter: "adapter", generation: 7, repo: true},
		{name: "composite-foreign-device", adapter: "adapter", generation: 7, repo: true, device: true, tenant: "foreign", credential: "cred"},
		{name: "composite-no-credential", adapter: "adapter", generation: 7, repo: true, device: true, tenant: "tenant"},
		{name: "legacy-physical-requires-migration", kind: "FISCAL_DEVICE", repo: true, device: true, tenant: "tenant", credential: "cred"},
		{name: "legacy-unclassified-requires-migration", repo: true, device: true, tenant: "tenant", credential: "cred"},
		{name: "legacy-smart-no-credential", kind: "SMART_DEVICE", repo: true, device: true, tenant: "tenant"},
		{name: "standalone-smart", kind: "SMART_DEVICE", repo: true, device: true, tenant: "tenant", credential: "cred", valid: true},
		{name: "standalone-simulator-without-repo", valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := domain.NewMemoryRepository()
			transport := "fiscal"
			if tc.adapter != "" {
				transport = tc.adapter
			}
			if tc.device {
				if err := r.PutResource(domain.ResourceRecord{Kind: "device", ID: transport, TenantID: tc.tenant, Version: 1, Data: map[string]any{"kind": tc.kind, "credential_id": tc.credential}}); err != nil {
					t.Fatal(err)
				}
			}
			b := NewBridge("0123456789abcdef0123456789abcdef")
			if tc.repo {
				b = NewBridge("0123456789abcdef0123456789abcdef", r)
			}
			c := &captureClient{}
			b.bind(c)
			sale := domain.Sale{ID: "sale", TenantID: "tenant", FiscalDevice: domain.FiscalDeviceSnapshot{DeviceID: "fiscal", AdapterDeviceID: tc.adapter, BindingVersion: 99, BindingGeneration: tc.generation}}
			err := b.Queue(domain.Operation{ID: "op", Type: "SALE_FINALIZE"}, sale, domain.PaymentRequest{})
			if tc.valid {
				if err != nil || c.topic == "" {
					t.Fatalf("valid route failed: %v", err)
				}
			} else if err == nil || c.topic != "" {
				t.Fatalf("ambiguous authority published command: topic=%q err=%v", c.topic, err)
			}
		})
	}
}
