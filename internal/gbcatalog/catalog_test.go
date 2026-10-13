package gbcatalog

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time          { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.mu.Lock(); c.now = c.now.Add(d); c.mu.Unlock() }

type fakeStore struct {
	mu             sync.Mutex
	refreshes      map[key]Refresh
	starts         int
	replacements   []Snapshot
	stales         []string
	startErr       error
	replaceErr     error
	staleErr       error
	deadlineSeen   bool
	replaceEntered chan struct{}
	replaceRelease chan struct{}
}

func (s *fakeStore) StartRefresh(ctx context.Context, refresh Refresh) (Refresh, error) {
	if deadline, ok := ctx.Deadline(); !ok || !deadline.After(time.Now()) {
		return Refresh{}, context.DeadlineExceeded
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deadlineSeen = true
	s.starts++
	if s.startErr != nil {
		return Refresh{}, s.startErr
	}
	if s.refreshes == nil {
		s.refreshes = make(map[key]Refresh)
	}
	k := key{device: refresh.DeviceID, sn: refresh.SN}
	if old, ok := s.refreshes[k]; ok {
		return old, nil
	}
	s.refreshes[k] = refresh
	return refresh, nil
}

func (s *fakeStore) ReplaceCatalog(ctx context.Context, snapshot Snapshot) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("missing deadline")
	}
	if s.replaceEntered != nil {
		close(s.replaceEntered)
		<-s.replaceRelease
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.replaceErr != nil {
		return s.replaceErr
	}
	s.replacements = append(s.replacements, snapshot)
	return nil
}

func (s *fakeStore) MarkStale(ctx context.Context, device string, _ time.Time) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("missing deadline")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.staleErr != nil {
		return s.staleErr
	}
	s.stales = append(s.stales, device)
	return nil
}

