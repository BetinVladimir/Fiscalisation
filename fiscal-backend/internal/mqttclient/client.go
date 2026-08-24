package mqttclient

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"fiscalisation/fiscal-backend/internal/config"
	"fiscalisation/fiscal-backend/internal/domain"
)

var syncTopic = regexp.MustCompile(`^tenants/([^/]+)/devices/([^/]+)/sync/batches/([^/]+)$`)
var activationTopic = regexp.MustCompile(`^beefiscal/v1/devices/([^/]+)/activation$`)
var bindingAckTopic = regexp.MustCompile(`^tenants/([^/]+)/devices/([^/]+)/bindings/acks$`)
var statusTopic = regexp.MustCompile(`^(?:beeloy/v1/)?tenants/([^/]+)/devices/([^/]+)/(?:status|health)$`)

type syncService interface {
	SyncBatchForTenant(string, domain.EdgeSyncBatch) (domain.SyncAck, error)
}
type commandExpiryService interface{ ExpireDeviceCommand(string) error }
type activationService interface {
	ActivateDeviceCredential(string, string, string, string) (domain.DeviceActivationRequest, error)
	SignDeviceActivationAcknowledgement([]byte) (string, error)
}
type bindingApplyService interface {
	ApplyCompositeBindingByAdapter(string, string, string, int64) (map[string]any, error)
}
type deviceHealthService interface {
	UpsertDeviceHealth(string, string, domain.DeviceHealthStatus) (map[string]any, error)
}

type Processor struct {
	service    syncService
	signingKey []byte
}

type Bridge struct {
	client     mqtt.Client
	signingKey []byte
	repo       domain.Repository
	expire     func(string) error
}

// PublishCompositeBinding delivers the immutable signed provisioning envelope
// to the adapter control topic. The adapter applies it monotonically and only
// then calls the authenticated apply endpoint with the exact generation.
func (b *Bridge) PublishCompositeBinding(tenant, adapterID string, envelope []byte) error {
	if err := b.Probe(); err != nil {
		return err
	}
	if tenant == "" || adapterID == "" || len(envelope) == 0 {
		return fmt.Errorf("invalid composite binding envelope")
	}
	topic := fmt.Sprintf("tenants/%s/devices/%s/bindings", tenant, adapterID)
	token := b.client.Publish(topic, 1, false, envelope)
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("mqtt binding publish timeout")
	}
	return token.Error()
}

func NewBridge(signingKey string, repositories ...domain.Repository) *Bridge {
	b := &Bridge{signingKey: []byte(signingKey)}
	if len(repositories) > 0 {
		b.repo = repositories[0]
	}
	return b
}
func (b *Bridge) bind(client mqtt.Client) { b.client = client }
func (b *Bridge) Probe() error {
	if b == nil || b.client == nil || !b.client.IsConnected() {
		return fmt.Errorf("mqtt device route unavailable")
	}
	return nil
}
// DeviceTime exposes the adapter host's UTC clock as the trusted transport
// clock. Physical fiscal-device drift remains part of the signed command
// result; this capability only gates workstation readiness before a sale.
func (b *Bridge) DeviceTime() (time.Time, error) {
	if err := b.Probe(); err != nil { return time.Time{}, err }
	return time.Now().UTC(), nil
}
func (b *Bridge) SetDeviceTime(at time.Time) error {
	if at.IsZero() { return fmt.Errorf("invalid device time") }
	return b.Probe()
}
func (b *Bridge) Execute(domain.Operation, domain.Sale, domain.PaymentRequest) (string, string) {
	return "", "MQTT_ASYNC_DRIVER_REQUIRES_QUEUE"
}
func (b *Bridge) Queue(op domain.Operation, sale domain.Sale, payment domain.PaymentRequest) error {
	command, err := b.Prepare(op, sale, payment)
	if err != nil {
		return err
	}
	return b.Publish(command)
}

