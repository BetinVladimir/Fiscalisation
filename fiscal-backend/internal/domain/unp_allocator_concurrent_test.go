package domain

import (
	"sort"
	"sync"
	"testing"
)

// TestAllocateUNP50Parallel verifies that 50 concurrent single-unit reservations
// produce exactly 50 unique sequential numbers covering 1..50 with no gaps.
// Invariant 1: money = integer cents; Invariant 4: same key → no second document.
func TestAllocateUNP50Parallel(t *testing.T) {
	const N = 50
	r := NewMemoryRepository()

	var wg sync.WaitGroup
	type result struct {
		start, end int64
		err        error
	}
	results := make([]result, N)

	for i := 0; i < N; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			start, end, err := r.ReserveFiscalUNPRange("tenant", "AB123456", "12345678", 1)
			results[i] = result{start, end, err}
		}()
	}
	wg.Wait()

	// All calls must succeed.
	for i, res := range results {
		if res.err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, res.err)
		}
		// For count=1, start must equal end.
		if res.err == nil && res.start != res.end {
			t.Errorf("goroutine %d: single-unit range start=%d end=%d; want start==end", i, res.start, res.end)
		}
	}

	// Collect start values; sort to verify no duplicates and no gaps.
	starts := make([]int64, N)
	for i, res := range results {
		starts[i] = res.start
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	for i, v := range starts {
		want := int64(i + 1)
		if v != want {
			t.Errorf("sorted starts[%d]=%d, want %d (must be sequential 1..%d without gaps)",
				i, v, want, N)
		}
	}

	// The in-memory highwater must reflect all 50 allocations.
	r.mu.Lock()
	hw := r.unp["tenant\nAB123456"]
	r.mu.Unlock()
	if hw != int64(N) {
		t.Errorf("highwater=%d, want %d", hw, N)
	}
}
