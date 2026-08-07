package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	Load() ([]byte, error)
	Save([]byte) error
}
type TenantEntityReader interface {
	LoadTenantEntity(collection, tenant, id string) ([]byte, error)
	LoadTenantEntities(collection, tenant string) ([][]byte, error)
}
type ReplayRecord struct {
	Hash   string `json:"hash"`
	Status int    `json:"status"`
	Body   []byte `json:"body"`
}
type OutboxItem struct {
	ID          string                    `json:"id"`
	Event       WebhookEvent              `json:"event"`
	Attempts    int                       `json:"attempts"`
	NextAttempt time.Time                 `json:"next_attempt"`
	DeliveredAt *time.Time                `json:"delivered_at,omitempty"`
	Deliveries  map[string]OutboxDelivery `json:"deliveries,omitempty"`
}
type OutboxDelivery struct {
	Attempts    int        `json:"attempts"`
	NextAttempt time.Time  `json:"next_attempt"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}
type ResourceRecord struct {
	Kind      string         `json:"kind"`
	TenantID  string         `json:"tenant_id"`
	ID        string         `json:"id"`
	Version   int64          `json:"version"`
	Data      map[string]any `json:"data"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}
type AuditEvent struct {
	EventID    string         `json:"event_id"`
	TenantID   string         `json:"tenant_id,omitempty"`
	ActorID    string         `json:"actor_id"`
	Action     string         `json:"action"`
	ObjectType string         `json:"object_type"`
	ObjectID   string         `json:"object_id"`
	UNP        string         `json:"unp,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
	Before     map[string]any `json:"before"`
	After      map[string]any `json:"after"`
	PrevHash   string         `json:"prev_hash,omitempty"`
	EventHash  string         `json:"event_hash"`
}
type Repository interface {
	Sale(string) (Sale, error)
	Sales(string) []Sale
	PutSale(Sale) error
	Operation(string) (Operation, error)
	Operations() []Operation
	PutOperation(Operation) error
	CommitSaleOperation(Sale, Operation) error
	NextUNP(string, string) (string, error)
	OpenShift(string, string, string) (Shift, error)
	CloseShift(string) (Shift, error)
	Shift(string) (Shift, error)
	Shifts(string) []Shift
	Replay(string) (ReplayRecord, bool)
	PutReplay(string, ReplayRecord) error
	AddOutbox(OutboxItem) error
	PendingOutbox(time.Time) []OutboxItem
	UpdateOutbox(OutboxItem) error
	PutBLESession(BLESessionRecord) error
	CommitBLESessionEvent(BLESessionRecord, OutboxItem) error
	BLESession(string) (BLESessionRecord, error)
	LastSyncAck(string) (SyncAck, bool)
	PutSyncAck(SyncAck) error
	CommitEdgeSync(SyncAck, []Sale, []Operation, map[string][]byte, []OutboxItem, []EdgePendingCommand, []string) error
	EdgePendingCommand(string) (EdgePendingCommand, error)
	PutConnectivityProbe(ConnectivityProbe) error
	ConnectivityProbe(string) (ConnectivityProbe, error)
	PutResource(ResourceRecord) error
	Resource(string, string) (ResourceRecord, error)
	Resources(string, string) []ResourceRecord
	PutArtifact(string, []byte) error
	Artifact(string) ([]byte, error)
	AuditEvents(string) []AuditEvent
}
type MemoryRepository struct {
	mu                 sync.RWMutex
	sales              map[string]Sale
	operations         map[string]Operation
	devices            map[string]Device
	shifts             map[string]Shift
	unp                map[string]int64
	replays            map[string]ReplayRecord
	outbox             map[string]OutboxItem
	bleSessions        map[string]BLESessionRecord
	syncAcks           map[string]SyncAck
	connectivityProbes map[string]ConnectivityProbe
	resources          map[string]ResourceRecord
	artifacts          map[string][]byte
	audit              []AuditEvent
	edgePending        map[string]EdgePendingCommand
	store              Store
}
type repositorySnapshot struct {
	Sales              map[string]Sale               `json:"sales"`
	Operations         map[string]Operation          `json:"operations"`
	Devices            map[string]Device             `json:"devices"`
	Shifts             map[string]Shift              `json:"shifts"`
	UNP                map[string]int64              `json:"unp"`
	Replays            map[string]ReplayRecord       `json:"replays"`
	Outbox             map[string]OutboxItem         `json:"outbox"`
	BLESessions        map[string]BLESessionRecord   `json:"ble_sessions"`
	SyncAcks           map[string]SyncAck            `json:"sync_acks"`
	ConnectivityProbes map[string]ConnectivityProbe  `json:"connectivity_probes"`
	Resources          map[string]ResourceRecord     `json:"resources"`
	Artifacts          map[string][]byte             `json:"artifacts"`
	Audit              []AuditEvent                  `json:"audit"`
	EdgePending        map[string]EdgePendingCommand `json:"edge_pending"`
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{sales: map[string]Sale{}, operations: map[string]Operation{}, devices: map[string]Device{}, shifts: map[string]Shift{}, unp: map[string]int64{}, replays: map[string]ReplayRecord{}, outbox: map[string]OutboxItem{}, bleSessions: map[string]BLESessionRecord{}, syncAcks: map[string]SyncAck{}, connectivityProbes: map[string]ConnectivityProbe{}, resources: map[string]ResourceRecord{}, artifacts: map[string][]byte{}, audit: []AuditEvent{}, edgePending: map[string]EdgePendingCommand{}}
}
func NewPersistentRepository(store Store) (*MemoryRepository, error) {
	r := NewMemoryRepository()
	r.store = store
	b, e := store.Load()
	if e != nil {
		return nil, e
	}
	if len(b) > 0 {
		var x repositorySnapshot
		if e = json.Unmarshal(b, &x); e != nil {
			return nil, e
		}
		r.sales = x.Sales
		r.operations = x.Operations
		r.devices = x.Devices
		r.shifts = x.Shifts
		r.unp = x.UNP
		r.replays = x.Replays
		if r.replays == nil {
			r.replays = map[string]ReplayRecord{}
		}
		r.outbox = x.Outbox
		if r.outbox == nil {
			r.outbox = map[string]OutboxItem{}
		}
		r.bleSessions = x.BLESessions
		if r.bleSessions == nil {
			r.bleSessions = map[string]BLESessionRecord{}
		}
		r.syncAcks = x.SyncAcks
		if r.syncAcks == nil {
			r.syncAcks = map[string]SyncAck{}
		}
		r.connectivityProbes = x.ConnectivityProbes
		if r.connectivityProbes == nil {
			r.connectivityProbes = map[string]ConnectivityProbe{}
		}
		r.resources = x.Resources
		if r.resources == nil {
			r.resources = map[string]ResourceRecord{}
		}
		r.artifacts = x.Artifacts
		if r.artifacts == nil {
			r.artifacts = map[string][]byte{}
		}
		r.audit = x.Audit
		if r.audit == nil {
			r.audit = []AuditEvent{}
		}
		r.edgePending = x.EdgePending
		if r.edgePending == nil {
			r.edgePending = map[string]EdgePendingCommand{}
		}
	}
	return r, nil
}
func (r *MemoryRepository) persistLocked() error {
	if r.store == nil {
		return nil
	}
	b, e := json.Marshal(repositorySnapshot{Sales: r.sales, Operations: r.operations, Devices: r.devices, Shifts: r.shifts, UNP: r.unp, Replays: r.replays, Outbox: r.outbox, BLESessions: r.bleSessions, SyncAcks: r.syncAcks, ConnectivityProbes: r.connectivityProbes, Resources: r.resources, Artifacts: r.artifacts, Audit: r.audit, EdgePending: r.edgePending})
	if e != nil {
		return e
	}
	return r.store.Save(b)
}
func (r *MemoryRepository) Replay(k string) (ReplayRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.replays[k]
	return v, ok
}
func (r *MemoryRepository) PutReplay(k string, v ReplayRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.replays[k] = v
	return r.persistLocked()
}
func (r *MemoryRepository) AddOutbox(v OutboxItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.outbox[v.ID]; !ok {
		r.outbox[v.ID] = v
	}
	return r.persistLocked()
}
func (r *MemoryRepository) PendingOutbox(now time.Time) []OutboxItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var v []OutboxItem
	for _, x := range r.outbox {
		if x.DeliveredAt == nil && !now.Before(x.NextAttempt) {
			v = append(v, x)
		}
	}
	return v
}
func (r *MemoryRepository) UpdateOutbox(v OutboxItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outbox[v.ID] = v
	return r.persistLocked()
}
func (r *MemoryRepository) PutBLESession(v BLESessionRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bleSessions[v.SessionID] = v
	return r.persistLocked()
}
func (r *MemoryRepository) CommitBLESessionEvent(v BLESessionRecord, event OutboxItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bleSessions[v.SessionID] = v
	if _, exists := r.outbox[event.ID]; !exists {
		r.outbox[event.ID] = event
	}
	return r.persistLocked()
}
func (r *MemoryRepository) BLESession(id string) (BLESessionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.bleSessions[id]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) LastSyncAck(edge string) (SyncAck, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.syncAcks[edge]
	return v, ok
}
func (r *MemoryRepository) PutSyncAck(v SyncAck) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.syncAcks[v.EdgeID] = v
	return r.persistLocked()
}
func (r *MemoryRepository) CommitEdgeSync(ack SyncAck, sales []Sale, operations []Operation, artifacts map[string][]byte, outbox []OutboxItem, pending []EdgePendingCommand, completed []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, sale := range sales {
		r.sales[sale.ID] = sale
	}
	for _, operation := range operations {
		r.operations[operation.ID] = operation
	}
	for id, body := range artifacts {
		if _, exists := r.artifacts[id]; !exists {
			r.artifacts[id] = append([]byte(nil), body...)
		}
	}
	for _, item := range outbox {
		if _, exists := r.outbox[item.ID]; !exists {
			r.outbox[item.ID] = item
		}
	}
	for _, item := range pending {
		r.edgePending[item.OperationID] = item
	}
	for _, id := range completed {
		delete(r.edgePending, id)
	}
	r.syncAcks[ack.EdgeID] = ack
	return r.persistLocked()
}
func (r *MemoryRepository) EdgePendingCommand(id string) (EdgePendingCommand, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.edgePending[id]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) PutConnectivityProbe(v ConnectivityProbe) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connectivityProbes[v.ProbeID] = v
	return r.persistLocked()
}
func (r *MemoryRepository) ConnectivityProbe(id string) (ConnectivityProbe, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.connectivityProbes[id]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func resourceKey(kind, id string) string { return kind + ":" + id }
func (r *MemoryRepository) PutResource(v ResourceRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var before map[string]any
	if old, ok := r.resources[resourceKey(v.Kind, v.ID)]; ok {
		before = old.Data
	}
	r.resources[resourceKey(v.Kind, v.ID)] = v
	r.appendAuditLocked(v.TenantID, "system", "UPSERT", v.Kind, v.ID, "", before, v.Data)
	return r.persistLocked()
}
func (r *MemoryRepository) Resource(kind, id string) (ResourceRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.resources[resourceKey(kind, id)]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) Resources(kind, tenant string) []ResourceRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := make([]ResourceRecord, 0)
	for _, x := range r.resources {
		if x.Kind == kind && (tenant == "" || x.TenantID == tenant) {
			v = append(v, x)
		}
	}
	return v
}
func (r *MemoryRepository) PutArtifact(id string, b []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.artifacts[id]; exists {
		return errors.New("artifact immutable")
	}
	r.artifacts[id] = append([]byte(nil), b...)
	return r.persistLocked()
}
func (r *MemoryRepository) Artifact(id string) ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.artifacts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), b...), nil
}
func (r *MemoryRepository) AuditEvents(tenant string) []AuditEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := make([]AuditEvent, 0)
	for _, x := range r.audit {
		if tenant == "" || x.TenantID == tenant {
			v = append(v, x)
		}
	}
	return v
}
func (r *MemoryRepository) appendAuditLocked(tenant, actor, action, kind, id, unp string, before, after map[string]any) {
	prev := ""
	if len(r.audit) > 0 {
		prev = r.audit[len(r.audit)-1].EventHash
	}
	v := AuditEvent{TenantID: tenant, ActorID: actor, Action: action, ObjectType: kind, ObjectID: id, UNP: unp, OccurredAt: time.Now().UTC(), Before: before, After: after, PrevHash: prev}
	v.EventID, _ = newUUID()
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(append([]byte(prev), raw...))
	v.EventHash = hex.EncodeToString(sum[:])
	r.audit = append(r.audit, v)
}
func (r *MemoryRepository) Sale(id string) (Sale, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.sales[id]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) SaleForTenant(id, tenant string) (Sale, error) {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("sales", tenant, id)
		if err != nil {
			return Sale{}, err
		}
		var v Sale
		if err = json.Unmarshal(raw, &v); err != nil {
			return Sale{}, err
		}
		return v, nil
	}
	v, err := r.Sale(id)
	if err != nil || v.TenantID != tenant {
		return Sale{}, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) Sales(tenant string) []Sale {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		rows, err := reader.LoadTenantEntities("sales", tenant)
		if err == nil {
			out := make([]Sale, 0, len(rows))
			for _, raw := range rows {
				var v Sale
				if json.Unmarshal(raw, &v) != nil {
					return []Sale{}
				}
				out = append(out, v)
			}
			return out
		}
		return []Sale{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := make([]Sale, 0)
	for _, x := range r.sales {
		if tenant == "" || x.TenantID == tenant {
			v = append(v, x)
		}
	}
	return v
}
func (r *MemoryRepository) OperationsForTenant(tenant string) []Operation {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		rows, err := reader.LoadTenantEntities("operations", tenant)
		if err == nil {
			out := make([]Operation, 0, len(rows))
			for _, raw := range rows {
				var v Operation
				if json.Unmarshal(raw, &v) != nil {
					return []Operation{}
				}
				out = append(out, v)
			}
			return out
		}
		return []Operation{}
	}
	all := r.Operations()
	out := make([]Operation, 0)
	for _, v := range all {
		if tenant == "" || v.TenantID == tenant {
			out = append(out, v)
		}
	}
	return out
}
func (r *MemoryRepository) PutSale(v Sale) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var before map[string]any
	if old, ok := r.sales[v.ID]; ok {
		before = asMap(old)
	}
	r.sales[v.ID] = v
	r.appendAuditLocked(v.TenantID, v.OperatorID, "UPSERT", "sale", v.ID, v.UNP, before, asMap(v))
	return r.persistLocked()
}
func (r *MemoryRepository) Operation(id string) (Operation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.operations[id]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) OperationForTenant(id, tenant string) (Operation, error) {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("operations", tenant, id)
		if err != nil {
			return Operation{}, err
		}
		var v Operation
		if err = json.Unmarshal(raw, &v); err != nil {
			return Operation{}, err
		}
		return v, nil
	}
	v, err := r.Operation(id)
	if err != nil || v.TenantID != tenant {
		return Operation{}, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) Operations() []Operation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := make([]Operation, 0, len(r.operations))
	for _, x := range r.operations {
		v = append(v, x)
	}
	return v
}
func (r *MemoryRepository) PutOperation(v Operation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var before map[string]any
	if old, ok := r.operations[v.ID]; ok {
		before = asMap(old)
	}
	r.operations[v.ID] = v
	r.appendAuditLocked(v.TenantID, "system", "UPSERT", "operation", v.ID, "", before, asMap(v))
	return r.persistLocked()
}
func (r *MemoryRepository) CommitSaleOperation(s Sale, o Operation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var before map[string]any
	if old, ok := r.sales[s.ID]; ok {
		before = asMap(old)
	}
	r.sales[s.ID] = s
	r.operations[o.ID] = o
	r.appendAuditLocked(s.TenantID, s.OperatorID, o.Type, "sale", s.ID, s.UNP, before, asMap(s))
	return r.persistLocked()
}
func asMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}
func (r *MemoryRepository) NextUNP(register, operator string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.unp[register]++
	v := register + "-" + operator + "-" + pad7(r.unp[register])
	return v, r.persistLocked()
}
func (r *MemoryRepository) OpenShift(register, operator, tenant string) (Shift, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.shifts {
		if v.RegisterID == register && v.State == "OPEN" {
			return Shift{}, errors.New("shift already open")
		}
	}
	v := Shift{ID: newID("shift"), TenantID: tenant, RegisterID: register, OperatorID: operator, State: "OPEN", Version: 1, OpenedAt: time.Now().UTC()}
	r.shifts[v.ID] = v
	return v, r.persistLocked()
}
func (r *MemoryRepository) Shift(id string) (Shift, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.shifts[id]
	if !ok {
		return v, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) Shifts(tenant string) []Shift {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := make([]Shift, 0)
	for _, x := range r.shifts {
		if tenant == "" || x.TenantID == tenant {
			v = append(v, x)
		}
	}
	return v
}
func (r *MemoryRepository) CloseShift(id string) (Shift, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.shifts[id]
	if !ok {
		return v, ErrNotFound
	}
	if v.State != "OPEN" {
		return v, errors.New("shift not open")
	}
	for _, op := range r.operations {
		if op.State == "UNKNOWN" || op.State == "FISCAL_RESULT_UNKNOWN" {
			v.State = "RECONCILIATION_REQUIRED"
			r.shifts[id] = v
			_ = r.persistLocked()
			return v, errors.New("unknown operation blocks close")
		}
	}
	now := time.Now().UTC()
	v.State = "CLOSED"
	v.ClosedAt = &now
	v.Version++
	r.shifts[id] = v
	return v, r.persistLocked()
}
