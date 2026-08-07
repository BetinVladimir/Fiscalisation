package authority

import (
	"errors"
	"sync"
	"time"
)

type Lease struct {
	RegisterID, EdgeID                        string
	FencingToken                              int64
	OperationFrom, OperationTo, NextOperation int64
	UNPFrom, UNPTo, NextUNP                   int64
	ExpiresAt                                 time.Time
	Revoked                                   bool
}
type Manager struct {
	mu    sync.Mutex
	lease Lease
}

func New(v Lease) *Manager { return &Manager{lease: v} }
func (m *Manager) Allocate(now time.Time, fence int64) (int64, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.lease.Revoked || !now.Before(m.lease.ExpiresAt) || fence != m.lease.FencingToken {
		return 0, 0, errors.New("authority unavailable")
	}
	if m.lease.NextOperation == 0 {
		m.lease.NextOperation = m.lease.OperationFrom
	}
	if m.lease.NextUNP == 0 {
		m.lease.NextUNP = m.lease.UNPFrom
	}
	if m.lease.NextOperation > m.lease.OperationTo || m.lease.NextUNP > m.lease.UNPTo {
		return 0, 0, errors.New("allocation exhausted")
	}
	o, u := m.lease.NextOperation, m.lease.NextUNP
	m.lease.NextOperation++
	m.lease.NextUNP++
	return o, u, nil
}
func (m *Manager) Revoke() { m.mu.Lock(); defer m.mu.Unlock(); m.lease.Revoked = true }

// Restore advances counters from the durable journal after a process restart.
// It never moves an allocation counter backwards.
func (m *Manager) Restore(nextOperation, nextUNP int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if nextOperation > m.lease.NextOperation {
		m.lease.NextOperation = nextOperation
	}
	if nextUNP > m.lease.NextUNP {
		m.lease.NextUNP = nextUNP
	}
}
