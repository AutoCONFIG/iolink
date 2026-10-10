package rtp

import (
	"context"
	"errors"
	"sync"
)

const (
	MinRTPPort = 30000
	MaxRTPPort = 30038
	RTCPOffset = 1

	portPairCount = (MaxRTPPort-MinRTPPort)/2 + 1
)

var (
	ErrExhausted      = errors.New("rtp port pairs exhausted")
	ErrInvalidPair    = errors.New("invalid rtp port pair")
	ErrInvalidContext = errors.New("invalid rtp allocator context")
	ErrNotAllocated   = errors.New("rtp port pair not allocated")
)

type PortPair struct {
	RTP  int
	RTCP int
}

func (p PortPair) Valid() bool {
	return p.RTP >= MinRTPPort && p.RTP <= MaxRTPPort &&
		p.RTP%2 == 0 && p.RTCP == p.RTP+RTCPOffset
}

type Allocator struct {
	mu        sync.Mutex
	allocated [portPairCount]bool
}

func NewAllocator() *Allocator {
	return &Allocator{}
}

func (a *Allocator) Allocate(ctx context.Context) (PortPair, error) {
	if a == nil {
		return PortPair{}, ErrExhausted
	}
	if ctx == nil {
		return PortPair{}, ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return PortPair{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return PortPair{}, err
	}
	for index := range a.allocated {
		if a.allocated[index] {
			continue
		}
		a.allocated[index] = true
		return PortPair{RTP: MinRTPPort + index*2, RTCP: MinRTPPort + index*2 + RTCPOffset}, nil
	}
	return PortPair{}, ErrExhausted
}

func (a *Allocator) Release(ctx context.Context, pair PortPair) error {
	if a == nil {
		return ErrNotAllocated
	}
	if ctx == nil {
		return ErrInvalidContext
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !pair.Valid() {
		return ErrInvalidPair
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	index := (pair.RTP - MinRTPPort) / 2
	if !a.allocated[index] {
		return ErrNotAllocated
	}
	a.allocated[index] = false
	return nil
}
