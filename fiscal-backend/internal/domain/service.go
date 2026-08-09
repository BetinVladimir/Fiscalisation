package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var seq atomic.Uint64

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), seq.Add(1))
}
func pad7(n int64) string  { return fmt.Sprintf("%07d", n) }
func Hash(b []byte) string { v := sha256.Sum256(b); return hex.EncodeToString(v[:]) }

type Driver interface {
	Execute(Operation, Sale, PaymentRequest) (string, string)
	Probe() error
}
type Simulator struct {
	enabled               bool
	cardTerminalAvailable bool
	outcomeUnknown        bool
}

func NewSimulator(enabled bool) *Simulator { return &Simulator{enabled: enabled} }
func NewSimulatorWithCardTerminal(enabled, available bool) *Simulator {
	return &Simulator{enabled: enabled, cardTerminalAvailable: available}
}
func (s *Simulator) SetOutcomeUnknown(v bool) { s.outcomeUnknown = v }
func (s *Simulator) Execute(op Operation, sale Sale, p PaymentRequest) (string, string) {
	if !s.enabled {
		return "", "SIMULATOR_DISABLED"
	}
	if p.Type == "CARD" && !s.cardTerminalAvailable {
		return "", "PAYMENT_TERMINAL_UNAVAILABLE"
	}
	if s.outcomeUnknown {
		return "", "FISCAL_RESULT_UNKNOWN"
	}
	return "SIM-" + op.ID, ""
}
func (s *Simulator) Probe() error {
	if !s.enabled {
		return errors.New("fiscal device unavailable")
	}
	return nil
}

type Service struct {
	repo          Repository
	driver        Driver
	bleSigningKey []byte
	policy        PolicyCatalog
}

func (s *Service) CountryPolicy(at time.Time) (CountryPolicy, error) { return s.policy.Policy(at) }
func (s *Service) TaxGroups(at time.Time) ([]TaxGroup, error)        { return s.policy.TaxGroups(at) }

func (s *Service) OpenShift(register, operator, tenant string) (Shift, error) {
	if register == "" || operator == "" {
		return Shift{}, errors.New("invalid shift")
	}
	if tenant != "" && !s.registerHasActiveFiscalDevice(register, tenant) {
		return Shift{}, errors.New("fiscal device unavailable")
	}
	if tenant != "" && !s.activeOperatorResourceForTenant(operator, tenant, time.Now().UTC()) {
		return Shift{}, errors.New("operator unavailable")
	}
	return s.repo.OpenShift(register, operator, tenant)
}

