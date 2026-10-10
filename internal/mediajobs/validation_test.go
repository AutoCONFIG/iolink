package mediajobs

import (
	"errors"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
)

func TestMemoryStoreNilReceiverFailsClosed(t *testing.T) {
	var store *MemoryStore
	now := time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)
	if err := store.Enqueue(t.Context(), sampleInput(now)); !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("nil enqueue err=%v", err)
	}
	if err := store.SetAuthorization(t.Context(), "job", media.JobAuthorization{}); !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("nil authorization err=%v", err)
	}
	if _, ok, err := store.ClaimJob(t.Context(), now, time.Second, "worker"); !errors.Is(err, ErrInvalidJob) || ok {
		t.Fatalf("nil claim ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.Snapshot(t.Context(), "job"); !errors.Is(err, ErrInvalidJob) || ok {
		t.Fatalf("nil snapshot ok=%v err=%v", ok, err)
	}
}
