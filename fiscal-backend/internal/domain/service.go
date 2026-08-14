package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
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

// Driver is the transport-neutral device boundary. Domain code reserves state
// before calling it and maps ambiguous failures to UNKNOWN, never to a retry of
// a potentially completed physical side effect.
type Driver interface {
	Execute(Operation, Sale, PaymentRequest) (string, string)
	Probe() error
}
type queuedDriver interface {
	Queue(Operation, Sale, PaymentRequest) error
}
type durableQueuedDriver interface {
	Prepare(Operation, Sale, PaymentRequest) (ResourceRecord, error)
	Publish(ResourceRecord) error
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
	digest := sha256.Sum256([]byte(op.ID))
	document := int64(binary.BigEndian.Uint32(digest[:4])%9_999_999) + 1
	return strconv.FormatInt(document, 10), ""
}
func (s *Simulator) Probe() error {
	if !s.enabled {
		return errors.New("fiscal device unavailable")
	}
	return nil
}
func (s *Simulator) DeviceTime() (time.Time, error) {
	if !s.enabled {
		return time.Time{}, errors.New("fiscal device unavailable")
	}
	return time.Now().UTC(), nil
}
func (s *Simulator) SetDeviceTime(at time.Time) error {
	if !s.enabled || at.IsZero() {
		return errors.New("fiscal device unavailable")
	}
	return nil
}

// Service is the authoritative fiscalization layer. It owns legal identifiers,
// lifecycle transitions, route fencing, idempotency and immutable evidence;
// POS clients submit intent and render its projections.
type Service struct {
	repo                            Repository
	driver                          Driver
	bleSigningKey                   []byte
	policy                          PolicyCatalog
	requireHardwareSyncSignatures   bool
	deviceCredentialIssuer          DeviceCredentialIssuer
	compositeBindingPublisher       CompositeBindingPublisher
	localTokenIssuer                string
	localTokenSigningKID            string
	localTokenPublicKeyDERBase64    string
	spaDeploymentDescriptorURL      string
	spaDeploymentSigningKID         string
	spaDeploymentPublicKeyDERBase64 string
}

type CompositeBindingPublisher interface {
	PublishCompositeBinding(string, string, []byte) error
}

func (s *Service) SetCompositeBindingPublisher(v CompositeBindingPublisher) {
	s.compositeBindingPublisher = v
}

// boundDriver revalidates the complete immutable route snapshot immediately
// before use. The configured driver is a transport dispatcher (MQTT/simulator),
// not a device choice; the selected device remains the snapshot carried in the
// command. Any rebind or identity mismatch is fenced without fallback.
func (s *Service) boundDriver(tenant, register string, expected FiscalDeviceSnapshot) (Driver, error) {
	if s.driver == nil {
		return nil, errors.New("device route unavailable")
	}
	if tenant == "" { // isolated unit/dev simulator path has no registry authority
		return s.driver, nil
	}
	current, err := s.activeFiscalDeviceSnapshot(register, tenant, time.Now().UTC())
	if err != nil || expected.DeviceID == "" || current.DeviceID != expected.DeviceID ||
		current.BindingVersion != expected.BindingVersion || current.Serial != expected.Serial ||
		current.FiscalMemoryNumber != expected.FiscalMemoryNumber || current.Vendor != expected.Vendor ||
		current.Model != expected.Model {
		return nil, errors.New("stale or mismatched device route")
	}
	return s.driver, nil
}

func (s *Service) SetRequireHardwareSyncSignatures(required bool) {
	s.requireHardwareSyncSignatures = required
}

