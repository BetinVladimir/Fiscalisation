package persistence

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"
)

func TestPostgresConcurrentUNPAllocatorHasNoDuplicatesOrGaps(t *testing.T) {
	url := os.Getenv("PG_INTEGRATION_URL")
	if url == "" {
		t.Skip("PG_INTEGRATION_URL is required")
	}
	p, err := Open(url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	const count = 64
	tenant := "90000000-0000-4000-8000-000000000001"
	_, _ = p.db.Exec(`delete from unp_allocations where tenant_id=$1`, tenant)
	sequences := make(chan int64, count)
	errors := make(chan error, count)
	var workers sync.WaitGroup
	for index := 1; index <= count; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			allocationID := fmt.Sprintf("90000000-0000-4000-8000-%012d", index+100)
			unp, sequence, allocateErr := p.AllocateUNP(context.Background(), tenant, allocationID, "AB123456", "A001")
			if allocateErr != nil {
				errors <- allocateErr
				return
			}
			if unp != fmt.Sprintf("AB123456-A001-%07d", sequence) {
				errors <- fmt.Errorf("UNP/sequence mismatch: %s %d", unp, sequence)
				return
			}
			sequences <- sequence
		}(index)
	}
	workers.Wait()
	close(errors)
	close(sequences)
	for allocateErr := range errors {
		t.Fatal(allocateErr)
	}
	got := make([]int, 0, count)
	for sequence := range sequences {
		got = append(got, int(sequence))
	}
	sort.Ints(got)
	if len(got) != count {
		t.Fatalf("allocated %d/%d", len(got), count)
	}
	for index, sequence := range got {
		if sequence != index+1 {
			t.Fatalf("gap or duplicate at %d: %#v", index+1, got)
		}
	}
}
