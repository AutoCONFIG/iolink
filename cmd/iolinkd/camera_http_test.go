package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestCameraHTTPRealPGScopesByCurrentRoleFarmAndTenant(t *testing.T) {
	// Given: two tenants, two farms in tenant A, one farm grant for each lower role.
	f := newCameraHTTPFixture(t)
	f.installLicense(t, []string{"video"}, false)
	for index, role := range []string{"owner", "admin", "member", "viewer", "support"} {
		for _, mini := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_mini_%t", role, mini), func(t *testing.T) {
				token := f.login(t, 1701+index, mini)
				prefix := "/user/v1"
				if mini {
					prefix = "/api/v1"
				}
				// When: listing and accessing permitted, hidden, foreign and nonexistent camera resources.
				raw := f.request(t, token, http.MethodGet, prefix+"/cameras", "", 200)
				// Then: managers see two cameras; lower roles see exactly their granted farm.
				var list struct{ Items []struct{ ID int64 } }
				if err := json.Unmarshal(raw, &list); err != nil {
					t.Fatal(err)
				}
				manager := index < 2
				wantCount := 1
				if manager {
					wantCount = 2
				}
				if len(list.Items) != wantCount || list.Items[0].ID != 1701 {
					t.Fatalf("items=%+v", list.Items)
				}
				f.request(t, token, http.MethodGet, prefix+"/cameras/1701", "", 200)
				hidden := 404
				if manager {
					hidden = 200
				}
				f.request(t, token, http.MethodGet, prefix+"/cameras/1702", "", hidden)
				for _, id := range []int{1703, 999999} {
					f.request(t, token, http.MethodGet, fmt.Sprintf("%s/cameras/%d", prefix, id), "", 404)
				}
				if !mini {
					want := 403
					if manager {
						want = 503
					}
					f.request(t, token, http.MethodPost, prefix+"/cameras", cameraHTTPConfig, want)
					f.request(t, token, http.MethodPut, prefix+"/cameras/1701", cameraHTTPConfig, want)
					if !manager {
						f.request(t, token, http.MethodDelete, prefix+"/cameras/1701", "", 403)
					}
				}
			})
		}
	}
	// When: the other tenant reads its own camera.
	token := f.login(t, 1706, false)
	// Then: tenant A metadata remains hidden.
	f.request(t, token, http.MethodGet, "/user/v1/cameras/1701", "", 404)
	f.request(t, token, http.MethodGet, "/user/v1/cameras/1703", "", 200)
}

func TestCameraHTTPRealPGRuntimeDisabledAndLicenseIndependentDisable(t *testing.T) {
	// Given: production composition lacks the approved video runtime.
	f := newCameraHTTPFixture(t)
	token := f.login(t, 1701, false)
	// When: applying new configurations with current License states.
	for _, state := range []struct {
		name     string
		features []string
		expired  bool
		want     int
	}{{"missing", nil, false, 403}, {"feature_denied", []string{}, false, 403}, {"expired", []string{"video"}, true, 403}, {"valid_runtime_disabled", []string{"video"}, false, 503}} {
		t.Run(state.name, func(t *testing.T) {
			if state.name != "missing" {
				f.installLicense(t, state.features, state.expired)
			}
			for _, method := range []string{http.MethodPost, http.MethodPut} {
				path := "/user/v1/cameras"
				if method == http.MethodPut {
					path += "/1701"
				}
				f.request(t, token, method, path, cameraHTTPConfig, state.want)
			}
			// Then: denied mutation leaves the original configuration and audit untouched.
			var count, version, audits int
			if err := f.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM video_cameras),source_version,(SELECT count(*) FROM audit_events WHERE action LIKE 'camera.%') FROM video_cameras WHERE id=1701`).Scan(&count, &version, &audits); err != nil {
				t.Fatal(err)
			}
			if count != 3 || version != 1 || audits != 0 {
				t.Fatalf("partial mutation count=%d version=%d audits=%d", count, version, audits)
			}
			t.Logf("state=%s camera_count=%d source_version=%d camera_audits=%d", state.name, count, version, audits)
		})
	}
	// When: the License expires and owner logically disables a camera twice.
	f.installLicense(t, []string{"video"}, true)
	f.request(t, token, http.MethodGet, "/user/v1/cameras/1701", "", 200)
	f.request(t, token, http.MethodDelete, "/user/v1/cameras/1701", "", 204)
	f.request(t, token, http.MethodDelete, "/user/v1/cameras/1701", "", 204)
	// Then: the camera is hidden while its stored history remains.
	f.request(t, token, http.MethodGet, "/user/v1/cameras/1701", "", 404)
	var enabled bool
	var audits int
	if err := f.pool.QueryRow(t.Context(), `SELECT enabled,(SELECT count(*) FROM audit_events WHERE action='camera.disabled') FROM video_cameras WHERE id=1701`).Scan(&enabled, &audits); err != nil {
		t.Fatal(err)
	}
	if enabled || audits != 1 {
		t.Fatalf("enabled=%t audits=%d", enabled, audits)
	}
	t.Logf("logical_disable enabled=%t retained_row=true audit_count=%d", enabled, audits)
}

func TestCameraHTTPRealPGRevocationIsLive(t *testing.T) {
	// Given: viewer JWT issued while its farm grant is active.
	f := newCameraHTTPFixture(t)
	token := f.login(t, 1704, true)
	if _, err := f.pool.Exec(t.Context(), `UPDATE farm_memberships SET active=false WHERE user_id=1704`); err != nil {
		t.Fatal(err)
	}
	// When: the same signed JWT accesses a camera after grant removal.
	raw := f.request(t, token, http.MethodGet, "/api/v1/cameras", "", 200)
	// Then: list is empty and details return indistinguishable 404.
	var list struct{ Items []any }
	if err := json.Unmarshal(raw, &list); err != nil || len(list.Items) != 0 {
		t.Fatalf("list=%s", raw)
	}
	f.request(t, token, http.MethodGet, "/api/v1/cameras/1701", "", 404)
}