func testCoordinator(t *testing.T) (*Coordinator, *fakeStore, *fakeClock) {
	t.Helper()
	clock := &fakeClock{now: time.Now().UTC()}
	store := &fakeStore{}
	c, err := New(store, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	return c, store, clock
}

func item(id, name string, online bool) Item { return Item{ChannelID: id, Name: name, Online: online} }

func TestAcceptPageAggregatesAndAtomicallyReplacesSortedSnapshot(t *testing.T) {
	c, store, _ := testCoordinator(t)
	device := "34020000002000000001"
	if result, err := c.AcceptPage(context.Background(), Page{DeviceID: device, SN: 7, SumNum: 2, Items: []Item{item("34020000001310000102", "b", false)}}); err != nil || result.Complete {
		t.Fatalf("partial result=%#v err=%v", result, err)
	}
	result, err := c.AcceptPage(context.Background(), Page{DeviceID: device, SN: 7, SumNum: 2, Items: []Item{item("34020000001310000101", "a", true)}})
	if err != nil || !result.Complete {
		t.Fatalf("complete result=%#v err=%v", result, err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.replacements) != 1 || len(store.replacements[0].Items) != 2 {
		t.Fatalf("replacements=%#v", store.replacements)
	}
	if store.replacements[0].Items[0].ChannelID != "34020000001310000101" {
		t.Fatalf("items=%#v", store.replacements[0].Items)
	}
}

func TestDuplicatePageAndItemAreIdempotentButConflictsFail(t *testing.T) {
	c, store, _ := testCoordinator(t)
	device := "34020000002000000001"
	page := Page{DeviceID: device, SN: 9, SumNum: 2, Items: []Item{item("34020000001310000101", "a", true)}}
	if _, err := c.AcceptPage(context.Background(), page); err != nil {
		t.Fatal(err)
	}
	result, err := c.AcceptPage(context.Background(), page)
	if err != nil || !result.Duplicate || result.Complete {
		t.Fatalf("duplicate=%#v err=%v", result, err)
	}
	if _, err := c.AcceptPage(context.Background(), Page{DeviceID: device, SN: 9, SumNum: 2, Items: []Item{item("34020000001310000101", "changed", true)}}); !errors.Is(err, ErrDuplicateConflict) {
		t.Fatalf("conflict=%v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.replacements) != 0 {
		t.Fatalf("premature replacement=%#v", store.replacements)
	}
}

func TestCompletedRefreshIgnoresLateDuplicateAndDoesNotCreateNewSnapshot(t *testing.T) {
	c, store, _ := testCoordinator(t)
	device := "34020000002000000001"
	page := Page{DeviceID: device, SN: 10, SumNum: 1, Items: []Item{item("34020000001310000101", "a", true)}}
	if result, err := c.AcceptPage(context.Background(), page); err != nil || !result.Complete {
		t.Fatalf("first=%#v err=%v", result, err)
	}
	result, err := c.AcceptPage(context.Background(), page)
	if err != nil || !result.Complete || !result.Duplicate {
		t.Fatalf("late=%#v err=%v", result, err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.replacements) != 1 || store.starts != 1 {
		t.Fatalf("store=%#v starts=%d", store.replacements, store.starts)
	}
}

func TestCompletedRefreshRejectsDifferentSumNum(t *testing.T) {
	c, store, _ := testCoordinator(t)
	device := "34020000002000000001"
	page := Page{DeviceID: device, SN: 15, SumNum: 1, Items: []Item{item("34020000001310000101", "a", true)}}
	if _, err := c.AcceptPage(context.Background(), page); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(context.Background(), device, 15, 2); !errors.Is(err, ErrInvalid) {
		t.Fatalf("start with changed expected count=%v", err)
	}
	if _, err := c.AcceptPage(context.Background(), Page{DeviceID: device, SN: 15, SumNum: 2, Items: []Item{page.Items[0]}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("page with changed SumNum=%v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.replacements) != 1 || store.starts != 1 {
		t.Fatalf("store=%#v starts=%d", store.replacements, store.starts)
	}
}

func TestExpireMarksStaleWithoutReplacingOldDirectory(t *testing.T) {
	c, store, clock := testCoordinator(t)
	device := "34020000002000000001"
	if _, err := c.AcceptPage(context.Background(), Page{DeviceID: device, SN: 11, SumNum: 2, Items: []Item{item("34020000001310000101", "a", true)}}); err != nil {
		t.Fatal(err)
	}
	clock.Advance(RefreshTimeout + time.Nanosecond)
	if err := c.Expire(context.Background(), device, 11, clock.Now()); err != nil {
		t.Fatalf("expire=%v", err)
	}
	if _, err := c.Start(nil, device, 11, 2); !errors.Is(err, ErrDeadline) {
		t.Fatalf("same stale refresh=%v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.stales) != 1 || len(store.replacements) != 0 {
		t.Fatalf("stale=%#v replacements=%#v", store.stales, store.replacements)
	}
	if _, err := c.AcceptPage(context.Background(), Page{DeviceID: device, SN: 11, SumNum: 2, Items: []Item{item("34020000001310000102", "late", true)}}); !errors.Is(err, ErrDeadline) {
		t.Fatalf("late page=%v", err)
	}
}

func TestCanceledStaleMarkDoesNotPoisonPendingRefresh(t *testing.T) {
	c, store, clock := testCoordinator(t)
	device := "34020000002000000001"
	if _, err := c.AcceptPage(context.Background(), Page{DeviceID: device, SN: 16, SumNum: 2, Items: []Item{item("34020000001310000101", "a", true)}}); err != nil {
		t.Fatal(err)
	}
	clock.Advance(RefreshTimeout)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.AcceptPage(canceled, Page{DeviceID: device, SN: 16, SumNum: 2, Items: []Item{item("34020000001310000102", "b", true)}}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled stale mark=%v", err)
	}
	if err := c.Expire(context.Background(), device, 16, clock.Now()); err != nil {
		t.Fatalf("retry stale mark=%v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.stales) != 1 || len(store.replacements) != 0 {
		t.Fatalf("stales=%#v replacements=%#v", store.stales, store.replacements)
	}
}

func TestConcurrentCompletionReplacesOnce(t *testing.T) {
	c, store, _ := testCoordinator(t)
	page := Page{DeviceID: "34020000002000000001", SN: 12, SumNum: 1, Items: []Item{item("34020000001310000101", "a", true)}}
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = c.AcceptPage(context.Background(), page) }()
	}
	wg.Wait()
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.replacements) != 1 {
		t.Fatalf("replacements=%d", len(store.replacements))
	}
}

func TestReplacePortCallDoesNotHoldCoordinatorLock(t *testing.T) {
	c, store, _ := testCoordinator(t)
	store.replaceEntered = make(chan struct{})
	store.replaceRelease = make(chan struct{})
	page := Page{DeviceID: "34020000002000000001", SN: 13, SumNum: 1, Items: []Item{item("34020000001310000101", "a", true)}}
	done := make(chan error, 1)
	go func() { _, err := c.AcceptPage(context.Background(), page); done <- err }()
	select {
	case <-store.replaceEntered:
	case <-time.After(time.Second):
		t.Fatal("replace did not start")
	}
	startDone := make(chan error, 1)
	go func() { _, err := c.Start(context.Background(), page.DeviceID, 14, 1); startDone <- err }()
	select {
	case err := <-startDone:
		if err != nil {
			t.Fatalf("start while replace=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("coordinator lock held across store call")
	}
	close(store.replaceRelease)
	if err := <-done; err != nil {
		t.Fatalf("replace=%v", err)
	}
}

func TestInvalidPagesAndDeadlineAreRejected(t *testing.T) {
	c, _, clock := testCoordinator(t)
	bad := []Page{
		{DeviceID: "short", SN: 1, SumNum: 0},
		{DeviceID: "34020000002000000001", SN: 0, SumNum: 0},
		{DeviceID: "34020000002000000001", SN: 1, SumNum: 1, Items: []Item{{ChannelID: "3402", Name: "bad"}}},
		{DeviceID: "34020000002000000001", SN: 2, SumNum: 1, Items: []Item{{ChannelID: "34020000001310000101", Name: "", Online: true}}},
		{DeviceID: "34020000002000000001", SN: 3, SumNum: 1},
	}
	for _, page := range bad {
		if _, err := c.AcceptPage(context.Background(), page); !errors.Is(err, ErrInvalid) {
			t.Fatalf("page=%#v err=%v", page, err)
		}
	}
	clock.Advance(RefreshTimeout + time.Second)
	if _, err := c.AcceptPage(context.Background(), Page{DeviceID: "34020000002000000001", SN: 4, SumNum: 0}); err != nil {
		t.Fatalf("empty complete should be valid: %v", err)
	}
}

func TestStartIsIdempotentAndPassesDeadlineToStore(t *testing.T) {
	c, store, _ := testCoordinator(t)
	device := "34020000002000000001"
	first, err := c.Start(context.Background(), device, 20, 3)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Start(context.Background(), device, 20, 3)
	if err != nil || first != second {
		t.Fatalf("first=%#v second=%#v err=%v", first, second, err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.starts != 1 || !store.deadlineSeen {
		t.Fatalf("starts=%d deadline=%v", store.starts, store.deadlineSeen)
	}
}

func TestStoreErrorsDoNotMarkComplete(t *testing.T) {
	c, store, _ := testCoordinator(t)
	store.replaceErr = net.UnknownNetworkError("store unavailable")
	page := Page{DeviceID: "34020000002000000001", SN: 30, SumNum: 1, Items: []Item{item("34020000001310000101", "a", true)}}
	if _, err := c.AcceptPage(context.Background(), page); !errors.Is(err, ErrStore) {
		t.Fatalf("err=%v", err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.replacements) != 0 {
		t.Fatal("store error replaced snapshot")
	}
}
