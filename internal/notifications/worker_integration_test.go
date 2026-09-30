package notifications_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/notifications"
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

type recordingSender struct {
	mu         sync.Mutex
	deliveries []notifications.Delivery
	err        error
}

type deadlineStore struct{ result notifications.Result }

func (s *deadlineStore) Claim(context.Context, time.Time, time.Duration, int) (notifications.Claim, bool, error) {
	return notifications.Claim{ID: 1, Attempts: 1, LeaseToken: "lease", Delivery: notifications.Delivery{RecipientID: "owner"}}, true, nil
}

func (s *deadlineStore) Finish(_ context.Context, _ notifications.Claim, result notifications.Result, _ time.Time) error {
	s.result = result
	return nil
}

type blockingSender struct{}

func (blockingSender) Send(ctx context.Context, _ notifications.Delivery) error {
	<-ctx.Done()
	return ctx.Err()
}

func (s *recordingSender) Send(_ context.Context, d notifications.Delivery) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveries = append(s.deliveries, d)
	return s.err
}

func (s *recordingSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.deliveries)
}

func setupWorkerDB(t *testing.T, owner *int64, openID string) (*pgxpool.Pool, int64) {
	t.Helper()
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,authority,wechat_subscribed) VALUES(1,$1,'USER',TRUE),(2,'owner-two','USER',TRUE)`, openID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO tenants(id,name) VALUES(2,$1)`, fmt.Sprintf("tenant-%d", time.Now().UnixNano())); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(2,1,'owner'),(2,2,'member')`); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO farms(id,owner_id,tenant_id,name) VALUES(1,$1,2,'farm')`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO ponds(id,farm_id,name) VALUES(1,1,'pond')`); err != nil {
		t.Fatal(err)
	}
	var alarmID int64
	if err := p.QueryRow(ctx, `INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level,message) VALUES('device-1',1,'temperature',31,30,'warning','temperature high') RETURNING id`).Scan(&alarmID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO notification_outbox(alarm_id) VALUES($1)`, alarmID); err != nil {
		t.Fatal(err)
	}
	return p, alarmID
}

func fixedClock() func() time.Time {
	return func() time.Time { return time.Now().UTC().Add(time.Hour) }
}

