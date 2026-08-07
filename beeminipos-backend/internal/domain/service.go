package domain

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
type Shift struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	RegisterID string     `json:"register_id"`
	EmployeeID string     `json:"employee_id"`
	State      string     `json:"state"`
	OpenedAt   time.Time  `json:"opened_at"`
	ClosedAt   *time.Time `json:"closed_at,omitempty"`
	Version    int64      `json:"version"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
type Configuration struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	LocationName     string    `json:"location_name"`
	LocationAddress  string    `json:"location_address"`
	WorkstationName  string    `json:"workstation_name"`
	FiscalRegisterID string    `json:"fiscal_register_id"`
	Version          int64     `json:"version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
type Line struct {
	LineID    string `json:"line_id"`
	ProductID string `json:"product_id,omitempty"`
	Name      string `json:"name"`
	Quantity  string `json:"quantity"`
	UnitPrice Money  `json:"unit_price"`
	TaxGroup  string `json:"tax_group"`
}
type Order struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	ExternalID        string    `json:"external_id"`
	ShiftID           string    `json:"shift_id"`
	RegisterID        string    `json:"register_id"`
	OperatorCode      string    `json:"operator_code"`
	State             string    `json:"state"`
	Lines             []Line    `json:"lines"`
	Total             Money     `json:"total"`
	FiscalSaleID      string    `json:"fiscal_sale_id,omitempty"`
	FiscalOperationID string    `json:"fiscal_operation_id,omitempty"`
	FiscalVersion     int64     `json:"fiscal_version,omitempty"`
	Version           int64     `json:"version"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
type Service struct {
	mu              sync.RWMutex
	products        map[string]Product
	employees       map[string]Employee
	shifts          map[string]Shift
	orders          map[string]Order
	checkouts       map[string]Order
	checkoutHashes  map[string]string
	apiReplays      map[string]APIReplay
	webhookInbox    map[string]WebhookInboxRecord
	configurations  map[string]Configuration
	fiscal, version string
	client          *http.Client
	authToken       string
	authProvider    AccessTokenProvider
	store           Store
	sequence        uint64
}

func (s *Service) SetFiscalAuthToken(v string) {
	s.authToken = v
	if v != "" {
		s.authProvider = staticTokenProvider(v)
	}
}
func (s *Service) SetFiscalAuthProvider(v AccessTokenProvider) { s.authProvider = v; s.authToken = "" }

type Store interface {
	Load() ([]byte, error)
	Save([]byte) error
}
type TenantEntityReader interface {
	LoadTenantEntity(collection, tenant, id string) ([]byte, error)
	LoadTenantEntities(collection, tenant string) ([][]byte, error)
}
type snapshot struct {
	Products       map[string]Product            `json:"products"`
	Employees      map[string]Employee           `json:"employees"`
	Shifts         map[string]Shift              `json:"shifts"`
	Orders         map[string]Order              `json:"orders"`
	Checkouts      map[string]Order              `json:"checkouts"`
	CheckoutHashes map[string]string             `json:"checkout_hashes"`
	APIReplays     map[string]APIReplay          `json:"api_replays"`
	WebhookInbox   map[string]WebhookInboxRecord `json:"webhook_inbox"`
	Configurations map[string]Configuration      `json:"configurations"`
	Sequence       uint64                        `json:"sequence"`
}

func NewService(f, v string) *Service {
	return &Service{products: map[string]Product{}, employees: map[string]Employee{}, shifts: map[string]Shift{}, orders: map[string]Order{}, checkouts: map[string]Order{}, checkoutHashes: map[string]string{}, apiReplays: map[string]APIReplay{}, webhookInbox: map[string]WebhookInboxRecord{}, configurations: map[string]Configuration{}, fiscal: f, version: v, client: &http.Client{Timeout: 10 * time.Second}}
}
func NewPersistentService(f, v string, store Store) (*Service, error) {
	s := NewService(f, v)
	s.store = store
	b, e := store.Load()
	if e != nil {
		return nil, e
	}
	if len(b) > 0 {
		var x snapshot
		if e = json.Unmarshal(b, &x); e != nil {
			return nil, e
		}
		s.products = x.Products
		s.employees = x.Employees
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
	return s, nil
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
	b, e := json.Marshal(snapshot{Products: s.products, Employees: s.employees, Shifts: s.shifts, Orders: s.orders, Checkouts: s.checkouts, CheckoutHashes: s.checkoutHashes, APIReplays: s.apiReplays, WebhookInbox: s.webhookInbox, Configurations: s.configurations, Sequence: s.sequence})
	if e != nil {
		return e
	}
	return s.store.Save(b)
}

type APIReplay struct {
	Hash        string `json:"hash"`
	Status      int    `json:"status"`
	Body        []byte `json:"body"`
	ContentType string `json:"content_type"`
}
type WebhookInboxRecord struct {
	EventID     string     `json:"event_id"`
	Hash        string     `json:"hash"`
	Raw         []byte     `json:"raw"`
	ReceivedAt  time.Time  `json:"received_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
}

