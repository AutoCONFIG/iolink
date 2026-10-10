package mediajobs

import (
	"sync"
	"testing"
	"time"
)

func TestMemoryStoreClaimsOneWorkerUnderConcurrency(t *testing.T) {
	now := time.Date(2026, 10, 11, 5, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return now })
	if err := store.Enqueue(t.Context(), sampleInput(now)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	claimed := 0
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, ok, err := store.ClaimJob(t.Context(), now, time.Second, "worker-"+string(rune('a'+i))); err != nil {
				t.Errorf("claim err=%v", err)
			} else if ok {
				mu.Lock()
				claimed++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if claimed != 1 {
		t.Fatalf("claimed=%d, want 1", claimed)
	}
}
