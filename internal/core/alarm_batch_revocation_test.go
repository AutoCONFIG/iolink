package core_test

import (
	"context"
	"errors"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestM6bManagerBatchRejectsLiveRevocation(t *testing.T) {
	for _, change := range []string{"role", "membership", "tenant", "platform_authority"} {
		for _, byActor := range []bool{false, true} {
			name := change + "_batch"
			if byActor {
				name += "_by_actor"
			}
			t.Run(name, func(t *testing.T) {
				f := newTelemetryPermissionFixture(t)
				seedAppScopeReadings(t, f)
				f.role(t, "admin", true)
				ctx := telemetryActor("admin", 9402)
				query := map[string]string{
					"role":               `UPDATE tenant_memberships SET role='viewer',permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
					"membership":         `UPDATE tenant_memberships SET active=false,permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`,
					"tenant":             `UPDATE tenants SET active=false WHERE id=9401`,
					"platform_authority": `UPDATE users SET authority='ADMIN' WHERE id=9402`,
				}[change]
				if _, err := f.pool.Exec(context.Background(), query); err != nil {
					t.Fatal(err)
				}
				var id int64
				if err := f.pool.QueryRow(context.Background(), `SELECT id FROM alarms WHERE device_no='v2-hidden'`).Scan(&id); err != nil {
					t.Fatal(err)
				}
				before := f.snapshot(t)
				var n int64
				var err error
				if byActor {
					n, err = f.svc.BatchConfirmByActor(ctx, []int64{id}, 9402)
				} else {
					n, err = f.svc.BatchConfirm(ctx, []int64{id})
				}
				after := f.snapshot(t)
				t.Logf("revocation=%s actor_entry=%t confirmed=%d error=%v unchanged=%t", change, byActor, n, err, before == after)
				if !errors.Is(err, domain.ErrNotFound) || n != 0 || before != after {
					t.Fatalf("revoked manager confirmed alarm: n=%d err=%v changed=%t", n, err, before != after)
				}
			})
		}
	}
}