func (s *Service) APIReplay(key string) (APIReplay, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.apiReplays[key]
	return v, ok
}
func (s *Service) PutAPIReplay(key string, v APIReplay) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.apiReplays[key]; ok && old.Hash != v.Hash {
		return errors.New("idempotency mismatch")
	}
	s.apiReplays[key] = v
	return s.persistLocked()
}

func (s *Service) ConfigurationFor(tenant string) (Configuration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.configurations[tenant]
	if !ok {
		return Configuration{}, errors.New("configuration not found")
	}
	return v, nil
}

func (s *Service) SaveConfiguration(tenant string, expected int64, v Configuration) (Configuration, error) {
	if strings.TrimSpace(v.LocationName) == "" || strings.TrimSpace(v.WorkstationName) == "" || strings.TrimSpace(v.FiscalRegisterID) == "" || len(v.LocationName) > 120 || len(v.LocationAddress) > 240 || len(v.WorkstationName) > 120 || len(v.FiscalRegisterID) > 120 {
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
			return out
		}
		return []Product{}
	}
	all := s.Products()
	if tenant == "" {
		return all
	}
	v := make([]Product, 0)
	for _, x := range all {
		if x.TenantID == tenant {
			v = append(v, x)
		}
	}
	return v
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
	}
	v.ID = s.nextID("product")
	if v.Unit == "" {
		v.Unit = "pcs"
	}
	if v.Status == "" {
		v.Status = "ACTIVE"
	}
	v.Active = true
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
			return out
		}
		return []Employee{}
	}
	all := s.Employees()
	if tenant == "" {
		return all
	}
	v := make([]Employee, 0)
	for _, x := range all {
		if x.TenantID == tenant {
			v = append(v, x)
		}
	}
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
	v.Active = true
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
func (s *Service) OpenShift(register, employee string) (Shift, error) {
	return s.OpenShiftForTenant(register, employee, "")
}
func (s *Service) OpenShiftForTenant(register, employee, tenant string) (Shift, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	emp, ok := s.employees[employee]
	if !ok || (tenant != "" && emp.TenantID != tenant) {
		return Shift{}, errors.New("employee not found")
	}
	for _, v := range s.shifts {
		if v.RegisterID == register && v.State == "OPEN" {
			return Shift{}, errors.New("shift already open")
		}
	}
	now := time.Now().UTC()
	v := Shift{ID: s.nextID("shift"), TenantID: tenant, RegisterID: register, EmployeeID: employee, State: "OPEN", OpenedAt: now, Version: 1, CreatedAt: now, UpdatedAt: now}
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
		if o.ShiftID == id && (o.State == "OPEN" || o.State == "FISCAL_PENDING" || o.State == "UNKNOWN") {
			return v, errors.New("unresolved order blocks close")
		}
	}
	now := time.Now().UTC()
	v.State = "CLOSED"
	v.ClosedAt = &now
	v.Version++
	v.UpdatedAt = now
	s.shifts[id] = v
	return v, s.persistLocked()
}
func (s *Service) Shift(id string) (Shift, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.shifts[id]
	if !ok {
		return v, errors.New("not found")
	}
	return v, nil
}
func (s *Service) ShiftForTenant(id, tenant string) (Shift, error) {
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		raw, e := reader.LoadTenantEntity("shifts", tenant, id)
		if e != nil {
			return Shift{}, e
		}
		var v Shift
		e = json.Unmarshal(raw, &v)
		return v, e
	}
	v, e := s.Shift(id)
	if e != nil || v.TenantID != tenant {
		return Shift{}, errors.New("not found")
	}
	return v, nil
}
func (s *Service) CreateOrder(v Order) (Order, error) {
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
			return out
		}
		return []Order{}
	}
	all := s.Orders()
	if tenant == "" {
		return all
	}
	v := make([]Order, 0)
	for _, x := range all {
		if x.TenantID == tenant {
			v = append(v, x)
		}
	}
	return v
}
func (s *Service) SalesReport() map[string]any {
	return s.SalesReportFor("")
}
func (s *Service) SalesReportFor(tenant string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	completed := 0
	for _, x := range s.orders {
		if tenant != "" && x.TenantID != tenant {
			continue
		}
		if x.State == "COMPLETED" {
			completed++
		}
	}
	total := 0
	for _, x := range s.orders {
		if tenant == "" || x.TenantID == tenant {
			total++
		}
	}
	return map[string]any{"currency": "EUR", "completed_orders": completed, "total_orders": total, "generated_at": time.Now().UTC()}
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
	if !validUUID(v.LineID) || v.Name == "" || !validMoney(v.UnitPrice) || !validTax(v.TaxGroup) || !validQuantity(v.Quantity) {
		return o, errors.New("invalid line")
	}
	if v.ProductID != "" {
		p, exists := s.products[v.ProductID]
		if !exists || p.TenantID != o.TenantID {
			return o, errors.New("product not found")
		}
	}
	o.Lines = append(o.Lines, v)
	o.Total = orderTotal(o.Lines)
	o.State = "OPEN"
	o.Version++
	o.UpdatedAt = time.Now().UTC()
	s.orders[o.ID] = o
	return o, s.persistLocked()
}
func orderTotal(lines []Line) Money {
	var cents int64
	for _, l := range lines {
		p, _ := strconv.ParseInt(strings.ReplaceAll(l.UnitPrice.Amount, ".", ""), 10, 64)
		q, _ := strconv.ParseInt(strings.ReplaceAll(l.Quantity, ".", ""), 10, 64)
		cents += (p*q + 500) / 1000
	}
	return Money{Amount: fmt.Sprintf("%d.%02d", cents/100, cents%100), Currency: "EUR"}
}
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
	if v.FirstName == "" || v.LastName == "" || len(v.OperatorCode) != 4 || (v.Status != "" && v.Status != "ACTIVE" && v.Status != "INACTIVE") {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, o := range s.orders {
		if o.FiscalSaleID != aggregateID && (externalID == "" || o.ExternalID != externalID) {
			continue
		}
		if version <= o.FiscalVersion {
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
		default:
			return errors.New("unknown fiscal state")
		}
		o.FiscalOperationID = operation
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
	if eventID == "" {
		return errors.New("event id required")
	}
	sum := sha256.Sum256(raw)
	hash := fmt.Sprintf("%x", sum)
	s.mu.Lock()
	if old, ok := s.webhookInbox[eventID]; ok {
		if old.Hash != hash {
			s.mu.Unlock()
			return errors.New("event id payload mismatch")
		}
		if old.ProcessedAt != nil {
			s.mu.Unlock()
			return nil
		}
	} else {
		s.webhookInbox[eventID] = WebhookInboxRecord{EventID: eventID, Hash: hash, Raw: append([]byte(nil), raw...), ReceivedAt: time.Now().UTC()}
		if e := s.persistLocked(); e != nil {
			s.mu.Unlock()
			return e
		}
	}
	s.mu.Unlock()
	err := s.ApplyFiscalEventLinked(resourceID, externalID, state, operation, version)
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
	requestBody, e := json.Marshal(p)
	if e != nil {
		return Order{}, errors.New("invalid payment payload")
	}
	requestHash := fmt.Sprintf("%x", sha256.Sum256(append([]byte(order+"\n"), requestBody...)))
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
	if o.State != "OPEN" {
		s.mu.Unlock()
		return o, errors.New("not payable")
	}
	o.State = "FISCAL_PENDING"
	o.Version++
	s.orders[o.ID] = o
	if e := s.persistLocked(); e != nil {
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
	for i, l := range o.Lines {
		var ignored map[string]any
		if e := s.call("POST", "/sales/"+o.FiscalSaleID+"/lines", fmt.Sprintf("%s-line-%d", key, i), l, &ignored); e != nil {
			return s.fail(o, e)
		}
	}
	var op map[string]any
	if e := s.call("POST", "/sales/"+o.FiscalSaleID+"/payments", key+"-payment", p, &op); e != nil {
		return s.fail(o, e)
	}
	operationID, ok := op["operation_id"].(string)
	if !ok || operationID == "" {
		return s.fail(o, errors.New("fiscal response missing operation id"))
	}
	state, ok := op["state"].(string)
	if !ok || state == "" {
		return s.fail(o, errors.New("fiscal response missing operation state"))
	}
	o.FiscalOperationID = operationID
	if state == "FISCALIZED" {
		o.State = "COMPLETED"
	} else if state == "FAILED" || state == "CANCELLED" {
		o.State = "FAILED"
	} else {
		o.State = "UNKNOWN"
	}
	o.Version++
	s.mu.Lock()
	s.orders[o.ID] = o
	s.checkouts[replayKey] = o
	s.checkoutHashes[replayKey] = requestHash
	e = s.persistLocked()
	s.mu.Unlock()
	return o, e
}
func (s *Service) fail(o Order, e error) (Order, error) {
	o.State = "UNKNOWN"
	o.Version++
	s.mu.Lock()
	s.orders[o.ID] = o
	_ = s.persistLocked()
	s.mu.Unlock()
	return o, e
}
func (s *Service) call(method, path, key string, in, out any) error {
	b, _ := json.Marshal(in)
	return s.callAttempt(context.Background(), method, path, key, b, out, true)
}
func (s *Service) callAttempt(ctx context.Context, method, path, key string, b []byte, out any, allowAuthRetry bool) error {
	r, _ := http.NewRequestWithContext(ctx, method, s.fiscal+path, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Api-Version", s.version)
	r.Header.Set("Idempotency-Key", key)
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
		return s.callAttempt(ctx, method, path, key, b, out, false)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("fiscal status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
