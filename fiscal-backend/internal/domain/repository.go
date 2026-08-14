package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("not found")
var ErrSaleExternalIDConflict = errors.New("sale external_id already exists")
var ErrSaleUNPConflict = errors.New("sale unp already exists")

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
type SystemEntityReader interface {
	LoadSystemEntities(collection string) ([][]byte, error)
}
type ReplayRecord struct {
	Hash        string              `json:"hash"`
	Status      int                 `json:"status"`
	Body        []byte              `json:"body"`
	ContentType string              `json:"content_type,omitempty"`
	Headers     map[string][]string `json:"headers,omitempty"`
	Pending     bool                `json:"pending,omitempty"`
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
	PutActivationChallenge(DeviceActivationChallenge) error
	ConsumeChallengeAndCreateActivation(string, string, DeviceActivationRequest) error
	ActivationRequest(string) (DeviceActivationRequest, error)
	ActivationRequestByCode(string) (DeviceActivationRequest, error)
	ConfirmActivationRequest(string, string, string, string, string, []string, string, time.Time) (DeviceActivationRequest, error)
	MarkActivationCredentialIssued(string, DeviceCredential, time.Time) (DeviceActivationRequest, error)
	ActivateDeviceRequest(string, string, time.Time) (DeviceActivationRequest, error)
	RevokeActivatedDevice(string, string, string, time.Time) (DeviceActivationRequest, error)
	ReserveUNPRange(string, string, int64) (int64, int64, error)
	Sale(string) (Sale, error)
	Sales(string) []Sale
	PutSale(Sale) error
	OpenSaleWithFirstLine(Sale, SaleLine, string) (Sale, error)
	AddSaleLineExpected(string, string, int64, SaleLine) (Sale, error)
	ReplaceSaleLinesExpected(string, string, int64, []SaleLine, string) (Sale, error)
	Operation(string) (Operation, error)
	Operations() []Operation
	PutOperation(Operation) error
	CommitOperationEvent(Operation, OutboxItem) error
	ReserveSalePayment(string, string, int64, Operation, FiscalDeviceSnapshot) (Sale, error)
	ReserveSalePaymentCommand(string, string, int64, Operation, FiscalDeviceSnapshot, ResourceRecord) (Sale, error)
	ReserveSaleReversal(string, string, int64, Operation) (Sale, error)
	ReserveSaleReversalCommand(string, string, int64, Operation, ResourceRecord) (Sale, error)
	CommitSaleOperation(Sale, Operation) error
	CommitSaleOperationEvent(Sale, Operation, OutboxItem) error
	CommitSaleOperationArtifact(Sale, Operation, string, string, []byte) error
	CommitSaleOperationArtifactEvent(Sale, Operation, string, string, []byte, OutboxItem) error
	CommitResourceArtifactsOperation(ResourceRecord, Operation, map[string][]byte) error
	CommitResourceArtifactsOperationEvents(ResourceRecord, Operation, map[string][]byte, []OutboxItem) error
	NextUNP(string, string, string) (string, error)
	OpenShift(string, string, string) (Shift, error)
	CloseShift(string) (Shift, error)
	Shift(string) (Shift, error)
	ShiftForTenant(string, string) (Shift, error)
	Shifts(string) []Shift
	Replay(string) (ReplayRecord, bool)
	ClaimReplay(string, string) (ReplayRecord, bool, error)
	PutReplay(string, ReplayRecord) error
	PutAmbiguousReplay(string, ReplayRecord) error
	AddOutbox(OutboxItem) error
	PendingOutbox(time.Time) []OutboxItem
	UpdateOutbox(OutboxItem) error
	PutBLESession(BLESessionRecord) error
	CommitBLESessionEvent(BLESessionRecord, OutboxItem) error
	BLESession(string) (BLESessionRecord, error)
	LastSyncAck(string, string) (SyncAck, bool)
	PutSyncAck(SyncAck) error
	CommitEdgeSync(string, SyncAck, []Sale, []Operation, map[string][]byte, []OutboxItem, []EdgePendingCommand, []string) error
	EdgePendingCommand(string, string) (EdgePendingCommand, error)
	PutConnectivityProbe(ConnectivityProbe) error
	ConnectivityProbe(string) (ConnectivityProbe, error)
	PutResource(ResourceRecord) error
	CommitCompositeBinding(ResourceRecord, ResourceRecord) error
	Resource(string, string) (ResourceRecord, error)
	Resources(string, string) []ResourceRecord
	PutArtifact(string, string, []byte) error
	Artifact(string, string) ([]byte, error)
	AuditEvents(string) []AuditEvent
	AppendAudit(string, string, string, string, string, string, map[string]any, map[string]any) error
}

func (r *MemoryRepository) ReserveUNPRange(tenant, fmin string, count int64) (int64, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if tenant == "" || fmin == "" || count < 1 || count > 10000 {
		return 0, 0, errors.New("invalid UNP range reservation")
	}
	key := tenant + "\n" + fmin
	start := r.unp[key] + 1
	end := r.unp[key] + count
	if end > 9_999_999 {
		return 0, 0, errors.New("UNP range exhausted")
	}
	r.unp[key] = end
	return start, end, r.persistLocked()
}

