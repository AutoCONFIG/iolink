package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/golang-jwt/jwt/v5"
)

func TestCameraHTTPRejectsInvalidQueryAndPath(t *testing.T) {
	for _, path := range []string{"/user/v1/cameras?limit=0", "/user/v1/cameras?limit=101", "/user/v1/cameras?limit=", "/user/v1/cameras?limit=1&limit=2", "/user/v1/cameras?after_id=0", "/user/v1/cameras?after_id=-1", "/user/v1/cameras?after_id=9223372036854775808", "/user/v1/cameras?tenant_id=7", "/user/v1/cameras?limit=%zz", "/user/v1/cameras/0", "/user/v1/cameras/-1", "/user/v1/cameras/9223372036854775808", "/user/v1/cameras/11?limit=1"} {
		t.Run(path, func(t *testing.T) {
			// Given
			api := &cameraBoundaryAPI{}
			server, token := cameraBoundaryServer(t, "USER", api)
			// When
			response := cameraBoundaryRequest(server.UserRoutes(), http.MethodGet, path, "", token)
			// Then
			if response.Code != 400 || api.calls != 0 {
				t.Fatalf("status=%d calls=%d", response.Code, api.calls)
			}
			t.Logf("status=%d application_calls=%d", response.Code, api.calls)
		})
	}
}
func TestCameraHTTPErrorsAreSafeContractObjects(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{{"invalid", camera.ErrInvalid, 400, "invalid_request"}, {"forbidden", domain.ErrForbidden, 403, "forbidden"}, {"missing", domain.ErrNotFound, 404, "not_found"}, {"conflict", domain.ErrConflict, 409, "conflict"}, {"unavailable", camera.ErrUnavailable, 503, "unavailable"}, {"provider_raw", errors.New("rtsp://secret-user:secret-pass@private-host/live"), 500, "internal_error"}} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			api := &cameraBoundaryAPI{err: tc.err}
			server, token := cameraBoundaryServer(t, "USER", api)
			// When
			response := cameraBoundaryRequest(server.UserRoutes(), http.MethodGet, "/user/v1/cameras/11", "", token)
			// Then
			var got map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil || response.Code != tc.status || len(got) != 2 || got["code"] != tc.code || got["message"] != tc.code {
				t.Fatalf("status=%d response=%s", response.Code, response.Body)
			}
			t.Logf("status=%d safe_code=%s", response.Code, got["code"])
		})
	}
}
func TestCameraHTTPAuthKeepsSurfacesSeparate(t *testing.T) {
	for _, tc := range []struct {
		name, authority, tokenMode, path string
		admin                            bool
		status                           int
	}{{"unknown", "USER", "invalid", "/user/v1/cameras", false, 401}, {"missing_context", "USER", "missing", "/user/v1/cameras", false, 403}, {"expired", "USER", "expired", "/user/v1/cameras", false, 401}, {"platform_actor", "ADMIN", "valid", "/user/v1/cameras", false, 403}, {"admin_routes", "ADMIN", "valid", "/admin/v1/cameras", true, 404}} {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			api := &cameraBoundaryAPI{}
			server, token := cameraBoundaryServer(t, tc.authority, api)
			if tc.tokenMode == "invalid" {
				token = "invalid"
			}
			if tc.tokenMode == "missing" || tc.tokenMode == "expired" {
				claims := jwt.MapClaims{"aid": 9, "ver": 0, "exp": time.Now().Add(time.Hour).Unix()}
				if tc.tokenMode == "expired" {
					claims["exp"] = time.Now().Add(-time.Hour).Unix()
				}
				var err error
				token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(platform.DeriveAdminKey("test-key"))
				if err != nil {
					t.Fatal(err)
				}
			}
			handler := server.UserRoutes()
			if tc.admin {
				handler = server.Routes()
			}
			// When
			response := cameraBoundaryRequest(handler, http.MethodGet, tc.path, "", token)
			// Then
			if response.Code != tc.status || api.calls != 0 {
				t.Fatalf("status=%d calls=%d", response.Code, api.calls)
			}
			if tc.status != 404 {
				var got map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil || len(got) != 2 || got["code"] == "" {
					t.Fatalf("response=%s", response.Body)
				}
			}
			t.Logf("scenario=%s status=%d application_calls=%d", tc.name, response.Code, api.calls)
		})
	}
}

func TestCameraHTTPMutationRoutesReturnContractStatuses(t *testing.T) {
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{{http.MethodPost, "/user/v1/cameras", validCameraBody, 201}, {http.MethodPut, "/user/v1/cameras/11", validCameraBody, 200}, {http.MethodDelete, "/user/v1/cameras/11", "", 204}} {
		t.Run(tc.method, func(t *testing.T) {
			// Given
			api := &cameraBoundaryAPI{}
			server, token := cameraBoundaryServer(t, "USER", api)
			// When
			response := cameraBoundaryRequest(server.UserRoutes(), tc.method, tc.path, tc.body, token)
			// Then
			if response.Code != tc.status || api.calls != 1 {
				t.Fatalf("status=%d calls=%d", response.Code, api.calls)
			}
			if tc.status == 204 && response.Body.Len() != 0 {
				t.Fatal("disable returned a response body")
			}
			t.Logf("method=%s status=%d application_calls=%d", tc.method, response.Code, api.calls)
		})
	}
}
