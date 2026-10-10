package gbcatalog

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

type key struct {
	device string
	sn     int
}

type Coordinator struct {
	store CatalogStore
	now   func() time.Time

	mu        sync.Mutex
	pending   map[key]*pending
	completed map[key]Refresh
	stale     map[key]Refresh
}

func New(store CatalogStore, now func() time.Time) (*Coordinator, error) {
	if store == nil {
		return nil, ErrInvalid
	}
	if now == nil {
		now = time.Now
	}
	return &Coordinator{store: store, now: now, pending: make(map[key]*pending), completed: make(map[key]Refresh), stale: make(map[key]Refresh)}, nil
}

func (c *Coordinator) Start(ctx context.Context, deviceID string, sn, expected int) (Refresh, error) {
	if c == nil || c.store == nil || !validDeviceID(deviceID) || sn <= 0 || expected < 0 || expected > MaxItems {
		return Refresh{}, ErrInvalid
	}
	now := c.now().UTC()
	refresh := Refresh{DeviceID: deviceID, SN: sn, ExpectedCount: expected, Deadline: now.Add(RefreshTimeout)}
	return c.start(ctx, refresh)
}

func (c *Coordinator) start(ctx context.Context, refresh Refresh) (Refresh, error) {
	k := key{device: refresh.DeviceID, sn: refresh.SN}
	c.mu.Lock()
	if p := c.pending[k]; p != nil {
		if p.refresh.ExpectedCount != refresh.ExpectedCount {
			c.mu.Unlock()
			return Refresh{}, ErrInvalid
		}
		got := p.refresh
		c.mu.Unlock()
		return got, nil
	}
	if _, ok := c.stale[k]; ok {
		c.mu.Unlock()
		return Refresh{}, ErrDeadline
	}
	if got, ok := c.completed[k]; ok {
		if got.ExpectedCount != refresh.ExpectedCount {
			c.mu.Unlock()
			return Refresh{}, ErrInvalid
		}
		c.mu.Unlock()
		return got, nil
	}
	c.mu.Unlock()

	callCtx, cancel, err := boundedContext(ctx, refresh.Deadline, c.now())
	if err != nil {
		return Refresh{}, err
	}
	got, err := c.store.StartRefresh(callCtx, refresh)
	cancel()
	if err != nil {
		return Refresh{}, errors.Join(ErrStore, err)
	}
	if got.DeviceID != refresh.DeviceID || got.SN != refresh.SN || got.ExpectedCount != refresh.ExpectedCount || got.Deadline.IsZero() {
		return Refresh{}, ErrStore
	}
	c.mu.Lock()
	if p := c.pending[k]; p != nil {
		got = p.refresh
	} else if done, ok := c.completed[k]; ok {
		got = done
	} else {
		c.pending[k] = &pending{refresh: got, items: make(map[string]Item)}
	}
	c.mu.Unlock()
	return got, nil
}

