package domain

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}
type Product struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	SKU       string    `json:"sku"`
	Barcode   *string   `json:"barcode,omitempty"`
	Name      string    `json:"name"`
	Unit      string    `json:"unit"`
	Price     Money     `json:"price"`
	TaxGroup  string    `json:"tax_group"`
	Active    bool      `json:"active"`
	Status    string    `json:"status"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type TaxGroup struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Rate      string    `json:"rate"`
	Status    string    `json:"status"`
	Version   int64     `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Employee struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	OperatorCode string    `json:"operator_code"`
	Active       bool      `json:"active"`
	Roles        []string  `json:"roles"`
	Status       string    `json:"status"`
	Version      int64     `json:"version"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type IdentityBinding struct {
	TenantID       string    `json:"tenant_id"`
	EmployeeID     string    `json:"employee_id"`
	SubjectHash    string    `json:"subject_hash"`
	IdentityIssuer string    `json:"identity_issuer"`
	BoundAt        time.Time `json:"bound_at"`
}
type OperatorSession struct {
	Authenticated bool      `json:"authenticated"`
	Employee      *Employee `json:"employee,omitempty"`
	SessionID     string    `json:"session_id"`
	ExpiresAt     time.Time `json:"expires_at"`
}
type OperatorSessionRecord struct {
	TenantID      string     `json:"tenant_id"`
	EmployeeID    string     `json:"employee_id"`
	AppInstanceID string     `json:"app_instance_id"`
	TokenHash     string     `json:"token_hash"`
	State         string     `json:"state"`
	FirstSeen     time.Time  `json:"first_seen"`
	ExpiresAt     time.Time  `json:"expires_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}
type Shift struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	RegisterID       string     `json:"register_id"`
	EmployeeID       string     `json:"employee_id"`
	State            string     `json:"state"`
	OpenedAt         time.Time  `json:"opened_at"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	ZOperationID     string     `json:"z_operation_id,omitempty"`
	ZFiscalReference string     `json:"z_fiscal_reference,omitempty"`
	CloseError       string     `json:"close_error,omitempty"`
	AllowedActions   []string   `json:"allowed_actions"`
	Version          int64      `json:"version"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// withShiftActions derives commands from persisted state rather than trusting
// client-supplied actions. A reconciliation-blocked shift intentionally cannot
// sell or close until the ambiguous Z-report is resolved.
func withShiftActions(shift Shift) Shift {
	switch shift.State {
	case "OPEN":
		shift.AllowedActions = []string{"SALE", "CLOSE"}
	case "CLOSING":
		shift.AllowedActions = []string{"READ"}
	case "BLOCKED_RECONCILIATION":
		shift.AllowedActions = []string{"READ", "RECONCILE"}
	default:
		shift.AllowedActions = []string{}
	}
	return shift
}

type Configuration struct {
	ID                     string    `json:"id"`
	TenantID               string    `json:"tenant_id"`
	LocationName           string    `json:"location_name"`
	LocationAddress        string    `json:"location_address"`
	WorkstationName        string    `json:"workstation_name"`
	FiscalRegisterID       string    `json:"fiscal_register_id"`
	LocationID             string    `json:"location_id"`
	FiscalAdapterID        string    `json:"fiscal_adapter_id"`
	BindingGeneration      int64     `json:"binding_generation"`
	AdapterBaseURL         string    `json:"adapter_base_url"`
	BLEAdvertisingIdentity string    `json:"ble_advertising_identity,omitempty"`
	BLEServiceUUID         string    `json:"ble_service_uuid,omitempty"`
	BLECommandUUID         string    `json:"ble_command_uuid,omitempty"`
	BLEEventUUID           string    `json:"ble_event_uuid,omitempty"`
	Version                int64     `json:"version"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}
type Line struct {
	LineID    string `json:"line_id"`
	ProductID string `json:"product_id,omitempty"`
	Name      string `json:"name"`
	Quantity  string `json:"quantity"`
	UnitPrice Money  `json:"unit_price"`
	Discount  *Money `json:"discount,omitempty"`
	TaxGroup  string `json:"tax_group"`
}
type Order struct {
	ID                      string         `json:"id"`
	TenantID                string         `json:"tenant_id"`
	ExternalID              string         `json:"external_id"`
	ShiftID                 string         `json:"shift_id"`
	RegisterID              string         `json:"register_id"`
	OperatorCode            string         `json:"operator_code"`
	State                   string         `json:"state"`
	Lines                   []Line         `json:"lines"`
	Total                   Money          `json:"total"`
	Payments                []OrderPayment `json:"payments,omitempty"`
	FiscalSaleID            string         `json:"fiscal_sale_id,omitempty"`
	FiscalOperationID       string         `json:"fiscal_operation_id,omitempty"`
	ReceiptReference        string         `json:"receipt_reference,omitempty"`
	CheckoutOperationID     string         `json:"checkout_operation_id,omitempty"`
	ReceiptSessionID        string         `json:"receipt_session_id,omitempty"`
	ReversalOperationID     string         `json:"reversal_operation_id,omitempty"`
	ReversalFiscalReference string         `json:"reversal_fiscal_reference,omitempty"`
	ReversalReasonCode      string         `json:"reversal_reason_code,omitempty"`
	FiscalVersion           int64          `json:"fiscal_version,omitempty"`
	Version                 int64          `json:"version"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
}

type OrderPayment struct {
	PaymentID      string `json:"payment_id,omitempty"`
	Type           string `json:"type"`
	Amount         Money  `json:"amount"`
	TerminalPolicy string `json:"terminal_policy,omitempty"`
}

type OfflineOrderImport struct {
	ExternalID        string         `json:"external_id"`
	ShiftID           string         `json:"shift_id"`
	ClientOperationID string         `json:"client_operation_id"`
	ReceiptSessionID  string         `json:"receipt_session_id"`
	FiscalReference   string         `json:"fiscal_reference,omitempty"`
	FiscalState       string         `json:"fiscal_state"`
	Lines             []Line         `json:"lines"`
	Payments          []OrderPayment `json:"payments"`
}

func (o Order) MarshalJSON() ([]byte, error) {
	type orderAlias Order
	actions := []string{}
	switch o.State {
	case "DRAFT", "OPEN":
		actions = append(actions, "ADD_LINE")
		if len(o.Lines) > 0 {
			actions = append(actions, "CHECKOUT")
		}
	case "FISCAL_PENDING", "UNKNOWN":
		actions = append(actions, "READ")
	case "COMPLETED":
		actions = append(actions, "RECEIPT", "REVERSE")
	case "REVERSED":
		actions = append(actions, "RECEIPT")
	}
	return json.Marshal(struct {
		orderAlias
		AllowedActions []string `json:"allowed_actions"`
	}{orderAlias: orderAlias(o), AllowedActions: actions})
}

