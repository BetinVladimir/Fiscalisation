package domain

import (
	"database/sql"
	"encoding/json"
	"errors"
)

var ErrRecoveryStateUnavailable = errors.New("authoritative recovery state unavailable")
var ErrRecoveryOrderNotFound = errors.New("recovery order not found")

// Read readiness and the returned source order share one lock/linearization
// point. Never call Order/OrderForTenant here: their fallback RLock is recursive.
func (s *Service) RecoveryOrderForTenant(id, tenant string) (Order, uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.persistenceRecoveryPending {
		s.restoreLocked()
	}
	if s.persistenceRecoveryPending {
		return Order{}, 0, ErrRecoveryStateUnavailable
	}
	var order Order
	if reader, ok := s.store.(TenantEntityReader); ok && tenant != "" {
		raw, err := reader.LoadTenantEntity("orders", tenant, id)
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, 0, ErrRecoveryOrderNotFound
		}
		if err != nil || json.Unmarshal(raw, &order) != nil {
			return Order{}, 0, ErrRecoveryStateUnavailable
		}
	} else {
		var ok bool
		order, ok = s.orders[id]
		if !ok {
			return Order{}, 0, ErrRecoveryOrderNotFound
		}
	}
	if tenant == "" || order.TenantID != tenant || order.ID != id {
		return Order{}, 0, ErrRecoveryOrderNotFound
	}
	return order, s.recoveryReadEpoch, nil
}

// Authorization reads follow the exact returned order. Any intervening local
// mutation, including a failed save with unchanged durable generation, retires it.
func (s *Service) RecoveryReadCurrent(epoch uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return !s.persistenceRecoveryPending && s.recoveryReadEpoch == epoch
}