func (c *Coordinator) AcceptPage(ctx context.Context, page Page) (PageResult, error) {
	if c == nil || c.store == nil || !validPage(page) {
		return PageResult{}, ErrInvalid
	}
	now := c.now().UTC()
	_, err := c.start(ctx, Refresh{DeviceID: page.DeviceID, SN: page.SN, ExpectedCount: page.SumNum, Deadline: now.Add(RefreshTimeout)})
	if err != nil {
		return PageResult{}, err
	}
	k := key{device: page.DeviceID, sn: page.SN}
	c.mu.Lock()
	if done, ok := c.completed[k]; ok {
		c.mu.Unlock()
		return PageResult{Refresh: done, Complete: true, Duplicate: true}, nil
	}
	p := c.pending[k]
	if p == nil {
		c.mu.Unlock()
		return PageResult{}, ErrStore
	}
	if p.staling {
		c.mu.Unlock()
		return PageResult{}, ErrDeadline
	}
	if p.committing {
		got := p.refresh
		c.mu.Unlock()
		return PageResult{Refresh: got, Duplicate: true}, nil
	}
	if !now.Before(p.refresh.Deadline) {
		p.staling = true
		c.mu.Unlock()
		if err := c.markStale(ctx, p.refresh, now); err != nil {
			return PageResult{}, err
		}
		return PageResult{}, ErrDeadline
	}
	duplicate := true
	for _, item := range page.Items {
		old, exists := p.items[item.ChannelID]
		if !exists {
			p.items[item.ChannelID] = item
			duplicate = false
			continue
		}
		if old != item {
			c.mu.Unlock()
			return PageResult{}, ErrDuplicateConflict
		}
	}
	if len(p.items) > p.refresh.ExpectedCount {
		c.mu.Unlock()
		return PageResult{}, ErrInvalid
	}
	if len(p.items) != p.refresh.ExpectedCount {
		got := p.refresh
		c.mu.Unlock()
		return PageResult{Refresh: got, Duplicate: duplicate}, nil
	}
	items := make([]Item, 0, len(p.items))
	for _, item := range p.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ChannelID < items[j].ChannelID })
	got := p.refresh
	p.committing = true
	c.mu.Unlock()

	callCtx, cancel, callErr := boundedContext(ctx, got.Deadline, now)
	if callErr != nil {
		c.mu.Lock()
		if current := c.pending[k]; current == p {
			current.committing = false
		}
		c.mu.Unlock()
		return PageResult{}, callErr
	}
	storeErr := c.store.ReplaceCatalog(callCtx, Snapshot{DeviceID: got.DeviceID, SN: got.SN, Items: items, ObservedAt: now})
	cancel()
	if storeErr != nil {
		c.mu.Lock()
		if current := c.pending[k]; current == p {
			current.committing = false
		}
		c.mu.Unlock()
		return PageResult{}, errors.Join(ErrStore, storeErr)
	}
	c.mu.Lock()
	if current := c.pending[k]; current != p || !current.committing {
		c.mu.Unlock()
		return PageResult{}, ErrStore
	}
	delete(c.pending, k)
	c.completed[k] = got
	c.mu.Unlock()
	return PageResult{Refresh: got, Complete: true, Duplicate: duplicate}, nil
}

func (c *Coordinator) Expire(ctx context.Context, deviceID string, sn int, now time.Time) error {
	if c == nil || c.store == nil || !validDeviceID(deviceID) || sn <= 0 {
		return ErrInvalid
	}
	k := key{device: deviceID, sn: sn}
	c.mu.Lock()
	p := c.pending[k]
	if p == nil {
		c.mu.Unlock()
		return nil
	}
	if now.Before(p.refresh.Deadline) {
		c.mu.Unlock()
		return ErrInvalid
	}
	if p.committing {
		c.mu.Unlock()
		return ErrInvalid
	}
	if p.staling {
		c.mu.Unlock()
		return nil
	}
	p.staling = true
	refresh := p.refresh
	c.mu.Unlock()
	return c.markStale(ctx, refresh, now.UTC())
}

func (c *Coordinator) markStale(ctx context.Context, refresh Refresh, now time.Time) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		c.resetStaling(refresh)
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Second)
	if err := c.store.MarkStale(callCtx, refresh.DeviceID, now); err != nil {
		cancel()
		c.mu.Lock()
		if p := c.pending[key{device: refresh.DeviceID, sn: refresh.SN}]; p != nil {
			p.staling = false
		}
		c.mu.Unlock()
		return errors.Join(ErrStore, err)
	}
	cancel()
	c.mu.Lock()
	k := key{device: refresh.DeviceID, sn: refresh.SN}
	delete(c.pending, k)
	c.stale[k] = refresh
	c.mu.Unlock()
	return nil
}

func (c *Coordinator) resetStaling(refresh Refresh) {
	c.mu.Lock()
	if p := c.pending[key{device: refresh.DeviceID, sn: refresh.SN}]; p != nil {
		p.staling = false
	}
	c.mu.Unlock()
}
