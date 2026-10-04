package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestAppResourceScopeWhenTenantManagerHasNoFarmAssignment(t *testing.T) {
	for _, role := range []string{"owner", "admin"} {
		t.Run(role, func(t *testing.T) {
			// Given: a USER tenant manager who neither owns nor is assigned either farm.
			f := newTelemetryPermissionFixture(t)
			f.role(t, role, true)
			if _, err := f.pool.Exec(context.Background(), `DELETE FROM farm_memberships WHERE user_id=9402`); err != nil {
				t.Fatal(err)
			}
			ctx := telemetryActor(role, 9402)
			// When: the model is read through the public repository boundary.
			latest, err := f.svc.Telemetry().(domain.DeviceModelLatestRepo).ModelLatestForUser(ctx, "v2-hidden", 9402)
			// Then: same-tenant model data is accessible without farm membership.
			t.Logf("scenario=manager_%s_no_farm_assignment latest=%+v error=%v", role, latest, err)
			if err != nil || latest.DeviceNo != "v2-hidden" || len(latest.Fields) == 0 {
				t.Fatalf("latest=%+v error=%v", latest, err)
			}
		})
	}
}

func TestAppTelemetryWriteWhenTenantManagerHasNoFarmAssignment(t *testing.T) {
	for _, role := range []string{"owner", "admin"} {
		t.Run(role, func(t *testing.T) {
			// Given: a USER tenant manager with no farm ownership or assignments.
			f := newTelemetryPermissionFixture(t)
			f.role(t, role, true)
			if _, err := f.pool.Exec(context.Background(), `DELETE FROM farm_memberships WHERE user_id=9402`); err != nil {
				t.Fatal(err)
			}
			before := f.snapshot(t)
			// When: submitting valid telemetry to a different owner's farm in this tenant.
			result, err := f.repo.SubmitTelemetry(telemetryActor(role, 9402), "v2-hidden", 9402, telemetryTimestamp(), telemetryProperties())
			// Then: the authorized telemetry, compatibility projection and shadow commit together.
			after := f.snapshot(t)
			t.Logf("scenario=manager_%s_no_farm_assignment_write result=%+v error=%v before=%s after=%s", role, result, err, before, after)
			if err != nil || len(result.Accepted) != 1 || before == after {
				t.Fatalf("result=%+v error=%v changed=%t", result, err, before != after)
			}
			var telemetry, water, shadow int
			err = f.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM telemetry WHERE device_no='v2-hidden'),(SELECT count(*) FROM sensor_data WHERE device_no='v2-hidden'),(SELECT count(*) FROM device_shadows WHERE device_no='v2-hidden')`).Scan(&telemetry, &water, &shadow)
			if err != nil || telemetry != 1 || water != 1 || shadow != 1 {
				t.Fatalf("committed counts telemetry=%d water=%d shadow=%d error=%v", telemetry, water, shadow, err)
			}
		})
	}
}

func TestAppResourceScopeWhenTenantManagerRoleRevoked(t *testing.T) {
	// Given: an admin context whose live role is reduced to viewer, without assignments.
	f := newTelemetryPermissionFixture(t)
	f.role(t, "admin", true)
	ctx := telemetryActor("admin", 9402)
	if _, err := f.pool.Exec(context.Background(), `DELETE FROM farm_memberships WHERE user_id=9402`); err != nil {
		t.Fatal(err)
	}
	f.role(t, "viewer", true)
	before := f.snapshot(t)
	// When: the stale admin context is used to write to the unassigned farm.
	_, err := f.repo.SubmitTelemetry(ctx, "v2-hidden", 9402, time.Now(), telemetryProperties())
	// Then: no tenant-wide access survives and no partial state is persisted.
	after := f.snapshot(t)
	t.Logf("scenario=manager_role_revoked error=%v before=%s after=%s", err, before, after)
	if !errors.Is(err, domain.ErrNotFound) || before != after {
		t.Fatalf("error=%v state_changed=%t", err, before != after)
	}
}

func TestAppResourceScopeWhenManagerContextIsForged(t *testing.T) {
	for _, change := range []string{"actor", "role", "membership", "platform_authority", "missing_actor"} {
		t.Run(change, func(t *testing.T) {
			// Given: an unassigned tenant admin and a context or live membership contradiction.
			f := newTelemetryPermissionFixture(t)
			f.role(t, "admin", true)
			if _, err := f.pool.Exec(context.Background(), `DELETE FROM farm_memberships WHERE user_id=9402`); err != nil {
				t.Fatal(err)
			}
			ctx := telemetryActor("admin", 9402)
			switch change {
			case "actor":
				ctx = domain.WithTenantUserID(ctx, 9401)
			case "role":
				f.role(t, "viewer", true)
			case "membership":
				f.role(t, "admin", false)
			case "platform_authority":
				if _, err := f.pool.Exec(context.Background(), `UPDATE users SET authority='ADMIN' WHERE id=9402`); err != nil {
					t.Fatal(err)
				}
			case "missing_actor":
				ctx = domain.WithTenantRole(domain.WithTenantID(context.Background(), 9401), "admin")
			}
			// When: claiming tenant-wide access through the core read boundary.
			_, err := f.svc.Telemetry().(domain.DeviceModelLatestRepo).ModelLatestForUser(ctx, "v2-hidden", 9402)
			// Then: the forged or revoked privilege does not expose an unassigned farm.
			t.Logf("scenario=manager_%s error=%v", change, err)
			if !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestAppAlarmConfirmDeniedWhenLiveRoleCannotConfirm(t *testing.T) {
	for _, role := range []string{"viewer", "revoked_admin"} {
		t.Run(role, func(t *testing.T) {
			// Given: an assigned viewer or a stale admin context with a live viewer role.
			f := newTelemetryPermissionFixture(t)
			seedAppScopeReadings(t, f)
			claim := "viewer"
			if role == "revoked_admin" {
				claim = "admin"
			}
			before := f.snapshot(t)
			var id int64
			if err := f.pool.QueryRow(context.Background(), `SELECT id FROM alarms WHERE device_no='v2-permission'`).Scan(&id); err != nil {
				t.Fatal(err)
			}
			// When: bypassing the HTTP guard and invoking alarm confirmation directly.
			err := f.svc.Alarms().ConfirmByUser(telemetryActor(claim, 9402), id, 9402)
			// Then: shared storage rejects the write without changing persisted business state.
			after := f.snapshot(t)
			t.Logf("scenario=confirm_%s error=%v before=%s after=%s", role, err, before, after)
			if !errors.Is(err, domain.ErrNotFound) || before != after {
				t.Fatalf("error=%v state_changed=%t", err, before != after)
			}
		})
	}
}

func TestAppAlarmBatchConfirmAllowsTenantManagerWithoutFarmAssignment(t *testing.T) {
	for _, role := range []string{"owner", "admin"} {
		t.Run(role, func(t *testing.T) {
			// Given: a tenant manager with no farm assignment and alarms on two same-tenant farms.
			f := newTelemetryPermissionFixture(t)
			seedAppScopeReadings(t, f)
			f.role(t, role, true)
			if _, err := f.pool.Exec(context.Background(), `DELETE FROM farm_memberships WHERE user_id=9402`); err != nil {
				t.Fatal(err)
			}
			var ids []int64
			rows, err := f.pool.Query(context.Background(), `SELECT id FROM alarms WHERE device_no IN ('v2-permission','v2-hidden') ORDER BY id`)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					t.Fatal(err)
				}
				ids = append(ids, id)
			}
			rows.Close()
			if len(ids) != 2 {
				t.Fatalf("alarm ids=%v, want two", ids)
			}
			before := f.snapshot(t)
			// When: the core actor-scoped batch operation confirms both alarms.
			confirmed, err := f.svc.BatchConfirmByActor(telemetryActor(role, 9402), ids, 9402)
			// Then: tenant-wide manager scope applies atomically to both same-tenant alarms.
			after := f.snapshot(t)
			t.Logf("scenario=batch_confirm_manager_%s confirmed=%d error=%v before=%s after=%s", role, confirmed, err, before, after)
			if err != nil || confirmed != 2 || before == after {
				t.Fatalf("confirmed=%d error=%v state_changed=%t", confirmed, err, before != after)
			}
		})
	}
}
