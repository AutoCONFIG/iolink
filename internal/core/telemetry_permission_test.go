package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestTelemetryWriteDeniedWhenActorCannotWrite(t *testing.T) {
	for _, tc := range []struct {
		name, role, claim string
		actor, user       int64
		active            bool
		tenant            int64
		want              error
	}{
		{"viewer", "viewer", "viewer", 9402, 9402, true, 9401, domain.ErrForbidden},
		{"member", "member", "member", 9402, 9402, true, 9401, domain.ErrForbidden},
		{"support", "support", "support", 9402, 9402, true, 9401, domain.ErrForbidden},
		{"forged_role", "viewer", "owner", 9402, 9402, true, 9401, domain.ErrForbidden},
		{"unknown_role", "viewer", "unknown", 9402, 9402, true, 9401, domain.ErrForbidden},
		{"forged_actor", "viewer", "owner", 9402, 9401, true, 9401, domain.ErrNotFound},
		{"revoked", "admin", "admin", 9402, 9402, false, 9401, domain.ErrNotFound},
		{"cross_tenant", "owner", "owner", 9402, 9402, true, 9402, domain.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given: live membership, claimed identity, and a complete business-state snapshot.
			f := newTelemetryPermissionFixture(t)
			f.role(t, tc.role, tc.active)
			ctx := domain.WithTenantID(telemetryActor(tc.claim, tc.actor), tc.tenant)
			before := f.snapshot(t)
			// When: the shared repository is called directly.
			_, err := f.repo.SubmitTelemetry(ctx, "v2-permission", tc.user, telemetryTimestamp(), telemetryProperties())
			// Then: a typed rejection and byte-identical persisted state.
			after := f.snapshot(t)
			t.Logf("scenario=%s error=%v before=%s after=%s", tc.name, err, before, after)
			if !errors.Is(err, tc.want) {
				t.Errorf("error=%v want=%v", err, tc.want)
			}
			if after != before {
				t.Errorf("rejected write mutated business state")
			}
		})
	}
}

func TestTelemetryWriteAcceptedWhenLiveOwnerOrAdmin(t *testing.T) {
	for _, role := range []string{"owner", "admin"} {
		t.Run(role, func(t *testing.T) {
			// Given: assigned actor with a live role allowed to write.
			f := newTelemetryPermissionFixture(t)
			f.role(t, role, true)
			before := f.snapshot(t)
			// When: submitting valid water telemetry at the repository boundary.
			result, err := f.repo.SubmitTelemetry(telemetryActor(role, 9402), "v2-permission", 9402, telemetryTimestamp(), telemetryProperties())
			// Then: telemetry, water projection, shadow, alarm and outbox are committed.
			after := f.snapshot(t)
			t.Logf("scenario=%s result=%+v error=%v before=%s after=%s", role, result, err, before, after)
			if err != nil || len(result.Accepted) != 1 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			var telemetry, water, shadow, alarm, outbox int
			err = f.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM telemetry),(SELECT count(*) FROM sensor_data),(SELECT count(*) FROM device_shadows),(SELECT count(*) FROM alarms),(SELECT count(*) FROM notification_outbox)`).Scan(&telemetry, &water, &shadow, &alarm, &outbox)
			if err != nil || telemetry != 1 || water != 1 || shadow != 1 || alarm != 1 || outbox != 1 {
				t.Fatalf("committed counts telemetry=%d water=%d shadow=%d alarm=%d outbox=%d err=%v", telemetry, water, shadow, alarm, outbox, err)
			}
		})
	}
}

func TestTelemetryWriteDeniedWhenMembershipChangesAfterContextCreation(t *testing.T) {
	for _, change := range []string{"role", "expired", "tenant_inactive"} {
		t.Run(change, func(t *testing.T) {
			// Given: an old authorized context followed by a live permission change.
			f := newTelemetryPermissionFixture(t)
			f.role(t, "admin", true)
			ctx := telemetryActor("admin", 9402)
			query := map[string]string{
				"role":            `UPDATE tenant_memberships SET role='viewer' WHERE tenant_id=9401 AND user_id=9402`,
				"expired":         `UPDATE tenant_memberships SET expires_at=now()-interval '1 hour' WHERE tenant_id=9401 AND user_id=9402`,
				"tenant_inactive": `UPDATE tenants SET active=false WHERE id=9401`,
			}[change]
			if _, err := f.pool.Exec(context.Background(), query); err != nil {
				t.Fatal(err)
			}
			before := f.snapshot(t)
			// When: reusing the old context.
			_, err := f.repo.SubmitTelemetry(ctx, "v2-permission", 9402, telemetryTimestamp(), telemetryProperties())
			// Then: live state prevents the write and leaves no partial mutations.
			after := f.snapshot(t)
			t.Logf("scenario=%s error=%v before=%s after=%s", change, err, before, after)
			if !errors.Is(err, domain.ErrForbidden) && !errors.Is(err, domain.ErrNotFound) {
				t.Errorf("error=%v", err)
			}
			if before != after {
				t.Error("rejected request mutated business state")
			}
		})
	}
}

func TestTelemetryWriteDeniedWhenPolicyUnavailable(t *testing.T) {
	// Given: storage configured without its required policy capability.
	f := newTelemetryPermissionFixture(t)
	svc, err := corepkg.New(context.Background(), f.pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	before := f.snapshot(t)
	// When: a visible owner invokes the shared write boundary.
	_, err = svc.Telemetry().(domain.GenericTelemetryRepo).SubmitTelemetry(telemetryActor("owner", 9401), "v2-permission", 9401, time.Now(), telemetryProperties())
	// Then: the missing capability fails closed.
	after := f.snapshot(t)
	t.Logf("scenario=missing_policy error=%v before=%s after=%s", err, before, after)
	if !errors.Is(err, domain.ErrForbidden) || before != after {
		t.Fatalf("error=%v state_changed=%t", err, before != after)
	}
}

func TestTelemetryWriteDeniedWhenActorContextMissing(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{"unscoped", context.Background()},
		{"missing_actor", domain.WithTenantRole(domain.WithTenantID(context.Background(), 9401), "owner")},
		{"missing_role", domain.WithTenantUserID(domain.WithTenantID(context.Background(), 9401), 9401)},
		{"missing_tenant", domain.WithTenantUserID(domain.WithTenantRole(context.Background(), "owner"), 9401)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given: a visible owner without complete authenticated actor context.
			f := newTelemetryPermissionFixture(t)
			before := f.snapshot(t)
			// When: calling the write operation directly.
			_, err := f.repo.SubmitTelemetry(tc.ctx, "v2-permission", 9401, telemetryTimestamp(), telemetryProperties())
			// Then: fail closed without persistent changes.
			after := f.snapshot(t)
			t.Logf("scenario=%s error=%v before=%s after=%s", tc.name, err, before, after)
			if !errors.Is(err, domain.ErrForbidden) || before != after {
				t.Fatalf("error=%v changed=%t", err, before != after)
			}
		})
	}
}