type MemoryRepository struct {
	mu                   sync.RWMutex
	sales                map[string]Sale
	operations           map[string]Operation
	devices              map[string]Device
	shifts               map[string]Shift
	unp                  map[string]int64
	replays              map[string]ReplayRecord
	outbox               map[string]OutboxItem
	bleSessions          map[string]BLESessionRecord
	syncAcks             map[string]SyncAck
	connectivityProbes   map[string]ConnectivityProbe
	resources            map[string]ResourceRecord
	artifacts            map[string][]byte
	audit                []AuditEvent
	edgePending          map[string]EdgePendingCommand
	activationChallenges map[string]DeviceActivationChallenge
	activationRequests   map[string]DeviceActivationRequest
	store                Store
	generation           int64
	confirmedSnapshot    []byte
}
type repositorySnapshot struct {
	Sales                map[string]Sale                      `json:"sales"`
	Operations           map[string]Operation                 `json:"operations"`
	Devices              map[string]Device                    `json:"devices"`
	Shifts               map[string]Shift                     `json:"shifts"`
	UNP                  map[string]int64                     `json:"unp"`
	Replays              map[string]ReplayRecord              `json:"replays"`
	Outbox               map[string]OutboxItem                `json:"outbox"`
	BLESessions          map[string]BLESessionRecord          `json:"ble_sessions"`
	SyncAcks             map[string]SyncAck                   `json:"sync_acks"`
	ConnectivityProbes   map[string]ConnectivityProbe         `json:"connectivity_probes"`
	Resources            map[string]ResourceRecord            `json:"resources"`
	Artifacts            map[string][]byte                    `json:"artifacts"`
	Audit                []AuditEvent                         `json:"audit"`
	EdgePending          map[string]EdgePendingCommand        `json:"edge_pending"`
	ActivationChallenges map[string]DeviceActivationChallenge `json:"activation_challenges"`
	ActivationRequests   map[string]DeviceActivationRequest   `json:"activation_requests"`
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{sales: map[string]Sale{}, operations: map[string]Operation{}, devices: map[string]Device{}, shifts: map[string]Shift{}, unp: map[string]int64{}, replays: map[string]ReplayRecord{}, outbox: map[string]OutboxItem{}, bleSessions: map[string]BLESessionRecord{}, syncAcks: map[string]SyncAck{}, connectivityProbes: map[string]ConnectivityProbe{}, resources: map[string]ResourceRecord{}, artifacts: map[string][]byte{}, audit: []AuditEvent{}, edgePending: map[string]EdgePendingCommand{}, activationChallenges: map[string]DeviceActivationChallenge{}, activationRequests: map[string]DeviceActivationRequest{}}
}
func NewPersistentRepository(store Store) (*MemoryRepository, error) {
	r := NewMemoryRepository()
	r.store = store
	var b []byte
	var e error
	if versioned, ok := store.(VersionedStore); ok {
		b, r.generation, e = versioned.LoadVersioned()
	} else {
		b, e = store.Load()
	}
	if e != nil {
		return nil, e
	}
	r.confirmedSnapshot = append([]byte(nil), b...)
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
		r.activationChallenges = x.ActivationChallenges
		if r.activationChallenges == nil {
			r.activationChallenges = map[string]DeviceActivationChallenge{}
		}
		r.activationRequests = x.ActivationRequests
		if r.activationRequests == nil {
			r.activationRequests = map[string]DeviceActivationRequest{}
		}
	}
	if e = r.recoverInterruptedOperations(); e != nil {
		return nil, e
	}
	if e = r.recoverOrphanReplays(); e != nil {
		return nil, e
	}
	return r, nil
}

func (r *MemoryRepository) recoverOrphanReplays() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := false
	for key, replay := range r.replays {
		if !replay.Pending || replay.Status != 102 || len(replay.Body) != 0 {
			continue
		}
		replay.Status = 503
		replay.ContentType = "application/problem+json"
		replay.Body = []byte(`{"type":"urn:beefiscal:error:idempotency_outcome_unknown","title":"IDEMPOTENCY_OUTCOME_UNKNOWN","status":503,"code":"IDEMPOTENCY_OUTCOME_UNKNOWN","retryable":false}`)
		r.replays[key] = replay
		changed = true
	}
	if !changed {
		return nil
	}
	return r.persistLocked()
}

