package domain

import (
	"fmt"
	"testing"
	"time"
)

// BenchmarkPersistLockedN measures persistLocked with N sales in the repository.
// Threshold: p95 ≤ 50 ms at N=10 000. Evidence: docs/MVP1/evidence/TG-001b.md.
func BenchmarkPersistLockedN(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			store := &testStore{}
			repo, err := NewPersistentRepository(store)
			if err != nil {
				b.Fatal(err)
			}
			now := time.Now().UTC()
			for i := range n {
				repo.mu.Lock()
				repo.sales[fmt.Sprintf("sale-%d", i)] = Sale{
					ID: fmt.Sprintf("sale-%d", i), State: "OPEN", Version: 1,
					CreatedAt: now, UpdatedAt: now,
				}
				repo.mu.Unlock()
			}
			b.ResetTimer()
			for range b.N {
				repo.mu.Lock()
				if err := repo.persistLocked(); err != nil {
					repo.mu.Unlock()
					b.Fatal(err)
				}
				repo.mu.Unlock()
			}
		})
	}
}