// ExpireDeviceCommand fail-closes an immutable command that could not be
// delivered while its signed authority was valid. It never creates a fresh
// envelope and never repeats a possible device side effect.
func (s *Service) ExpireDeviceCommand(operationID string) error {
	op, err := s.repo.Operation(operationID)
	if err != nil || op.State != "EXECUTING" {
		return err
	}
	now := time.Now().UTC()
	op.State = "UNKNOWN"
	op.ErrorCode = "DEVICE_COMMAND_EXPIRED_BEFORE_ACCEPTANCE"
	op.AllowedActions = []string{"RECONCILE"}
	op.Version++
	op.UpdatedAt = now
	if op.SaleID != "" {
		sale, saleErr := s.repo.Sale(op.SaleID)
		if saleErr != nil {
			return saleErr
		}
		sale.State = "UNKNOWN"
		sale.Version++
		sale.UpdatedAt = now
		return s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
	}
	return s.repo.CommitOperationEvent(op, fiscalCommandEvent(op.RegisterID, op))
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
func (s *Service) Shifts(tenant string) []Shift {
	items := s.repo.Shifts(tenant)
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
}
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
	driver, routeErr := s.boundDriver(sale.TenantID, sale.RegisterID, sale.FiscalDevice)
	if routeErr != nil || driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	documentNumber, parseErr := strconv.ParseInt(original.FiscalReference, 10, 64)
	if parseErr != nil || documentNumber < 1 || documentNumber > 9999999 || !regexp.MustCompile(`^[0-9]{8}$`).MatchString(sale.FiscalDevice.FiscalMemoryNumber) {
		return Operation{}, errors.New("original fiscal document metadata unavailable")
	}
	op := Operation{ID: newID("op"), TenantID: sale.TenantID, SaleID: saleID, RegisterID: sale.RegisterID, Type: "REVERSAL", State: "EXECUTING", Version: 1, OriginalFiscalReference: original.FiscalReference, ReasonCode: reason, OriginalDocumentNumber: documentNumber, OriginalDocumentAt: original.UpdatedAt, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	if queued, ok := driver.(durableQueuedDriver); ok {
		command, prepareErr := queued.Prepare(op, sale, PaymentRequest{})
		if prepareErr != nil {
			return Operation{}, prepareErr
		}
		sale, e = s.repo.ReserveSaleReversalCommand(saleID, tenant, sale.Version, op, command)
		if e != nil {
			return Operation{}, e
		}
		if e = queued.Publish(command); e != nil {
			return op, nil
		}
		return op, nil
	}
	sale, e = s.repo.ReserveSaleReversal(saleID, tenant, sale.Version, op)
	if e != nil {
		return Operation{}, e
	}
	ref, code := driver.Execute(op, sale, PaymentRequest{})
	op.Version++
	op.UpdatedAt = time.Now().UTC()
	if code == "FISCAL_RESULT_UNKNOWN" {
		op.State, op.ErrorCode, op.AllowedActions = "UNKNOWN", code, []string{"RECONCILE"}
		sale.State = "UNKNOWN"
		sale.Version++
		sale.UpdatedAt = op.UpdatedAt
		return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
	}
	if code != "" {
		op.State, op.ErrorCode = "FAILED", code
		sale.State = "COMPLETED"
		sale.Version++
		sale.UpdatedAt = op.UpdatedAt
		return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
	}
	op.State, op.FiscalReference = "FISCALIZED", ref
	// The locked Fiscal Sale contract represents a successfully stornoed sale as
	// CANCELLED; the REVERSAL operation and webhook retain explicit storno semantics.
	sale.State = "CANCELLED"
	sale.Version++
	sale.UpdatedAt = now
	return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
}
func (s *Service) FiscalOperation(register, typ, tenant string) (Operation, error) {
	op, e := s.reserveFiscalOperation(register, typ, tenant)
	if e != nil {
		return Operation{}, e
	}
	device, deviceErr := s.activeFiscalDeviceSnapshot(register, tenant, time.Now().UTC())
	if deviceErr == nil {
		if driver, routeErr := s.boundDriver(tenant, register, device); routeErr == nil {
			if queued, ok := driver.(durableQueuedDriver); ok {
				command, prepareErr := queued.Prepare(op, Sale{TenantID: tenant, RegisterID: register, FiscalDevice: device}, PaymentRequest{})
				if prepareErr != nil {
					return Operation{}, prepareErr
				}
				if putErr := s.repo.PutResource(command); putErr != nil {
					return Operation{}, putErr
				}
				if publishErr := queued.Publish(command); publishErr != nil {
					return op, nil
				}
				return op, nil
			}
		}
	}
	op = s.executeReservedFiscalOperation(op, register, tenant)
	return op, s.repo.CommitOperationEvent(op, fiscalCommandEvent(register, op))
}

func (s *Service) reserveFiscalOperation(register, typ, tenant string) (Operation, error) {
	if register == "" {
		return Operation{}, errors.New("register required")
	}
	allowed := []string{"CASH_IN", "CASH_OUT", "X", "Z", "KLEN", "FISCAL_MEMORY", "OPERATOR", "DEPARTMENT", "PLU", "PRINTER_TEST", "DEVICE_IDENTITY", "DEVICE_TIME"}
	if !contains(allowed, typ) {
		return Operation{}, errors.New("unsupported operation")
	}
	if tenant != "" && !s.registerHasActiveFiscalDevice(register, tenant) {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	device := FiscalDeviceSnapshot{}
	if tenant != "" {
		var err error
		device, err = s.activeFiscalDeviceSnapshot(register, tenant, time.Now().UTC())
		if err != nil {
			return Operation{}, errors.New("fiscal device unavailable")
		}
	}
	driver, routeErr := s.boundDriver(tenant, register, device)
	if routeErr != nil || driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	now := time.Now().UTC()
	op := Operation{ID: newID("op"), TenantID: tenant, RegisterID: register, Type: typ, State: "EXECUTING", Version: 1, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.PutOperation(op); err != nil {
		return Operation{}, err
	}
	return op, nil
}

func (s *Service) executeReservedFiscalOperation(op Operation, register, tenant string) Operation {
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
	return op
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

type clockCapableDriver interface {
	DeviceTime() (time.Time, error)
	SetDeviceTime(time.Time) error
}

func (s *Service) RefreshReadiness(workstation, tenant string) (ReadinessLease, error) {
	if len(s.bleSigningKey) < 16 {
		return ReadinessLease{}, errors.New("readiness signing unavailable")
	}
	now := time.Now().UTC()
	device, err := s.activeFiscalDeviceSnapshot(workstation, tenant, now)
	if err != nil || !bgFMINPattern.MatchString(device.FiscalDeviceNumber) || s.driver == nil || s.driver.Probe() != nil {
		return ReadinessLease{}, errors.New("fiscal device not ready")
	}
	lease := ReadinessLease{LeaseID: newID("ready"), TenantID: tenant, WorkstationID: workstation, FiscalDeviceID: device.DeviceID, FiscalDeviceNumber: device.FiscalDeviceNumber, ProfileVersion: BGProfileVersion, Ready: true, CheckedAt: now, ValidUntil: now.Add(BGReadinessLeaseMax)}
	lease.Signature = s.signReadiness(lease)
	err = s.repo.PutResource(ResourceRecord{Kind: "readiness_lease", TenantID: tenant, ID: lease.LeaseID, Version: 1, Data: asMap(lease), CreatedAt: now, UpdatedAt: now})
	return lease, err
}
func (s *Service) CurrentReadiness(workstation, tenant string) (ReadinessLease, error) {
	var newest ReadinessLease
	for _, resource := range s.repo.Resources("readiness_lease", tenant) {
		var lease ReadinessLease
		b, _ := json.Marshal(resource.Data)
		if json.Unmarshal(b, &lease) == nil && lease.WorkstationID == workstation && lease.CheckedAt.After(newest.CheckedAt) {
			newest = lease
		}
	}
	device, err := s.activeFiscalDeviceSnapshot(workstation, tenant, time.Now().UTC())
	if err != nil || !newest.Ready || !time.Now().UTC().Before(newest.ValidUntil) || newest.FiscalDeviceID != device.DeviceID || newest.FiscalDeviceNumber != device.FiscalDeviceNumber || newest.ProfileVersion != BGProfileVersion || !hmac.Equal([]byte(newest.Signature), []byte(s.signReadiness(newest))) {
		return ReadinessLease{}, errors.New("readiness lease unavailable")
	}
	return newest, nil
}
func (s *Service) signReadiness(v ReadinessLease) string {
	mac := hmac.New(sha256.New, s.bleSigningKey)
	fmt.Fprintf(mac, "%s\n%s\n%s\n%s\n%s\n%s\n%d", v.TenantID, v.WorkstationID, v.FiscalDeviceID, v.FiscalDeviceNumber, v.ProfileVersion, v.CheckedAt.UTC().Format(time.RFC3339Nano), v.ValidUntil.UnixNano())
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Service) SyncWorkstationClock(workstation, tenant string) (DeviceClockSync, error) {
	driver, ok := s.driver.(clockCapableDriver)
	if !ok {
		return DeviceClockSync{}, errors.New("device clock capability unavailable")
	}
	now := time.Now().UTC()
	device, err := s.activeFiscalDeviceSnapshot(workstation, tenant, now)
	if err != nil {
		return DeviceClockSync{}, err
	}
	deviceTime, err := driver.DeviceTime()
	if err != nil {
		return DeviceClockSync{}, err
	}
	drift := int64(deviceTime.Sub(now).Seconds())
	set := drift > 30 || drift < -30
	if set {
		if err = driver.SetDeviceTime(now); err != nil {
			return DeviceClockSync{}, err
		}
		deviceTime, err = driver.DeviceTime()
		if err != nil {
			return DeviceClockSync{}, err
		}
		drift = int64(deviceTime.Sub(now).Seconds())
	}
	event := DeviceClockSync{EventID: newID("clock"), TenantID: tenant, WorkstationID: workstation, DeviceID: device.DeviceID, BusinessDate: now.Format("2006-01-02"), TrustedTime: now, DeviceTime: deviceTime, DriftSeconds: drift, SetPerformed: set, Verified: drift <= 30 && drift >= -30, OccurredAt: now}
	if !event.Verified {
		return DeviceClockSync{}, errors.New("device clock drift not corrected")
	}
	err = s.repo.PutResource(ResourceRecord{Kind: "device_clock_sync_event", TenantID: tenant, ID: event.EventID, Version: 1, Data: asMap(event), CreatedAt: now, UpdatedAt: now})
	return event, err
}
func (s *Service) HasDailyClockSync(workstation, tenant string, at time.Time) bool {
	date := at.UTC().Format("2006-01-02")
	for _, r := range s.repo.Resources("device_clock_sync_event", tenant) {
		if stringField(r.Data, "workstation_id") == workstation && stringField(r.Data, "business_date") == date && r.Data["verified"] == true {
			return true
		}
	}
	return false
}
func (s *Service) BLESession(register, operator, app, tenant, actorSubject, clientPublicKey string) (map[string]any, error) {
	decodedPublicKey, publicKeyErr := base64.RawURLEncoding.DecodeString(clientPublicKey)
	if register == "" || operator == "" || app == "" || actorSubject == "" || publicKeyErr != nil || len(decodedPublicKey) != 32 || len(s.bleSigningKey) < 16 {
		return nil, errors.New("BLE session unavailable")
	}
	now := time.Now().UTC()
	device, deviceErr := s.activeFiscalDeviceSnapshot(register, tenant, now)
	registerResource, registerErr := s.repo.Resource("register", register)
	locationID := ""
	if registerErr == nil && registerResource.TenantID == tenant {
		locationID = stringField(registerResource.Data, "location_id")
	}
	if deviceErr != nil || locationID == "" || !s.activeOperatorResourceForTenant(operator, tenant, now) {
		return nil, errors.New("BLE session unavailable")
	}
	nonceBytes := make([]byte, 16)
	if _, e := rand.Read(nonceBytes); e != nil {
		return nil, e
	}
	v := BLESessionRecord{SessionID: newID("ble"), TenantID: tenant, LocationID: locationID, RegisterID: register, OperatorID: operator, AppInstanceID: app, ActorSubject: actorSubject, ClientPublicKey: clientPublicKey, DeviceID: register + "-edge", FiscalDeviceID: device.DeviceID, Scopes: []string{"fiscal.execute", "fiscal.read"}, FencingToken: now.UnixNano(), ExpiresAt: now.Add(8 * time.Hour), Nonce: base64.RawURLEncoding.EncodeToString(nonceBytes)}
	if e := s.repo.PutBLESession(v); e != nil {
		return nil, e
	}
	return s.bleResponse(v)
}
func (s *Service) RefreshBLE(id, tenant, actorSubject string) (map[string]any, error) {
	v, e := s.bleSessionForTenant(id, tenant)
	now := time.Now().UTC()
	currentDevice, deviceErr := s.activeFiscalDeviceSnapshot(v.RegisterID, v.TenantID, now)
	currentRegister, registerErr := s.repo.Resource("register", v.RegisterID)
	if e != nil || actorSubject == "" || v.ActorSubject == "" || v.ActorSubject != actorSubject || v.Revoked || (tenant != "" && v.TenantID != tenant) || !now.Before(v.ExpiresAt) || deviceErr != nil || currentDevice.DeviceID != v.FiscalDeviceID || registerErr != nil || currentRegister.TenantID != v.TenantID || stringField(currentRegister.Data, "location_id") != v.LocationID || !s.activeOperatorResourceForTenant(v.OperatorID, v.TenantID, now) {
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
	_, err := s.activeFiscalDeviceSnapshot(registerID, tenant, time.Now().UTC())
	return err == nil
}

func (s *Service) activeFiscalDeviceSnapshot(registerID, tenant string, now time.Time) (FiscalDeviceSnapshot, error) {
	register, err := s.repo.Resource("register", registerID)
	if err != nil || register.TenantID != tenant || stringField(register.Data, "status") != "ACTIVE" {
		return FiscalDeviceSnapshot{}, errors.New("register unavailable")
	}
	if !bindingIsActive(register.Data, "fiscal_device_active_from", now) {
		return FiscalDeviceSnapshot{}, errors.New("fiscal device binding inactive")
	}
	deviceID := stringField(register.Data, "fiscal_device_id")
	device, err := s.repo.Resource("device", deviceID)
	if err != nil || device.TenantID != tenant || stringField(device.Data, "status") != "ACTIVE" || !oneOf(stringField(device.Data, "kind"), "FISCAL_DEVICE", "SMART_DEVICE") {
		return FiscalDeviceSnapshot{}, errors.New("fiscal device unavailable")
	}
	return FiscalDeviceSnapshot{DeviceID: device.ID, BindingVersion: register.Version, Serial: stringField(device.Data, "serial"), FiscalDeviceNumber: stringField(device.Data, "fiscal_device_number"), FiscalMemoryNumber: stringField(device.Data, "fiscal_memory_number"), Vendor: stringField(device.Data, "vendor"), Model: stringField(device.Data, "model"), Firmware: stringField(device.Data, "firmware")}, nil
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
		SessionID, TenantID, LocationID, RegisterID, DeviceID, FiscalDeviceID, AppInstanceID, ActorSubject, ClientPublicKey string
		Scopes                                                                                                              []string
		FencingToken                                                                                                        int64
		ExpiresAt                                                                                                           time.Time
		Nonce                                                                                                               string
	}{v.SessionID, v.TenantID, v.LocationID, v.RegisterID, v.DeviceID, v.FiscalDeviceID, v.AppInstanceID, v.ActorSubject, v.ClientPublicKey, v.Scopes, v.FencingToken, v.ExpiresAt, v.Nonce}
	b, e := json.Marshal(ticket)
	if e != nil {
		return nil, e
	}
	ticketKey := s.bleSigningKey
	if device, deviceErr := s.repo.Resource("device", v.FiscalDeviceID); deviceErr == nil {
		if credentialID, _ := device.Data["credential_id"].(string); credentialID != "" {
			ticketKey = DeriveDeviceTransportKey(s.bleSigningKey, "ble-ticket", v.FiscalDeviceID, credentialID)
		}
	}
	m := hmac.New(sha256.New, ticketKey)
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
		"ble_session_id": v.SessionID, "tenant_id": v.TenantID, "location_id": v.LocationID, "register_id": v.RegisterID,
		"edge_id": v.DeviceID, "device_id": v.FiscalDeviceID, "binding_version": v.FencingToken,
		"operator_id": v.OperatorID, "app_instance_id": v.AppInstanceID,
		"protocol_version": "2026-08-07", "expires_at": v.ExpiresAt, "scopes": v.Scopes,
		"security_mode":               "OPEN_MVP",
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

type OpenSaleWithFirstLineRequest struct {
	TenantID              string   `json:"-"`
	ClientSaleSurrogateID string   `json:"client_sale_surrogate_id"`
	WorkstationID         string   `json:"workstation_id"`
	OperatorSessionID     string   `json:"operator_session_id"`
	OperatorCode          string   `json:"-"`
	Line                  SaleLine `json:"line"`
}

func (s *Service) OpenWorkstationSession(workstation, operatorCode, appInstance, actor, tenant string) (WorkstationSession, error) {
	if workstation == "" || appInstance == "" || actor == "" || !bgOperatorPattern.MatchString(operatorCode) {
		_ = s.repo.AppendAudit(tenant, actor, "LOGIN_FAILED", "workstation_session", workstation, "", nil, map[string]any{"operator_code": operatorCode, "app_instance_id": appInstance, "reason": "INVALID_SESSION_INPUT"})
		return WorkstationSession{}, errors.New("invalid workstation session")
	}
	if _, err := s.activeFiscalDeviceSnapshot(workstation, tenant, time.Now().UTC()); err != nil {
		_ = s.repo.AppendAudit(tenant, actor, "LOGIN_FAILED", "workstation_session", workstation, "", nil, map[string]any{"operator_code": operatorCode, "app_instance_id": appInstance, "reason": "FISCAL_DEVICE_UNAVAILABLE"})
		return WorkstationSession{}, err
	}
	operatorID := ""
	for _, op := range s.repo.Resources("operator", tenant) {
		if stringField(op.Data, "code") == operatorCode {
			operatorID = op.ID
			break
		}
	}
	if operatorID == "" || !s.activeOperatorForTenant(operatorCode, tenant, time.Now().UTC()) {
		_ = s.repo.AppendAudit(tenant, actor, "LOGIN_FAILED", "workstation_session", workstation, "", nil, map[string]any{"operator_code": operatorCode, "app_instance_id": appInstance, "reason": "OPERATOR_UNAVAILABLE"})
		return WorkstationSession{}, errors.New("operator unavailable")
	}
	now := time.Now().UTC()
	v := WorkstationSession{SessionID: newID("ws"), TenantID: tenant, WorkstationID: workstation, OperatorID: operatorID, OperatorCode: operatorCode, AppInstanceID: appInstance, ActorSubject: actor, ExpiresAt: now.Add(8 * time.Hour), CreatedAt: now}
	data := asMap(v)
	data["actor_subject"] = actor
	err := s.repo.PutResource(ResourceRecord{Kind: "workstation_session", TenantID: tenant, ID: v.SessionID, Version: 1, Data: data, CreatedAt: now, UpdatedAt: now})
	return v, err
}

func (s *Service) LogoutWorkstationSession(id, workstation, actor, tenant string) error {
	r, err := s.repo.Resource("workstation_session", id)
	if err != nil || r.TenantID != tenant || stringField(r.Data, "workstation_id") != workstation || stringField(r.Data, "actor_subject") != actor {
		return ErrNotFound
	}
	if revoked, _ := r.Data["revoked"].(bool); revoked {
		return nil
	}
	before := cloneMap(r.Data)
	r.Data["revoked"] = true
	r.Data["revoked_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	r.Version++
	r.UpdatedAt = time.Now().UTC()
	if err = s.repo.PutResource(r); err != nil {
		return err
	}
	return s.repo.AppendAudit(tenant, actor, "LOGOUT", "workstation_session", id, "", before, cloneMap(r.Data))
}
func (s *Service) WorkstationSession(id, workstation, tenant string) (WorkstationSession, error) {
	r, err := s.repo.Resource("workstation_session", id)
	if err != nil || r.TenantID != tenant {
		return WorkstationSession{}, ErrNotFound
	}
	var v WorkstationSession
	b, _ := json.Marshal(r.Data)
	revoked, _ := r.Data["revoked"].(bool)
	if json.Unmarshal(b, &v) != nil || revoked || v.WorkstationID != workstation || !time.Now().UTC().Before(v.ExpiresAt) || !s.activeOperatorForTenant(v.OperatorCode, tenant, time.Now().UTC()) {
		return WorkstationSession{}, errors.New("workstation session inactive")
	}
	return v, nil
}

func (s *Service) OpenSaleWithFirstLine(in OpenSaleWithFirstLineRequest) (Sale, error) {
	if in.TenantID == "" || in.ClientSaleSurrogateID == "" || in.WorkstationID == "" || in.OperatorSessionID == "" {
		return Sale{}, errors.New("invalid open-with-line request")
	}
	session, err := s.WorkstationSession(in.OperatorSessionID, in.WorkstationID, in.TenantID)
	if err != nil {
		return Sale{}, err
	}
	in.OperatorCode = session.OperatorCode
	if in.Line.LineID == "" || in.Line.Name == "" || !validMoney(in.Line.UnitPrice) || !validDiscount(in.Line.Discount) || !validQuantity(in.Line.Quantity) || !validTax(in.Line.TaxGroup) || !s.policy.AllowsTaxGroup(in.Line.TaxGroup, time.Now().UTC()) {
		return Sale{}, errors.New("invalid first line")
	}
	now := time.Now().UTC()
	if !s.HasDailyClockSync(in.WorkstationID, in.TenantID, now) {
		return Sale{}, errors.New("daily clock synchronization required")
	}
	if _, err := s.CurrentReadiness(in.WorkstationID, in.TenantID); err != nil {
		return Sale{}, err
	}
	device, err := s.activeFiscalDeviceSnapshot(in.WorkstationID, in.TenantID, now)
	if err != nil || !bgFMINPattern.MatchString(device.FiscalDeviceNumber) {
		return Sale{}, errors.New("verified fiscal device number unavailable")
	}
	if !s.activeOperatorForTenant(in.OperatorCode, in.TenantID, time.Now().UTC()) {
		return Sale{}, errors.New("operator unavailable")
	}
	hasShift := false
	for _, shift := range s.repo.Shifts(in.TenantID) {
		if shift.RegisterID == in.WorkstationID && shift.OperatorID == in.OperatorCode && shift.State == "OPEN" {
			hasShift = true
			break
		}
	}
	if !hasShift {
		return Sale{}, errors.New("open shift required")
	}
	register, err := s.repo.Resource("register", in.WorkstationID)
	if err != nil || stringField(register.Data, "location_id") == "" {
		return Sale{}, errors.New("workstation location unavailable")
	}
	sale := Sale{ID: newID("sale"), TenantID: in.TenantID, ExternalID: in.ClientSaleSurrogateID, LocationID: stringField(register.Data, "location_id"), RegisterID: in.WorkstationID, OperatorID: in.OperatorCode, State: "OPEN", Version: 1, Lines: []SaleLine{}, Payments: []PaymentRecord{}, FiscalDevice: device, CreatedAt: now, UpdatedAt: now}
	return s.repo.OpenSaleWithFirstLine(sale, in.Line, device.FiscalDeviceNumber)
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
	locationID := ""
	if in.TenantID != "" {
		register, err := s.repo.Resource("register", in.RegisterID)
		if err != nil || register.TenantID != in.TenantID || stringField(register.Data, "location_id") == "" {
			return Sale{}, errors.New("register location unavailable")
		}
		locationID = stringField(register.Data, "location_id")
	}
	now := time.Now().UTC()
	device := FiscalDeviceSnapshot{}
	if in.TenantID == "" {
		device = FiscalDeviceSnapshot{DeviceID: in.RegisterID, FiscalDeviceNumber: "00000001", FiscalMemoryNumber: "00000001", Vendor: "SIMULATOR", Model: "DEV"}
	}
	v := Sale{ID: newID("sale"), TenantID: in.TenantID, ExternalID: in.ExternalID, LocationID: locationID, RegisterID: in.RegisterID, OperatorID: in.OperatorID, State: "DRAFT", Version: 1, Lines: []SaleLine{}, Payments: []PaymentRecord{}, FiscalDevice: device, CreatedAt: now, UpdatedAt: now}
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

func (s *Service) ChangeLineExpectedForTenant(id, lineID string, expected int64, replacement SaleLine, tenant string) (Sale, error) {
	if lineID == "" || replacement.LineID != lineID || replacement.Name == "" || !validMoney(replacement.UnitPrice) || !validDiscount(replacement.Discount) || !validQuantity(replacement.Quantity) || !validTax(replacement.TaxGroup) || !s.policy.AllowsTaxGroup(replacement.TaxGroup, time.Now().UTC()) {
		return Sale{}, errors.New("invalid line")
	}
	v, err := s.saleForTenantMutation(id, tenant)
	if err != nil || v.Version != expected {
		return Sale{}, errors.New("version conflict")
	}
	lines := append([]SaleLine(nil), v.Lines...)
	found := false
	for i := range lines {
		if lines[i].LineID == lineID {
			lines[i] = replacement
			found = true
			break
		}
	}
	if !found {
		return Sale{}, ErrNotFound
	}
	probe := v
	probe.Lines = lines
	if _, err = saleTotal(probe); err != nil {
		return Sale{}, errors.New("line total overflow")
	}
	return s.repo.ReplaceSaleLinesExpected(id, tenant, expected, lines, "SALE_LINE_CHANGED")
}

func (s *Service) CancelLineExpectedForTenant(id, lineID string, expected int64, tenant string) (Sale, error) {
	if lineID == "" {
		return Sale{}, errors.New("line required")
	}
	v, err := s.saleForTenantMutation(id, tenant)
	if err != nil || v.Version != expected {
		return Sale{}, errors.New("version conflict")
	}
	lines := make([]SaleLine, 0, len(v.Lines))
	found := false
	for _, line := range v.Lines {
		if line.LineID == lineID {
			found = true
			continue
		}
		lines = append(lines, line)
	}
	if !found {
		return Sale{}, ErrNotFound
	}
	return s.repo.ReplaceSaleLinesExpected(id, tenant, expected, lines, "SALE_LINE_CANCELLED")
}

func (s *Service) SalesForTenant(tenant, operator, register, state string) []Sale {
	items := s.repo.Sales(tenant)
	out := make([]Sale, 0, len(items))
	for _, v := range items {
		if operator != "" && v.OperatorID != operator {
			continue
		}
		if register != "" && v.RegisterID != register {
			continue
		}
		if state != "" && v.State != state {
			continue
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out
}
func (s *Service) addLineForTenant(id string, expected int64, line SaleLine, tenant string) (Sale, error) {
	if line.LineID == "" || line.Name == "" || !validMoney(line.UnitPrice) || !validDiscount(line.Discount) || !validQuantity(line.Quantity) || !validTax(line.TaxGroup) || !s.policy.AllowsTaxGroup(line.TaxGroup, time.Now().UTC()) {
		return Sale{}, errors.New("invalid line")
	}
	current, err := s.saleForTenantMutation(id, tenant)
	if err != nil || current.Version != expected {
		return Sale{}, errors.New("version conflict")
	}
	current.Lines = append(current.Lines, line)
	if _, err = saleTotal(current); err != nil {
		return Sale{}, errors.New("line total overflow")
	}
	return s.repo.AddSaleLineExpected(id, tenant, expected, line)
}
func (s *Service) Pay(id string, p PaymentRequest) (Operation, error) {
	return s.payForTenant(id, p, "")
}
func (s *Service) PayForTenant(id string, p PaymentRequest, tenant string) (Operation, error) {
	return s.payForTenant(id, p, tenant)
}

// FinalizeSaleForTenant reserves one immutable operation for an entire receipt.
// Adapters expand it into ordered payment/fiscal steps and journal each boundary
// before I/O, so split tender cannot create multiple physical receipts.
func (s *Service) FinalizeSaleForTenant(id string, in SaleFinalizeRequest, tenant string) (Operation, error) {
	sale, err := s.saleForTenantMutation(id, tenant)
	if err != nil {
		return Operation{}, err
	}
	plan := ReceiptFinalizePlan{ClientOperationID: in.ClientOperationID, ReceiptSessionID: in.ReceiptSessionID, SaleID: id, UNP: sale.UNP, Items: sale.Lines, Payments: in.Payments, ExpectedTotal: in.ExpectedTotal}
	if err = ValidateReceiptPlan(plan); err != nil {
		return Operation{}, err
	}
	total, err := saleTotal(sale)
	if err != nil {
		return Operation{}, err
	}
	expected, err := receiptMoneyCents(in.ExpectedTotal.Amount)
	if err != nil || expected != total {
		return Operation{}, errors.New("finalize total mismatch")
	}
	if sale.State != "OPEN" || len(sale.Lines) == 0 {
		return Operation{}, errors.New("sale not finalizable")
	}
	if sale.TenantID != "" && !s.registerHasActiveFiscalDevice(sale.RegisterID, sale.TenantID) {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	if sale.TenantID != "" {
		sale.FiscalDevice, err = s.activeFiscalDeviceSnapshot(sale.RegisterID, sale.TenantID, time.Now().UTC())
		if err != nil {
			return Operation{}, errors.New("fiscal device unavailable")
		}
	}
	for _, p := range in.Payments {
		if p.Type == "CARD" && (p.TerminalPolicy == "NONE" || (sale.TenantID != "" && !s.registerHasActivePaymentTerminal(sale.RegisterID, sale.TenantID))) {
			return Operation{}, errors.New("payment terminal unavailable")
		}
	}
	driver, routeErr := s.boundDriver(sale.TenantID, sale.RegisterID, sale.FiscalDevice)
	if routeErr != nil || driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	now := time.Now().UTC()
	op := Operation{ID: in.ClientOperationID, ClientOperationID: in.ClientOperationID, ReceiptSessionID: in.ReceiptSessionID, TenantID: sale.TenantID, SaleID: id, RegisterID: sale.RegisterID, Type: "SALE_FINALIZE", State: "EXECUTING", Version: 1, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	for _, p := range in.Payments {
		sale.Payments = append(sale.Payments, PaymentRecord{PaymentID: p.PaymentID, Type: p.Type, Amount: p.Amount, CreatedAt: now})
	}
	if queued, ok := driver.(durableQueuedDriver); ok {
		command, prepErr := queued.Prepare(op, sale, PaymentRequest{})
		if prepErr != nil {
			return Operation{}, prepErr
		}
		sale, err = s.repo.ReserveSalePaymentCommand(id, tenant, sale.Version, op, sale.FiscalDevice, command)
		if err != nil {
			return Operation{}, err
		}
		if publishErr := queued.Publish(command); publishErr != nil {
			return op, nil
		}
		return op, nil
	}
	ref, code := driver.Execute(op, sale, PaymentRequest{})
	op.Version++
	op.UpdatedAt = time.Now().UTC()
	if code != "" {
		op.State = "UNKNOWN"
		op.ErrorCode = code
		op.AllowedActions = []string{"RECONCILE"}
		sale.State = "UNKNOWN"
	} else {
		op.State = "FISCALIZED"
		op.FiscalReference = ref
		sale.State = "COMPLETED"
		sale.FiscalOperationID = op.ID
	}
	sale.Version++
	sale.UpdatedAt = op.UpdatedAt
	return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
}
func (s *Service) payForTenant(id string, p PaymentRequest, tenant string) (Operation, error) {
	sale, e := s.saleForTenantMutation(id, tenant)
	if e != nil {
		return Operation{}, e
	}
	if sale.State != "OPEN" || len(sale.Lines) == 0 || p.PaymentID == "" || !validMoney(p.Amount) || !contains([]string{"CASH", "CARD"}, p.Type) {
		return Operation{}, errors.New("payment not allowed")
	}
	// New SUPTO aggregates are identifiable by their immutable regulatory
	// projection. Payment always performs a fresh physical check; a lease used
	// to open the sale is intentionally insufficient at this irreversible edge.
	if len(sale.RegulatoryIdentifiers) > 0 {
		if !s.HasDailyClockSync(sale.RegisterID, sale.TenantID, time.Now().UTC()) {
			return Operation{}, errors.New("daily clock synchronization required")
		}
		if _, e = s.RefreshReadiness(sale.RegisterID, sale.TenantID); e != nil {
			return Operation{}, errors.New("fresh payment readiness failed")
		}
	}
	if sale.TenantID != "" && !s.registerHasActiveFiscalDevice(sale.RegisterID, sale.TenantID) {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	if sale.TenantID != "" {
		snapshot, snapshotErr := s.activeFiscalDeviceSnapshot(sale.RegisterID, sale.TenantID, time.Now().UTC())
		if snapshotErr != nil {
			return Operation{}, errors.New("fiscal device unavailable")
		}
		sale.FiscalDevice = snapshot
	}
	if p.Type == "CARD" {
		if p.TerminalPolicy == "NONE" {
			return Operation{}, errors.New("card payment requires terminal")
		}
		if sale.TenantID != "" && !s.registerHasActivePaymentTerminal(sale.RegisterID, sale.TenantID) {
			return Operation{}, errors.New("payment terminal unavailable")
		}
	}
	driver, routeErr := s.boundDriver(sale.TenantID, sale.RegisterID, sale.FiscalDevice)
	if routeErr != nil || driver.Probe() != nil {
		return Operation{}, errors.New("fiscal device unavailable")
	}
	amount, _ := parseFixed(p.Amount.Amount, 2)
	total, totalErr := saleTotal(sale)
	if totalErr != nil {
		return Operation{}, errors.New("invalid sale total")
	}
	paid := int64(0)
	for _, x := range sale.Payments {
		if x.PaymentID == p.PaymentID {
			return Operation{}, errors.New("duplicate payment id")
		}
		v, _ := parseFixed(x.Amount.Amount, 2)
		if v > math.MaxInt64-paid {
			return Operation{}, errors.New("invalid paid total")
		}
		paid += v
	}
	if amount <= 0 || paid > total || amount > total-paid {
		return Operation{}, errors.New("payment amount exceeds balance")
	}
	now := time.Now().UTC()
	op := Operation{ID: newID("op"), TenantID: sale.TenantID, SaleID: id, RegisterID: sale.RegisterID, Type: "FISCAL_SALE", State: "EXECUTING", Version: 1, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
	if queued, ok := driver.(durableQueuedDriver); ok {
		command, prepareErr := queued.Prepare(op, sale, p)
		if prepareErr != nil {
			return Operation{}, prepareErr
		}
		sale, e = s.repo.ReserveSalePaymentCommand(id, tenant, sale.Version, op, sale.FiscalDevice, command)
		if e != nil {
			return Operation{}, e
		}
		if e = queued.Publish(command); e != nil {
			// The command remains durable and will be republished by the bridge
			// after reconnect/restart. Publication failure is not a fiscal result.
			return op, nil
		}
		return op, nil
	}
	sale, e = s.repo.ReserveSalePayment(id, tenant, sale.Version, op, sale.FiscalDevice)
	if e != nil {
		return Operation{}, e
	}
	if queued, ok := driver.(queuedDriver); ok {
		if e = queued.Queue(op, sale, p); e != nil {
			op.State = "UNKNOWN"
			op.ErrorCode = "MQTT_COMMAND_PUBLICATION_UNKNOWN"
			op.AllowedActions = []string{"RECONCILE"}
			op.Version++
			op.UpdatedAt = time.Now().UTC()
			sale.State = "UNKNOWN"
			_ = s.repo.CommitSaleOperation(sale, op)
			return op, nil
		}
		return op, nil
	}
	ref, code := driver.Execute(op, sale, p)
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
			receiptBytes, _ := json.Marshal(map[string]any{"sale_id": sale.ID, "location_id": sale.LocationID, "register_id": sale.RegisterID, "operator_id": sale.OperatorID, "operation_id": op.ID, "unp": sale.UNP, "fiscal_reference": op.FiscalReference, "issued_at": op.UpdatedAt, "total": Money{Amount: formatFixed(total), Currency: "EUR"}, "fiscal_device": sale.FiscalDevice, "fiscal_device_number": sale.FiscalDevice.FiscalDeviceNumber, "fiscal_memory_number": sale.FiscalDevice.FiscalMemoryNumber, "lines": sale.Lines, "payments": sale.Payments})
			sale.Version++
			sale.UpdatedAt = op.UpdatedAt
			return op, s.repo.CommitSaleOperationArtifactEvent(sale, op, sale.ReceiptArtifactID, sale.TenantID, receiptBytes, fiscalOperationEvent(sale, op))
		} else {
			op.State = "PAYMENT_ACCEPTED"
			sale.State = "OPEN"
		}
	}
	sale.Version++
	sale.UpdatedAt = op.UpdatedAt
	return op, s.repo.CommitSaleOperationEvent(sale, op, fiscalOperationEvent(sale, op))
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
	var items []Operation
	if r, ok := s.repo.(interface{ OperationsForTenant(string) []Operation }); ok {
		items = r.OperationsForTenant(tenant)
	} else {
		all := s.repo.Operations()
		if tenant == "" {
			items = all
		} else {
			items = make([]Operation, 0)
			for _, x := range all {
				if x.TenantID == tenant {
					items = append(items, x)
				}
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items
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
	op := Operation{ID: newID("op"), TenantID: v.TenantID, SaleID: v.ID, RegisterID: v.RegisterID, Type: "CANCEL_SALE", State: "CANCELLED", Version: 1, Simulated: true, AllowedActions: []string{}, CreatedAt: now, UpdatedAt: now}
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
	return op, s.repo.CommitOperationEvent(op, reconciliationRequiredEvent(op))
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
	total, err := saleTotal(v)
	if err != nil {
		return nil, errors.New("invalid sale total")
	}
	return map[string]any{"sale_id": v.ID, "operation_id": v.FiscalOperationID, "unp": v.UNP, "state": v.State, "fiscal_reference": ref, "issued_at": v.UpdatedAt, "total": Money{Amount: formatFixed(total), Currency: "EUR"}, "artifact_id": v.ReceiptArtifactID, "fiscal_device": v.FiscalDevice, "fiscal_device_number": v.FiscalDevice.FiscalDeviceNumber, "fiscal_memory_number": v.FiscalDevice.FiscalMemoryNumber, "lines": v.Lines, "payments": v.Payments}, nil
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
func (s *Service) Replay(k string) (ReplayRecord, bool) { return s.repo.Replay(k) }
func (s *Service) ClaimReplay(k, hash string) (ReplayRecord, bool, error) {
	return s.repo.ClaimReplay(k, hash)
}
func (s *Service) PutReplay(k string, v ReplayRecord) error { return s.repo.PutReplay(k, v) }
func (s *Service) PutAmbiguousReplay(k string, v ReplayRecord) error {
	return s.repo.PutAmbiguousReplay(k, v)
}
func (s *Service) QueueFiscalEvent(saleID string, op Operation) error {
	externalID := ""
	if sale, err := s.repo.Sale(saleID); err == nil {
		externalID = sale.ExternalID
	}
	e := WebhookEvent{EventID: "event-" + op.ID, EventType: "fiscal.operation.updated", APIVersion: "2026-08-07", TenantID: op.TenantID, ResourceID: saleID, ResourceVersion: op.Version, OccurredAt: time.Now().UTC(), Data: map[string]any{"state": op.State, "operation_id": op.ID, "operation_type": op.Type, "external_id": externalID, "fiscal_reference": op.FiscalReference, "error_code": op.ErrorCode}}
	return s.repo.AddOutbox(OutboxItem{ID: e.EventID, Event: e, NextAttempt: time.Now().UTC()})
}

func fiscalOperationEvent(sale Sale, op Operation) OutboxItem {
	at := op.UpdatedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	e := WebhookEvent{EventID: "event-" + op.ID, EventType: "fiscal.operation.updated", APIVersion: "2026-08-07", TenantID: op.TenantID, ResourceID: sale.ID, ResourceVersion: op.Version, OccurredAt: at, Data: map[string]any{"state": op.State, "operation_id": op.ID, "operation_type": op.Type, "external_id": sale.ExternalID, "fiscal_reference": op.FiscalReference, "error_code": op.ErrorCode}}
	return OutboxItem{ID: e.EventID, Event: e, NextAttempt: at}
}

func fiscalCommandEvent(register string, op Operation) OutboxItem {
	at := op.UpdatedAt
	if at.IsZero() {
		at = time.Now().UTC()
	}
	e := WebhookEvent{EventID: "event-" + op.ID, EventType: "fiscal.operation.updated", APIVersion: "2026-08-07", TenantID: op.TenantID, ResourceID: op.ID, ResourceVersion: op.Version, OccurredAt: at, Data: map[string]any{"state": op.State, "operation_id": op.ID, "operation_type": op.Type, "register_id": register, "fiscal_reference": op.FiscalReference, "error_code": op.ErrorCode}}
	return OutboxItem{ID: e.EventID, Event: e, NextAttempt: at}
}

func reconciliationRequiredEvent(op Operation) OutboxItem {
	at := op.UpdatedAt
	resourceID := op.SaleID
	if resourceID == "" {
		resourceID = op.ID
	}
	e := WebhookEvent{EventID: fmt.Sprintf("event-reconcile-%s-%d", op.ID, op.Version), EventType: "fiscal.operation.reconciliation_required", APIVersion: "2026-08-07", TenantID: op.TenantID, ResourceID: resourceID, ResourceVersion: op.Version, OccurredAt: at, Data: map[string]any{"state": op.State, "operation_id": op.ID, "operation_type": op.Type, "sale_id": op.SaleID, "fiscal_reference": op.FiscalReference, "error_code": op.ErrorCode, "lookup_only": true}}
	return OutboxItem{ID: e.EventID, Event: e, NextAttempt: at}
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
func lineTotalCents(price Money, quantity string) (int64, error) {
	p, err := parseFixed(price.Amount, 2)
	if err != nil || p < 0 {
		return 0, errors.New("invalid price")
	}
	q, err := parseFixed(quantity, 3)
	if err != nil || q <= 0 || (p > 0 && q > (math.MaxInt64-500)/p) {
		return 0, errors.New("invalid quantity or line overflow")
	}
	return (p*q + 500) / 1000, nil
}

func discountedLineTotalCents(line SaleLine) (int64, error) {
	gross, err := lineTotalCents(line.UnitPrice, line.Quantity)
	if err != nil {
		return 0, err
	}
	if line.Discount == nil {
		return gross, nil
	}
	discount, err := parseFixed(line.Discount.Amount, 2)
	if err != nil || line.Discount.Currency != "EUR" || discount < 0 || discount > gross {
		return 0, errors.New("invalid line discount")
	}
	return gross - discount, nil
}

func validDiscount(v *Money) bool {
	if v == nil {
		return true
	}
	amount, err := parseFixed(v.Amount, 2)
	return err == nil && amount >= 0 && v.Currency == "EUR"
}

func saleTotal(s Sale) (int64, error) {
	var total int64
	for _, l := range s.Lines {
		line, err := discountedLineTotalCents(l)
		if err != nil || line > math.MaxInt64-total {
			return 0, errors.New("sale total overflow")
		}
		total += line
	}
	return total, nil
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