func (r *MemoryRepository) recoverInterruptedOperations() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := false
	now := time.Now().UTC()
	for id, op := range r.operations {
		if op.State != "EXECUTING" {
			continue
		}
		// A durable device command is safe to republish with the same operation
		// identity. It has not become ambiguous merely because this process
		// restarted before receiving the signed device result.
		if command, ok := r.resources[resourceKey("device_command_outbox", id)]; ok && command.TenantID == op.TenantID {
			expires, _ := command.Data["expires_at"].(string)
			if deadline, err := time.Parse(time.RFC3339Nano, expires); err == nil && now.Before(deadline) {
				continue
			}
		}
		op.State = "UNKNOWN"
		op.ErrorCode = "INTERRUPTED_AFTER_DEVICE_DISPATCH"
		op.AllowedActions = []string{"RECONCILE"}
		op.Version++
		op.UpdatedAt = now
		r.operations[id] = op
		var recoveryEvent OutboxItem
		if op.Type == "FISCAL_SALE" || op.Type == "REVERSAL" {
			if sale, ok := r.sales[op.SaleID]; ok && ((op.Type == "FISCAL_SALE" && sale.State == "PAYMENT_PENDING") || (op.Type == "REVERSAL" && sale.State == "FISCALIZATION_PENDING")) {
				before := asMap(sale)
				sale.State = "UNKNOWN"
				sale.Version++
				sale.UpdatedAt = now
				r.sales[sale.ID] = sale
				r.appendAuditLocked(sale.TenantID, "system", "INTERRUPTED_FISCAL_OPERATION_RECOVERY", "sale", sale.ID, sale.UNP, before, asMap(sale))
				recoveryEvent = fiscalOperationEvent(sale, op)
			}
		}
		if recoveryEvent.ID == "" {
			recoveryEvent = fiscalCommandEvent(op.RegisterID, op)
		}
		if recoveryEvent.ID != "" {
			r.outbox[recoveryEvent.ID] = recoveryEvent
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return r.persistLocked()
}
func (r *MemoryRepository) persistLocked() error {
	if r.store == nil {
		return nil
	}
	b, e := json.Marshal(repositorySnapshot{Sales: r.sales, Operations: r.operations, Devices: r.devices, Shifts: r.shifts, UNP: r.unp, Replays: r.replays, Outbox: r.outbox, BLESessions: r.bleSessions, SyncAcks: r.syncAcks, ConnectivityProbes: r.connectivityProbes, Resources: r.resources, Artifacts: r.artifacts, Audit: r.audit, EdgePending: r.edgePending, ActivationChallenges: r.activationChallenges, ActivationRequests: r.activationRequests})
	if e != nil {
		return e
	}
	if delta, ok := r.store.(DeltaVersionedStore); ok {
		generation, err := delta.SaveDeltaVersioned(r.confirmedSnapshot, b, r.generation)
		if err == nil {
			r.generation = generation
			r.confirmedSnapshot = append(r.confirmedSnapshot[:0], b...)
			return nil
		}
		r.restoreLocked()
		return err
	}
	if versioned, ok := r.store.(VersionedStore); ok {
		generation, err := versioned.SaveVersioned(b, r.generation)
		if err == nil {
			r.generation = generation
			r.confirmedSnapshot = append(r.confirmedSnapshot[:0], b...)
			return nil
		}
		r.restoreLocked()
		return err
	}
	if err := r.store.Save(b); err != nil {
		r.restoreLocked()
		return err
	}
	return nil
}
func (r *MemoryRepository) restoreLocked() {
	var b []byte
	var err error
	if versioned, ok := r.store.(VersionedStore); ok {
		b, r.generation, err = versioned.LoadVersioned()
	} else {
		b, err = r.store.Load()
	}
	if err != nil {
		return
	}
	r.confirmedSnapshot = append(r.confirmedSnapshot[:0], b...)
	if len(b) == 0 {
		r.resetLocked()
		return
	}
	var x repositorySnapshot
	if json.Unmarshal(b, &x) != nil {
		return
	}
	r.sales, r.operations, r.devices, r.shifts, r.unp = x.Sales, x.Operations, x.Devices, x.Shifts, x.UNP
	r.replays, r.outbox, r.bleSessions, r.syncAcks = x.Replays, x.Outbox, x.BLESessions, x.SyncAcks
	r.connectivityProbes, r.resources, r.artifacts, r.audit, r.edgePending = x.ConnectivityProbes, x.Resources, x.Artifacts, x.Audit, x.EdgePending
	r.activationChallenges, r.activationRequests = x.ActivationChallenges, x.ActivationRequests
	if r.sales == nil {
		r.sales = map[string]Sale{}
	}
	if r.operations == nil {
		r.operations = map[string]Operation{}
	}
	if r.devices == nil {
		r.devices = map[string]Device{}
	}
	if r.shifts == nil {
		r.shifts = map[string]Shift{}
	}
	if r.unp == nil {
		r.unp = map[string]int64{}
	}
	if r.replays == nil {
		r.replays = map[string]ReplayRecord{}
	}
	if r.outbox == nil {
		r.outbox = map[string]OutboxItem{}
	}
	if r.bleSessions == nil {
		r.bleSessions = map[string]BLESessionRecord{}
	}
	if r.syncAcks == nil {
		r.syncAcks = map[string]SyncAck{}
	}
	if r.connectivityProbes == nil {
		r.connectivityProbes = map[string]ConnectivityProbe{}
	}
	if r.resources == nil {
		r.resources = map[string]ResourceRecord{}
	}
	if r.artifacts == nil {
		r.artifacts = map[string][]byte{}
	}
	if r.audit == nil {
		r.audit = []AuditEvent{}
	}
	if r.edgePending == nil {
		r.edgePending = map[string]EdgePendingCommand{}
	}
	if r.activationChallenges == nil {
		r.activationChallenges = map[string]DeviceActivationChallenge{}
	}
	if r.activationRequests == nil {
		r.activationRequests = map[string]DeviceActivationRequest{}
	}
}

func (r *MemoryRepository) resetLocked() {
	r.sales = map[string]Sale{}
	r.operations = map[string]Operation{}
	r.devices = map[string]Device{}
	r.shifts = map[string]Shift{}
	r.unp = map[string]int64{}
	r.replays = map[string]ReplayRecord{}
	r.outbox = map[string]OutboxItem{}
	r.bleSessions = map[string]BLESessionRecord{}
	r.syncAcks = map[string]SyncAck{}
	r.connectivityProbes = map[string]ConnectivityProbe{}
	r.resources = map[string]ResourceRecord{}
	r.artifacts = map[string][]byte{}
	r.audit = []AuditEvent{}
	r.edgePending = map[string]EdgePendingCommand{}
	r.activationChallenges = map[string]DeviceActivationChallenge{}
	r.activationRequests = map[string]DeviceActivationRequest{}
}
func (r *MemoryRepository) Replay(k string) (ReplayRecord, bool) {
	if reader, ok := r.store.(TenantEntityReader); ok {
		parts := strings.SplitN(k, " ", 4)
		if len(parts) == 4 && parts[0] != "" {
			raw, err := reader.LoadTenantEntity("replays", parts[0], k)
			if err != nil {
				return ReplayRecord{}, false
			}
			var v ReplayRecord
			if json.Unmarshal(raw, &v) != nil {
				return ReplayRecord{}, false
			}
			return v, true
		}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.replays[k]
	return v, ok
}
func (r *MemoryRepository) ClaimReplay(k, hash string) (ReplayRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if versioned, ok := r.store.(VersionedStore); ok {
		_, current, err := versioned.LoadVersioned()
		if err != nil {
			return ReplayRecord{}, false, err
		}
		if current != r.generation {
			r.restoreLocked()
			if r.generation != current {
				return ReplayRecord{}, false, errors.New("idempotency snapshot refresh failed")
			}
		}
	}
	if old, ok := r.replays[k]; ok {
		if old.Hash != hash {
			return old, false, errors.New("idempotency mismatch")
		}
		return old, false, nil
	}
	pending := ReplayRecord{Hash: hash, Status: 102, Pending: true}
	r.replays[k] = pending
	if err := r.persistLocked(); err != nil {
		if old, ok := r.replays[k]; ok {
			if old.Hash != hash {
				return old, false, errors.New("idempotency mismatch")
			}
			return old, false, nil
		}
		return ReplayRecord{}, false, err
	}
	return pending, true, nil
}
func (r *MemoryRepository) PutReplay(k string, v ReplayRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.replays[k]; ok && old.Hash != v.Hash {
		return errors.New("idempotency mismatch")
	}
	v.Pending = false
	r.replays[k] = v
	return r.persistLocked()
}
func (r *MemoryRepository) PutAmbiguousReplay(k string, v ReplayRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.replays[k]
	if !ok || old.Hash != v.Hash || !old.Pending {
		return errors.New("idempotency claim unavailable")
	}
	v.Pending = true
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
	if reader, ok := r.store.(SystemEntityReader); ok {
		rows, err := reader.LoadSystemEntities("outbox")
		if err != nil {
			return []OutboxItem{}
		}
		out := make([]OutboxItem, 0, len(rows))
		for _, raw := range rows {
			var v OutboxItem
			if json.Unmarshal(raw, &v) != nil {
				return []OutboxItem{}
			}
			if v.DeliveredAt == nil && !now.Before(v.NextAttempt) {
				out = append(out, v)
			}
		}
		return out
	}
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
func (r *MemoryRepository) BLESessionForTenant(id, tenant string) (BLESessionRecord, error) {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("ble_sessions", tenant, id)
		if err != nil {
			return BLESessionRecord{}, err
		}
		var v BLESessionRecord
		if err = json.Unmarshal(raw, &v); err != nil {
			return BLESessionRecord{}, err
		}
		return v, nil
	}
	v, err := r.BLESession(id)
	if err != nil || (tenant != "" && v.TenantID != tenant) {
		return BLESessionRecord{}, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) LastSyncAck(tenant, edge string) (SyncAck, bool) {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("sync_acks", tenant, edge)
		if err != nil {
			return SyncAck{}, false
		}
		var v SyncAck
		if json.Unmarshal(raw, &v) != nil {
			return SyncAck{}, false
		}
		return v, true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := edge
	if tenant != "" {
		key = tenant + "\n" + edge
	}
	v, ok := r.syncAcks[key]
	return v, ok
}
func (r *MemoryRepository) PutSyncAck(v SyncAck) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.syncAcks[v.EdgeID] = v
	return r.persistLocked()
}
func (r *MemoryRepository) CommitEdgeSync(tenant string, ack SyncAck, sales []Sale, operations []Operation, artifacts map[string][]byte, outbox []OutboxItem, pending []EdgePendingCommand, completed []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	artifactTenants := make(map[string]string, len(artifacts))
	for _, sale := range sales {
		if sale.ReceiptArtifactID != "" && sale.TenantID != "" {
			artifactTenants[sale.ReceiptArtifactID] = sale.TenantID
		}
	}
	for id := range artifacts {
		if artifactTenants[id] == "" {
			return errors.New("edge artifact tenant ownership required")
		}
	}
	for _, sale := range sales {
		r.sales[sale.ID] = sale
		action := "EDGE_SALE_PROJECTED"
		switch sale.State {
		case "OPEN":
			action = "SALE_OPENED"
		case "COMPLETED":
			action = "SALE_COMPLETED"
		case "CANCELLED":
			action = "SALE_CANCELLED"
		case "UNKNOWN":
			action = "FISCAL_RESULT_UNKNOWN"
		}
		r.appendAuditLocked(sale.TenantID, sale.OperatorID, action, "sale", sale.ID, sale.UNP, nil, asMap(sale))
	}
	for _, operation := range operations {
		r.operations[operation.ID] = operation
	}
	for id, body := range artifacts {
		key := artifactTenants[id] + "\n" + id
		if _, exists := r.artifacts[key]; !exists {
			if _, legacy := r.artifacts[id]; !legacy {
				r.artifacts[key] = append([]byte(nil), body...)
			}
		}
	}
	for _, item := range outbox {
		if _, exists := r.outbox[item.ID]; !exists {
			r.outbox[item.ID] = item
		}
	}
	for _, item := range pending {
		r.edgePending[item.OperationID] = item
		saleID := stringAny(item.Payload, "server_sale_id", "client_sale_surrogate_id", "external_id")
		r.appendAuditLocked(item.TenantID, "edge:"+ack.EdgeID, "EDGE_INTENT_ACCEPTED", "sale", saleID, stringAny(item.Payload, "unp"), nil, map[string]any{"operation_id": item.OperationID, "command_type": item.CommandType, "raw_intent": cloneMap(item.Payload), "edge_id": ack.EdgeID})
	}
	for _, id := range completed {
		delete(r.edgePending, id)
	}
	ackKey := ack.EdgeID
	if tenant != "" {
		ackKey = tenant + "\n" + ack.EdgeID
	}
	r.syncAcks[ackKey] = ack
	return r.persistLocked()
}
func (r *MemoryRepository) EdgePendingCommand(id, tenant string) (EdgePendingCommand, error) {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("edge_pending", tenant, id)
		if err != nil {
			return EdgePendingCommand{}, err
		}
		var v EdgePendingCommand
		if err = json.Unmarshal(raw, &v); err != nil {
			return EdgePendingCommand{}, err
		}
		return v, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.edgePending[id]
	if !ok || (tenant != "" && v.TenantID != tenant) {
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
func (r *MemoryRepository) ConnectivityProbeForTenant(id, tenant string) (ConnectivityProbe, error) {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("connectivity_probes", tenant, id)
		if err != nil {
			return ConnectivityProbe{}, err
		}
		var v ConnectivityProbe
		if err = json.Unmarshal(raw, &v); err != nil {
			return ConnectivityProbe{}, err
		}
		return v, nil
	}
	v, err := r.ConnectivityProbe(id)
	if err != nil || (tenant != "" && v.TenantID != tenant) {
		return ConnectivityProbe{}, ErrNotFound
	}
	return v, nil
}
func resourceKey(kind, id string) string { return kind + ":" + id }
func (r *MemoryRepository) PutResource(v ResourceRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	uniqueField := map[string]string{"location": "code", "register": "code", "operator": "code", "device": "serial"}[v.Kind]
	for _, existing := range r.resources {
		if existing.ID == v.ID || existing.TenantID != v.TenantID || existing.Kind != v.Kind {
			continue
		}
		if v.Kind == "organization" || (uniqueField != "" && strings.EqualFold(stringField(existing.Data, uniqueField), stringField(v.Data, uniqueField))) {
			if v.Kind == "organization" {
				return errors.New("duplicate organization")
			}
			return errors.New("duplicate " + uniqueField)
		}
	}
	var before map[string]any
	if old, ok := r.resources[resourceKey(v.Kind, v.ID)]; ok {
		if v.Version != old.Version+1 {
			return errors.New("resource version conflict")
		}
		before = cloneMap(old.Data)
	} else if v.Version != 1 {
		return errors.New("new resource version must be 1")
	}
	v.Data = cloneMap(v.Data)
	r.resources[resourceKey(v.Kind, v.ID)] = v
	action := "CONFIGURATION_CHANGED"
	if v.Kind == "operator" {
		action = "OPERATOR_CREATED"
		if before != nil {
			action = "OPERATOR_CHANGED"
			if !reflect.DeepEqual(before["roles"], v.Data["roles"]) {
				action = "OPERATOR_ROLE_CHANGED"
			}
			if stringField(before, "active_to") == "" && stringField(v.Data, "active_to") != "" {
				action = "OPERATOR_DEACTIVATED"
			}
		}
	} else if v.Kind == "workstation_session" {
		if before == nil {
			action = "LOGIN_SUCCEEDED"
		}
	}
	r.appendAuditLocked(v.TenantID, "system", action, v.Kind, v.ID, "", before, v.Data)
	if v.Kind == "workstation_session" && before == nil {
		r.appendAuditLocked(v.TenantID, "system", "WORKSTATION_STARTED", v.Kind, v.ID, "", nil, v.Data)
	}
	return r.persistLocked()
}
func (r *MemoryRepository) CommitCompositeBinding(register, binding ResourceRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	oldRegister, rok := r.resources[resourceKey("register", register.ID)]
	oldBinding, bok := r.resources[resourceKey("composite_binding", binding.ID)]
	if !rok || !bok || register.TenantID != binding.TenantID || register.Version != oldRegister.Version+1 || binding.Version != oldBinding.Version+1 {
		return errors.New("composite binding atomic version conflict")
	}
	r.resources[resourceKey("register", register.ID)] = register
	r.resources[resourceKey("composite_binding", binding.ID)] = binding
	if err := r.persistLocked(); err != nil {
		return err
	}
	return nil
}
func (r *MemoryRepository) Resource(kind, id string) (ResourceRecord, error) {
	if reader, ok := r.store.(TenantEntityReader); ok {
		r.mu.RLock()
		cached, exists := r.resources[resourceKey(kind, id)]
		r.mu.RUnlock()
		if exists && cached.TenantID != "" {
			raw, err := reader.LoadTenantEntity("resources:"+kind, cached.TenantID, id)
			if err != nil {
				return ResourceRecord{}, err
			}
			var v ResourceRecord
			if err = json.Unmarshal(raw, &v); err != nil {
				return ResourceRecord{}, err
			}
			return v, nil
		}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.resources[resourceKey(kind, id)]
	if !ok {
		return v, ErrNotFound
	}
	v.Data = cloneMap(v.Data)
	return v, nil
}
func (r *MemoryRepository) Resources(kind, tenant string) []ResourceRecord {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		rows, err := reader.LoadTenantEntities("resources:"+kind, tenant)
		if err != nil {
			return []ResourceRecord{}
		}
		out := make([]ResourceRecord, 0, len(rows))
		for _, raw := range rows {
			var v ResourceRecord
			if json.Unmarshal(raw, &v) != nil {
				return []ResourceRecord{}
			}
			out = append(out, v)
		}
		return out
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	v := make([]ResourceRecord, 0)
	for _, x := range r.resources {
		if x.Kind == kind && (tenant == "" || x.TenantID == tenant) {
			x.Data = cloneMap(x.Data)
			v = append(v, x)
		}
	}
	return v
}
func (r *MemoryRepository) PutArtifact(id, tenant string, b []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id == "" {
		return errors.New("artifact id required")
	}
	key := id
	if tenant != "" {
		key = tenant + "\n" + id
	}
	if _, exists := r.artifacts[key]; exists {
		return errors.New("artifact immutable")
	}
	if key != id {
		if _, legacy := r.artifacts[id]; legacy {
			return errors.New("artifact immutable")
		}
	}
	r.artifacts[key] = append([]byte(nil), b...)
	return r.persistLocked()
}
func (r *MemoryRepository) Artifact(id, tenant string) ([]byte, error) {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("artifacts", tenant, id)
		if err == nil {
			var body []byte
			if err = json.Unmarshal(raw, &body); err != nil {
				return nil, err
			}
			return body, nil
		}
		return nil, err
	}
	if id == "" {
		return nil, ErrNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := id
	if tenant != "" {
		key = tenant + "\n" + id
	}
	b, ok := r.artifacts[key]
	if !ok && tenant != "" {
		if legacy, exists := r.artifacts[id]; exists {
			r.artifacts[key] = legacy
			delete(r.artifacts, id)
			if err := r.persistLocked(); err != nil {
				delete(r.artifacts, key)
				r.artifacts[id] = legacy
				return nil, err
			}
			b, ok = legacy, true
		}
	}
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), b...), nil
}
func (r *MemoryRepository) AuditEvents(tenant string) []AuditEvent {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		rows, err := reader.LoadTenantEntities("audit", tenant)
		if err != nil {
			return []AuditEvent{}
		}
		out := make([]AuditEvent, 0, len(rows))
		for _, raw := range rows {
			var v AuditEvent
			if json.Unmarshal(raw, &v) != nil {
				return []AuditEvent{}
			}
			out = append(out, v)
		}
		return out
	}
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
func (r *MemoryRepository) AppendAudit(tenant, actor, action, kind, id, unp string, before, after map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appendAuditLocked(tenant, actor, action, kind, id, unp, before, after)
	return r.persistLocked()
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
	for id, existing := range r.sales {
		if id == v.ID || existing.TenantID != v.TenantID {
			continue
		}
		if existing.ExternalID == v.ExternalID {
			return ErrSaleExternalIDConflict
		}
		if v.UNP != "" && existing.UNP == v.UNP {
			return ErrSaleUNPConflict
		}
	}
	var before map[string]any
	if old, ok := r.sales[v.ID]; ok {
		before = asMap(old)
	}
	r.sales[v.ID] = v
	r.appendAuditLocked(v.TenantID, v.OperatorID, "SALE_LINE_ADDED", "sale", v.ID, v.UNP, before, asMap(v))
	return r.persistLocked()
}

// OpenSaleWithFirstLine is the compliance boundary: allocation, sale, first
// line, identifier binding projection and audit become visible together.
func (r *MemoryRepository) OpenSaleWithFirstLine(v Sale, line SaleLine, fmin string) (Sale, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.sales {
		if existing.TenantID == v.TenantID && existing.ExternalID == v.ExternalID {
			return Sale{}, ErrSaleExternalIDConflict
		}
	}
	key := v.TenantID + "\n" + fmin
	for {
		r.unp[key]++
		u, err := NewBGUNP(fmin, v.OperatorID, r.unp[key])
		if err != nil {
			return Sale{}, err
		}
		if !r.saleUNPExistsLocked(v.TenantID, u.String(), "") {
			v.UNP = u.String()
			v.RegulatoryIdentifiers = []RegulatoryIdentifier{u.RegulatoryIdentifier()}
			break
		}
	}
	v.State = "OPEN"
	v.Version = 1
	v.Lines = []SaleLine{line}
	r.sales[v.ID] = v
	r.appendAuditLocked(v.TenantID, v.OperatorID, "SALE_OPENED", "sale", v.ID, v.UNP, nil, asMap(v))
	if err := r.persistLocked(); err != nil {
		delete(r.sales, v.ID)
		return Sale{}, err
	}
	return v, nil
}
func (r *MemoryRepository) AddSaleLineExpected(id, tenant string, expected int64, line SaleLine) (Sale, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.sales[id]
	if !ok || v.TenantID != tenant {
		return Sale{}, ErrNotFound
	}
	if v.Version != expected {
		return v, errors.New("sale version conflict")
	}
	if v.State != "DRAFT" && v.State != "OPEN" {
		return v, errors.New("sale not editable")
	}
	before := asMap(v)
	if v.UNP == "" {
		key := v.RegisterID
		if v.TenantID != "" {
			key = v.TenantID + "\n" + v.RegisterID
		}
		for {
			r.unp[key]++
			candidate := v.RegisterID + "-" + v.OperatorID + "-" + pad7(r.unp[key])
			if !r.saleUNPExistsLocked(v.TenantID, candidate, v.ID) {
				v.UNP = candidate
				break
			}
		}
		v.State = "OPEN"
	}
	v.Lines = append(v.Lines, line)
	v.Version++
	v.UpdatedAt = time.Now().UTC()
	r.sales[v.ID] = v
	r.appendAuditLocked(v.TenantID, v.OperatorID, "UPSERT", "sale", v.ID, v.UNP, before, asMap(v))
	if err := r.persistLocked(); err != nil {
		return Sale{}, err
	}
	return v, nil
}

func (r *MemoryRepository) ReplaceSaleLinesExpected(id, tenant string, expected int64, lines []SaleLine, action string) (Sale, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.sales[id]
	if !ok || v.TenantID != tenant {
		return Sale{}, ErrNotFound
	}
	if v.Version != expected {
		return v, errors.New("sale version conflict")
	}
	if v.State != "OPEN" || v.UNP == "" || len(v.Payments) != 0 {
		return v, errors.New("sale not editable")
	}
	if action != "SALE_LINE_CHANGED" && action != "SALE_LINE_CANCELLED" {
		return v, errors.New("invalid sale line action")
	}
	before := asMap(v)
	v.Lines = append([]SaleLine(nil), lines...)
	v.Version++
	v.UpdatedAt = time.Now().UTC()
	r.sales[v.ID] = v
	r.appendAuditLocked(v.TenantID, v.OperatorID, action, "sale", v.ID, v.UNP, before, asMap(v))
	if err := r.persistLocked(); err != nil {
		return Sale{}, err
	}
	return v, nil
}

func (r *MemoryRepository) saleUNPExistsLocked(tenant, unp, exceptID string) bool {
	for id, sale := range r.sales {
		if id != exceptID && sale.TenantID == tenant && sale.UNP == unp {
			return true
		}
	}
	return false
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

func (r *MemoryRepository) CommitOperationEvent(operation Operation, event OutboxItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if operation.ID == "" || event.ID == "" || event.Event.EventID != event.ID || event.Event.TenantID != operation.TenantID {
		return errors.New("invalid operation event commit")
	}
	var before map[string]any
	if old, ok := r.operations[operation.ID]; ok {
		before = asMap(old)
	}
	r.operations[operation.ID] = operation
	if _, exists := r.outbox[event.ID]; !exists {
		r.outbox[event.ID] = event
	}
	r.appendAuditLocked(operation.TenantID, "system", "FISCAL_OPERATION_RESULT", "operation", operation.ID, "", before, asMap(operation))
	return r.persistLocked()
}
func (r *MemoryRepository) ReserveSalePayment(id, tenant string, expected int64, op Operation, device FiscalDeviceSnapshot) (Sale, error) {
	return r.reserveSalePayment(id, tenant, expected, op, device, nil)
}

func (r *MemoryRepository) ReserveSalePaymentCommand(id, tenant string, expected int64, op Operation, device FiscalDeviceSnapshot, command ResourceRecord) (Sale, error) {
	return r.reserveSalePayment(id, tenant, expected, op, device, &command)
}

func (r *MemoryRepository) reserveSalePayment(id, tenant string, expected int64, op Operation, device FiscalDeviceSnapshot, command *ResourceRecord) (Sale, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sale, ok := r.sales[id]
	if !ok || sale.TenantID != tenant {
		return Sale{}, ErrNotFound
	}
	if sale.Version != expected || sale.State != "OPEN" {
		return sale, errors.New("sale payment state conflict")
	}
	if command != nil && (command.Kind != "device_command_outbox" || command.ID != op.ID || command.TenantID != sale.TenantID || command.Version != 1 || command.Data == nil) {
		return Sale{}, errors.New("invalid device command outbox")
	}
	before := asMap(sale)
	if sale.TenantID != "" && device.DeviceID == "" {
		return sale, errors.New("fiscal device snapshot required")
	}
	sale.FiscalDevice = device
	sale.State = "PAYMENT_PENDING"
	sale.Version++
	sale.UpdatedAt = op.CreatedAt
	r.sales[id] = sale
	r.operations[op.ID] = op
	if command != nil {
		r.resources[resourceKey(command.Kind, command.ID)] = *command
	}
	r.appendAuditLocked(sale.TenantID, sale.OperatorID, "PAYMENT_INTENT_CREATED", "sale", sale.ID, sale.UNP, before, asMap(sale))
	r.appendAuditLocked(sale.TenantID, sale.OperatorID, "FISCAL_COMMAND_DURABLE", "sale", sale.ID, sale.UNP, before, asMap(sale))
	if err := r.persistLocked(); err != nil {
		return Sale{}, err
	}
	return sale, nil
}

func (r *MemoryRepository) ReserveSaleReversal(id, tenant string, expected int64, op Operation) (Sale, error) {
	return r.reserveSaleReversal(id, tenant, expected, op, nil)
}
func (r *MemoryRepository) ReserveSaleReversalCommand(id, tenant string, expected int64, op Operation, command ResourceRecord) (Sale, error) {
	return r.reserveSaleReversal(id, tenant, expected, op, &command)
}
func (r *MemoryRepository) reserveSaleReversal(id, tenant string, expected int64, op Operation, command *ResourceRecord) (Sale, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sale, ok := r.sales[id]
	if !ok || sale.TenantID != tenant {
		return Sale{}, ErrNotFound
	}
	if sale.Version != expected || sale.State != "COMPLETED" {
		return sale, errors.New("sale reversal state conflict")
	}
	if command != nil && (command.Kind != "device_command_outbox" || command.ID != op.ID || command.TenantID != sale.TenantID || command.Version != 1 || command.Data == nil) {
		return Sale{}, errors.New("invalid device command outbox")
	}
	before := asMap(sale)
	sale.State = "FISCALIZATION_PENDING"
	sale.Version++
	sale.UpdatedAt = op.CreatedAt
	r.sales[id] = sale
	r.operations[op.ID] = op
	if command != nil {
		r.resources[resourceKey(command.Kind, command.ID)] = *command
	}
	r.appendAuditLocked(sale.TenantID, sale.OperatorID, "REVERSAL_RESERVED", "sale", sale.ID, sale.UNP, before, asMap(sale))
	if err := r.persistLocked(); err != nil {
		return Sale{}, err
	}
	return sale, nil
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

func (r *MemoryRepository) CommitSaleOperationEvent(s Sale, o Operation, event OutboxItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if event.ID == "" || event.Event.EventID != event.ID || event.Event.ResourceID != s.ID || event.Event.TenantID != s.TenantID {
		return errors.New("invalid fiscal outbox event")
	}
	var before map[string]any
	if old, ok := r.sales[s.ID]; ok {
		before = asMap(old)
	}
	r.sales[s.ID] = s
	r.operations[o.ID] = o
	if _, exists := r.outbox[event.ID]; !exists {
		r.outbox[event.ID] = event
	}
	r.appendAuditLocked(s.TenantID, s.OperatorID, o.Type, "sale", s.ID, s.UNP, before, asMap(s))
	return r.persistLocked()
}

func (r *MemoryRepository) CommitSaleOperationArtifact(s Sale, o Operation, artifactID, tenant string, body []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if artifactID == "" || artifactID != s.ReceiptArtifactID || tenant != s.TenantID || len(body) == 0 {
		return errors.New("invalid sale artifact commit")
	}
	key := artifactID
	if tenant != "" {
		key = tenant + "\n" + artifactID
	}
	if _, exists := r.artifacts[key]; exists {
		return errors.New("artifact immutable")
	}
	if key != artifactID {
		if _, legacy := r.artifacts[artifactID]; legacy {
			return errors.New("artifact immutable")
		}
	}
	var before map[string]any
	if old, ok := r.sales[s.ID]; ok {
		before = asMap(old)
	}
	r.sales[s.ID] = s
	r.operations[o.ID] = o
	r.artifacts[key] = append([]byte(nil), body...)
	r.appendAuditLocked(s.TenantID, s.OperatorID, o.Type, "sale", s.ID, s.UNP, before, asMap(s))
	return r.persistLocked()
}

func (r *MemoryRepository) CommitSaleOperationArtifactEvent(s Sale, o Operation, artifactID, tenant string, body []byte, event OutboxItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if artifactID == "" || artifactID != s.ReceiptArtifactID || tenant != s.TenantID || len(body) == 0 || event.ID == "" || event.Event.EventID != event.ID || event.Event.ResourceID != s.ID || event.Event.TenantID != s.TenantID {
		return errors.New("invalid sale artifact event commit")
	}
	key := artifactID
	if tenant != "" {
		key = tenant + "\n" + artifactID
	}
	if _, exists := r.artifacts[key]; exists {
		return errors.New("artifact immutable")
	}
	if key != artifactID {
		if _, legacy := r.artifacts[artifactID]; legacy {
			return errors.New("artifact immutable")
		}
	}
	var before map[string]any
	if old, ok := r.sales[s.ID]; ok {
		before = asMap(old)
	}
	r.sales[s.ID] = s
	r.operations[o.ID] = o
	r.artifacts[key] = append([]byte(nil), body...)
	if _, exists := r.outbox[event.ID]; !exists {
		r.outbox[event.ID] = event
	}
	r.appendAuditLocked(s.TenantID, s.OperatorID, o.Type, "sale", s.ID, s.UNP, before, asMap(s))
	return r.persistLocked()
}

func (r *MemoryRepository) CommitResourceArtifactsOperation(resource ResourceRecord, operation Operation, artifacts map[string][]byte) error {
	return r.commitResourceArtifactsOperationEvents(resource, operation, artifacts, nil)
}

func (r *MemoryRepository) CommitResourceArtifactsOperationEvents(resource ResourceRecord, operation Operation, artifacts map[string][]byte, events []OutboxItem) error {
	return r.commitResourceArtifactsOperationEvents(resource, operation, artifacts, events)
}

func (r *MemoryRepository) commitResourceArtifactsOperationEvents(resource ResourceRecord, operation Operation, artifacts map[string][]byte, events []OutboxItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if resource.ID == "" || resource.Kind == "" || operation.ID == "" || operation.TenantID != resource.TenantID || operation.State != "FISCALIZED" || len(artifacts) == 0 {
		return errors.New("invalid resource artifact commit")
	}
	key := resourceKey(resource.Kind, resource.ID)
	if _, exists := r.resources[key]; exists {
		return errors.New("resource version conflict")
	}
	for id, body := range artifacts {
		if id == "" || len(body) == 0 {
			return errors.New("invalid artifact")
		}
		key := id
		if resource.TenantID != "" {
			key = resource.TenantID + "\n" + id
		}
		if _, exists := r.artifacts[key]; exists {
			return errors.New("artifact immutable")
		}
		if _, legacy := r.artifacts[id]; legacy {
			return errors.New("artifact immutable")
		}
	}
	for _, event := range events {
		if event.ID == "" || event.Event.EventID != event.ID || event.Event.TenantID != operation.TenantID {
			return errors.New("invalid resource outbox event")
		}
	}
	r.resources[key] = resource
	r.operations[operation.ID] = operation
	for id, body := range artifacts {
		key := id
		if resource.TenantID != "" {
			key = resource.TenantID + "\n" + id
		}
		r.artifacts[key] = append([]byte(nil), body...)
	}
	for _, event := range events {
		if _, exists := r.outbox[event.ID]; !exists {
			r.outbox[event.ID] = event
		}
	}
	r.appendAuditLocked(resource.TenantID, "system", "PUBLISH", resource.Kind, resource.ID, "", nil, asMap(resource))
	return r.persistLocked()
}
func asMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}
func (r *MemoryRepository) NextUNP(register, operator, tenant string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if tenant == "" {
		r.unp[register]++ // explicit in-memory DEV compatibility; PROD auth always supplies a tenant
		return register + "-" + operator + "-" + pad7(r.unp[register]), r.persistLocked()
	}
	key := tenant + "\n" + register
	if _, exists := r.unp[key]; !exists {
		if legacy, ok := r.unp[register]; ok {
			r.unp[key] = legacy
			delete(r.unp, register)
		}
	}
	r.unp[key]++
	v := register + "-" + operator + "-" + pad7(r.unp[key])
	return v, r.persistLocked()
}
func (r *MemoryRepository) OpenShift(register, operator, tenant string) (Shift, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.shifts {
		if v.TenantID == tenant && v.RegisterID == register && (v.State == "OPEN" || v.State == "CLOSING" || v.State == "RECONCILIATION_REQUIRED") {
			return Shift{}, errors.New("shift already open")
		}
	}
	now := time.Now().UTC()
	id, err := newUUID()
	if err != nil {
		return Shift{}, err
	}
	v := Shift{ID: id, TenantID: tenant, RegisterID: register, OperatorID: operator, State: "OPEN", Version: 1, OpenedAt: now, CreatedAt: now, UpdatedAt: now}
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
func (r *MemoryRepository) ShiftForTenant(id, tenant string) (Shift, error) {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("shifts", tenant, id)
		if err != nil {
			return Shift{}, err
		}
		var v Shift
		if err = json.Unmarshal(raw, &v); err != nil {
			return Shift{}, err
		}
		return v, nil
	}
	v, err := r.Shift(id)
	if err != nil || (tenant != "" && v.TenantID != tenant) {
		return Shift{}, ErrNotFound
	}
	return v, nil
}
func (r *MemoryRepository) Shifts(tenant string) []Shift {
	if reader, ok := r.store.(TenantEntityReader); ok && tenant != "" {
		rows, err := reader.LoadTenantEntities("shifts", tenant)
		if err != nil {
			return []Shift{}
		}
		out := make([]Shift, 0, len(rows))
		for _, raw := range rows {
			var v Shift
			if json.Unmarshal(raw, &v) != nil {
				return []Shift{}
			}
			out = append(out, v)
		}
		return out
	}
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
		if op.State != "UNKNOWN" && op.State != "FISCAL_RESULT_UNKNOWN" && op.State != "RECONCILING" && op.State != "EXECUTING" {
			continue
		}
		if op.TenantID != v.TenantID {
			continue
		}
		registerID := op.RegisterID
		if registerID == "" && op.SaleID != "" {
			if sale, exists := r.sales[op.SaleID]; exists {
				registerID = sale.RegisterID
			}
		}
		// Legacy unresolved operations without a register remain fail-closed
		// within their tenant, but can never block another tenant.
		if registerID != "" && registerID != v.RegisterID {
			continue
		}
		v.State = "RECONCILIATION_REQUIRED"
		r.shifts[id] = v
		_ = r.persistLocked()
		return v, errors.New("unresolved register operation blocks close")
	}
	now := time.Now().UTC()
	v.State = "CLOSED"
	v.ClosedAt = &now
	v.Version++
	r.shifts[id] = v
	return v, r.persistLocked()
}