func (s *Service) activeOperatorResourceForTenant(idOrCode, tenant string, at time.Time) bool {
	for _, operator := range s.repo.Resources("operator", tenant) {
		if operator.ID != idOrCode && stringField(operator.Data, "code") != idOrCode {
			continue
		}
		activeFrom, err := time.Parse(time.RFC3339, stringField(operator.Data, "active_from"))
		if err != nil || activeFrom.After(at) {
			return false
		}
		activeTo := stringField(operator.Data, "active_to")
		if activeTo == "" {
			return true
		}
		until, err := time.Parse(time.RFC3339, activeTo)
		return err == nil && at.Before(until)
	}
	return false
}
func (s *Service) CloseShift(id string) (Shift, error) {
	return s.repo.CloseShift(id)
}
func (s *Service) CloseShiftForTenant(id, tenant string) (Shift, error) {
	if _, err := s.repo.ShiftForTenant(id, tenant); err != nil {
		return Shift{}, err
	}
	return s.repo.CloseShift(id)
}
func (s *Service) GetShift(id, tenant string) (Shift, error) {
	return s.repo.ShiftForTenant(id, tenant)
}
func (s *Service) Shifts(tenant string) []Shift { return s.repo.Shifts(tenant) }
func (s *Service) Reverse(saleID, reason string) (Operation, error) {
	return s.reverseForTenant(saleID, reason, "", "")
}
func (s *Service) ReverseForTenant(saleID, reason, tenant string) (Operation, error) {
	return s.reverseForTenant(saleID, reason, "", tenant)
}
func (s *Service) ReverseForTenantWithReference(saleID, reason, originalReference, tenant string) (Operation, error) {
	return s.reverseForTenant(saleID, reason, originalReference, tenant)
}
func (s *Service) reverseForTenant(saleID, reason, originalReference, tenant string) (Operation, error) {
	sale, e := s.saleForTenantMutation(saleID, tenant)
	if e != nil {
		return Operation{}, e
	}
	reason = strings.TrimSpace(reason)
	if sale.State != "COMPLETED" || reason == "" || len(reason) > 64 || strings.ContainsAny(reason, "\r\n\t") || sale.FiscalOperationID == "" {
		return Operation{}, errors.New("reversal not allowed")
	}
	original, e := s.repo.Operation(sale.FiscalOperationID)
	if e != nil || original.FiscalReference == "" || (originalReference != "" && originalReference != original.FiscalReference) {
		return Operation{}, errors.New("original fiscal reference mismatch")
	}
	now := time.Now().UTC()
	if !reversalAllowed(reason, original.CreatedAt, now) {
		return Operation{}, errors.New("reversal reason or deadline not allowed")
	}
	if sale.TenantID != "" && !s.registerHasActiveFiscalDevice(sale.RegisterID, sale.TenantID) {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	if s.driver == nil || s.driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	op := Operation{ID: newID("op"), TenantID: sale.TenantID, SaleID: saleID, Type: "REVERSAL", State: "EXECUTING", Version: 1, OriginalFiscalReference: original.FiscalReference, ReasonCode: reason, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	ref, code := s.driver.Execute(op, sale, PaymentRequest{})
	op.Version++
	op.UpdatedAt = time.Now().UTC()
	if code == "FISCAL_RESULT_UNKNOWN" {
		op.State, op.ErrorCode, op.AllowedActions = "UNKNOWN", code, []string{"RECONCILE"}
		return op, s.repo.PutOperation(op)
	}
	if code != "" {
		op.State, op.ErrorCode = "FAILED", code
		return op, s.repo.PutOperation(op)
	}
	op.State, op.FiscalReference = "FISCALIZED", ref
	sale.State = "REVERSED"
	sale.Version++
	sale.UpdatedAt = now
	return op, s.repo.CommitSaleOperation(sale, op)
}
func (s *Service) FiscalOperation(register, typ, tenant string) (Operation, error) {
	if register == "" {
		return Operation{}, errors.New("register required")
	}
	allowed := []string{"CASH_IN", "CASH_OUT", "X", "Z", "KLEN", "FISCAL_MEMORY", "OPERATOR", "DEPARTMENT", "PLU"}
	if !contains(allowed, typ) {
		return Operation{}, errors.New("unsupported operation")
	}
	if tenant != "" && !s.registerHasActiveFiscalDevice(register, tenant) {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	if s.driver == nil || s.driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	now := time.Now().UTC()
	op := Operation{ID: newID("op"), TenantID: tenant, Type: typ, State: "EXECUTING", Version: 1, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	ref, code := s.driver.Execute(op, Sale{RegisterID: register, TenantID: tenant}, PaymentRequest{})
	op.Version++
	op.UpdatedAt = time.Now().UTC()
	if code == "FISCAL_RESULT_UNKNOWN" {
		op.State, op.ErrorCode, op.AllowedActions = "UNKNOWN", code, []string{"RECONCILE"}
	} else if code != "" {
		op.State, op.ErrorCode = "FAILED", code
	} else {
		op.State, op.FiscalReference = "FISCALIZED", ref
	}
	return op, s.repo.PutOperation(op)
}
func (s *Service) Connectivity(register, tenant string) (ConnectivityProbe, error) {
	now := time.Now().UTC()
	v := ConnectivityProbe{ProbeID: newID("probe"), TenantID: tenant, RegisterID: register, State: "SUCCEEDED", ObservedAt: now, RecommendedTransport: "REST", Hops: map[string]map[string]any{"public_api": {"state": "READY", "latency_ms": 0}, "fiscal_core": {"state": "READY", "latency_ms": 0}, "edge_runtime": {"state": "READY", "latency_ms": 0}, "driver": {"state": "READY", "latency_ms": 0}, "fiscal_device": {"state": "READY", "latency_ms": 0}}}
	registerRecord, registerErr := s.repo.Resource("register", register)
	deviceID := ""
	registryReady := registerErr == nil && registerRecord.TenantID == tenant && stringField(registerRecord.Data, "status") == "ACTIVE"
	if registryReady {
		deviceID = stringField(registerRecord.Data, "fiscal_device_id")
		registryReady = s.activeDeviceForTenant(deviceID, tenant)
	}
	if !registryReady {
		v.Hops["fiscal_core"] = map[string]any{"state": "BLOCKED", "latency_ms": 0}
	}
	if !registryReady || s.driver == nil || s.driver.Probe() != nil {
		v.State, v.RecommendedTransport = "FAILED", "BLOCK"
		v.Hops["driver"] = map[string]any{"state": "UNAVAILABLE", "latency_ms": 0}
		v.Hops["fiscal_device"] = map[string]any{"state": "UNAVAILABLE", "latency_ms": 0, "device_id": deviceID}
	}
	return v, s.repo.PutConnectivityProbe(v)
}
func (s *Service) GetConnectivityProbe(id, tenant string) (ConnectivityProbe, error) {
	v, e := s.repo.ConnectivityProbe(id)
	if r, ok := s.repo.(interface {
		ConnectivityProbeForTenant(string, string) (ConnectivityProbe, error)
	}); ok {
		v, e = r.ConnectivityProbeForTenant(id, tenant)
	}
	if e != nil || (tenant != "" && v.TenantID != tenant) {
		return ConnectivityProbe{}, ErrNotFound
	}
	return v, nil
}
func (s *Service) SetBLESigningKey(v string) { s.bleSigningKey = []byte(v) }
func (s *Service) BLESession(register, operator, app, tenant, actorSubject, clientPublicKey string) (map[string]any, error) {
	decodedPublicKey, publicKeyErr := base64.RawURLEncoding.DecodeString(clientPublicKey)
	if register == "" || operator == "" || app == "" || actorSubject == "" || publicKeyErr != nil || len(decodedPublicKey) != 32 || len(s.bleSigningKey) < 16 {
		return nil, errors.New("BLE session unavailable")
	}
	now := time.Now().UTC()
	if !s.registerHasActiveFiscalDevice(register, tenant) || !s.activeOperatorResourceForTenant(operator, tenant, now) {
		return nil, errors.New("BLE session unavailable")
	}
	nonceBytes := make([]byte, 16)
	if _, e := rand.Read(nonceBytes); e != nil {
		return nil, e
	}
	v := BLESessionRecord{SessionID: newID("ble"), TenantID: tenant, RegisterID: register, OperatorID: operator, AppInstanceID: app, ActorSubject: actorSubject, ClientPublicKey: clientPublicKey, DeviceID: register + "-edge", Scopes: []string{"fiscal.execute", "fiscal.read"}, FencingToken: now.UnixNano(), ExpiresAt: now.Add(8 * time.Hour), Nonce: base64.RawURLEncoding.EncodeToString(nonceBytes)}
	if e := s.repo.PutBLESession(v); e != nil {
		return nil, e
	}
	return s.bleResponse(v)
}
func (s *Service) RefreshBLE(id, tenant, actorSubject string) (map[string]any, error) {
	v, e := s.bleSessionForTenant(id, tenant)
	now := time.Now().UTC()
	if e != nil || actorSubject == "" || v.ActorSubject == "" || v.ActorSubject != actorSubject || v.Revoked || (tenant != "" && v.TenantID != tenant) || !now.Before(v.ExpiresAt) || !s.registerHasActiveFiscalDevice(v.RegisterID, v.TenantID) || !s.activeOperatorResourceForTenant(v.OperatorID, v.TenantID, now) {
		return nil, errors.New("BLE session inactive")
	}
	nonceBytes := make([]byte, 16)
	if _, e = rand.Read(nonceBytes); e != nil {
		return nil, e
	}
	nextToken := now.UnixNano()
	if nextToken <= v.FencingToken {
		nextToken = v.FencingToken + 1
	}
	v.FencingToken = nextToken
	v.Nonce = base64.RawURLEncoding.EncodeToString(nonceBytes)
	v.ExpiresAt = now.Add(8 * time.Hour)
	if e = s.repo.PutBLESession(v); e != nil {
		return nil, e
	}
	return s.bleResponse(v)
}

func (s *Service) registerHasActiveFiscalDevice(registerID, tenant string) bool {
	register, err := s.repo.Resource("register", registerID)
	if err != nil || register.TenantID != tenant || stringField(register.Data, "status") != "ACTIVE" {
		return false
	}
	if !bindingIsActive(register.Data, "fiscal_device_active_from", time.Now().UTC()) {
		return false
	}
	return s.activeDeviceForTenant(stringField(register.Data, "fiscal_device_id"), tenant)
}

func (s *Service) registerHasActivePaymentTerminal(registerID, tenant string) bool {
	register, err := s.repo.Resource("register", registerID)
	if err != nil || register.TenantID != tenant || stringField(register.Data, "status") != "ACTIVE" {
		return false
	}
	if !bindingIsActive(register.Data, "payment_terminal_active_from", time.Now().UTC()) {
		return false
	}
	terminalID := stringField(register.Data, "payment_terminal_id")
	terminal, err := s.repo.Resource("device", terminalID)
	if err != nil || terminal.TenantID != tenant || stringField(terminal.Data, "status") != "ACTIVE" {
		return false
	}
	return oneOf(stringField(terminal.Data, "kind"), "PAYMENT_TERMINAL", "SMART_DEVICE")
}

func bindingIsActive(data map[string]any, field string, now time.Time) bool {
	raw := stringField(data, field)
	if raw == "" { // Compatibility with bindings created before active_from persistence.
		return true
	}
	activeFrom, err := time.Parse(time.RFC3339, raw)
	return err == nil && !activeFrom.After(now)
}

func (s *Service) activeOperatorForTenant(code, tenant string, at time.Time) bool {
	for _, operator := range s.repo.Resources("operator", tenant) {
		if stringField(operator.Data, "code") != code {
			continue
		}
		activeFrom, err := time.Parse(time.RFC3339, stringField(operator.Data, "active_from"))
		if err != nil || activeFrom.After(at) {
			return false
		}
		activeTo := stringField(operator.Data, "active_to")
		if activeTo == "" {
			return true
		}
		until, err := time.Parse(time.RFC3339, activeTo)
		return err == nil && at.Before(until)
	}
	return false
}
func (s *Service) RevokeBLE(id, tenant, actorSubject string) error {
	v, e := s.bleSessionForTenant(id, tenant)
	if e != nil || actorSubject == "" || v.ActorSubject == "" || v.ActorSubject != actorSubject || (tenant != "" && v.TenantID != tenant) {
		return ErrNotFound
	}
	v.Revoked = true
	now := time.Now().UTC()
	event := WebhookEvent{EventID: "event-ble-revoke-" + v.SessionID, EventType: "ble.session.revoked", APIVersion: "2026-08-07", TenantID: v.TenantID, ResourceID: v.SessionID, ResourceVersion: v.FencingToken, OccurredAt: now, Data: map[string]any{"ble_session_id": v.SessionID, "edge_id": v.DeviceID, "expires_at": v.ExpiresAt}}
	return s.repo.CommitBLESessionEvent(v, OutboxItem{ID: event.EventID, Event: event, NextAttempt: now})
}
func (s *Service) bleSessionForTenant(id, tenant string) (BLESessionRecord, error) {
	if r, ok := s.repo.(interface {
		BLESessionForTenant(string, string) (BLESessionRecord, error)
	}); ok {
		return r.BLESessionForTenant(id, tenant)
	}
	v, err := s.repo.BLESession(id)
	if err != nil || (tenant != "" && v.TenantID != tenant) {
		return BLESessionRecord{}, ErrNotFound
	}
	return v, nil
}
func (s *Service) bleResponse(v BLESessionRecord) (map[string]any, error) {
	ticket := struct {
		SessionID, TenantID, RegisterID, DeviceID, AppInstanceID, ActorSubject, ClientPublicKey string
		Scopes                                                                                  []string
		FencingToken                                                                            int64
		ExpiresAt                                                                               time.Time
		Nonce                                                                                   string
	}{v.SessionID, v.TenantID, v.RegisterID, v.DeviceID, v.AppInstanceID, v.ActorSubject, v.ClientPublicKey, v.Scopes, v.FencingToken, v.ExpiresAt, v.Nonce}
	b, e := json.Marshal(ticket)
	if e != nil {
		return nil, e
	}
	m := hmac.New(sha256.New, s.bleSigningKey)
	m.Write(b)
	wrapped, e := json.Marshal(struct {
		Payload   string `json:"payload"`
		Signature string `json:"signature"`
	}{base64.RawURLEncoding.EncodeToString(b), base64.RawURLEncoding.EncodeToString(m.Sum(nil))})
	if e != nil {
		return nil, e
	}
	raw := base64.RawURLEncoding.EncodeToString(wrapped)
	return map[string]any{
		"ble_session_id": v.SessionID, "register_id": v.RegisterID,
		"edge_id": v.DeviceID, "device_id": v.RegisterID + "-device", "binding_version": v.FencingToken,
		"operator_id": v.OperatorID, "app_instance_id": v.AppInstanceID,
		"protocol_version": "2026-08-07", "expires_at": v.ExpiresAt, "scopes": v.Scopes,
		"service_uuid":                "7b6f0000-7c6d-4c7a-9e4f-424545464953",
		"command_characteristic_uuid": "7b6f0002-7c6d-4c7a-9e4f-424545464953",
		"event_characteristic_uuid":   "7b6f0003-7c6d-4c7a-9e4f-424545464953",
		"advertising_identity":        v.DeviceID,
		"signed_session_ticket":       raw,
	}, nil
}
func NewService(r Repository, d Driver) *Service {
	return &Service{repo: r, driver: d, policy: DefaultBGPolicyCatalog()}
}

type CreateSale struct {
	TenantID   string `json:"-"`
	ExternalID string `json:"external_id"`
	RegisterID string `json:"register_id"`
	OperatorID string `json:"operator_id"`
}

func (s *Service) CreateSale(in CreateSale) (Sale, error) {
	if in.ExternalID == "" || in.RegisterID == "" || len(in.OperatorID) != 4 {
		return Sale{}, errors.New("invalid sale")
	}
	if in.TenantID != "" && !s.registerHasActiveFiscalDevice(in.RegisterID, in.TenantID) {
		return Sale{}, errors.New("fiscal device unavailable")
	}
	if in.TenantID != "" && !s.activeOperatorForTenant(in.OperatorID, in.TenantID, time.Now().UTC()) {
		return Sale{}, errors.New("operator unavailable")
	}
	now := time.Now().UTC()
	v := Sale{ID: newID("sale"), TenantID: in.TenantID, ExternalID: in.ExternalID, RegisterID: in.RegisterID, OperatorID: in.OperatorID, State: "DRAFT", Version: 1, Lines: []SaleLine{}, Payments: []PaymentRecord{}, CreatedAt: now, UpdatedAt: now}
	return v, s.repo.PutSale(v)
}
func (s *Service) AddLine(id string, line SaleLine) (Sale, error) {
	v, err := s.repo.Sale(id)
	if err != nil {
		return v, err
	}
	return s.addLineForTenant(id, v.Version, line, v.TenantID)
}
func (s *Service) AddLineForTenant(id string, line SaleLine, tenant string) (Sale, error) {
	v, err := s.saleForTenantMutation(id, tenant)
	if err != nil {
		return v, err
	}
	return s.addLineForTenant(id, v.Version, line, tenant)
}
func (s *Service) AddLineExpectedForTenant(id string, expected int64, line SaleLine, tenant string) (Sale, error) {
	return s.addLineForTenant(id, expected, line, tenant)
}
func (s *Service) addLineForTenant(id string, expected int64, line SaleLine, tenant string) (Sale, error) {
	if line.LineID == "" || line.Name == "" || !validMoney(line.UnitPrice) || !validQuantity(line.Quantity) || !validTax(line.TaxGroup) || !s.policy.AllowsTaxGroup(line.TaxGroup, time.Now().UTC()) {
		return Sale{}, errors.New("invalid line")
	}
	return s.repo.AddSaleLineExpected(id, tenant, expected, line)
}
func (s *Service) Pay(id string, p PaymentRequest) (Operation, error) {
	return s.payForTenant(id, p, "")
}
func (s *Service) PayForTenant(id string, p PaymentRequest, tenant string) (Operation, error) {
	return s.payForTenant(id, p, tenant)
}
func (s *Service) payForTenant(id string, p PaymentRequest, tenant string) (Operation, error) {
	sale, e := s.saleForTenantMutation(id, tenant)
	if e != nil {
		return Operation{}, e
	}
	if sale.State != "OPEN" || len(sale.Lines) == 0 || p.PaymentID == "" || !validMoney(p.Amount) || !contains([]string{"CASH", "CARD"}, p.Type) {
		return Operation{}, errors.New("payment not allowed")
	}
	if sale.TenantID != "" && !s.registerHasActiveFiscalDevice(sale.RegisterID, sale.TenantID) {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	if p.Type == "CARD" {
		if p.TerminalPolicy == "NONE" {
			return Operation{}, errors.New("card payment requires terminal")
		}
		if sale.TenantID != "" && !s.registerHasActivePaymentTerminal(sale.RegisterID, sale.TenantID) {
			return Operation{}, errors.New("payment terminal unavailable")
		}
	}
	if s.driver == nil || s.driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	amount, _ := parseFixed(p.Amount.Amount, 2)
	total := saleTotal(sale)
	paid := int64(0)
	for _, x := range sale.Payments {
		if x.PaymentID == p.PaymentID {
			return Operation{}, errors.New("duplicate payment id")
		}
		v, _ := parseFixed(x.Amount.Amount, 2)
		paid += v
	}
	if amount <= 0 || paid+amount > total {
		return Operation{}, errors.New("payment amount exceeds balance")
	}
	now := time.Now().UTC()
	op := Operation{ID: newID("op"), TenantID: sale.TenantID, SaleID: id, Type: "FISCAL_SALE", State: "EXECUTING", Version: 1, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	if e = s.repo.PutOperation(op); e != nil {
		return Operation{}, e
	}
	ref, code := s.driver.Execute(op, sale, p)
	op.Version++
	op.UpdatedAt = time.Now().UTC()
	if code == "FISCAL_RESULT_UNKNOWN" {
		op.State = "UNKNOWN"
		op.ErrorCode = code
		op.AllowedActions = []string{"RECONCILE"}
		sale.State = "UNKNOWN"
	} else if code != "" {
		op.State = "FAILED"
		op.ErrorCode = code
		sale.State = "OPEN"
	} else {
		op.FiscalReference = ref
		sale.Payments = append(sale.Payments, PaymentRecord{PaymentID: p.PaymentID, Type: p.Type, Amount: p.Amount, FiscalReference: ref, CreatedAt: op.UpdatedAt})
		if paid+amount == total {
			op.State = "FISCALIZED"
			sale.State = "COMPLETED"
			sale.FiscalOperationID = op.ID
			sale.ReceiptArtifactID, e = newUUID()
			if e != nil {
				return Operation{}, e
			}
			receiptBytes, _ := json.Marshal(map[string]any{"sale_id": sale.ID, "operation_id": op.ID, "unp": sale.UNP, "fiscal_reference": op.FiscalReference, "issued_at": op.UpdatedAt, "total": Money{Amount: formatFixed(total), Currency: "EUR"}, "lines": sale.Lines, "payments": sale.Payments})
			if e = s.repo.PutArtifact(sale.ReceiptArtifactID, sale.TenantID, receiptBytes); e != nil {
				return Operation{}, e
			}
		} else {
			op.State = "PAYMENT_ACCEPTED"
			sale.State = "OPEN"
		}
	}
	sale.Version++
	sale.UpdatedAt = op.UpdatedAt
	return op, s.repo.CommitSaleOperation(sale, op)
}
func (s *Service) GetSale(id string) (Sale, error)           { return s.repo.Sale(id) }
func (s *Service) GetOperation(id string) (Operation, error) { return s.repo.Operation(id) }
func (s *Service) GetSaleForTenant(id, tenant string) (Sale, error) {
	if r, ok := s.repo.(interface {
		SaleForTenant(string, string) (Sale, error)
	}); ok {
		return r.SaleForTenant(id, tenant)
	}
	v, e := s.repo.Sale(id)
	if e != nil || v.TenantID != tenant {
		return Sale{}, ErrNotFound
	}
	return v, nil
}
func (s *Service) GetOperationForTenant(id, tenant string) (Operation, error) {
	if r, ok := s.repo.(interface {
		OperationForTenant(string, string) (Operation, error)
	}); ok {
		return r.OperationForTenant(id, tenant)
	}
	v, e := s.repo.Operation(id)
	if e != nil || v.TenantID != tenant {
		return Operation{}, ErrNotFound
	}
	return v, nil
}
func (s *Service) Operations(tenant string) []Operation {
	if r, ok := s.repo.(interface{ OperationsForTenant(string) []Operation }); ok {
		return r.OperationsForTenant(tenant)
	}
	all := s.repo.Operations()
	if tenant == "" {
		return all
	}
	v := make([]Operation, 0)
	for _, x := range all {
		if x.TenantID == tenant {
			v = append(v, x)
		}
	}
	return v
}
func (s *Service) CancelSale(id string) (Operation, error) {
	return s.cancelSaleForTenant(id, "")
}
func (s *Service) CancelSaleForTenant(id, tenant string) (Operation, error) {
	return s.cancelSaleForTenant(id, tenant)
}
func (s *Service) cancelSaleForTenant(id, tenant string) (Operation, error) {
	v, e := s.saleForTenantMutation(id, tenant)
	if e != nil {
		return Operation{}, e
	}
	if (v.State != "DRAFT" && v.State != "OPEN") || len(v.Payments) != 0 {
		return Operation{}, errors.New("sale cannot be cancelled")
	}
	now := time.Now().UTC()
	op := Operation{ID: newID("op"), TenantID: v.TenantID, SaleID: v.ID, Type: "CANCEL_SALE", State: "CANCELLED", Version: 1, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	v.State, v.UpdatedAt, v.Version = "CANCELLED", now, v.Version+1
	return op, s.repo.CommitSaleOperation(v, op)
}
func (s *Service) ReconcileOperation(id string) (Operation, error) {
	return s.reconcileOperationForTenant(id, "")
}
func (s *Service) ReconcileOperationForTenant(id, tenant string) (Operation, error) {
	return s.reconcileOperationForTenant(id, tenant)
}
func (s *Service) reconcileOperationForTenant(id, tenant string) (Operation, error) {
	op, e := s.operationForTenantMutation(id, tenant)
	if e != nil {
		return op, e
	}
	if op.State != "UNKNOWN" {
		return op, errors.New("operation does not require reconciliation")
	}
	op.State, op.Version, op.UpdatedAt = "RECONCILING", op.Version+1, time.Now().UTC()
	op.AllowedActions = []string{}
	return op, s.repo.PutOperation(op)
}
func (s *Service) Receipt(id string) (map[string]any, error) {
	return s.receiptForTenant(id, "")
}
func (s *Service) ReceiptForTenant(id, tenant string) (map[string]any, error) {
	return s.receiptForTenant(id, tenant)
}
func (s *Service) receiptForTenant(id, tenant string) (map[string]any, error) {
	v, e := s.saleForTenantMutation(id, tenant)
	if e != nil {
		return nil, e
	}
	if v.State != "COMPLETED" {
		return nil, errors.New("receipt unavailable")
	}
	ref := ""
	if len(v.Payments) > 0 {
		ref = v.Payments[len(v.Payments)-1].FiscalReference
	}
	return map[string]any{"sale_id": v.ID, "operation_id": v.FiscalOperationID, "unp": v.UNP, "state": v.State, "fiscal_reference": ref, "issued_at": v.UpdatedAt, "total": Money{Amount: formatFixed(saleTotal(v)), Currency: "EUR"}, "artifact_id": v.ReceiptArtifactID, "lines": v.Lines, "payments": v.Payments}, nil
}
func (s *Service) saleForTenantMutation(id, tenant string) (Sale, error) {
	if tenant == "" {
		return s.repo.Sale(id)
	}
	return s.GetSaleForTenant(id, tenant)
}
func (s *Service) operationForTenantMutation(id, tenant string) (Operation, error) {
	if tenant == "" {
		return s.repo.Operation(id)
	}
	return s.GetOperationForTenant(id, tenant)
}
func (s *Service) Replay(k string) (ReplayRecord, bool)     { return s.repo.Replay(k) }
func (s *Service) PutReplay(k string, v ReplayRecord) error { return s.repo.PutReplay(k, v) }
func (s *Service) QueueFiscalEvent(saleID string, op Operation) error {
	externalID := ""
	if sale, err := s.repo.Sale(saleID); err == nil {
		externalID = sale.ExternalID
	}
	e := WebhookEvent{EventID: "event-" + op.ID, EventType: "fiscal.operation.updated", APIVersion: "2026-08-07", TenantID: op.TenantID, ResourceID: saleID, ResourceVersion: op.Version, OccurredAt: time.Now().UTC(), Data: map[string]any{"state": op.State, "operation_id": op.ID, "operation_type": op.Type, "external_id": externalID, "fiscal_reference": op.FiscalReference, "error_code": op.ErrorCode}}
	return s.repo.AddOutbox(OutboxItem{ID: e.EventID, Event: e, NextAttempt: time.Now().UTC()})
}
func (s *Service) PendingOutbox(now time.Time) []OutboxItem { return s.repo.PendingOutbox(now) }
func (s *Service) UpdateOutbox(v OutboxItem) error          { return s.repo.UpdateOutbox(v) }
func (s *Service) Readiness(device string) map[string]any {
	ready := s.driver != nil && s.driver.Probe() == nil
	state, transport := "READY", "REST"
	if !ready {
		state, transport = "UNAVAILABLE", "BLOCK"
	}
	return map[string]any{"ready": ready, "device_id": device, "observed_at": time.Now().UTC(), "components": map[string]string{"driver": state, "fiscal_device": state}, "recommended_transport": transport}
}
func (s *Service) ReadinessForTenant(device, tenant string) map[string]any {
	registryReady := s.activeDeviceForTenant(device, tenant)
	driverReady := registryReady && s.driver != nil && s.driver.Probe() == nil
	state, transport := "READY", "REST"
	if !driverReady {
		state, transport = "UNAVAILABLE", "BLOCK"
	}
	registryState := "ACTIVE"
	if !registryReady {
		registryState = "UNAVAILABLE"
	}
	return map[string]any{"ready": driverReady, "device_id": device, "observed_at": time.Now().UTC(), "components": map[string]string{"registry": registryState, "driver": state, "fiscal_device": state}, "recommended_transport": transport}
}
func (s *Service) activeDeviceForTenant(device, tenant string) bool {
	if device == "" || tenant == "" {
		return false
	}
	v, err := s.repo.Resource("device", device)
	if err != nil || v.TenantID != tenant || stringField(v.Data, "status") != "ACTIVE" {
		return false
	}
	return oneOf(stringField(v.Data, "kind"), "FISCAL_DEVICE", "SMART_DEVICE")
}
func validTax(v string) bool { return len(v) == 1 && strings.Contains("ABCDEFGH", v) }
func validMoney(m Money) bool {
	if m.Currency != "EUR" {
		return false
	}
	parts := strings.Split(m.Amount, ".")
	if len(parts) != 2 || len(parts[1]) != 2 {
		return false
	}
	_, e := strconv.ParseInt(strings.ReplaceAll(m.Amount, ".", ""), 10, 64)
	return e == nil
}
func validQuantity(v string) bool { x, e := parseFixed(v, 3); return e == nil && x > 0 }
func parseFixed(v string, scale int) (int64, error) {
	parts := strings.Split(v, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, errors.New("invalid decimal")
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
	}
	if len(frac) > scale {
		return 0, errors.New("scale")
	}
	for len(frac) < scale {
		frac += "0"
	}
	return strconv.ParseInt(parts[0]+frac, 10, 64)
}
func saleTotal(s Sale) int64 {
	var total int64
	for _, l := range s.Lines {
		price, _ := parseFixed(l.UnitPrice.Amount, 2)
		qty, _ := parseFixed(l.Quantity, 3)
		total += (price*qty + 500) / 1000
	}
	return total
}
func formatFixed(v int64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}
func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
