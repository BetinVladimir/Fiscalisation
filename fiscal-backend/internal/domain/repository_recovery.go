package domain

import (
	"bytes"
	"encoding/json"
	"errors"
)

var ErrRepositoryUnavailable = errors.New("repository persistence recovery required; retry after authoritative refresh")

// Called under the existing repository mutex. A recovery refresh never commits
// the caller's potentially stale mutation; the caller must retry from new state.
func (r *MemoryRepository) persistenceReadyLocked() error {
	if !r.persistenceRecoveryPending {
		return nil
	}
	r.restoreLocked()
	return ErrRepositoryUnavailable
}
func (r *MemoryRepository) persistenceReadableLocked() error {
	if r.persistenceRecoveryPending {
		return ErrRepositoryUnavailable
	}
	return nil
}
func (r *MemoryRepository) ensurePersistenceReady() error {
	r.mu.RLock()
	pending := r.persistenceRecoveryPending
	r.mu.RUnlock()
	if !pending {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.persistenceReadyLocked()
}
func (r *MemoryRepository) PersistenceReady() error { return r.ensurePersistenceReady() }
func (s *Service) PersistenceReady() error {
	if repository, ok := s.repo.(interface{ PersistenceReady() error }); ok {
		return repository.PersistenceReady()
	}
	return nil
}

// Recovery-only baseline-aware shape check. Historical sparse snapshots are
// accepted, but an acknowledged map/audit collection cannot disappear.
func validRepositorySnapshotShape(raw, baseline []byte) bool {
	var next, previous map[string]json.RawMessage
	if json.Unmarshal(raw, &next) != nil || next == nil || len(next) == 0 {
		return false
	}
	_ = json.Unmarshal(baseline, &previous)
	maps := []string{"sales", "operations", "devices", "shifts", "unp", "replays", "outbox", "ble_sessions", "sync_acks", "connectivity_probes", "resources", "artifacts", "edge_pending", "activation_challenges", "activation_requests"}
	recognized := false
	optional := map[string]bool{"edge_pending": true, "activation_challenges": true, "activation_requests": true}
	for _, key := range maps {
		value, present := next[key]
		if _, existed := previous[key]; existed && !present {
			return false
		}
		if !present {
			continue
		}
		var entries map[string]json.RawMessage
		if json.Unmarshal(value, &entries) != nil {
			return false
		}
		if entries == nil {
			var old map[string]json.RawMessage
			if !optional[key] || json.Unmarshal(previous[key], &old) != nil && len(previous[key]) > 0 || len(old) > 0 {
				return false
			}
			continue
		}
		recognized = true
	}
	audit, present := next["audit"]
	if _, existed := previous["audit"]; existed && !present {
		return false
	}
	if present {
		var entries []json.RawMessage
		if bytes.Equal(bytes.TrimSpace(audit), []byte("null")) || json.Unmarshal(audit, &entries) != nil {
			return false
		}
		recognized = true
	}
	return recognized
}

// Direct SQL readers must not bypass a concurrent in-memory uncertainty fence.
// These checks add no database query and never hold the repository mutex over IO.
func (r *MemoryRepository) readFence() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.persistenceReadableLocked()
}

type fencedTenantReader struct {
	repository *MemoryRepository
	reader     TenantEntityReader
}

func (r *MemoryRepository) tenantReader() (TenantEntityReader, bool) {
	reader, ok := r.store.(TenantEntityReader)
	return fencedTenantReader{r, reader}, ok
}
func (f fencedTenantReader) LoadTenantEntity(collection, tenant, id string) ([]byte, error) {
	if e := f.repository.readFence(); e != nil {
		return nil, e
	}
	raw, e := f.reader.LoadTenantEntity(collection, tenant, id)
	if e != nil {
		return nil, e
	}
	if e = f.repository.readFence(); e != nil {
		return nil, e
	}
	return raw, nil
}
func (f fencedTenantReader) LoadTenantEntities(collection, tenant string) ([][]byte, error) {
	if e := f.repository.readFence(); e != nil {
		return nil, e
	}
	raw, e := f.reader.LoadTenantEntities(collection, tenant)
	if e != nil {
		return nil, e
	}
	if e = f.repository.readFence(); e != nil {
		return nil, e
	}
	return raw, nil
}

type fencedSystemReader struct {
	repository *MemoryRepository
	reader     SystemEntityReader
}

func (r *MemoryRepository) systemReader() (SystemEntityReader, bool) {
	reader, ok := r.store.(SystemEntityReader)
	return fencedSystemReader{r, reader}, ok
}
func (f fencedSystemReader) LoadSystemEntities(collection string) ([][]byte, error) {
	if e := f.repository.readFence(); e != nil {
		return nil, e
	}
	raw, e := f.reader.LoadSystemEntities(collection)
	if e != nil {
		return nil, e
	}
	if e = f.repository.readFence(); e != nil {
		return nil, e
	}
	return raw, nil
}

// PersistenceEpoch changes for failed-save recovery and unavailable/corrupt
// refreshes. A healthy remote-generation refresh does not invalidate a new
// request before business execution; restorationEpoch still guards local rollback.
func (r *MemoryRepository) PersistenceEpoch() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.availabilityEpoch
}
func (s *Service) PersistenceEpoch() uint64 {
	if r, ok := s.repo.(interface{ PersistenceEpoch() uint64 }); ok {
		return r.PersistenceEpoch()
	}
	return 0
}