func (b *Bridge) Prepare(op domain.Operation, sale domain.Sale, payment domain.PaymentRequest) (domain.ResourceRecord, error) {
	if sale.TenantID == "" || sale.FiscalDevice.DeviceID == "" || sale.FiscalDevice.BindingVersion < 1 {
		return domain.ResourceRecord{}, fmt.Errorf("mqtt command binding missing")
	}
	payments := []domain.PaymentRequest{payment}
	if op.Type == "SALE_FINALIZE" {
		payments = make([]domain.PaymentRequest, 0, len(sale.Payments))
		for _, p := range sale.Payments {
			payments = append(payments, domain.PaymentRequest{PaymentID: p.PaymentID, Type: p.Type, Amount: p.Amount})
		}
	}
	payload := map[string]any{"currency": "EUR", "server_sale_id": sale.ExternalID, "external_id": sale.ExternalID, "location_id": sale.LocationID, "operator_id": sale.OperatorID, "items": sale.Lines, "payments": payments, "metadata": map[string]any{}}
	commandType := op.Type
	if mapped, ok := map[string]string{"X": "REPORT_X", "Z": "REPORT_Z"}[op.Type]; ok {
		commandType = mapped
	}
	if commandType == "PRINTER_TEST" || commandType == "REPORT_X" || commandType == "REPORT_Z" || commandType == "DEVICE_IDENTITY" || commandType == "DEVICE_TIME" {
		payload = map[string]any{"metadata": map[string]any{}}
	}
	if op.Type == "REVERSAL" {
		sofia, err := time.LoadLocation("Europe/Sofia")
		if err != nil {
			return domain.ResourceRecord{}, fmt.Errorf("load Europe/Sofia timezone: %w", err)
		}
		payments := make([]domain.PaymentRequest, 0, len(sale.Payments))
		for _, p := range sale.Payments {
			payments = append(payments, domain.PaymentRequest{PaymentID: p.PaymentID, Type: p.Type, Amount: p.Amount})
		}
		reason := map[string]int{"OPERATOR_ERROR": 0, "CUSTOMER_RETURN": 1, "CUSTOMER_COMPLAINT": 1, "TAX_BASE_REDUCTION": 2}[op.ReasonCode]
		payload = map[string]any{"currency": "EUR", "server_sale_id": sale.ID, "external_id": sale.ExternalID, "location_id": sale.LocationID, "operator_id": sale.OperatorID, "unp": sale.UNP, "original_operation_id": sale.FiscalOperationID, "reason_code": op.ReasonCode, "original_document": map[string]any{"reason": reason, "document_number": op.OriginalDocumentNumber, "document_datetime": op.OriginalDocumentAt.In(sofia).Format("02-01-06 15:04:05"), "fiscal_memory_number": sale.FiscalDevice.FiscalMemoryNumber, "original_unp": sale.UNP}, "items": sale.Lines, "payments": payments, "metadata": map[string]any{}}
	}
	// Round-trip typed nested values into maps so encoding/json sorts keys at
	// every object level. Android/Web clients use the same recursive canonical
	// representation for payload_sha256 and the command HMAC.
	typedPayload, _ := json.Marshal(payload)
	var canonicalPayload map[string]any
	if err := json.Unmarshal(typedPayload, &canonicalPayload); err != nil {
		return domain.ResourceRecord{}, fmt.Errorf("mqtt payload canonicalization: %w", err)
	}
	payload = canonicalPayload
	payloadBytes, _ := json.Marshal(payload)
	sum := sha256.Sum256(payloadBytes)
	now := time.Now().UTC()
	clientOperationID := op.ClientOperationID
	if clientOperationID == "" {
		clientOperationID = op.ID
	}
	receiptSessionID := op.ReceiptSessionID
	if receiptSessionID == "" {
		receiptSessionID = op.ID
	}
	envelope := map[string]any{"event_id": op.ID, "correlation_id": clientOperationID, "causation_id": op.ID, "operation_id": op.ID, "client_operation_id": clientOperationID, "receipt_session_id": receiptSessionID, "tenant_id": sale.TenantID, "register_id": sale.RegisterID, "device_id": sale.FiscalDevice.DeviceID, "fencing_token": sale.FiscalDevice.BindingVersion, "command_type": commandType, "issued_at": now.Format(time.RFC3339Nano), "expires_at": now.Add(2 * time.Minute).Format(time.RFC3339Nano), "payload": payload, "payload_sha256": fmt.Sprintf("%x", sum)}
	chainCanonical,_:=json.Marshal(envelope);chainHash:=sha256.Sum256(chainCanonical);envelope["event_hash"]=fmt.Sprintf("%x",chainHash)
	unsigned, _ := json.Marshal(envelope)
	signingKey := b.signingKey
	if b.repo != nil {
		if device, deviceErr := b.repo.Resource("device", sale.FiscalDevice.DeviceID); deviceErr == nil {
			if credentialID, _ := device.Data["credential_id"].(string); credentialID != "" {
				signingKey = domain.DeriveDeviceTransportKey(b.signingKey, "command", sale.FiscalDevice.DeviceID, credentialID)
			}
		}
	}
	mac := hmac.New(sha256.New, signingKey)
	mac.Write(unsigned)
	envelope["signature"] = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	body, _ := json.Marshal(envelope)
	topic := fmt.Sprintf("beeloy/v1/tenants/%s/devices/%s/commands/%s", sale.TenantID, sale.FiscalDevice.DeviceID, op.ID)
	return domain.ResourceRecord{Kind: "device_command_outbox", TenantID: sale.TenantID, ID: op.ID, Version: 1, Data: map[string]any{"topic": topic, "body": string(body), "expires_at": envelope["expires_at"], "device_id": sale.FiscalDevice.DeviceID, "state": "PENDING"}, CreatedAt: now, UpdatedAt: now}, nil
}

