package mediajobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/media"
)

func TestMemoryStoreLeaseCASAndUnknownIsTerminal(t *testing.T) {
	now := time.Date(2026, 10, 11, 2, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}
	store := NewMemoryStore(clock.Now)
	if err := store.Enqueue(t.Context(), sampleInput(now)); err != nil {
		t.Fatal(err)
	}
	first, ok, err := store.ClaimJob(t.Context(), now, 10*time.Second, "worker-a")
	if err != nil || !ok {
		t.Fatalf("first claim ok=%v err=%v", ok, err)
	}
	if first.Attempts != 1 || first.LeaseToken == "" || !first.LeasedUntil.Equal(now.Add(10*time.Second)) {
		t.Fatalf("first lease=%+v", first)
	}
	if _, ok, err = store.ClaimJob(t.Context(), now.Add(time.Hour), 10*time.Second, "worker-b"); err != nil || ok {
		t.Fatalf("claim before lease expiry ok=%v err=%v", ok, err)
	}
	clock.Set(now.Add(10 * time.Second))
	second, ok, err := store.ClaimJob(t.Context(), now, 10*time.Second, "worker-b")
	if err != nil || !ok || second.LeaseToken == first.LeaseToken || second.Attempts != 2 {
		t.Fatalf("reclaimed lease=%+v ok=%v err=%v", second, ok, err)
	}
	if _, err := store.RevalidateJob(t.Context(), first); !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("stale revalidate err=%v", err)
	}
	if err := store.FinishJob(t.Context(), first, media.JobResult{Status: media.JobReady, NextAt: now.Add(11 * time.Second)}, now.Add(10*time.Second)); !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("stale finish err=%v", err)
	}
	if _, err := store.RevalidateJob(t.Context(), second); err != nil {
		t.Fatalf("current revalidate err=%v", err)
	}
	if err := store.FinishJob(t.Context(), second, media.JobResult{Status: media.JobUnknown, ErrorCode: "provider_unknown", NextAt: now.Add(10 * time.Second)}, now.Add(10*time.Second+time.Second)); err != nil {
		t.Fatalf("unknown finish err=%v", err)
	}
	snapshot, ok, err := store.Snapshot(t.Context(), "job-1")
	if err != nil || !ok || snapshot.Status != "unknown" || !snapshot.LeasedUntil.IsZero() || snapshot.LeaseOwner != "" {
		t.Fatalf("terminal snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
	if err := store.FinishJob(t.Context(), second, media.JobResult{Status: media.JobReady, NextAt: now.Add(time.Minute)}, now.Add(12*time.Second)); !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("duplicate finish err=%v", err)
	}
	if _, ok, err := store.ClaimJob(t.Context(), now.Add(time.Minute), 10*time.Second, "worker-c"); err != nil || ok {
		t.Fatalf("terminal claim ok=%v err=%v", ok, err)
	}
}

func TestMemoryStoreSourceFenceAndRetrySchedule(t *testing.T) {
	now := time.Date(2026, 10, 11, 3, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}
	store := NewMemoryStore(clock.Now)
	if err := store.Enqueue(t.Context(), sampleInput(now)); err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(t.Context(), now, time.Second, "worker")
	if err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	auth := sampleInput(now).Authorization
	auth.Target.SourceVersion = 8
	auth.SourceCurrent = false
	if err := store.SetAuthorization(t.Context(), lease.ID, auth); err != nil {
		t.Fatal(err)
	}
	got, err := store.RevalidateJob(t.Context(), lease)
	if err != nil || got.SourceCurrent {
		t.Fatalf("stale auth=%+v err=%v", got, err)
	}
	if err := store.FinishJob(t.Context(), lease, media.JobResult{Status: media.JobReady, NextAt: now.Add(time.Second)}, now); !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("stale source finish err=%v", err)
	}

	clock.Set(now.Add(2 * time.Second))
	if err := store.SetAuthorization(t.Context(), lease.ID, sampleInput(now).Authorization); err != nil {
		t.Fatal(err)
	}
	lease, ok, err = store.ClaimJob(t.Context(), now.Add(2*time.Second), time.Second, "worker-2")
	if err != nil || !ok {
		t.Fatalf("reclaim after stale finish ok=%v err=%v", ok, err)
	}
	next := now.Add(5 * time.Second)
	if err := store.FinishJob(t.Context(), lease, media.JobResult{Status: media.JobRetryable, ErrorCode: "source_unreachable", NextAt: next}, now.Add(2*time.Second)); err != nil {
		t.Fatalf("retry finish err=%v", err)
	}
	if _, ok, err := store.ClaimJob(t.Context(), now.Add(4*time.Second), time.Second, "worker-3"); err != nil || ok {
		t.Fatalf("early retry claim ok=%v err=%v", ok, err)
	}
	clock.Set(next)
	lease, ok, err = store.ClaimJob(t.Context(), next, time.Second, "worker-3")
	if err != nil || !ok || lease.Attempts != 3 {
		t.Fatalf("scheduled retry lease=%+v ok=%v err=%v", lease, ok, err)
	}
}