func worker(t *testing.T, p *pgxpool.Pool, sender notifications.Sender, id string) *notifications.Worker {
	t.Helper()
	w, err := notifications.NewWorker(notifications.Config{Store: persistence.NewNotificationStore(p), Sender: sender, Now: fixedClock(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestWorkerRevalidatesCurrentOwnerBeforeSend(t *testing.T) {
	owner := int64(1)
	p, _ := setupWorkerDB(t, &owner, "owner-one")
	defer p.Close()
	ctx := context.Background()
	if _, err := p.Exec(ctx, `UPDATE farms SET owner_id=2 WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	sender := &recordingSender{}
	w := worker(t, p, sender, "worker-a")
	if ok, err := w.RunOnce(ctx); err != nil || !ok {
		t.Fatalf("run once ok=%v err=%v", ok, err)
	}
	if sender.count() != 1 {
		t.Fatal("sender was not called")
	}
	sender.mu.Lock()
	got := sender.deliveries[0].RecipientID
	sender.mu.Unlock()
	if got != "owner-two" {
		t.Fatalf("recipient=%q", got)
	}
	var status string
	if err := p.QueryRow(ctx, `SELECT status FROM notification_outbox`).Scan(&status); err != nil || status != "sent" {
		t.Fatalf("status=%q err=%v", status, err)
	}
}

func TestWorkerDisablesWithoutCurrentOwnerSubscription(t *testing.T) {
	p, _ := setupWorkerDB(t, nil, "owner-one")
	defer p.Close()
	if _, err := p.Exec(context.Background(), `UPDATE farms SET owner_id=NULL WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	w := worker(t, p, &recordingSender{}, "worker-disabled")
	if ok, err := w.RunOnce(context.Background()); err != nil && !ok {
		t.Fatalf("run once ok=%v err=%v", ok, err)
	}
	var status, reason string
	if err := p.QueryRow(context.Background(), `SELECT status,last_error FROM notification_outbox`).Scan(&status, &reason); err != nil || status != "disabled" || reason == "" {
		t.Fatalf("status=%q reason=%q err=%v", status, reason, err)
	}
}

func TestWorkerDisablesWithoutConsent(t *testing.T) {
	p, _ := setupWorkerDB(t, func() *int64 { v := int64(1); return &v }(), "owner-one")
	defer p.Close()
	if _, err := p.Exec(context.Background(), `UPDATE users SET wechat_subscribed=FALSE WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	w := worker(t, p, &recordingSender{}, "worker-no-consent")
	if ok, err := w.RunOnce(context.Background()); !ok || err == nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	var status string
	if err := p.QueryRow(context.Background(), `SELECT status FROM notification_outbox`).Scan(&status); err != nil || status != "disabled" {
		t.Fatalf("status=%q err=%v", status, err)
	}
}

func TestWorkerDisablesAfterOwnerMembershipRevoked(t *testing.T) {
	owner := int64(1)
	p, _ := setupWorkerDB(t, &owner, "owner-one")
	defer p.Close()
	if _, err := p.Exec(context.Background(), `UPDATE tenant_memberships SET active=FALSE,permission_version=permission_version+1 WHERE tenant_id=2 AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	w := worker(t, p, &recordingSender{}, "worker-revoked-membership")
	if ok, err := w.RunOnce(context.Background()); err != nil || ok {
		t.Fatalf("run once ok=%v err=%v, want deferred claim", ok, err)
	}
	var status string
	if err := p.QueryRow(context.Background(), `SELECT status FROM notification_outbox`).Scan(&status); err != nil || status != "pending" {
		t.Fatalf("status=%q err=%v", status, err)
	}
	if _, err := p.Exec(context.Background(), `UPDATE tenant_memberships SET active=TRUE,permission_version=permission_version+1 WHERE tenant_id=2 AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	if ok, err := w.RunOnce(context.Background()); err != nil || !ok {
		t.Fatalf("restored run once ok=%v err=%v", ok, err)
	}
}

func TestWorkerRetriesWithBoundedBackoffAndFails(t *testing.T) {
	owner := int64(1)
	p, _ := setupWorkerDB(t, &owner, "owner-one")
	defer p.Close()
	sender := &recordingSender{err: errors.New("temporary network failure")}
	w := worker(t, p, sender, "worker-retry")
	ctx := context.Background()
	for attempt := 1; attempt <= 6; attempt++ {
		if _, err := w.RunOnce(ctx); err == nil {
			t.Fatal("failed send reported as success")
		}
		var status string
		if err := p.QueryRow(ctx, `SELECT status FROM notification_outbox`).Scan(&status); err != nil {
			t.Fatal(err)
		}
		want := "retryable"
		if attempt == 6 {
			want = "failed"
		}
		if status != want {
			t.Fatalf("attempt=%d status=%q want=%q", attempt, status, want)
		}
		if attempt < 6 {
			if attempt == 1 {
				var next time.Time
				if err := p.QueryRow(ctx, `SELECT next_at FROM notification_outbox`).Scan(&next); err != nil || !next.After(time.Now().UTC()) {
					t.Fatalf("next_at=%v err=%v", next, err)
				}
			}
			if _, err := p.Exec(ctx, `UPDATE notification_outbox SET next_at=now()`); err != nil {
				t.Fatal(err)
			}
		}
	}
	var attempts int
	if err := p.QueryRow(ctx, `SELECT attempts FROM notification_outbox`).Scan(&attempts); err != nil || attempts != 6 {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
}

func TestWorkerDeadlineRecordsUnknown(t *testing.T) {
	store := &deadlineStore{}
	w, err := notifications.NewWorker(notifications.Config{Store: store, Sender: blockingSender{}, LeaseDuration: time.Second, DeliveryTimeout: time.Millisecond, PollInterval: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := w.RunOnce(context.Background()); !ok || err == nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if store.result.Status != "unknown" {
		t.Fatalf("status=%q", store.result.Status)
	}
}

func TestWorkerReclaimsExpiredProcessingLease(t *testing.T) {
	owner := int64(1)
	p, _ := setupWorkerDB(t, &owner, "owner-one")
	defer p.Close()
	if _, err := p.Exec(context.Background(), `UPDATE notification_outbox SET status='processing',attempts=1,leased_until=now()-interval '1 minute',lease_owner='dead-worker'`); err != nil {
		t.Fatal(err)
	}
	sender := &recordingSender{}
	w := worker(t, p, sender, "new-worker")
	if ok, err := w.RunOnce(context.Background()); err != nil || !ok || sender.count() != 1 {
		t.Fatalf("ok=%v err=%v sends=%d", ok, err, sender.count())
	}
	var status string
	if err := p.QueryRow(context.Background(), `SELECT status FROM notification_outbox`).Scan(&status); err != nil || status != "sent" {
		t.Fatalf("status=%q err=%v", status, err)
	}
}

func TestWorkersClaimOneRowOnly(t *testing.T) {
	owner := int64(1)
	p, _ := setupWorkerDB(t, &owner, "owner-one")
	defer p.Close()
	sender := &recordingSender{}
	w1 := worker(t, p, sender, "worker-one")
	w2 := worker(t, p, sender, "worker-two")
	var wg sync.WaitGroup
	results := make(chan bool, 2)
	for _, w := range []*notifications.Worker{w1, w2} {
		wg.Add(1)
		go func(w *notifications.Worker) {
			defer wg.Done()
			ok, _ := w.RunOnce(context.Background())
			results <- ok
		}(w)
	}
	wg.Wait()
	close(results)
	claimed := 0
	for ok := range results {
		if ok {
			claimed++
		}
	}
	if claimed != 1 || sender.count() != 1 {
		t.Fatalf("claimed=%d sends=%d", claimed, sender.count())
	}
}

func TestWorkerRecordsUnknownResult(t *testing.T) {
	owner := int64(1)
	p, _ := setupWorkerDB(t, &owner, "owner-one")
	defer p.Close()
	sender := &recordingSender{err: notifications.ErrUnknown}
	w := worker(t, p, sender, "worker-unknown")
	if ok, err := w.RunOnce(context.Background()); !ok || err == nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	var status string
	if err := p.QueryRow(context.Background(), `SELECT status FROM notification_outbox`).Scan(&status); err != nil || status != "unknown" {
		t.Fatalf("status=%q err=%v", status, err)
	}
}