func (b *Bridge) Publish(command domain.ResourceRecord) error {
	if err := b.Probe(); err != nil {
		return err
	}
	if command.Kind != "device_command_outbox" || command.ID == "" || command.TenantID == "" || command.Data == nil {
		return fmt.Errorf("invalid mqtt command outbox")
	}
	topic, _ := command.Data["topic"].(string)
	body, _ := command.Data["body"].(string)
	if topic == "" || body == "" {
		return fmt.Errorf("invalid mqtt command outbox payload")
	}
	token := b.client.Publish(topic, 1, false, []byte(body))
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("mqtt command publish timeout")
	}
	return token.Error()
}

// FlushOutbox republishes immutable commands whose authoritative operation is
// still EXECUTING. Device-side operation idempotency makes reconnect delivery safe.
func (b *Bridge) FlushOutbox() error {
	if b.repo == nil {
		return nil
	}
	for _, command := range b.repo.Resources("device_command_outbox", "") {
		op, err := b.repo.Operation(command.ID)
		if err != nil || op.State != "EXECUTING" {
			continue
		}
		if expires, _ := command.Data["expires_at"].(string); expires != "" {
			at, e := time.Parse(time.RFC3339Nano, expires)
			if e != nil || !time.Now().UTC().Before(at) {
				if b.expire != nil {
					if expireErr := b.expire(command.ID); expireErr != nil {
						return expireErr
					}
				}
				continue
			}
		}
		if err = b.Publish(command); err != nil {
			return err
		}
	}
	return nil
}

func NewProcessor(service syncService, signingKey ...string) *Processor {
	p := &Processor{service: service}
	if len(signingKey) > 0 {
		p.signingKey = []byte(signingKey[0])
	}
	return p
}

func (p *Processor) Process(topic string, payload []byte) (string, []byte, error) {
	if parts := statusTopic.FindStringSubmatch(topic); len(parts) == 3 {
		service, ok := p.service.(deviceHealthService)
		if !ok {
			return "", nil, fmt.Errorf("device health service unavailable")
		}
		var status domain.DeviceHealthStatus
		if json.Unmarshal(payload, &status) != nil {
			return "", nil, fmt.Errorf("invalid device status json")
		}
		if _, err := service.UpsertDeviceHealth(parts[1], parts[2], status); err != nil {
			return "", nil, err
		}
		return "", nil, nil
	}
	if parts := bindingAckTopic.FindStringSubmatch(topic); len(parts) == 3 {
		service, ok := p.service.(bindingApplyService)
		if !ok {
			return "", nil, fmt.Errorf("binding apply service unavailable")
		}
		var in struct {
			AdapterDeviceID string `json:"adapter_device_id"`
			RegisterID      string `json:"register_id"`
			Generation      int64  `json:"generation"`
		}
		if json.Unmarshal(payload, &in) != nil || in.AdapterDeviceID != parts[2] || in.RegisterID == "" || in.Generation < 1 {
			return "", nil, fmt.Errorf("invalid binding ack")
		}
		applied, err := service.ApplyCompositeBindingByAdapter(parts[1], parts[2], in.RegisterID, in.Generation)
		if err != nil {
			return "", nil, err
		}
		body, _ := json.Marshal(applied)
		return fmt.Sprintf("tenants/%s/devices/%s/bindings/acks/confirmed", parts[1], parts[2]), body, nil
	}
	if parts := activationTopic.FindStringSubmatch(topic); len(parts) == 2 {
		service, ok := p.service.(activationService)
		if !ok {
			return "", nil, fmt.Errorf("activation service unavailable")
		}
		var in struct {
			ActivationRequestID string `json:"activation_request_id"`
			CredentialID        string `json:"credential_id"`
			Nonce               string `json:"nonce"`
			Signature           string `json:"signature"`
		}
		if json.Unmarshal(payload, &in) != nil || in.ActivationRequestID == "" || in.CredentialID == "" || in.Nonce == "" || in.Signature == "" {
			return "", nil, fmt.Errorf("invalid activation payload")
		}
		v, err := service.ActivateDeviceCredential(in.ActivationRequestID, in.CredentialID, in.Nonce, in.Signature)
		if err != nil || v.DeviceInstanceID != parts[1] {
			return "", nil, fmt.Errorf("activation rejected")
		}
		body := map[string]any{"activation_request_id": v.ID, "device_instance_id": v.DeviceInstanceID, "state": v.State, "binding_version": v.BindingVersion, "organization_id": v.ClaimedTenantID, "location_id": v.ClaimedLocationID, "register_id": v.ClaimedRegisterID, "roles": v.ClaimedRoles}
		unsigned, _ := json.Marshal(body)
		signature, signErr := service.SignDeviceActivationAcknowledgement(unsigned)
		if signErr != nil {
			return "", nil, fmt.Errorf("activation acknowledgement signing failed: %w", signErr)
		}
		body["signature"] = signature
		encoded, _ := json.Marshal(body)
		return fmt.Sprintf("beefiscal/v1/devices/%s/activation/ack", parts[1]), encoded, nil
	}
	parts := syncTopic.FindStringSubmatch(topic)
	if len(parts) != 4 || p == nil || p.service == nil {
		return "", nil, fmt.Errorf("unsupported mqtt topic")
	}
	var batch domain.EdgeSyncBatch
	if err := json.Unmarshal(payload, &batch); err != nil {
		return "", nil, fmt.Errorf("invalid sync batch json: %w", err)
	}
	if batch.EdgeID != parts[2] {
		return "", nil, fmt.Errorf("mqtt device binding mismatch")
	}
	ack, err := p.service.SyncBatchForTenant(parts[1], batch)
	if err != nil {
		return "", nil, err
	}
	body, err := json.Marshal(ack)
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf("tenants/%s/devices/%s/sync/acks/%s", parts[1], parts[2], parts[3]), body, nil
}

