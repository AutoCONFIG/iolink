package rtp_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/media/rtp"
)

func TestAllocatorReservesTwentyDisjointPairsThenExhausts(t *testing.T) {
	allocator := rtp.NewAllocator()
	seen := make(map[rtp.PortPair]struct{}, 20)
	for index := 0; index < 20; index++ {
		pair, err := allocator.Allocate(t.Context())
		if err != nil {
			t.Fatalf("allocation %d: %v", index, err)
		}
		want := rtp.PortPair{RTP: rtp.MinRTPPort + index*2, RTCP: rtp.MinRTPPort + index*2 + rtp.RTCPOffset}
		if pair != want {
			t.Fatalf("allocation %d = %+v, want %+v", index, pair, want)
		}
		if !pair.Valid() {
			t.Fatalf("allocation %d returned invalid pair %+v", index, pair)
		}
		if _, exists := seen[pair]; exists {
			t.Fatalf("duplicate pair %+v", pair)
		}
		seen[pair] = struct{}{}
	}
	if _, err := allocator.Allocate(t.Context()); !errors.Is(err, rtp.ErrExhausted) {
		t.Fatalf("exhausted allocation error = %v, want ErrExhausted", err)
	}
}

func TestAllocatorReleaseRejectsInvalidAndDuplicatePairs(t *testing.T) {
	allocator := rtp.NewAllocator()
	allocated, err := allocator.Allocate(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []rtp.PortPair{
		{RTP: rtp.MinRTPPort + 1, RTCP: rtp.MinRTPPort + 2},
		{RTP: rtp.MinRTPPort - 2, RTCP: rtp.MinRTPPort - 1},
		{RTP: rtp.MaxRTPPort + 2, RTCP: rtp.MaxRTPPort + 3},
		{RTP: allocated.RTP, RTCP: allocated.RTCP + 2},
	} {
		if err := allocator.Release(t.Context(), invalid); !errors.Is(err, rtp.ErrInvalidPair) {
			t.Fatalf("release %+v error = %v, want ErrInvalidPair", invalid, err)
		}
	}
	if err := allocator.Release(t.Context(), allocated); err != nil {
		t.Fatalf("release allocated pair: %v", err)
	}
	if err := allocator.Release(t.Context(), allocated); !errors.Is(err, rtp.ErrNotAllocated) {
		t.Fatalf("duplicate release error = %v, want ErrNotAllocated", err)
	}
	if got, err := allocator.Allocate(t.Context()); err != nil || got != allocated {
		t.Fatalf("released pair was not reused first: got=%+v err=%v", got, err)
	}
}

func TestAllocatorCancellationDoesNotChangeCapacity(t *testing.T) {
	allocator := rtp.NewAllocator()
	if _, err := allocator.Allocate(nil); !errors.Is(err, rtp.ErrInvalidContext) {
		t.Fatalf("nil allocation context error = %v, want ErrInvalidContext", err)
	}
	if err := allocator.Release(nil, rtp.PortPair{}); !errors.Is(err, rtp.ErrInvalidContext) {
		t.Fatalf("nil release context error = %v, want ErrInvalidContext", err)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := allocator.Allocate(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled allocation error = %v, want context.Canceled", err)
	}
	pair, err := allocator.Allocate(t.Context())
	if err != nil || pair.RTP != rtp.MinRTPPort {
		t.Fatalf("first allocation after cancellation = %+v err=%v", pair, err)
	}
	if err := allocator.Release(canceled, pair); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled release error = %v, want context.Canceled", err)
	}
	if _, err := allocator.Allocate(t.Context()); err != nil {
		t.Fatalf("canceled release consumed no capacity but next allocation failed: %v", err)
	}
}

func TestAllocatorConcurrentAllocationsAreUniqueAndBounded(t *testing.T) {
	allocator := rtp.NewAllocator()
	results := make(chan struct {
		pair rtp.PortPair
		err  error
	}, 64)
	var group sync.WaitGroup
	for range 64 {
		group.Go(func() {
			pair, err := allocator.Allocate(context.Background())
			results <- struct {
				pair rtp.PortPair
				err  error
			}{pair: pair, err: err}
		})
	}
	group.Wait()
	close(results)

	seen := make(map[rtp.PortPair]struct{}, 20)
	var successes int
	for result := range results {
		if result.err == nil {
			successes++
			if _, exists := seen[result.pair]; exists {
				t.Fatalf("concurrent duplicate pair %+v", result.pair)
			}
			seen[result.pair] = struct{}{}
			continue
		}
		if !errors.Is(result.err, rtp.ErrExhausted) {
			t.Fatalf("unexpected concurrent allocation error: %v", result.err)
		}
	}
	if successes != 20 {
		t.Fatalf("concurrent successful allocations = %d, want 20", successes)
	}
}

func TestPortPairValidBoundaries(t *testing.T) {
	for _, pair := range []rtp.PortPair{
		{RTP: rtp.MinRTPPort, RTCP: rtp.MinRTPPort + rtp.RTCPOffset},
		{RTP: rtp.MaxRTPPort, RTCP: rtp.MaxRTPPort + rtp.RTCPOffset},
	} {
		if !pair.Valid() {
			t.Fatalf("boundary pair %+v rejected", pair)
		}
	}
	for _, pair := range []rtp.PortPair{
		{RTP: rtp.MinRTPPort - 2, RTCP: rtp.MinRTPPort - 1},
		{RTP: rtp.MaxRTPPort + 2, RTCP: rtp.MaxRTPPort + 3},
		{RTP: rtp.MinRTPPort + 2, RTCP: rtp.MinRTPPort + 2},
		{RTP: rtp.MinRTPPort + 1, RTCP: rtp.MinRTPPort + 2},
	} {
		if pair.Valid() {
			t.Fatalf("invalid pair accepted: %+v", pair)
		}
	}
}
