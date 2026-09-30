package persistence

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"git.hyhy.fun/rsplab/iolink/internal/notifications"
)

type NotificationStore struct{ pool *pgxpool.Pool }

func NewNotificationStore(pool *pgxpool.Pool) *NotificationStore {
	return &NotificationStore{pool: pool}
}

func (s *NotificationStore) Claim(ctx context.Context, now time.Time, lease time.Duration, maxAttempts int) (notifications.Claim, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return notifications.Claim{}, false, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE notification_outbox
 SET status='unknown',last_error='lease expired after maximum attempts; delivery may have occurred',leased_until=NULL,lease_owner=NULL,updated_at=$1
 WHERE status='processing' AND (leased_until IS NULL OR leased_until <= $1) AND attempts >= $2`, now, maxAttempts); err != nil {
		return notifications.Claim{}, false, err
	}
	c := notifications.Claim{LeaseToken: rand.Text()}
	err = tx.QueryRow(ctx, `WITH candidate AS (
 SELECT id FROM notification_outbox
 WHERE attempts < $4 AND (
 (status IN ('pending','retryable') AND next_at <= $1)
 OR (status='processing' AND (leased_until IS NULL OR leased_until <= $1)))
 AND EXISTS (
  SELECT 1 FROM alarms a JOIN ponds p ON p.id=a.pond_id JOIN farms f ON f.id=p.farm_id
  JOIN tenants t ON t.id=f.tenant_id
  WHERE a.id=notification_outbox.alarm_id AND t.active
  AND (f.owner_id IS NULL OR EXISTS (SELECT 1 FROM tenant_memberships tm WHERE tm.tenant_id=f.tenant_id AND tm.user_id=f.owner_id AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>now())))
 )
 ORDER BY next_at,id LIMIT 1 FOR UPDATE SKIP LOCKED)
 UPDATE notification_outbox o SET status='processing',attempts=o.attempts+1,
 leased_until=$2,lease_owner=$3,updated_at=$1
 FROM candidate c WHERE o.id=c.id RETURNING o.id,o.alarm_id,o.attempts`, now, now.Add(lease), c.LeaseToken, maxAttempts).Scan(&c.ID, &c.Delivery.AlarmID, &c.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, false, tx.Commit(ctx)
	}
	if err != nil {
		return c, false, err
	}
	var recipient *string
	err = tx.QueryRow(ctx, `SELECT a.device_no,a.pond_id,a.metric,a.current_value,a.threshold,
 a.level,coalesce(a.message,''),a.created_at,u.open_id
 FROM alarms a JOIN ponds p ON p.id=a.pond_id JOIN farms f ON f.id=p.farm_id
 JOIN tenants t ON t.id=f.tenant_id
 LEFT JOIN users u ON u.id=f.owner_id AND u.authority='USER' AND u.open_id <> '' AND u.open_id <> 'internal-admin' AND u.wechat_subscribed
 AND t.active AND EXISTS(SELECT 1 FROM tenant_memberships tm WHERE tm.tenant_id=f.tenant_id AND tm.user_id=u.id AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>now()))
 WHERE a.id=$1`, c.Delivery.AlarmID).Scan(&c.Delivery.DeviceNo, &c.Delivery.PondID, &c.Delivery.Metric,
		&c.Delivery.CurrentValue, &c.Delivery.Threshold, &c.Delivery.Level, &c.Delivery.Message, &c.Delivery.CreatedAt, &recipient)
	if err != nil {
		return c, false, err
	}
	if recipient != nil {
		c.Delivery.RecipientID = strings.TrimSpace(*recipient)
	}
	return c, true, tx.Commit(ctx)
}

func (s *NotificationStore) Finish(ctx context.Context, c notifications.Claim, result notifications.Result, now time.Time) error {
	ct, err := s.pool.Exec(ctx, `UPDATE notification_outbox
 SET status=$2,next_at=$3,leased_until=NULL,lease_owner=NULL,last_error=NULLIF($4,''),
 recipient_open_id=NULLIF($5,''),updated_at=$6
 WHERE id=$1 AND status='processing' AND lease_owner=$7`,
		c.ID, result.Status, result.NextAt, result.LastError, c.Delivery.RecipientID, now, c.LeaseToken)
	if err != nil {
		return err
	}
	if ct.RowsAffected() != 1 {
		return notifications.ErrLeaseLost
	}
	return nil
}

func (s *NotificationStore) QueueDepth(ctx context.Context) (int, error) {
	var depth int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM notification_outbox WHERE status IN ('pending','retryable','processing') AND next_at <= now()`).Scan(&depth)
	return depth, err
}