// Start connects to EMQX and subscribes to configured topics.
func Start(ctx context.Context, cfg config.Config, logger *log.Logger, service syncService, bridge *Bridge) (func(), error) {
	if cfg.EMQXBroker == "" || cfg.EMQXToken == "" || len(cfg.EMQXSubTopics) == 0 {
		logger.Printf("mqtt disabled: broker/token/topics are not fully configured")
		return nil, nil
	}

	processor := NewProcessor(service, cfg.BLESigningKey)
	if bridge != nil {
		if expiry, ok := service.(commandExpiryService); ok {
			bridge.expire = expiry.ExpireDeviceCommand
		}
	}
	handler := func(client mqtt.Client, msg mqtt.Message) {
		ackTopic, ack, err := processor.Process(msg.Topic(), msg.Payload())
		if err != nil {
			logger.Printf("mqtt message rejected topic=%s error=%v", msg.Topic(), err)
			return
		}
		if ackTopic == "" || len(ack) == 0 {
			return
		}
		go func() {
			token := client.Publish(ackTopic, 1, false, ack)
			if !token.WaitTimeout(10*time.Second) || token.Error() != nil {
				logger.Printf("mqtt business ack publish failed topic=%s error=%v", ackTopic, token.Error())
			}
		}()
	}

	subscribe := func(client mqtt.Client) error {
		topics := append([]string{}, cfg.EMQXSubTopics...)
		topics = append(topics, "tenants/+/devices/+/bindings/acks")
		for _, topic := range topics {
			token := client.Subscribe(topic, 1, handler)
			if ok := token.WaitTimeout(10 * time.Second); !ok {
				return fmt.Errorf("subscribe timeout for topic %s", topic)
			}
			if err := token.Error(); err != nil {
				return fmt.Errorf("subscribe topic %s: %w", topic, err)
			}
		}
		logger.Printf("mqtt subscribed to topics: %v", cfg.EMQXSubTopics)
		return nil
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.EMQXBroker)
	opts.SetClientID(cfg.EMQXClientID)
	opts.SetUsername(cfg.EMQXUsername)
	opts.SetPassword(cfg.EMQXToken)
	opts.SetAutoReconnect(true)
	opts.SetResumeSubs(false)
	opts.SetKeepAlive(20 * time.Second)
	opts.SetPingTimeout(5 * time.Second)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		go func() {
			if err := subscribe(client); err != nil { logger.Printf("mqtt subscribe error: %v", err); return }
			if bridge != nil { if err := bridge.FlushOutbox(); err != nil { logger.Printf("mqtt command outbox flush error: %v", err) } }
		}()
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		logger.Printf("mqtt connection lost: %v", err)
	})

	client := mqtt.NewClient(opts)
	if bridge != nil {
		bridge.bind(client)
	}
	connectToken := client.Connect()
	if ok := connectToken.WaitTimeout(15 * time.Second); !ok {
		return nil, fmt.Errorf("mqtt connect timeout")
	}
	if err := connectToken.Error(); err != nil {
		return nil, fmt.Errorf("mqtt connect: %w", err)
	}
	logger.Printf("mqtt connected: broker=%s client_id=%s", cfg.EMQXBroker, cfg.EMQXClientID)

	go func() {
		<-ctx.Done()
		if client.IsConnected() {
			client.Disconnect(250)
		}
	}()

	cleanup := func() {
		if client.IsConnected() {
			client.Disconnect(250)
		}
	}
	return cleanup, nil
}
