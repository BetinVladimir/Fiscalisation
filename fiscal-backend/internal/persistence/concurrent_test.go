package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
)

// TestConcurrentSalesPersistAfterRestart verifies:
//   - 20 goroutines each add a distinct sale with optimistic-lock retry on
//     ErrConcurrentState;
//   - after a repository restart from Postgres all 20 sales are present;
//   - the generation recorded in meta is strictly > 0 (monotonic each write).
func TestConcurrentSalesPersistAfterRestart(t *testing.T) {
	url, _ := isolatedPostgresFixture(t)
	p, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}

	base := map[string]any{
		"sales": map[string]any{}, "operations": map[string]any{},
		"devices": map[string]any{}, "shifts": map[string]any{},
		"unp": map[string]any{}, "replays": map[string]any{},
		"outbox": map[string]any{}, "ble_sessions": map[string]any{},
		"sync_acks": map[string]any{}, "connectivity_probes": map[string]any{},
		"resources": map[string]any{}, "artifacts": map[string]any{},
		"audit": []any{}, "edge_pending": map[string]any{},
		"activation_challenges": map[string]any{},
		"activation_requests":   map[string]any{},
	}
	seed, _ := json.Marshal(base)
	if err = p.Save(seed); err != nil {
		t.Fatal("seed save:", err)
	}

	const N = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	retries := 0
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			saleKey := fmt.Sprintf("sale-%02d", id)
			sale := map[string]any{"id": saleKey, "tenant_id": "tenant-1", "amount": 1367}
			for attempt := 0; ; attempt++ {
				snap, gen, loadErr := p.LoadVersioned()
				if loadErr != nil {
					t.Errorf("LoadVersioned goroutine %d: %v", id, loadErr)
					return
				}
				var state map[string]any
				if err2 := json.Unmarshal(snap, &state); err2 != nil {
					t.Errorf("unmarshal goroutine %d: %v", id, err2)
					return
				}
				salesMap, _ := state["sales"].(map[string]any)
				if salesMap == nil {
					salesMap = map[string]any{}
				}
				salesMap[saleKey] = sale
				state["sales"] = salesMap
				updated, _ := json.Marshal(state)
				_, saveErr := p.SaveVersioned(updated, gen)
				if saveErr == nil {
					return
				}
				if errors.Is(saveErr, ErrConcurrentState) {
					mu.Lock()
					retries++
					mu.Unlock()
					continue
				}
				t.Errorf("SaveVersioned goroutine %d: %v", id, saveErr)
				return
			}
		}(i)
	}
	wg.Wait()

	_ = p.Close()
	p2, err := Open(url)
	if err != nil {
		t.Fatal("reopen:", err)
	}
	defer p2.Close()

	loaded, gen, err := p2.LoadVersioned()
	if err != nil {
		t.Fatal("LoadVersioned after restart:", err)
	}
	if gen <= 0 {
		t.Fatalf("generation must be > 0 after %d concurrent writes, got %d", N, gen)
	}

	var final map[string]any
	if err = json.Unmarshal(loaded, &final); err != nil {
		t.Fatal("unmarshal final:", err)
	}
	salesFinal, _ := final["sales"].(map[string]any)
	if len(salesFinal) != N {
		t.Fatalf("expected %d sales after restart, found %d; retries=%d", N, len(salesFinal), retries)
	}
	for i := 0; i < N; i++ {
		key := fmt.Sprintf("sale-%02d", i)
		if _, ok := salesFinal[key]; !ok {
			t.Errorf("sale %q missing after restart", key)
		}
	}
	t.Logf("concurrent_sales_persist: %d goroutines, %d retries on ErrConcurrentState, generation=%d", N, retries, gen)
}
