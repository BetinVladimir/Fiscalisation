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
