package core_test

import (
	"context"
	"net/http"
	"testing"
)

func TestAppTelemetryHTTPWhenManagerFarmAssignmentIsRevoked(t *testing.T) {
	for _, role := range []string{"owner", "admin", "member", "viewer", "support"} {
		t.Run(role, func(t *testing.T) {
			// Given: live tenant membership, with all farm assignments revoked.
			f := newTelemetryPermissionFixture(t)
			f.role(t, role, true)
			if _, err := f.pool.Exec(context.Background(), `UPDATE farm_memberships SET active=false WHERE user_id=9402`); err != nil {
				t.Fatal(err)
			}
			server, token := appScopeServer(t, f)
			before := f.snapshot(t)
			// When: posting valid telemetry through the actual app endpoint.
			status, _ := appScopeHTTP(t, server, appScopeRequest{http.MethodPost, "/api/v2/devices/v2-hidden/telemetry", `{"ts":"2026-10-01T00:00:00Z","properties":{"temperature":25}}`, token})
			// Then: managers retain tenant-wide writes, and limited roles leave no partial state.
			after := f.snapshot(t)
			manager := role == "owner" || role == "admin"
			want := 404
			if manager {
				want = 202
			}
			t.Logf("scenario=%s_revoked_farm_assignment status=%d before=%s after=%s", role, status, before, after)
			if status != want || (manager && before == after) || (!manager && before != after) {
				t.Fatalf("status=%d want=%d state_changed=%t", status, want, before != after)
			}
			if manager {
				var telemetry, water, shadow int
				err := f.pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM telemetry WHERE device_no='v2-hidden'),(SELECT count(*) FROM sensor_data WHERE device_no='v2-hidden'),(SELECT count(*) FROM device_shadows WHERE device_no='v2-hidden')`).Scan(&telemetry, &water, &shadow)
				if err != nil || telemetry != 1 || water != 1 || shadow != 1 {
					t.Fatalf("committed telemetry=%d water=%d shadow=%d error=%v", telemetry, water, shadow, err)
				}
			}
		})
	}
}
