package core_test

import (
	"context"
	"errors"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestM6bAlarmMutationsRejectPartialTenantContext(t *testing.T) {
	for _, operation := range []string{"singular", "batch"} {
		for _, confirmed := range []bool{false, true} {
			t.Run(operation+map[bool]string{false: "_open", true: "_confirmed"}[confirmed], func(t *testing.T) {
				f := newTelemetryPermissionFixture(t)
				seedAppScopeReadings(t, f)
				var id int64
				if err := f.pool.QueryRow(context.Background(), `SELECT id FROM alarms WHERE device_no='v2-hidden'`).Scan(&id); err != nil {
					t.Fatal(err)
				}
				if confirmed {
					if _, err := f.pool.Exec(context.Background(), `UPDATE alarms SET confirmed_at=now() WHERE id=$1`, id); err != nil {
						t.Fatal(err)
					}
				}
				before := f.snapshot(t)
				for _, ctx := range []context.Context{
					domain.WithTenantID(context.Background(), 9401),
					domain.WithTenantUserID(context.Background(), 9402),
					domain.WithTenantRole(context.Background(), "admin"),
					domain.WithTenantPermissionVersion(context.Background(), 1),
					domain.WithTenantUserID(domain.WithTenantID(context.Background(), 9401), 9402),
				} {
					var err error
					if operation == "singular" {
						err = f.svc.ConfirmAlarm(ctx, id)
					} else {
						_, err = f.svc.BatchConfirm(ctx, []int64{id})
					}
					after := f.snapshot(t)
					t.Logf("operation=%s already_confirmed=%t err=%v unchanged=%t", operation, confirmed, err, before == after)
					if !errors.Is(err, domain.ErrForbidden) || before != after {
						t.Fatalf("partial context authorized alarm: err=%v changed=%t", err, before != after)
					}
				}
			})
		}
	}
}