type ReversalResult struct {
	OperationID     string    `json:"operation_id"`
	State           string    `json:"state"`
	FiscalReference string    `json:"fiscal_reference,omitempty"`
	Simulated       bool      `json:"simulated"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Service owns MiniPOS data and translates checkout intent into the public
// Fiscal API. It never allocates regulatory identifiers or implements device
// commands; those decisions stay inside the fiscalization boundary.
type Service struct {
	mu                sync.RWMutex
	products          map[string]Product
	taxGroups         map[string]TaxGroup
	employees         map[string]Employee
	identityBindings  map[string]IdentityBinding
	operatorSessions  map[string]OperatorSessionRecord
	shifts            map[string]Shift
	orders            map[string]Order
	checkouts         map[string]Order
	checkoutHashes    map[string]string
	apiReplays        map[string]APIReplay
	webhookInbox      map[string]WebhookInboxRecord
	configurations    map[string]Configuration
	fiscal, version   string
	client            *http.Client
	authToken         string
	authProvider      AccessTokenProvider
	store             Store
	sequence          uint64
	generation        int64
	confirmedSnapshot []byte
	fiscalReadiness   func(tenant, register string) bool
}

func (s *Service) SetFiscalReadinessCheck(v func(tenant, register string) bool) {
	s.fiscalReadiness = v
}

func (s *Service) SetFiscalAuthToken(v string) {
	s.authToken = v
	if v != "" {
		s.authProvider = staticTokenProvider(v)
	}
}
func (s *Service) SetFiscalAuthProvider(v AccessTokenProvider) { s.authProvider = v; s.authToken = "" }
func (s *Service) FiscalPing() error {
	return s.call(http.MethodGet, "/connectivity/ping", "", nil, nil)
}

type Store interface {
	Load() ([]byte, error)
	Save([]byte) error
}
type VersionedStore interface {
	Store
	LoadVersioned() ([]byte, int64, error)
	SaveVersioned([]byte, int64) (int64, error)
}
type DeltaVersionedStore interface {
	VersionedStore
	SaveDeltaVersioned(previous, current []byte, expected int64) (int64, error)
}
type TenantEntityReader interface {
	LoadTenantEntity(collection, tenant, id string) ([]byte, error)
	LoadTenantEntities(collection, tenant string) ([][]byte, error)
}
type snapshot struct {
	Products         map[string]Product               `json:"products"`
	TaxGroups        map[string]TaxGroup              `json:"tax_groups"`
	Employees        map[string]Employee              `json:"employees"`
	IdentityBindings map[string]IdentityBinding       `json:"identity_bindings"`
	OperatorSessions map[string]OperatorSessionRecord `json:"operator_sessions"`
	Shifts           map[string]Shift                 `json:"shifts"`
	Orders           map[string]Order                 `json:"orders"`
	Checkouts        map[string]Order                 `json:"checkouts"`
	CheckoutHashes   map[string]string                `json:"checkout_hashes"`
	APIReplays       map[string]APIReplay             `json:"api_replays"`
	WebhookInbox     map[string]WebhookInboxRecord    `json:"webhook_inbox"`
	Configurations   map[string]Configuration         `json:"configurations"`
	Sequence         uint64                           `json:"sequence"`
}

func NewService(f, v string) *Service {
	return &Service{products: map[string]Product{}, taxGroups: map[string]TaxGroup{}, employees: map[string]Employee{}, identityBindings: map[string]IdentityBinding{}, operatorSessions: map[string]OperatorSessionRecord{}, shifts: map[string]Shift{}, orders: map[string]Order{}, checkouts: map[string]Order{}, checkoutHashes: map[string]string{}, apiReplays: map[string]APIReplay{}, webhookInbox: map[string]WebhookInboxRecord{}, configurations: map[string]Configuration{}, fiscal: f, version: v, client: &http.Client{Timeout: 10 * time.Second}}
}
func NewPersistentService(f, v string, store Store) (*Service, error) {
	s := NewService(f, v)
	s.store = store
	var b []byte
	var e error
	if versioned, ok := store.(VersionedStore); ok {
		b, s.generation, e = versioned.LoadVersioned()
	} else {
		b, e = store.Load()
	}
	if e != nil {
		return nil, e
	}
	s.confirmedSnapshot = append([]byte(nil), b...)
	if len(b) > 0 {
		var x snapshot
		if e = json.Unmarshal(b, &x); e != nil {
			return nil, e
		}
		s.products = x.Products
		s.taxGroups = x.TaxGroups
		if s.taxGroups == nil {
			s.taxGroups = map[string]TaxGroup{}
		}
		s.employees = x.Employees
		s.identityBindings = x.IdentityBindings
		if s.identityBindings == nil {
			s.identityBindings = map[string]IdentityBinding{}
		}
		s.operatorSessions = x.OperatorSessions
		if s.operatorSessions == nil {
			s.operatorSessions = map[string]OperatorSessionRecord{}
		}
		s.shifts = x.Shifts
		s.orders = x.Orders
		s.checkouts = x.Checkouts
		s.checkoutHashes = x.CheckoutHashes
		if s.checkoutHashes == nil {
			s.checkoutHashes = map[string]string{}
		}
		s.apiReplays = x.APIReplays
		if s.apiReplays == nil {
			s.apiReplays = map[string]APIReplay{}
		}
		s.webhookInbox = x.WebhookInbox
		if s.webhookInbox == nil {
			s.webhookInbox = map[string]WebhookInboxRecord{}
		}
		s.configurations = x.Configurations
		if s.configurations == nil {
			s.configurations = map[string]Configuration{}
		}
		s.sequence = x.Sequence
	}
	if e = s.recoverOrphanAPIReplays(); e != nil {
		return nil, e
	}
	return s, nil
}

func (s *Service) recoverOrphanAPIReplays() error {
	changed := false
	for key, replay := range s.apiReplays {
		if !replay.Pending || replay.Status != 102 || len(replay.Body) != 0 {
			continue
		}
		replay.Status = 503
		replay.ContentType = "application/problem+json"
		replay.Body = []byte(`{"type":"urn:beeminipos:error:idempotency_outcome_unknown","title":"IDEMPOTENCY_OUTCOME_UNKNOWN","status":503,"code":"IDEMPOTENCY_OUTCOME_UNKNOWN","retryable":false,"detail":"request outcome is unknown after restart"}`)
		s.apiReplays[key] = replay
		changed = true
	}
	if !changed {
		return nil
	}
	return s.persistLocked()
}
func (s *Service) nextID(_ string) string {
	s.sequence++
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		return fmt.Sprintf("00000000-0000-4000-8000-%012d", s.sequence)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
func (s *Service) persistLocked() error {
	if s.store == nil {
		return nil
	}
	b, e := json.Marshal(snapshot{Products: s.products, TaxGroups: s.taxGroups, Employees: s.employees, IdentityBindings: s.identityBindings, OperatorSessions: s.operatorSessions, Shifts: s.shifts, Orders: s.orders, Checkouts: s.checkouts, CheckoutHashes: s.checkoutHashes, APIReplays: s.apiReplays, WebhookInbox: s.webhookInbox, Configurations: s.configurations, Sequence: s.sequence})
	if e != nil {
		return e
	}
	if delta, ok := s.store.(DeltaVersionedStore); ok {
		generation, err := delta.SaveDeltaVersioned(s.confirmedSnapshot, b, s.generation)
		if err == nil {
			s.generation = generation
			s.confirmedSnapshot = append(s.confirmedSnapshot[:0], b...)
			return nil
		}
		s.restoreLocked()
		return err
	}
	if versioned, ok := s.store.(VersionedStore); ok {
		generation, err := versioned.SaveVersioned(b, s.generation)
		if err == nil {
			s.generation = generation
			s.confirmedSnapshot = append(s.confirmedSnapshot[:0], b...)
			return nil
		}
		s.restoreLocked()
		return err
	}
	if err := s.store.Save(b); err != nil {
		s.restoreLocked()
		return err
	}
	return nil
}
func (s *Service) restoreLocked() {
	var b []byte
	var err error
	if versioned, ok := s.store.(VersionedStore); ok {
		b, s.generation, err = versioned.LoadVersioned()
	} else {
		b, err = s.store.Load()
	}
	if err != nil || len(b) == 0 {
		return
	}
	s.confirmedSnapshot = append(s.confirmedSnapshot[:0], b...)
	var x snapshot
	if json.Unmarshal(b, &x) != nil {
		return
	}
	s.products, s.taxGroups, s.employees, s.identityBindings, s.operatorSessions, s.shifts, s.orders = x.Products, x.TaxGroups, x.Employees, x.IdentityBindings, x.OperatorSessions, x.Shifts, x.Orders
	if s.identityBindings == nil {
		s.identityBindings = map[string]IdentityBinding{}
	}
	if s.operatorSessions == nil {
		s.operatorSessions = map[string]OperatorSessionRecord{}
	}
	s.checkouts, s.checkoutHashes, s.apiReplays, s.webhookInbox, s.configurations = x.Checkouts, x.CheckoutHashes, x.APIReplays, x.WebhookInbox, x.Configurations
	s.sequence = x.Sequence
	if s.products == nil {
		s.products = map[string]Product{}
	}
	if s.taxGroups == nil {
		s.taxGroups = map[string]TaxGroup{}
	}
	if s.employees == nil {
		s.employees = map[string]Employee{}
	}
	if s.shifts == nil {
		s.shifts = map[string]Shift{}
	}
	if s.orders == nil {
		s.orders = map[string]Order{}
	}
	if s.checkouts == nil {
		s.checkouts = map[string]Order{}
	}
	if s.checkoutHashes == nil {
		s.checkoutHashes = map[string]string{}
	}
	if s.apiReplays == nil {
		s.apiReplays = map[string]APIReplay{}
	}
	if s.webhookInbox == nil {
		s.webhookInbox = map[string]WebhookInboxRecord{}
	}
	if s.configurations == nil {
		s.configurations = map[string]Configuration{}
	}
}

type APIReplay struct {
	Hash        string              `json:"hash"`
	Status      int                 `json:"status"`
	Body        []byte              `json:"body"`
	ContentType string              `json:"content_type"`
	Headers     map[string][]string `json:"headers,omitempty"`
	Pending     bool                `json:"pending,omitempty"`
}

var ErrAPIReplayMismatch = errors.New("idempotency mismatch")

// ClaimAPIReplay durably reserves a public request before any business side
// effect. A failed cross-replica generation write reloads the winning claim and
// returns it instead of allowing the losing request to execute.
func (s *Service) ClaimAPIReplay(key, hash string) (APIReplay, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if versioned, ok := s.store.(VersionedStore); ok {
		_, current, err := versioned.LoadVersioned()
		if err != nil {
			return APIReplay{}, false, err
		}
		if current != s.generation {
			s.restoreLocked()
			if s.generation != current {
				return APIReplay{}, false, errors.New("idempotency snapshot refresh failed")
			}
		}
	}
	if old, ok := s.apiReplays[key]; ok {
		if old.Hash != hash {
			return old, false, ErrAPIReplayMismatch
		}
		return old, false, nil
	}
	pending := APIReplay{Hash: hash, Status: 102, Pending: true}
	s.apiReplays[key] = pending
	if err := s.persistLocked(); err != nil {
		// persistLocked restores the authoritative winning snapshot on a
		// version conflict. Surface that claim to the loser when available.
		if old, ok := s.apiReplays[key]; ok {
			if old.Hash != hash {
				return old, false, ErrAPIReplayMismatch
			}
			return old, false, nil
		}
		return APIReplay{}, false, err
	}
	return pending, true, nil
}

type WebhookInboxRecord struct {
	EventID     string     `json:"event_id"`
	TenantID    string     `json:"tenant_id"`
	Hash        string     `json:"hash"`
	Raw         []byte     `json:"raw"`
	ReceivedAt  time.Time  `json:"received_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

func (s *Service) APIReplay(key string) (APIReplay, bool) {
	if reader, ok := s.store.(TenantEntityReader); ok {
		parts := strings.Split(key, "\n")
		if len(parts) == 4 && parts[0] != "" {
			raw, err := reader.LoadTenantEntity("api_replays", parts[0], key)
			if err != nil {
				return APIReplay{}, false
			}
			var v APIReplay
			if json.Unmarshal(raw, &v) != nil {
				return APIReplay{}, false
			}
			return v, true
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.apiReplays[key]
	return v, ok
}
func (s *Service) PutAPIReplay(key string, v APIReplay) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.apiReplays[key]; ok && old.Hash != v.Hash {
		return ErrAPIReplayMismatch
	}
	v.Pending = false
	old, existed := s.apiReplays[key]
	s.apiReplays[key] = v
	if err := s.persistLocked(); err != nil {
		// Completion is side-effect free and may be retried once after a
		// generation conflict merged an unrelated replica mutation.
		if _, versioned := s.store.(VersionedStore); versioned {
			if current, ok := s.apiReplays[key]; ok && current.Hash == v.Hash {
				if !current.Pending {
					return nil
				}
				s.apiReplays[key] = v
				if retryErr := s.persistLocked(); retryErr == nil {
					return nil
				}
			}
			return err // persistLocked already restored the latest durable snapshot
		}
		if existed {
			s.apiReplays[key] = old
		} else {
			delete(s.apiReplays, key)
		}
		return err
	}
	return nil
}

func (s *Service) PutAPIAmbiguousReplay(key string, v APIReplay) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.apiReplays[key]
	if !ok || old.Hash != v.Hash || !old.Pending {
		return errors.New("idempotency claim unavailable")
	}
	v.Pending = true
	s.apiReplays[key] = v
	return s.persistLocked()
}

func (s *Service) ConfigurationFor(tenant string) (Configuration, error) {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		rows, err := reader.LoadTenantEntities("configurations", tenant)
		if err != nil || len(rows) != 1 {
			return Configuration{}, errors.New("configuration not found")
		}
		var v Configuration
		if err = json.Unmarshal(rows[0], &v); err != nil {
			return Configuration{}, err
		}
		return v, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.configurations[tenant]
	if !ok {
		return Configuration{}, errors.New("configuration not found")
	}
	return v, nil
}

func (s *Service) SaveConfiguration(tenant string, expected int64, v Configuration) (Configuration, error) {
	adapterURL, adapterURLErr := url.Parse(v.AdapterBaseURL)
	if strings.TrimSpace(v.LocationName) == "" || strings.TrimSpace(v.WorkstationName) == "" || !validUUID(strings.TrimSpace(v.LocationID)) || !validUUID(strings.TrimSpace(v.FiscalRegisterID)) || !validUUID(strings.TrimSpace(v.FiscalAdapterID)) || v.BindingGeneration < 1 || adapterURLErr != nil || adapterURL.Scheme != "http" || adapterURL.Host == "" || adapterURL.User != nil || strings.TrimSuffix(adapterURL.Path, "/") != "/beeloy/local/v1" || adapterURL.RawQuery != "" || adapterURL.Fragment != "" || len(v.LocationName) > 120 || len(v.LocationAddress) > 240 || len(v.WorkstationName) > 120 {
		return Configuration{}, errors.New("invalid configuration")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	current, exists := s.configurations[tenant]
	if exists && (expected < 1 || current.Version != expected) {
		return Configuration{}, errors.New("configuration version conflict")
	}
	if !exists && expected != 0 {
		return Configuration{}, errors.New("configuration version conflict")
	}
	if exists {
		v.ID, v.CreatedAt, v.Version = current.ID, current.CreatedAt, current.Version+1
	} else {
		v.ID, v.CreatedAt, v.Version = s.nextID("configuration"), now, 1
	}
	v.TenantID, v.UpdatedAt = tenant, now
	s.configurations[tenant] = v
	if err := s.persistLocked(); err != nil {
		if exists {
			s.configurations[tenant] = current
		} else {
			delete(s.configurations, tenant)
		}
		return Configuration{}, err
	}
	return v, nil
}
func (s *Service) Products() []Product {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := make([]Product, 0, len(s.products))
	for _, x := range s.products {
		v = append(v, x)
	}
	return v
}
func (s *Service) ProductsFor(tenant string) []Product {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		rows, e := reader.LoadTenantEntities("products", tenant)
		if e == nil {
			out := make([]Product, 0, len(rows))
			for _, raw := range rows {
				var v Product
				if json.Unmarshal(raw, &v) != nil {
					return []Product{}
				}
				out = append(out, v)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
			return out
		}
		return []Product{}
	}
	all := s.Products()
	if tenant == "" {
		sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
		return all
	}
	v := make([]Product, 0)
	for _, x := range all {
		if x.TenantID == tenant {
			v = append(v, x)
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
	return v
}
func (s *Service) TaxGroupsFor(tenant string) []TaxGroup {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		rows, err := reader.LoadTenantEntities("tax_groups", tenant)
		if err != nil {
			return []TaxGroup{}
		}
		out := make([]TaxGroup, 0, len(rows))
		for _, raw := range rows {
			var group TaxGroup
			if json.Unmarshal(raw, &group) != nil {
				return []TaxGroup{}
			}
			out = append(out, group)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
		return out
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TaxGroup, 0, len(s.taxGroups))
	for _, group := range s.taxGroups {
		if tenant == "" || group.TenantID == tenant {
			out = append(out, group)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

func normalizeTaxRate(value string) (string, error) {
	rate, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value), ",", "."), 64)
	if err != nil || math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 100 {
		return "", errors.New("tax rate must be between 0 and 100")
	}
	return strconv.FormatFloat(rate, 'f', 2, 64), nil
}

func (s *Service) CreateTaxGroup(group TaxGroup) (TaxGroup, error) {
	group.Code = strings.ToUpper(strings.TrimSpace(group.Code))
	group.Name = strings.TrimSpace(group.Name)
	if !validTax(group.Code) || group.Name == "" || (group.Status != "" && group.Status != "ACTIVE" && group.Status != "INACTIVE") {
		return group, errors.New("invalid tax group")
	}
	rate, err := normalizeTaxRate(group.Rate)
	if err != nil {
		return group, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.taxGroups {
		if existing.TenantID == group.TenantID && existing.Code == group.Code {
			return group, errors.New("duplicate tax group code")
		}
	}
	group.ID = s.nextID("tax-group")
	group.Rate = rate
	if group.Status == "" {
		group.Status = "ACTIVE"
	}
	group.Version = 1
	group.CreatedAt = time.Now().UTC()
	group.UpdatedAt = group.CreatedAt
	s.taxGroups[group.ID] = group
	return group, s.persistLocked()
}

func (s *Service) TaxGroupForTenant(id, tenant string) (TaxGroup, error) {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("tax_groups", tenant, id)
		if err != nil {
			return TaxGroup{}, err
		}
		var group TaxGroup
		err = json.Unmarshal(raw, &group)
		return group, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	group, ok := s.taxGroups[id]
	if !ok || group.TenantID != tenant {
		return TaxGroup{}, errors.New("not found")
	}
	return group, nil
}

func (s *Service) UpdateTaxGroupForTenant(id string, expected int64, group TaxGroup, tenant string) (TaxGroup, error) {
	current, err := s.TaxGroupForTenant(id, tenant)
	if err != nil || expected <= 0 || current.Version != expected {
		return TaxGroup{}, errors.New("version conflict")
	}
	group.Code = strings.ToUpper(strings.TrimSpace(group.Code))
	group.Name = strings.TrimSpace(group.Name)
	if !validTax(group.Code) || group.Name == "" || (group.Status != "" && group.Status != "ACTIVE" && group.Status != "INACTIVE") {
		return group, errors.New("invalid tax group")
	}
	rate, err := normalizeTaxRate(group.Rate)
	if err != nil {
		return group, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.taxGroups {
		if existing.ID != id && existing.TenantID == tenant && existing.Code == group.Code {
			return group, errors.New("duplicate tax group code")
		}
	}
	group.ID = id
	group.TenantID = tenant
	group.Rate = rate
	if group.Status == "" {
		group.Status = current.Status
	}
	group.Version = current.Version + 1
	group.CreatedAt = current.CreatedAt
	group.UpdatedAt = time.Now().UTC()
	s.taxGroups[id] = group
	return group, s.persistLocked()
}

func (s *Service) CreateProduct(v Product) (Product, error) {
	if v.SKU == "" || v.Name == "" || !validMoney(v.Price) || !validTax(v.TaxGroup) || (v.Status != "" && v.Status != "ACTIVE" && v.Status != "INACTIVE") {
		return v, errors.New("invalid product")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.products {
		if x.TenantID == v.TenantID && strings.EqualFold(x.SKU, v.SKU) {
			return v, errors.New("duplicate sku")
		}
		if v.Barcode != nil && strings.TrimSpace(*v.Barcode) != "" && x.TenantID == v.TenantID && x.Barcode != nil && *x.Barcode == strings.TrimSpace(*v.Barcode) {
			return v, errors.New("duplicate barcode")
		}
	}
	if v.Barcode != nil {
		barcode := strings.TrimSpace(*v.Barcode)
		if barcode == "" {
			v.Barcode = nil
		} else {
			v.Barcode = &barcode
		}
	}
	v.ID = s.nextID("product")
	if v.Unit == "" {
		v.Unit = "pcs"
	}
	if v.Status == "" {
		v.Status = "ACTIVE"
	}
	v.Active = v.Status == "ACTIVE"
	v.Version = 1
	v.CreatedAt = time.Now().UTC()
	v.UpdatedAt = v.CreatedAt
	s.products[v.ID] = v
	return v, s.persistLocked()
}
func (s *Service) Product(id string) (Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.products[id]
	if !ok {
		return v, errors.New("not found")
	}
	return v, nil
}
func (s *Service) ProductForTenant(id, tenant string) (Product, error) {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		raw, e := reader.LoadTenantEntity("products", tenant, id)
		if e != nil {
			return Product{}, e
		}
		var v Product
		e = json.Unmarshal(raw, &v)
		return v, e
	}
	v, e := s.Product(id)
	if e != nil || v.TenantID != tenant {
		return Product{}, errors.New("not found")
	}
	return v, nil
}
func (s *Service) UpdateProduct(id string, expected int64, v Product) (Product, error) {
	if v.SKU == "" || v.Name == "" || !validMoney(v.Price) || !validTax(v.TaxGroup) || (v.Status != "" && v.Status != "ACTIVE" && v.Status != "INACTIVE") {
		return v, errors.New("invalid product")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.products[id]
	if !ok {
		return v, errors.New("not found")
	}
	if expected <= 0 || old.Version != expected {
		return v, errors.New("version conflict")
	}
	if v.TenantID != "" && old.TenantID != v.TenantID {
		return v, errors.New("not found")
	}
	for _, x := range s.products {
		if x.ID != id && x.TenantID == old.TenantID && strings.EqualFold(x.SKU, v.SKU) {
			return v, errors.New("duplicate sku")
		}
		if x.ID != id && v.Barcode != nil && strings.TrimSpace(*v.Barcode) != "" && x.TenantID == old.TenantID && x.Barcode != nil && *x.Barcode == strings.TrimSpace(*v.Barcode) {
			return v, errors.New("duplicate barcode")
		}
	}
	if v.Barcode != nil {
		barcode := strings.TrimSpace(*v.Barcode)
		if barcode == "" {
			v.Barcode = nil
		} else {
			v.Barcode = &barcode
		}
	}
	v.TenantID = old.TenantID
	if v.Unit == "" {
		v.Unit = old.Unit
	}
	if v.Status == "" {
		v.Status = old.Status
	}
	v.Active = v.Status == "ACTIVE"
	v.ID = id
	v.Version = old.Version + 1
	v.CreatedAt = old.CreatedAt
	v.UpdatedAt = time.Now().UTC()
	s.products[id] = v
	return v, s.persistLocked()
}
func (s *Service) UpdateProductForTenant(id string, expected int64, v Product, tenant string) (Product, error) {
	current, err := s.ProductForTenant(id, tenant)
	if err != nil || current.Version != expected {
		return Product{}, errors.New("version conflict")
	}
	v.TenantID = tenant
	return s.UpdateProduct(id, expected, v)
}
func (s *Service) Employees() []Employee {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := make([]Employee, 0, len(s.employees))
	for _, x := range s.employees {
		v = append(v, x)
	}
	return v
}
func (s *Service) EmployeesFor(tenant string) []Employee {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		rows, e := reader.LoadTenantEntities("employees", tenant)
		if e == nil {
			out := make([]Employee, 0, len(rows))
			for _, raw := range rows {
				var v Employee
				if json.Unmarshal(raw, &v) != nil {
					return []Employee{}
				}
				out = append(out, v)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
			return out
		}
		return []Employee{}
	}
	all := s.Employees()
	if tenant == "" {
		sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
		return all
	}
	v := make([]Employee, 0)
	for _, x := range all {
		if x.TenantID == tenant {
			v = append(v, x)
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
	return v
}
func (s *Service) CreateEmployee(v Employee) (Employee, error) {
	if !validEmployee(v) {
		return v, errors.New("invalid employee")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, x := range s.employees {
		if x.TenantID == v.TenantID && strings.EqualFold(x.OperatorCode, v.OperatorCode) {
			return v, errors.New("duplicate operator code")
		}
	}
	v.ID = s.nextID("employee")
	if len(v.Roles) == 0 {
		v.Roles = []string{"CASHIER"}
	}
	if v.Status == "" {
		v.Status = "ACTIVE"
	}
	v.Active = v.Status == "ACTIVE"
	v.Version = 1
	v.CreatedAt = time.Now().UTC()
	v.UpdatedAt = v.CreatedAt
	s.employees[v.ID] = v
	return v, s.persistLocked()
}
func (s *Service) Employee(id string) (Employee, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.employees[id]
	if !ok {
		return v, errors.New("not found")
	}
	return v, nil
}
func (s *Service) EmployeeForTenant(id, tenant string) (Employee, error) {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		raw, e := reader.LoadTenantEntity("employees", tenant, id)
		if e != nil {
			return Employee{}, e
		}
		var v Employee
		e = json.Unmarshal(raw, &v)
		return v, e
	}
	v, e := s.Employee(id)
	if e != nil || v.TenantID != tenant {
		return Employee{}, errors.New("not found")
	}
	return v, nil
}
func (s *Service) UpdateEmployee(id string, expected int64, v Employee) (Employee, error) {
	if !validEmployee(v) {
		return v, errors.New("invalid employee")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok := s.employees[id]
	if !ok {
		return v, errors.New("not found")
	}
	if expected <= 0 || old.Version != expected {
		return v, errors.New("version conflict")
	}
	if v.TenantID != "" && old.TenantID != v.TenantID {
		return v, errors.New("not found")
	}
	for _, x := range s.employees {
		if x.ID != id && x.TenantID == old.TenantID && strings.EqualFold(x.OperatorCode, v.OperatorCode) {
			return v, errors.New("duplicate operator code")
		}
	}
	v.TenantID = old.TenantID
	if len(v.Roles) == 0 {
		v.Roles = old.Roles
	}
	if v.Status == "" {
		v.Status = old.Status
	}
	v.Active = v.Status == "ACTIVE"
	v.ID = id
	v.Version = old.Version + 1
	v.CreatedAt = old.CreatedAt
	v.UpdatedAt = time.Now().UTC()
	s.employees[id] = v
	return v, s.persistLocked()
}
func (s *Service) UpdateEmployeeForTenant(id string, expected int64, v Employee, tenant string) (Employee, error) {
	current, err := s.EmployeeForTenant(id, tenant)
	if err != nil || current.Version != expected {
		return Employee{}, errors.New("version conflict")
	}
	v.TenantID = tenant
	return s.UpdateEmployee(id, expected, v)
}
func identityBindingKey(tenant, issuer, subject string) string {
	sum := sha256.Sum256([]byte(tenant + "\x00" + canonicalIdentityIssuer(issuer) + "\x00" + subject))
	return tenant + "\n" + fmt.Sprintf("%x", sum[:])
}

func canonicalIdentityIssuer(issuer string) string {
	return strings.TrimSuffix(strings.TrimSpace(issuer), "/")
}

func (s *Service) BindEmployeeIdentity(tenant, employeeID, subject, issuer string) (IdentityBinding, error) {
	issuer = canonicalIdentityIssuer(issuer)
	if tenant == "" || employeeID == "" || strings.TrimSpace(subject) == "" || issuer == "" {
		return IdentityBinding{}, errors.New("invalid identity binding")
	}
	key := identityBindingKey(tenant, issuer, subject)
	s.mu.Lock()
	defer s.mu.Unlock()
	employee, exists := s.employees[employeeID]
	if !exists || employee.TenantID != tenant || !employee.Active || employee.Status != "ACTIVE" {
		return IdentityBinding{}, errors.New("active employee not found")
	}
	if current, ok := s.identityBindings[key]; ok {
		if current.EmployeeID != employeeID || current.IdentityIssuer != issuer {
			return IdentityBinding{}, errors.New("identity already bound")
		}
		return current, nil
	}
	for _, current := range s.identityBindings {
		if current.TenantID == tenant && current.EmployeeID == employeeID {
			return IdentityBinding{}, errors.New("employee already bound")
		}
	}
	now := time.Now().UTC()
	binding := IdentityBinding{TenantID: tenant, EmployeeID: employeeID, SubjectHash: strings.TrimPrefix(key, tenant+"\n"), IdentityIssuer: issuer, BoundAt: now}
	s.identityBindings[key] = binding
	return binding, s.persistLocked()
}
func (s *Service) EmployeeForIdentity(tenant, issuer, subject string) (Employee, error) {
	if tenant == "" || subject == "" {
		return Employee{}, errors.New("identity not bound")
	}
	s.mu.RLock()
	binding, ok := s.identityBindings[identityBindingKey(tenant, issuer, subject)]
	s.mu.RUnlock()
	if !ok {
		return Employee{}, errors.New("identity not bound")
	}
	employee, err := s.EmployeeForTenant(binding.EmployeeID, tenant)
	if err != nil || !employee.Active || employee.Status != "ACTIVE" {
		return Employee{}, errors.New("employee inactive")
	}
	return employee, nil
}
func (s *Service) IdentityMatchesEmployee(tenant, issuer, subject, employeeID string) bool {
	employee, err := s.EmployeeForIdentity(tenant, issuer, subject)
	return err == nil && employee.ID == employeeID
}
func (s *Service) RegisterOperatorSession(tenant, employeeID, appInstanceID, tokenHash string, expiresAt time.Time) (OperatorSessionRecord, error) {
	if tenant == "" || employeeID == "" || appInstanceID == "" || tokenHash == "" || !expiresAt.After(time.Now().UTC()) {
		return OperatorSessionRecord{}, errors.New("invalid operator session")
	}
	key := tenant + "\n" + tokenHash
	s.mu.Lock()
	defer s.mu.Unlock()
	employee, exists := s.employees[employeeID]
	if !exists || employee.TenantID != tenant || !employee.Active || employee.Status != "ACTIVE" {
		return OperatorSessionRecord{}, errors.New("active employee not found")
	}
	if current, ok := s.operatorSessions[key]; ok {
		if current.EmployeeID != employeeID || current.AppInstanceID != appInstanceID || current.State != "ACTIVE" {
			return OperatorSessionRecord{}, errors.New("operator session revoked or mismatched")
		}
		return current, nil
	}
	now := time.Now().UTC()
	record := OperatorSessionRecord{TenantID: tenant, EmployeeID: employeeID, AppInstanceID: appInstanceID, TokenHash: tokenHash, State: "ACTIVE", FirstSeen: now, ExpiresAt: expiresAt}
	s.operatorSessions[key] = record
	return record, s.persistLocked()
}
func (s *Service) RevokeOperatorSession(tenant, tokenHash string) (OperatorSessionRecord, error) {
	key := tenant + "\n" + tokenHash
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.operatorSessions[key]
	if !ok || record.TenantID != tenant {
		return OperatorSessionRecord{}, errors.New("operator session not found")
	}
	if record.State == "REVOKED" {
		return record, nil
	}
	now := time.Now().UTC()
	record.State, record.RevokedAt = "REVOKED", &now
	s.operatorSessions[key] = record
	return record, s.persistLocked()
}
func (s *Service) OperatorSessionActive(tenant, tokenHash, employeeID, appInstanceID string, now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.operatorSessions[tenant+"\n"+tokenHash]
	return ok && record.TenantID == tenant && record.EmployeeID == employeeID && record.AppInstanceID == appInstanceID && record.State == "ACTIVE" && record.ExpiresAt.After(now)
}
func (s *Service) OperatorSessionRevoked(tenant, tokenHash string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.operatorSessions[tenant+"\n"+tokenHash]
	return ok && record.State == "REVOKED"
}
func (s *Service) OpenShift(register, employee string) (Shift, error) {
	return s.OpenShiftForTenant(register, employee, "")
}
func (s *Service) ShiftsFor(tenant, employee, register, state string) []Shift {
	var all []Shift
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		rows, err := reader.LoadTenantEntities("shifts", tenant)
		if err != nil {
			return []Shift{}
		}
		all = make([]Shift, 0, len(rows))
		for _, raw := range rows {
			var shift Shift
			if json.Unmarshal(raw, &shift) != nil {
				return []Shift{}
			}
			all = append(all, shift)
		}
	} else {
		s.mu.RLock()
		all = make([]Shift, 0, len(s.shifts))
		for _, shift := range s.shifts {
			all = append(all, shift)
		}
		s.mu.RUnlock()
	}
	out := make([]Shift, 0, len(all))
	for _, shift := range all {
		if tenant != "" && shift.TenantID != tenant || employee != "" && shift.EmployeeID != employee || register != "" && shift.RegisterID != register || state != "" && shift.State != state {
			continue
		}
		out = append(out, withShiftActions(shift))
	}
	return out
}
func (s *Service) OpenShiftForTenant(register, employee, tenant string) (Shift, error) {
	if !validUUID(strings.TrimSpace(register)) {
		return Shift{}, errors.New("invalid fiscal register")
	}
	if tenant != "" {
		if s.fiscalReadiness != nil && !s.fiscalReadiness(tenant, register) {
			return Shift{}, errors.New("Fiscal binding and required resources are not ready")
		}
		if _, err := s.EmployeeForTenant(employee, tenant); err != nil {
			return Shift{}, errors.New("employee not found")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	emp, ok := s.employees[employee]
	if !ok || (tenant != "" && emp.TenantID != tenant) {
		return Shift{}, errors.New("employee not found")
	}
	for _, v := range s.shifts {
		if v.RegisterID == register && v.State != "CLOSED" {
			return Shift{}, errors.New("shift already open")
		}
	}
	now := time.Now().UTC()
	v := withShiftActions(Shift{ID: s.nextID("shift"), TenantID: tenant, RegisterID: register, EmployeeID: employee, State: "OPEN", OpenedAt: now, Version: 1, CreatedAt: now, UpdatedAt: now})
	s.shifts[v.ID] = v
	return v, s.persistLocked()
}
func (s *Service) CloseShift(id string) (Shift, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.shifts[id]
	if !ok {
		return v, errors.New("not found")
	}
	if v.State != "OPEN" {
		return v, errors.New("shift not open")
	}
	for _, o := range s.orders {
		if o.ShiftID == id && (o.State == "DRAFT" || o.State == "OPEN" || o.State == "FISCAL_PENDING" || o.State == "UNKNOWN") {
			return v, errors.New("unresolved order blocks close")
		}
	}
	now := time.Now().UTC()
	v.State = "CLOSED"
	v = withShiftActions(v)
	v.ClosedAt = &now
	v.Version++
	v.UpdatedAt = now
	s.shifts[id] = v
	return v, s.persistLocked()
}
func (s *Service) CloseShiftForTenant(id, tenant string) (Shift, error) {
	if tenant == "" {
		return s.CloseShift(id)
	}
	current, err := s.ShiftForTenant(id, tenant)
	if err != nil || current.State != "OPEN" {
		return Shift{}, errors.New("shift not open")
	}
	s.mu.Lock()
	current, ok := s.shifts[id]
	if !ok || current.TenantID != tenant || current.State != "OPEN" {
		s.mu.Unlock()
		return Shift{}, errors.New("shift not open")
	}
	for _, order := range s.orders {
		if order.ShiftID == id && (order.State == "DRAFT" || order.State == "OPEN" || order.State == "FISCAL_PENDING" || order.State == "UNKNOWN") {
			s.mu.Unlock()
			return Shift{}, errors.New("unresolved order blocks close")
		}
	}
	previous := current
	current.State = "CLOSING"
	current = withShiftActions(current)
	current.Version++
	current.UpdatedAt = time.Now().UTC()
	s.shifts[id] = current
	if err = s.persistLocked(); err != nil {
		s.shifts[id] = previous
		s.mu.Unlock()
		return Shift{}, err
	}
	s.mu.Unlock()
	var operation struct {
		ID              string `json:"operation_id"`
		State           string `json:"state"`
		FiscalReference string `json:"fiscal_reference"`
	}
	err = s.call("POST", "/registers/"+current.RegisterID+"/reports", "minipos-shift-"+id+"-z", map[string]any{"type": "Z"}, &operation)
	if err != nil || operation.State != "FISCALIZED" || operation.ID == "" || operation.FiscalReference == "" {
		detail := "Z report result is not final"
		if err != nil {
			detail = err.Error()
		}
		return s.finishShiftClose(id, "BLOCKED_RECONCILIATION", operation.ID, operation.FiscalReference, detail)
	}
	return s.finishShiftClose(id, "CLOSED", operation.ID, operation.FiscalReference, "")
}

func (s *Service) ReconcileShiftCloseForTenant(id, tenant string) (Shift, error) {
	current, err := s.ShiftForTenant(id, tenant)
	if err != nil || current.State != "BLOCKED_RECONCILIATION" {
		return Shift{}, errors.New("shift does not require reconciliation")
	}
	var operation struct {
		ID              string `json:"operation_id"`
		State           string `json:"state"`
		FiscalReference string `json:"fiscal_reference"`
	}
	if current.ZOperationID == "" {
		err = s.call("POST", "/registers/"+current.RegisterID+"/reports", "minipos-shift-"+id+"-z", map[string]any{"type": "Z"}, &operation)
	} else {
		err = s.call("GET", "/operations/"+current.ZOperationID, "", nil, &operation)
	}
	if err != nil {
		return s.finishShiftClose(id, "BLOCKED_RECONCILIATION", current.ZOperationID, current.ZFiscalReference, err.Error())
	}
	if operation.ID == "" {
		operation.ID = current.ZOperationID
	}
	if operation.State != "FISCALIZED" || operation.ID == "" || operation.FiscalReference == "" {
		detail := "Z report remains non-final: " + operation.State
		return s.finishShiftClose(id, "BLOCKED_RECONCILIATION", operation.ID, operation.FiscalReference, detail)
	}
	return s.finishShiftClose(id, "CLOSED", operation.ID, operation.FiscalReference, "")
}

func (s *Service) finishShiftClose(id, state, operationID, reference, closeError string) (Shift, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	shift, ok := s.shifts[id]
	if !ok || (shift.State != "CLOSING" && shift.State != "BLOCKED_RECONCILIATION") {
		return Shift{}, errors.New("shift close state changed")
	}
	previous := shift
	now := time.Now().UTC()
	shift.State, shift.ZOperationID, shift.ZFiscalReference, shift.CloseError = state, operationID, reference, closeError
	shift = withShiftActions(shift)
	shift.Version++
	shift.UpdatedAt = now
	if state == "CLOSED" {
		shift.ClosedAt = &now
	}
	s.shifts[id] = shift
	if err := s.persistLocked(); err != nil {
		s.shifts[id] = previous
		return Shift{}, err
	}
	if state != "CLOSED" {
		return shift, errors.New("Z report requires reconciliation")
	}
	return shift, nil
}
func (s *Service) Shift(id string) (Shift, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.shifts[id]
	if !ok {
		return v, errors.New("not found")
	}
	return withShiftActions(v), nil
}
func (s *Service) ShiftForTenant(id, tenant string) (Shift, error) {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		raw, e := reader.LoadTenantEntity("shifts", tenant, id)
		if e != nil {
			return Shift{}, e
		}
		var v Shift
		e = json.Unmarshal(raw, &v)
		return withShiftActions(v), e
	}
	v, e := s.Shift(id)
	if e != nil || v.TenantID != tenant {
		return Shift{}, errors.New("not found")
	}
	return withShiftActions(v), nil
}
func (s *Service) CreateOrder(v Order) (Order, error) {
	if v.TenantID != "" {
		if _, err := s.ShiftForTenant(v.ShiftID, v.TenantID); err != nil {
			return v, errors.New("shift not open")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sh, ok := s.shifts[v.ShiftID]
	if !ok || sh.State != "OPEN" || (v.TenantID != "" && sh.TenantID != v.TenantID) {
		return v, errors.New("shift not open")
	}
	emp := s.employees[sh.EmployeeID]
	v.ID = s.nextID("order")
	v.TenantID = sh.TenantID
	v.ExternalID = v.ID
	v.RegisterID = sh.RegisterID
	v.OperatorCode = emp.OperatorCode
	v.State = "DRAFT"
	v.Lines = []Line{}
	v.Total = Money{Amount: "0.00", Currency: "EUR"}
	v.Version = 1
	v.CreatedAt = time.Now().UTC()
	v.UpdatedAt = v.CreatedAt
	s.orders[v.ID] = v
	return v, s.persistLocked()
}

// ImportOfflineOrder materializes a POS-owned sale which was durably executed
// by a local fiscal adapter while the MiniPOS backend was unavailable. The
// external sale UUID is the immutable idempotency key and the authenticated
// shift remains the authority for tenant, register and operator identity.
func (s *Service) ImportOfflineOrder(tenant string, v OfflineOrderImport) (Order, error) {
	if !validUUID(v.ExternalID) || !validUUID(v.ClientOperationID) || !validUUID(v.ReceiptSessionID) || len(v.Lines) == 0 || len(v.Lines) > 200 {
		return Order{}, errors.New("invalid offline order identity")
	}
	if v.FiscalState != "FISCALIZED" && v.FiscalState != "UNKNOWN" && v.FiscalState != "FAILED" {
		return Order{}, errors.New("invalid offline fiscal state")
	}
	shift, err := s.ShiftForTenant(v.ShiftID, tenant)
	if err != nil || shift.State != "OPEN" {
		return Order{}, errors.New("shift not open")
	}
	total, err := orderTotal(v.Lines)
	if err != nil {
		return Order{}, errors.New("invalid offline order total")
	}
	paymentMaps := make([]map[string]any, 0, len(v.Payments))
	for _, payment := range v.Payments {
		policy := payment.TerminalPolicy
		if policy == "" && payment.Type == "CARD" {
			policy = "REQUIRED"
		}
		paymentMaps = append(paymentMaps, map[string]any{"payment_id": payment.PaymentID, "type": payment.Type, "amount": map[string]any{"amount": payment.Amount.Amount, "currency": payment.Amount.Currency}, "terminal_policy": policy})
	}
	if err = validateCheckoutPayments(total, paymentMaps); err != nil {
		return Order{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.orders {
		if existing.TenantID != tenant || existing.ExternalID != v.ExternalID {
			continue
		}
		if existing.CheckoutOperationID != v.ClientOperationID || existing.ReceiptSessionID != v.ReceiptSessionID || existing.Total != total {
			return Order{}, errors.New("offline order payload mismatch")
		}
		return existing, nil
	}
	for _, line := range v.Lines {
		if !validUUID(line.LineID) || line.Name == "" || !validMoney(line.UnitPrice) || !validDiscount(line.Discount) || !validTax(line.TaxGroup) || !validQuantity(line.Quantity) {
			return Order{}, errors.New("invalid offline line")
		}
		if line.ProductID != "" {
			product, ok := s.products[line.ProductID]
			if !ok || product.TenantID != tenant {
				return Order{}, errors.New("product not found")
			}
		}
	}
	employee := s.employees[shift.EmployeeID]
	now := time.Now().UTC()
	state := map[string]string{"FISCALIZED": "COMPLETED", "UNKNOWN": "UNKNOWN", "FAILED": "FAILED"}[v.FiscalState]
	order := Order{ID: s.nextID("order"), TenantID: tenant, ExternalID: v.ExternalID,
		ShiftID: v.ShiftID, RegisterID: shift.RegisterID, OperatorCode: employee.OperatorCode,
		State: state, Lines: append([]Line(nil), v.Lines...), Total: total,
		Payments: append([]OrderPayment(nil), v.Payments...), FiscalOperationID: v.ClientOperationID,
		CheckoutOperationID: v.ClientOperationID, ReceiptSessionID: v.ReceiptSessionID,
		ReceiptReference: v.FiscalReference, Version: 1, CreatedAt: now, UpdatedAt: now}
	s.orders[order.ID] = order
	return order, s.persistLocked()
}
func (s *Service) Orders() []Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v := make([]Order, 0, len(s.orders))
	for _, x := range s.orders {
		v = append(v, x)
	}
	return v
}
func (s *Service) OrdersFor(tenant string) []Order {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		rows, e := reader.LoadTenantEntities("orders", tenant)
		if e == nil {
			out := make([]Order, 0, len(rows))
			for _, raw := range rows {
				var v Order
				if json.Unmarshal(raw, &v) != nil {
					return []Order{}
				}
				out = append(out, v)
			}
			sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
			return out
		}
		return []Order{}
	}
	all := s.Orders()
	if tenant == "" {
		sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
		return all
	}
	v := make([]Order, 0)
	for _, x := range all {
		if x.TenantID == tenant {
			v = append(v, x)
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
	return v
}
func (s *Service) OrdersForEmployee(tenant, employee string) []Order {
	all := s.OrdersFor(tenant)
	out := make([]Order, 0, len(all))
	for _, order := range all {
		shift, err := s.ShiftForTenant(order.ShiftID, tenant)
		if err == nil && shift.EmployeeID == employee {
			out = append(out, order)
		}
	}
	return out
}
func (s *Service) SalesReport() map[string]any {
	now := time.Now().UTC()
	return s.SalesReportForPeriod("", time.Unix(0, 0).UTC(), now)
}
func (s *Service) SalesReportFor(tenant string) map[string]any {
	now := time.Now().UTC()
	return s.SalesReportForPeriod(tenant, time.Unix(0, 0).UTC(), now)
}
func (s *Service) SalesReportForPeriod(tenant string, from, to time.Time) map[string]any {
	orders := s.OrdersFor(tenant)
	var gross int64
	byPayment := map[string]int64{}
	for _, x := range orders {
		if x.CreatedAt.Before(from) || !x.CreatedAt.Before(to) || (x.State != "COMPLETED" && x.State != "REVERSED") {
			continue
		}
		sign := int64(1)
		if x.State == "REVERSED" {
			sign = -1
		}
		if cents, err := moneyCents(x.Total); err == nil {
			gross += sign * cents
		}
		if len(x.Payments) == 0 {
			if cents, err := moneyCents(x.Total); err == nil {
				byPayment["UNKNOWN"] += sign * cents
			}
			continue
		}
		for _, payment := range x.Payments {
			if cents, err := moneyCents(payment.Amount); err == nil {
				byPayment[payment.Type] += sign * cents
			}
		}
	}
	types := make([]string, 0, len(byPayment))
	for typ := range byPayment {
		types = append(types, typ)
	}
	sort.Strings(types)
	payments := make([]map[string]any, 0, len(types))
	for _, typ := range types {
		payments = append(payments, map[string]any{"type": typ, "amount": Money{Amount: formatCents(byPayment[typ]), Currency: "EUR"}})
	}
	return map[string]any{"from": from.UTC(), "to": to.UTC(), "currency": "EUR", "gross": Money{Amount: formatCents(gross), Currency: "EUR"}, "payments": payments, "generated_at": time.Now().UTC()}
}
func (s *Service) AddLine(order string, v Line) (Order, error) {
	return s.AddLineExpected(order, 0, v)
}
func (s *Service) AddLineExpected(order string, expected int64, v Line) (Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[order]
	if !ok {
		return o, errors.New("not found")
	}
	if o.State != "DRAFT" && o.State != "OPEN" {
		return o, errors.New("not editable")
	}
	if expected > 0 && o.Version != expected {
		return o, errors.New("version conflict")
	}
	if !validUUID(v.LineID) || v.Name == "" || !validMoney(v.UnitPrice) || !validDiscount(v.Discount) || !validTax(v.TaxGroup) || !validQuantity(v.Quantity) {
		return o, errors.New("invalid line")
	}
	if v.ProductID != "" {
		p, exists := s.products[v.ProductID]
		if !exists || p.TenantID != o.TenantID {
			return o, errors.New("product not found")
		}
	}
	nextLines := append(append([]Line(nil), o.Lines...), v)
	total, err := orderTotal(nextLines)
	if err != nil {
		return o, errors.New("order total overflow")
	}
	o.Lines = nextLines
	o.Total = total
	o.State = "OPEN"
	o.Version++
	o.UpdatedAt = time.Now().UTC()
	s.orders[o.ID] = o
	return o, s.persistLocked()
}
func (s *Service) AddLineExpectedForTenant(order string, expected int64, v Line, tenant string) (Order, error) {
	current, err := s.OrderForTenant(order, tenant)
	if err != nil || (expected > 0 && current.Version != expected) {
		return Order{}, errors.New("version conflict")
	}
	return s.AddLineExpected(order, expected, v)
}
func orderTotal(lines []Line) (Money, error) {
	var cents int64
	for _, l := range lines {
		p, _ := strconv.ParseInt(strings.ReplaceAll(l.UnitPrice.Amount, ".", ""), 10, 64)
		q, _ := strconv.ParseInt(strings.ReplaceAll(l.Quantity, ".", ""), 10, 64)
		if p < 0 || q <= 0 || (p > 0 && q > (math.MaxInt64-500)/p) {
			return Money{}, errors.New("invalid line total")
		}
		line := (p*q + 500) / 1000
		if l.Discount != nil {
			d, _ := strconv.ParseInt(strings.ReplaceAll(l.Discount.Amount, ".", ""), 10, 64)
			if d < 0 || d > line {
				return Money{}, errors.New("invalid line discount")
			}
			line -= d
		}
		if line > math.MaxInt64-cents {
			return Money{}, errors.New("order total overflow")
		}
		cents += line
	}
	return Money{Amount: fmt.Sprintf("%d.%02d", cents/100, cents%100), Currency: "EUR"}, nil
}
func validDiscount(v *Money) bool { return v == nil || validMoney(*v) }
func validMoney(v Money) bool {
	if v.Currency != "EUR" {
		return false
	}
	p := strings.Split(v.Amount, ".")
	if len(p) != 2 || p[0] == "" || len(p[1]) != 2 {
		return false
	}
	n, e := strconv.ParseInt(p[0]+p[1], 10, 64)
	return e == nil && n >= 0
}
func validTax(v string) bool { return len(v) == 1 && strings.Contains("ABCDEFGH", v) }
func validQuantity(v string) bool {
	p := strings.Split(v, ".")
	if len(p) != 2 || p[0] == "" || len(p[1]) != 3 {
		return false
	}
	n, e := strconv.ParseInt(p[0]+p[1], 10, 64)
	return e == nil && n > 0
}
func validUUID(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i, c := range v {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
func validEmployee(v Employee) bool {
	if strings.TrimSpace(v.FirstName) == "" || len(v.OperatorCode) != 4 || (v.Status != "" && v.Status != "ACTIVE" && v.Status != "INACTIVE") {
		return false
	}
	for _, c := range v.OperatorCode {
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9') {
			return false
		}
	}
	for _, r := range v.Roles {
		if r != "CASHIER" && r != "SUPERVISOR" && r != "ADMIN" {
			return false
		}
	}
	return true
}
func (s *Service) Order(order string) (Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orders[order]
	if !ok {
		return o, errors.New("not found")
	}
	return o, nil
}
func (s *Service) OrderForTenant(id, tenant string) (Order, error) {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		raw, e := reader.LoadTenantEntity("orders", tenant, id)
		if e != nil {
			return Order{}, e
		}
		var v Order
		e = json.Unmarshal(raw, &v)
		return v, e
	}
	v, e := s.Order(id)
	if e != nil || v.TenantID != tenant {
		return Order{}, errors.New("not found")
	}
	return v, nil
}
func (s *Service) ApplyFiscalEvent(aggregateID, state, operation string, version int64) error {
	return s.ApplyFiscalEventLinked(aggregateID, "", state, operation, version)
}
func (s *Service) ApplyFiscalEventLinked(aggregateID, externalID, state, operation string, version int64) error {
	return s.applyFiscalEventLinkedForTenant("", aggregateID, externalID, state, operation, "", version)
}
func (s *Service) applyFiscalEventLinkedForTenant(tenant, aggregateID, externalID, state, operation, fiscalReference string, version int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, o := range s.orders {
		if tenant != "" && o.TenantID != tenant {
			continue
		}
		if o.FiscalSaleID != aggregateID && (externalID == "" || o.ExternalID != externalID) {
			continue
		}
		if version <= o.FiscalVersion && operation != o.ReversalOperationID && !(state == "REVERSED" && o.State == "UNKNOWN") {
			return nil
		}
		switch state {
		case "FISCALIZED":
			o.State = "COMPLETED"
		case "FAILED":
			o.State = "FAILED"
		case "UNKNOWN":
			o.State = "UNKNOWN"
		case "REVERSED":
			o.State = "REVERSED"
			o.ReversalOperationID = operation
			if fiscalReference != "" {
				o.ReversalFiscalReference = fiscalReference
			}
		default:
			return errors.New("unknown fiscal state")
		}
		if state != "REVERSED" {
			o.FiscalOperationID = operation
			if fiscalReference != "" {
				o.ReceiptReference = fiscalReference
			}
		}
		if o.FiscalSaleID == "" {
			o.FiscalSaleID = aggregateID
		}
		o.FiscalVersion = version
		o.Version = version
		s.orders[id] = o
		return s.persistLocked()
	}
	return errors.New("order link not found")
}
func (s *Service) ProcessFiscalWebhook(eventID string, raw []byte, resourceID, state, operation string, version int64) error {
	return s.ProcessFiscalWebhookLinked(eventID, raw, resourceID, "", state, operation, version)
}
func (s *Service) ProcessFiscalWebhookLinked(eventID string, raw []byte, resourceID, externalID, state, operation string, version int64) error {
	var envelope struct {
		TenantID string `json:"tenant_id"`
	}
	_ = json.Unmarshal(raw, &envelope)
	return s.ProcessFiscalWebhookLinkedForTenant(envelope.TenantID, eventID, raw, resourceID, externalID, state, operation, version)
}
func (s *Service) ProcessFiscalWebhookLinkedForTenant(tenant, eventID string, raw []byte, resourceID, externalID, state, operation string, version int64) error {
	return s.ProcessFiscalWebhookLinkedForTenantWithReference(tenant, eventID, raw, resourceID, externalID, state, operation, "", version)
}
func (s *Service) ProcessFiscalWebhookLinkedForTenantWithReference(tenant, eventID string, raw []byte, resourceID, externalID, state, operation, fiscalReference string, version int64) error {
	if eventID == "" {
		return errors.New("event id required")
	}
	if tenant == "" {
		return errors.New("tenant id required")
	}
	sum := sha256.Sum256(raw)
	hash := fmt.Sprintf("%x", sum)
	s.mu.Lock()
	if old, ok := s.webhookInbox[eventID]; ok {
		if old.Hash != hash || old.TenantID != tenant {
			s.mu.Unlock()
			return errors.New("event id payload mismatch")
		}
		if old.ProcessedAt != nil {
			s.mu.Unlock()
			return nil
		}
	} else {
		s.webhookInbox[eventID] = WebhookInboxRecord{EventID: eventID, TenantID: tenant, Hash: hash, Raw: append([]byte(nil), raw...), ReceivedAt: time.Now().UTC()}
		if e := s.persistLocked(); e != nil {
			s.mu.Unlock()
			return e
		}
	}
	s.mu.Unlock()
	err := s.applyFiscalEventLinkedForTenant(tenant, resourceID, externalID, state, operation, fiscalReference, version)
	s.mu.Lock()
	v := s.webhookInbox[eventID]
	now := time.Now().UTC()
	if err == nil {
		v.ProcessedAt = &now
	} else {
		v.Error = err.Error()
	}
	s.webhookInbox[eventID] = v
	persistErr := s.persistLocked()
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return persistErr
}
func (s *Service) Checkout(order, key string, p map[string]any) (Order, error) {
	return s.checkoutForTenant(order, key, []map[string]any{p}, p, "")
}
func (s *Service) CheckoutForTenant(order, key string, p map[string]any, tenant string) (Order, error) {
	return s.checkoutForTenant(order, key, []map[string]any{p}, p, tenant)
}
func (s *Service) CheckoutBatchForTenant(order, key string, payments []map[string]any, metadata map[string]any, tenant string) (Order, error) {
	if len(payments) < 2 || len(payments) > 10 {
		return Order{}, errors.New("split checkout requires 2 to 10 payments")
	}
	payload := map[string]any{"payments": payments}
	if metadata != nil {
		payload["metadata"] = metadata
	}
	return s.checkoutForTenant(order, key, payments, payload, tenant)
}
func (s *Service) checkoutForTenant(order, key string, payments []map[string]any, hashPayload any, tenant string) (Order, error) {
	if tenant != "" {
		current, err := s.OrderForTenant(order, tenant)
		if err != nil {
			return Order{}, errors.New("not payable")
		}
		s.mu.RLock()
		mirror, exists := s.orders[order]
		s.mu.RUnlock()
		if !exists || mirror.TenantID != tenant || mirror.Version != current.Version || mirror.State != current.State {
			return Order{}, errors.New("order state changed; retry")
		}
	}
	requestBody, e := json.Marshal(hashPayload)
	if e != nil {
		return Order{}, errors.New("invalid payment payload")
	}
	requestHash := fmt.Sprintf("%x", sha256.Sum256(append([]byte(order+"\n"), requestBody...)))
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		replayKey := tenant + ":" + key
		rawResult, resultErr := reader.LoadTenantEntity("checkouts", tenant, replayKey)
		rawHash, hashErr := reader.LoadTenantEntity("checkout_hashes", tenant, replayKey)
		resultMissing := errors.Is(resultErr, sql.ErrNoRows)
		hashMissing := errors.Is(hashErr, sql.ErrNoRows)
		if resultErr == nil || hashErr == nil {
			if hashErr != nil {
				return Order{}, errors.New("incomplete checkout checkpoint")
			}
			var storedHash string
			if json.Unmarshal(rawHash, &storedHash) != nil {
				return Order{}, errors.New("invalid checkout checkpoint")
			}
			if storedHash != requestHash {
				return Order{}, errors.New("idempotency key payload mismatch")
			}
			// A durable hash without a result is an in-flight intent. The
			// downstream sale, line and payment calls all use deterministic
			// idempotency keys, so retrying the sequence is the recovery path.
			if resultMissing {
				goto loadOrder
			}
			var done Order
			if json.Unmarshal(rawResult, &done) != nil {
				return Order{}, errors.New("invalid checkout checkpoint")
			}
			return done, nil
		}
		if !resultMissing || !hashMissing {
			return Order{}, errors.New("checkout checkpoint unavailable")
		}
	}
loadOrder:
	s.mu.Lock()
	o, ok := s.orders[order]
	if !ok {
		s.mu.Unlock()
		return o, errors.New("not payable")
	}
	replayKey := o.TenantID + ":" + key
	if done, exists := s.checkouts[replayKey]; exists {
		if s.checkoutHashes[replayKey] != requestHash {
			s.mu.Unlock()
			return o, errors.New("idempotency key payload mismatch")
		}
		s.mu.Unlock()
		return done, nil
	}
	intentHash, intentExists := s.checkoutHashes[replayKey]
	if intentExists && intentHash != requestHash {
		s.mu.Unlock()
		return o, errors.New("idempotency key payload mismatch")
	}
	resuming := intentExists && o.State == "FISCAL_PENDING"
	if o.State != "OPEN" && !resuming {
		s.mu.Unlock()
		return o, errors.New("not payable")
	}
	if e = validateCheckoutPayments(o.Total, payments); e != nil {
		s.mu.Unlock()
		return o, e
	}
	previous := s.orders[o.ID]
	o.Payments = normalizedOrderPayments(payments)
	oldIntent, oldIntentExists := s.checkoutHashes[replayKey]
	if !resuming {
		// MiniPOS owns these UUIDs. Persist them before the first downstream
		// attempt and reuse them after timeout, restart, or REST/BLE failover.
		if o.CheckoutOperationID == "" {
			if validUUID(key) {
				o.CheckoutOperationID = key
			} else {
				o.CheckoutOperationID = s.nextID("checkout-operation")
			}
		}
		if o.ReceiptSessionID == "" {
			if envelope, ok := hashPayload.(map[string]any); ok {
				if metadata, present := envelope["metadata"].(map[string]any); present {
					if receipt, valid := metadata["receipt_session_id"].(string); valid && validUUID(receipt) {
						o.ReceiptSessionID = receipt
					}
				}
			}
			if o.ReceiptSessionID == "" {
				o.ReceiptSessionID = s.nextID("receipt-session")
			}
		}
		o.State = "FISCAL_PENDING"
		o.Version++
		s.orders[o.ID] = o
	}
	s.checkoutHashes[replayKey] = requestHash
	if e := s.persistLocked(); e != nil {
		s.orders[o.ID] = previous
		if oldIntentExists {
			s.checkoutHashes[replayKey] = oldIntent
		} else {
			delete(s.checkoutHashes, replayKey)
		}
		s.mu.Unlock()
		return o, e
	}
	s.mu.Unlock()
	var sale map[string]any
	if e := s.call("POST", "/sales", key, map[string]any{"external_id": o.ExternalID, "register_id": o.RegisterID, "operator_id": o.OperatorCode}, &sale); e != nil {
		return s.fail(o, e)
	}
	saleID, ok := sale["sale_id"].(string)
	if !ok || saleID == "" {
		return s.fail(o, errors.New("fiscal response missing sale id"))
	}
	o.FiscalSaleID = saleID
	fiscalVersion, ok := sale["version"].(float64)
	if !ok || fiscalVersion < 1 || fiscalVersion != float64(int64(fiscalVersion)) {
		return s.fail(o, errors.New("fiscal response missing sale version"))
	}
	for i, l := range o.Lines {
		var updated map[string]any
		if e := s.callWithIfMatch("POST", "/sales/"+o.FiscalSaleID+"/lines", fmt.Sprintf("%s-line-%d", key, i), strconv.FormatInt(int64(fiscalVersion), 10), l, &updated); e != nil {
			return s.fail(o, e)
		}
		nextVersion, valid := updated["version"].(float64)
		if !valid || nextVersion != fiscalVersion+1 {
			return s.fail(o, errors.New("invalid fiscal sale version progression"))
		}
		fiscalVersion = nextVersion
	}
	var op map[string]any
	finalize := map[string]any{"client_operation_id": o.CheckoutOperationID, "receipt_session_id": o.ReceiptSessionID, "payments": payments, "expected_total": o.Total}
	if envelope, ok := hashPayload.(map[string]any); ok {
		if metadata, present := envelope["metadata"].(map[string]any); present {
			finalize["metadata"] = metadata
		}
	}
	if e := s.call("POST", "/sales/"+o.FiscalSaleID+":finalize", o.CheckoutOperationID, finalize, &op); e != nil {
		return s.fail(o, e)
	}
	operationID, valid := op["operation_id"].(string)
	if !valid || operationID == "" {
		return s.fail(o, errors.New("fiscal response missing operation id"))
	}
	state, valid := op["state"].(string)
	if !valid || state == "" {
		return s.fail(o, errors.New("fiscal response missing operation state"))
	}
	o.FiscalOperationID = operationID
	if fiscalReference, valid := op["fiscal_reference"].(string); valid && fiscalReference != "" {
		o.ReceiptReference = fiscalReference
	}
	if state == "FISCALIZED" {
		if o.ReceiptReference == "" {
			return s.fail(o, errors.New("fiscal response missing fiscal reference"))
		}
		o.State = "COMPLETED"
	} else if state == "FAILED" || state == "CANCELLED" {
		o.State = "FAILED"
	} else {
		o.State = "UNKNOWN"
	}
	o.Version++
	s.mu.Lock()
	previous = s.orders[o.ID]
	oldCheckout, checkoutExisted := s.checkouts[replayKey]
	oldHash, hashExisted := s.checkoutHashes[replayKey]
	s.orders[o.ID] = o
	s.checkouts[replayKey] = o
	s.checkoutHashes[replayKey] = requestHash
	e = s.persistLocked()
	if e != nil {
		s.orders[o.ID] = previous
		if checkoutExisted {
			s.checkouts[replayKey] = oldCheckout
		} else {
			delete(s.checkouts, replayKey)
		}
		if hashExisted {
			s.checkoutHashes[replayKey] = oldHash
		} else {
			delete(s.checkoutHashes, replayKey)
		}
	}
	s.mu.Unlock()
	return o, e
}

func (s *Service) ReverseOrderForTenant(orderID, key, reason, tenant string) (ReversalResult, error) {
	reason = strings.TrimSpace(reason)
	allowed := map[string]bool{"OPERATOR_ERROR": true, "CUSTOMER_RETURN": true, "CUSTOMER_COMPLAINT": true, "TAX_BASE_REDUCTION": true}
	if !allowed[reason] {
		return ReversalResult{}, errors.New("invalid reversal reason")
	}
	s.mu.Lock()
	order, ok := s.orders[orderID]
	if !ok || order.TenantID != tenant || order.State != "COMPLETED" || order.FiscalSaleID == "" || order.FiscalOperationID == "" || order.ReceiptReference == "" {
		s.mu.Unlock()
		return ReversalResult{}, errors.New("order is not reversible")
	}
	previousOrder := order
	order.State = "UNKNOWN"
	order.ReversalReasonCode = reason
	order.Version++
	order.UpdatedAt = time.Now().UTC()
	s.orders[orderID] = order
	if err := s.persistLocked(); err != nil {
		s.orders[orderID] = previousOrder
		s.mu.Unlock()
		return ReversalResult{}, err
	}
	s.mu.Unlock()

	var operation map[string]any
	payload := map[string]any{"reason_code": reason, "original_fiscal_reference": order.ReceiptReference}
	if err := s.call("POST", "/sales/"+order.FiscalSaleID+"/reversals", key+"-fiscal-reversal", payload, &operation); err != nil {
		// The durable UNKNOWN transition happened before network I/O, so a
		// timeout, crash or concurrent request cannot issue a second storno.
		return ReversalResult{}, err
	}
	operationID, _ := operation["operation_id"].(string)
	state, _ := operation["state"].(string)
	fiscalReference, _ := operation["fiscal_reference"].(string)
	simulated, _ := operation["simulated"].(bool)
	if operationID == "" || (state != "FISCALIZED" && state != "FAILED" && state != "CANCELLED" && state != "UNKNOWN" && state != "RECONCILING") {
		return ReversalResult{}, errors.New("invalid fiscal reversal response")
	}
	now := time.Now().UTC()
	publicState := state
	if publicState == "RECONCILING" {
		publicState = "UNKNOWN"
	} else if publicState == "CANCELLED" {
		publicState = "FAILED"
	}
	result := ReversalResult{OperationID: operationID, State: publicState, FiscalReference: fiscalReference, Simulated: simulated, CreatedAt: now, UpdatedAt: now}
	s.mu.Lock()
	current := s.orders[orderID]
	if current.TenantID == tenant && current.State == "REVERSED" && current.ReversalOperationID == operationID {
		s.mu.Unlock()
		return result, nil
	}
	if current.TenantID != tenant || current.State != "UNKNOWN" || current.ReversalReasonCode != reason {
		s.mu.Unlock()
		return ReversalResult{}, errors.New("order state changed during reversal")
	}
	previous := current
	current.ReversalOperationID = operationID
	current.ReversalFiscalReference = fiscalReference
	current.ReversalReasonCode = reason
	if state == "FISCALIZED" {
		current.State = "REVERSED"
	} else if state == "UNKNOWN" || state == "RECONCILING" {
		current.State = "UNKNOWN"
	} else {
		current.State = "COMPLETED"
	}
	current.Version++
	current.UpdatedAt = now
	s.orders[orderID] = current
	if err := s.persistLocked(); err != nil {
		s.orders[orderID] = previous
		s.mu.Unlock()
		return ReversalResult{}, err
	}
	s.mu.Unlock()
	return result, nil
}

func validateCheckoutPayments(total Money, payments []map[string]any) error {
	if len(payments) == 0 || len(payments) > 10 {
		return errors.New("invalid payment count")
	}
	want, err := moneyCents(total)
	if err != nil || want <= 0 {
		return errors.New("invalid order total")
	}
	seen := map[string]bool{}
	var sum int64
	for _, raw := range payments {
		paymentID, _ := raw["payment_id"].(string)
		paymentType, _ := raw["type"].(string)
		policy, _ := raw["terminal_policy"].(string)
		amount := Money{}
		switch amountMap := raw["amount"].(type) {
		case map[string]any:
			amount.Amount, _ = amountMap["amount"].(string)
			amount.Currency, _ = amountMap["currency"].(string)
		case map[string]string:
			amount.Amount = amountMap["amount"]
			amount.Currency = amountMap["currency"]
		}
		cents, amountErr := moneyCents(amount)
		if !validUUID(paymentID) || seen[paymentID] || amountErr != nil || cents <= 0 || (paymentType != "CASH" && paymentType != "CARD") {
			return errors.New("invalid checkout payment")
		}
		if paymentType == "CARD" && policy != "REQUIRED" && policy != "AUTO_IF_AVAILABLE" {
			return errors.New("card payment requires terminal policy")
		}
		if paymentType == "CASH" && policy != "" && policy != "NONE" {
			return errors.New("cash payment cannot require terminal")
		}
		seen[paymentID] = true
		sum += cents
	}
	if sum != want {
		return errors.New("payment total does not equal order total")
	}
	return nil
}

func moneyCents(value Money) (int64, error) {
	if !validMoney(value) {
		return 0, errors.New("invalid money")
	}
	return strconv.ParseInt(strings.ReplaceAll(value.Amount, ".", ""), 10, 64)
}
func normalizedOrderPayments(payments []map[string]any) []OrderPayment {
	out := make([]OrderPayment, 0, len(payments))
	for _, raw := range payments {
		payment := OrderPayment{}
		payment.Type, _ = raw["type"].(string)
		switch amount := raw["amount"].(type) {
		case map[string]any:
			payment.Amount.Amount, _ = amount["amount"].(string)
			payment.Amount.Currency, _ = amount["currency"].(string)
		case map[string]string:
			payment.Amount.Amount = amount["amount"]
			payment.Amount.Currency = amount["currency"]
		}
		out = append(out, payment)
	}
	return out
}
func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}
func (s *Service) fail(o Order, e error) (Order, error) {
	o.State = "UNKNOWN"
	o.Version++
	s.mu.Lock()
	previous := s.orders[o.ID]
	s.orders[o.ID] = o
	if persistErr := s.persistLocked(); persistErr != nil {
		s.orders[o.ID] = previous
		s.mu.Unlock()
		return previous, fmt.Errorf("%v; persist UNKNOWN state: %w", e, persistErr)
	}
	s.mu.Unlock()
	return o, e
}
func (s *Service) call(method, path, key string, in, out any) error {
	return s.callWithIfMatch(method, path, key, "", in, out)
}
func (s *Service) callWithIfMatch(method, path, key, expected string, in, out any) error {
	var b []byte
	if in != nil {
		b, _ = json.Marshal(in)
	}
	return s.callAttempt(context.Background(), method, path, key, expected, b, out, true)
}
func (s *Service) callAttempt(ctx context.Context, method, path, key, expected string, b []byte, out any, allowAuthRetry bool) error {
	r, err := http.NewRequestWithContext(ctx, method, s.fiscal+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("build Fiscal public API request: %w", err)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Api-Version", s.version)
	r.Header.Set("Idempotency-Key", key)
	if expected != "" {
		r.Header.Set("If-Match", expected)
	}
	token := ""
	if s.authProvider != nil {
		var e error
		token, e = s.authProvider.Token(ctx)
		if e != nil {
			return e
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
	}
	resp, e := s.client.Do(r)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized && allowAuthRetry && s.authProvider != nil {
		s.authProvider.Invalidate(token)
		return s.callAttempt(ctx, method, path, key, expected, b, out, false)
	}
	if resp.StatusCode >= 300 {
		var problem struct {
			Code string `json:"code"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&problem)
		if problem.Code != "" {
			return fmt.Errorf("fiscal status %d: %s", resp.StatusCode, problem.Code)
		}
		return fmt.Errorf("fiscal status %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNoContent || out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
