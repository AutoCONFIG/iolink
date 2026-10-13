package core_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestCameraContractCurrentReadDisableAndUnavailableConfiguration(t *testing.T) {
	// Given: existing cameras and production-shaped adapters with Availability absent.
	f := newCameraContractFixture(t)
	for _, tc := range []cameraContractRequest{
		{"GET", "/user/v1/cameras", "/user/v1/cameras?limit=1", "", 200},
		{"GET", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/101", "", 200},
		{"GET", "/api/v1/cameras", "/api/v1/cameras", "", 200},
		{"GET", "/api/v1/cameras/{camera_id}", "/api/v1/cameras/101", "", 200},
		{"POST", "/user/v1/cameras", "/user/v1/cameras", cameraContractBody, 503},
		{"PUT", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/101", cameraContractBody, 503},
	} {
		t.Run(tc.method+tc.template, func(t *testing.T) {
			before := f.snapshot(t)
			token := f.user
			if strings.HasPrefix(tc.path, "/api/") {
				token = f.mini
			}
			// When: invoking the mounted TCP camera route.
			response := f.request(t, tc, token)
			// Then: reads match video OpenAPI, and unavailable configuration changes no camera state.
			if before != f.snapshot(t) {
				t.Fatal("read/unavailable configuration changed state")
			}
			assertCameraContract(t, tc, response)
			if strings.Contains(string(response.raw), "rtsp://") || strings.Contains(string(response.raw), "credential") {
				t.Fatal("camera response exposed source or credentials")
			}
			if tc.path == "/user/v1/cameras?limit=1" {
				var page struct {
					Items []struct{ ID int64 }
					Next  int64 `json:"next_after_id"`
				}
				if err := json.Unmarshal(response.raw, &page); err != nil || len(page.Items) != 1 || page.Items[0].ID != 101 || page.Next != 101 {
					t.Fatal("camera cursor page incorrect")
				}
			}
		})
	}
	t.Run("disable invalidates existing metadata atomically", func(t *testing.T) {
		execute(t, f.pool, `INSERT INTO video_streams(id,camera_id,tenant_id,source_version,state) SELECT '00000000-0000-4000-8000-000000000101',101,id,1,'ready' FROM tenants WHERE name='test-tenant';
 INSERT INTO video_sessions(id,stream_id,camera_id,tenant_id,source_version,user_id,user_token_version,tenant_permission_version,member_permission_version,token_hash,state,created_at,expires_at) SELECT '00000000-0000-4000-8000-000000000102','00000000-0000-4000-8000-000000000101',101,id,1,1,0,0,0,decode(repeat('ab',32),'hex'),'ready',now(),now()+interval '5 minutes' FROM tenants WHERE name='test-tenant'`)
		tc := cameraContractRequest{"DELETE", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/101", "", 204}
		assertCameraContract(t, tc, f.request(t, tc, f.user))
		if count(t, f.pool, `SELECT count(*) FROM video_cameras WHERE id=101 AND NOT enabled`) != 1 || count(t, f.pool, `SELECT count(*) FROM video_sessions WHERE camera_id=101 AND state='revoked'`) != 1 || count(t, f.pool, `SELECT count(*) FROM jobs WHERE kind='video.stop'`) != 1 || count(t, f.pool, `SELECT count(*) FROM audit_events WHERE resource_type='camera' AND action='camera.disabled'`) != 1 {
			t.Fatal("disable did not atomically persist metadata invalidation and intent")
		}
		before := f.snapshot(t)
		assertCameraContract(t, tc, f.request(t, tc, f.user))
		if before != f.snapshot(t) {
			t.Fatal("repeated disable changed persisted state")
		}
		for _, base := range []string{"/user/v1", "/api/v1"} {
			token := f.user
			if base == "/api/v1" {
				token = f.mini
			}
			tc = cameraContractRequest{"GET", base + "/cameras/{camera_id}", base + "/cameras/101", "", 404}
			assertCameraContract(t, tc, f.request(t, tc, token))
		}
	})
}

func TestCameraContractReadRolesAndWriteRejections(t *testing.T) {
	// Given: each existing tenant role, with only one explicit farm assignment.
	f := newCameraContractFixture(t)
	for _, role := range []string{"owner", "admin", "member", "viewer", "support"} {
		t.Run(role, func(t *testing.T) {
			execute(t, f.pool, `UPDATE tenant_memberships SET role=$1,permission_version=permission_version+1,expires_at=CASE WHEN $1='support' THEN now()+interval '1 hour' ELSE NULL END WHERE user_id=9010`, role)
			user, mini := f.login(t, "actor")
			for _, base := range []string{"/user/v1", "/api/v1"} {
				token := user
				if base == "/api/v1" {
					token = mini
				}
				tc := cameraContractRequest{"GET", base + "/cameras", base + "/cameras", "", 200}
				response := f.request(t, tc, token)
				assertCameraContract(t, tc, response)
				var page struct{ Items []struct{ ID int64 } }
				want := 1
				manager := role == "owner" || role == "admin"
				if manager {
					want = 2
				}
				if err := json.Unmarshal(response.raw, &page); err != nil || len(page.Items) != want {
					t.Fatal("camera list escaped role/farm scope")
				}
				status := 404
				if manager {
					status = 200
				}
				for _, resource := range []struct{ id, status int }{{102, status}, {103, 404}, {9999, 404}} {
					tc = cameraContractRequest{"GET", base + "/cameras/{camera_id}", fmt.Sprintf("%s/cameras/%d", base, resource.id), "", resource.status}
					assertCameraContract(t, tc, f.request(t, tc, token))
				}
			}
			if role != "owner" && role != "admin" {
				for _, tc := range []cameraContractRequest{{"POST", "/user/v1/cameras", "/user/v1/cameras", cameraContractBody, 403}, {"PUT", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/101", cameraContractBody, 403}, {"DELETE", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/101", "", 403}} {
					before := f.snapshot(t)
					// When: a limited role requests a camera configuration mutation.
					response := f.request(t, tc, user)
					// Then: the action is forbidden, with no persisted state change.
					if before != f.snapshot(t) {
						t.Fatal("forbidden camera mutation changed state")
					}
					assertCameraContract(t, tc, response)
				}
			}
		})
	}
}

func TestCameraContractRejectsMalformedQueryInputAndForeignWrites(t *testing.T) {
	// Given: valid business/mini tokens and configured camera records.
	f := newCameraContractFixture(t)
	for _, base := range []string{"/user/v1", "/api/v1"} {
		for _, path := range []string{"/cameras?limit=0", "/cameras?limit=101", "/cameras?after_id=0", "/cameras?limit=%zz", "/cameras?limit=1&limit=2", "/cameras?unknown=1", "/cameras/0", "/cameras/invalid", "/cameras/101?limit=1", "/cameras?after_id=9999"} {
			t.Run(base+path, func(t *testing.T) {
				token := f.user
				if base == "/api/v1" {
					token = f.mini
				}
				template := base + "/cameras"
				status := 400
				if strings.HasPrefix(path, "/cameras/") {
					template += "/{camera_id}"
				}
				if path == "/cameras?after_id=9999" {
					status = 200
				}
				tc := cameraContractRequest{"GET", template, base + path, "", status}
				assertCameraContract(t, tc, f.request(t, tc, token))
			})
		}
	}
	for _, body := range []string{"{", cameraContractBody + "{}", strings.Replace(cameraContractBody, `"name":`, `"Name":`, 1), strings.TrimSuffix(cameraContractBody, "}") + `,"extra":true}`, strings.Replace(cameraContractBody, `"name":"Configured"`, `"name":"Configured","name":"duplicate"`, 1), strings.Replace(cameraContractBody, `"uri":"rtsp://192.168.10.99:554/live"`, `"uri":"rtsp://user:password@192.168.10.99/live"`, 1)} {
		t.Run("invalid body", func(t *testing.T) {
			before := f.snapshot(t)
			tc := cameraContractRequest{"POST", "/user/v1/cameras", "/user/v1/cameras", body, 400}
			response := f.request(t, tc, f.user)
			if before != f.snapshot(t) {
				t.Fatal("invalid camera input wrote state")
			}
			assertCameraContract(t, tc, response)
		})
	}
	for _, tc := range []cameraContractRequest{{"PUT", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/103", cameraContractBody, 404}, {"DELETE", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/103", "", 404}, {"POST", "/user/v1/cameras", "/user/v1/cameras", strings.Replace(cameraContractBody, `"pond_id":1`, `"pond_id":9003`, 1), 404}} {
		t.Run("foreign "+tc.method, func(t *testing.T) {
			before := f.snapshot(t)
			response := f.request(t, tc, f.user)
			if before != f.snapshot(t) {
				t.Fatal("foreign camera write changed state")
			}
			assertCameraContract(t, tc, response)
		})
	}
}