func TestMemoryStoreRejectsExpiredFinishAndBadResults(t *testing.T) {
	now := time.Date(2026, 10, 11, 4, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}
	store := NewMemoryStore(clock.Now)
	if err := store.Enqueue(t.Context(), sampleInput(now)); err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(t.Context(), now, time.Second, "worker")
	if err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	if err := store.FinishJob(t.Context(), lease, media.JobResult{Status: media.JobReady, NextAt: now}, now.Add(time.Second)); !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("expired finish err=%v", err)
	}
	clock.Set(now.Add(time.Second))
	if _, err := store.RevalidateJob(t.Context(), lease); !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("expired revalidate err=%v", err)
	}
	clock.Set(now.Add(2 * time.Second))
	if err := store.FinishJob(t.Context(), lease, media.JobResult{Status: media.JobReady, NextAt: now}, now.Add(500*time.Millisecond)); !errors.Is(err, media.ErrLeaseLost) {
		t.Fatalf("stale caller time bypassed lease expiry: err=%v", err)
	}
	if err := store.FinishJob(t.Context(), lease, media.JobResult{Status: media.JobStatus("bogus")}, now); !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("bad result err=%v", err)
	}
	if _, ok, err := store.ClaimJob(t.Context(), now, 0, "worker"); !errors.Is(err, ErrInvalidJob) || ok {
		t.Fatalf("zero lease ok=%v err=%v", ok, err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, ok, err := store.ClaimJob(canceled, now, time.Second, "worker"); !errors.Is(err, context.Canceled) || ok {
		t.Fatalf("canceled claim ok=%v err=%v", ok, err)
	}
}

func TestMemoryStoreSkippedResultCancelsWithoutReplay(t *testing.T) {
	now := time.Date(2026, 10, 11, 6, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return now })
	if err := store.Enqueue(t.Context(), sampleInput(now)); err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(t.Context(), now, time.Second, "worker")
	if err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	if err := store.FinishJob(t.Context(), lease, media.JobResult{Status: media.JobSkipped, ErrorCode: "source_stale", NextAt: now}, now); err != nil {
		t.Fatalf("skip finish err=%v", err)
	}
	snapshot, ok, err := store.Snapshot(t.Context(), lease.ID)
	if err != nil || !ok || snapshot.Status != "cancelled" || !snapshot.LeasedUntil.IsZero() {
		t.Fatalf("cancelled snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
	if _, ok, err := store.ClaimJob(t.Context(), now.Add(time.Minute), time.Second, "worker-2"); err != nil || ok {
		t.Fatalf("cancelled claim ok=%v err=%v", ok, err)
	}
}

func TestMemoryStoreStaleStartCanCommitSkippedCancellation(t *testing.T) {
	now := time.Date(2026, 10, 11, 7, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return now })
	if err := store.Enqueue(t.Context(), sampleInput(now)); err != nil {
		t.Fatal(err)
	}
	lease, ok, err := store.ClaimJob(t.Context(), now, time.Second, "worker")
	if err != nil || !ok {
		t.Fatalf("claim ok=%v err=%v", ok, err)
	}
	auth := sampleInput(now).Authorization
	auth.SourceCurrent = false
	auth.Target.SourceVersion++
	if err := store.SetAuthorization(t.Context(), lease.ID, auth); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishJob(t.Context(), lease, media.JobResult{Status: media.JobSkipped, ErrorCode: "source_stale", NextAt: now}, now); err != nil {
		t.Fatalf("stale skip finish err=%v", err)
	}
	snapshot, ok, err := store.Snapshot(t.Context(), lease.ID)
	if err != nil || !ok || snapshot.Status != "cancelled" {
		t.Fatalf("stale skip snapshot=%+v ok=%v err=%v", snapshot, ok, err)
	}
}

func TestMemoryStoreBoundsWorkerAndMaxAttemptRetry(t *testing.T) {
	now := time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return now })
	input := sampleInput(now)
	input.Job.ID = "job-max"
	input.Job.Attempts = MaxAttempts - 1
	if err := store.Enqueue(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.ClaimJob(t.Context(), now, time.Second, strings.Repeat("w", 129)); !errors.Is(err, ErrInvalidJob) || ok {
		t.Fatalf("long worker ok=%v err=%v", ok, err)
	}
	lease, ok, err := store.ClaimJob(t.Context(), now, time.Second, "worker")
	if err != nil || !ok || lease.Attempts != MaxAttempts {
		t.Fatalf("last attempt lease=%+v ok=%v err=%v", lease, ok, err)
	}
	next := now.Add(time.Second)
	if err := store.FinishJob(t.Context(), lease, media.JobResult{Status: media.JobRetryable, NextAt: next}, now); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := store.ClaimJob(t.Context(), next, time.Second, "worker"); err != nil || ok {
		t.Fatalf("retry exceeded max ok=%v err=%v", ok, err)
	}
	bad := input
	bad.Job.ID = "job-too-many"
	bad.Job.Attempts = MaxAttempts
	if err := store.Enqueue(t.Context(), bad); !errors.Is(err, ErrInvalidJob) {
		t.Fatalf("max attempts enqueue err=%v", err)
	}
}
