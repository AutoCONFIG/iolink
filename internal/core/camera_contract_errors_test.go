package core_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCameraContractProviderFailureHasSafeCorrelatedDumpAndRollback(t *testing.T) {
	for _, tc := range []struct {
		table   string
		request cameraContractRequest
	}{
		{"video_cameras", cameraContractRequest{"GET", "/user/v1/cameras", "/user/v1/cameras", "", 500}},
		{"video_cameras", cameraContractRequest{"GET", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/101", "", 500}},
		{"video_cameras", cameraContractRequest{"DELETE", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/101", "", 500}},
		{"video_cameras", cameraContractRequest{"GET", "/api/v1/cameras", "/api/v1/cameras", "", 500}},
		{"video_cameras", cameraContractRequest{"GET", "/api/v1/cameras/{camera_id}", "/api/v1/cameras/101", "", 500}},
		{"jobs", cameraContractRequest{"DELETE", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/101", "", 500}},
	} {
		t.Run(tc.table+tc.request.method+tc.request.template, func(t *testing.T) {
			// Given: production camera adapters, a dump sink and a real relation fault.
			f := newCameraContractFixture(t)
			before := f.snapshot(t)
			execute(t, f.pool, "ALTER TABLE "+tc.table+" RENAME TO camera_fault_relation")
			token := f.user
			if strings.HasPrefix(tc.request.path, "/api/") {
				token = f.mini
			}
			// When: the actual TCP operation reaches failed PostgreSQL storage.
			response := f.request(t, tc.request, token)
			execute(t, f.pool, "ALTER TABLE camera_fault_relation RENAME TO "+tc.table)
			f.server.Close()
			if err := f.sink.Close(); err != nil {
				t.Fatal(err)
			}
			// Then: the safe contract error is correlated and no partial mutation remains.
			if before != f.snapshot(t) {
				t.Fatal("camera provider failure left partial state")
			}
			assertCameraContract(t, tc.request, response)
			logs, err := os.ReadFile(f.dumpPath)
			if err != nil {
				t.Fatal("camera dump could not be read")
			}
			for _, private := range []string{f.user, f.mini, "camera_fault_relation", "isolated-camera-password", "private-camera-user", "private-camera-password", "rtsp://"} {
				if strings.Contains(string(logs), private) {
					t.Fatal("camera dump exposed private request/provider data")
				}
			}
			found := false
			for _, line := range strings.Split(strings.TrimSpace(string(logs)), "\n") {
				var record struct {
					Stage, Level, Err string
					RequestID         string `json:"request_id"`
				}
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal("camera dump record malformed")
				}
				if record.Stage == "http_provider" && record.Level == "ERROR" && record.RequestID == response.requestID && record.Err != "" && record.Err != "redacted" {
					found = true
				}
			}
			if !found || response.requestID == "" {
				t.Fatal("camera provider failure had no correlated safe-class diagnostic")
			}
			t.Log("status=500 code=internal_error safe_class=present correlation=matched business_state=unchanged")
		})
	}
}

func TestCameraContractMissingOrExpiredIdentityCannotMutate(t *testing.T) {
	// Given: existing records and both production authentication surfaces.
	f := newCameraContractFixture(t)
	for _, tc := range []cameraContractRequest{
		{"GET", "/user/v1/cameras", "/user/v1/cameras", "", 401},
		{"GET", "/api/v1/cameras", "/api/v1/cameras", "", 401},
		{"POST", "/user/v1/cameras", "/user/v1/cameras", cameraContractBody, 401},
		{"DELETE", "/user/v1/cameras/{camera_id}", "/user/v1/cameras/101", "", 401},
	} {
		t.Run("missing "+tc.template+tc.method, func(t *testing.T) {
			before := f.snapshot(t)
			response := f.request(t, tc, "")
			if before != f.snapshot(t) {
				t.Fatal("unauthenticated camera request mutated state")
			}
			assertCameraContract(t, tc, response)
		})
	}
	user, mini := f.login(t, "actor")
	execute(t, f.pool, `UPDATE tenant_memberships SET expires_at=now()-interval '1 second' WHERE user_id=9010`)
	for _, base := range []string{"/user/v1", "/api/v1"} {
		t.Run("expired "+base, func(t *testing.T) {
			token := user
			if base == "/api/v1" {
				token = mini
			}
			tc := cameraContractRequest{"GET", base + "/cameras", base + "/cameras", "", 401}
			// When: the already-issued token is used after live membership expiry.
			response := f.request(t, tc, token)
			// Then: authentication rejects the token with the camera error shape.
			assertCameraContract(t, tc, response)
		})
	}
	var platform struct{ Token string }
	response := f.request(t, cameraContractRequest{"POST", "", "/admin/v1/login", `{"username":"camera-platform","password":"isolated-camera-password"}`, 200}, "")
	if err := json.Unmarshal(response.raw, &platform); err != nil || response.status != 200 || platform.Token == "" {
		t.Fatal("platform fixture login failed")
	}
	tc := cameraContractRequest{"GET", "/user/v1/cameras", "/user/v1/cameras", "", 403}
	assertCameraContract(t, tc, f.request(t, tc, platform.Token))
}
