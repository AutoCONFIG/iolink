package adminapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/golang-jwt/jwt/v5"
)

type cameraBoundaryAPI struct {
	calls int
	err   error
}

func (a *cameraBoundaryAPI) List(context.Context, camera.Page) (camera.List, error) {
	a.calls++
	return camera.List{Items: []camera.Camera{cameraBoundaryDTO()}}, a.err
}
func (a *cameraBoundaryAPI) Get(context.Context, int64) (camera.Camera, error) {
	a.calls++
	return cameraBoundaryDTO(), a.err
}
func (a *cameraBoundaryAPI) Create(context.Context, camera.Configuration) (camera.Camera, error) {
	a.calls++
	return cameraBoundaryDTO(), a.err
}
func (a *cameraBoundaryAPI) Replace(context.Context, int64, camera.Configuration) (camera.Camera, error) {
	a.calls++
	return cameraBoundaryDTO(), a.err
}
func (a *cameraBoundaryAPI) Disable(context.Context, int64) error { a.calls++; return a.err }
func cameraBoundaryDTO() camera.Camera {
	return camera.Camera{ID: 11, PondID: 12, Name: "camera", SourceKind: domain.VideoRTSP, SourceVersion: 1, Status: "offline"}
}
func cameraBoundaryServer(t *testing.T, authority string, api camera.API) (*Server, string) {
	t.Helper()
	store := &fakeStore{admin: &domain.User{ID: 9, Authority: authority}, tenantRole: "owner"}
	server := New(Config{SecretKey: "test-key"}, Deps{Store: store, Cameras: api})
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"aid": 9, "ver": 0, "tenant_id": 7, "tenant_ver": 0, "tenant_role": "owner", "exp": time.Now().Add(time.Hour).Unix()}).SignedString(platform.DeriveAdminKey("test-key"))
	if err != nil {
		t.Fatal(err)
	}
	return server, token
}
func cameraBoundaryRequest(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

const validCameraBody = `{"name":"camera","pond_id":12,"source":{"kind":"rtsp","uri":"rtsp://192.168.10.20:554/live"}}`

func TestCameraHTTPRejectsInvalidBodiesBeforeApplication(t *testing.T) {
	cases := []struct{ name, body string }{
		{"case_field", strings.Replace(validCameraBody, `"name"`, `"NAME"`, 1)},
		{"duplicate_field", strings.Replace(validCameraBody, `"name"`, `"name":"first","name"`, 1)},
		{"null", "null"}, {"array", "[]"}, {"empty", ""}, {"trailing", validCameraBody + "{}"},
		{"unknown_root", strings.Replace(validCameraBody, `"name"`, `"tenant_id":7,"name"`, 1)},
		{"farm_forgery", strings.Replace(validCameraBody, `"name"`, `"farm_id":7,"name"`, 1)},
		{"missing_source", `{"name":"camera","pond_id":12}`}, {"null_source", `{"name":"camera","pond_id":12,"source":null}`},
		{"unknown_source", strings.Replace(validCameraBody, `"kind"`, `"other":1,"kind"`, 1)},
		{"wrong_kind_field", strings.Replace(validCameraBody, `"kind"`, `"device_id":0,"kind"`, 1)},
		{"null_credentials", strings.Replace(validCameraBody, `"kind"`, `"credentials":null,"kind"`, 1)},
		{"missing_credential_password", strings.Replace(validCameraBody, `"kind"`, `"credentials":{"username":"user"},"kind"`, 1)},
		{"unknown_credential", strings.Replace(validCameraBody, `"kind"`, `"credentials":{"username":"user","password":"pass","other":1},"kind"`, 1)},
		{"long_credential", strings.Replace(validCameraBody, `"kind"`, fmt.Sprintf(`"credentials":{"username":"user","password":%q},"kind"`, strings.Repeat("界", 129)), 1)},
		{"long_name", strings.Replace(validCameraBody, `"camera"`, fmt.Sprintf("%q", strings.Repeat("界", 129)), 1)},
		{"userinfo", strings.Replace(validCameraBody, "rtsp://", "rtsp://secret@", 1)},
		{"query", strings.Replace(validCameraBody, "/live", "/live?token=secret", 1)},
		{"traversal", strings.Replace(validCameraBody, "/live", "/%2e%2e/live", 1)},
		{"long_uri", strings.Replace(validCameraBody, "/live", "/"+strings.Repeat("a", 2048), 1)},
		{"wrong_id", strings.Replace(validCameraBody, `"pond_id":12`, `"pond_id":"12"`, 1)},
		{"oversized", strings.Repeat(" ", 16<<10) + validCameraBody},
		{"gb_wrong_field", `{"name":"camera","pond_id":12,"source":{"kind":"gb28181","device_id":1,"channel_id":"34020000001320000001","uri":""}}`},
		{"gb_invalid_channel", `{"name":"camera","pond_id":12,"source":{"kind":"gb28181","device_id":1,"channel_id":"3402000000132000000x"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			api := &cameraBoundaryAPI{}
			server, token := cameraBoundaryServer(t, "USER", api)
			// When
			response := cameraBoundaryRequest(server.UserRoutes(), http.MethodPost, "/user/v1/cameras", tc.body, token)
			// Then
			if response.Code != 400 || api.calls != 0 {
				t.Fatalf("status=%d application_calls=%d", response.Code, api.calls)
			}
			var failure map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil || len(failure) != 2 || failure["code"] != "invalid_request" {
				t.Fatalf("unsafe error shape: %s", response.Body)
			}
			t.Logf("scenario=%s status=%d application_calls=%d safe_error=true", tc.name, response.Code, api.calls)
		})
	}
}

func TestCameraHTTPAcceptsCompleteSourceVariants(t *testing.T) {
	for _, body := range []string{validCameraBody, strings.Replace(validCameraBody, `"kind"`, fmt.Sprintf(`"credentials":{"username":%q,"password":%q},"kind"`, strings.Repeat("界", 128), strings.Repeat("😀", 128)), 1), `{"name":"camera","pond_id":12,"source":{"kind":"gb28181","device_id":1,"channel_id":"34020000001320000001"}}`} {
		t.Run(fmt.Sprintf("variant_%d", len(body)), func(t *testing.T) {
			// Given
			api := &cameraBoundaryAPI{}
			server, token := cameraBoundaryServer(t, "USER", api)
			// When
			response := cameraBoundaryRequest(server.UserRoutes(), http.MethodPost, "/user/v1/cameras", body, token)
			// Then
			if response.Code != 201 || api.calls != 1 {
				t.Fatalf("status=%d calls=%d", response.Code, api.calls)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil || len(got) != 6 {
				t.Fatalf("unsafe camera shape: %s", response.Body)
			}
			t.Logf("status=%d redacted_fields=%d application_calls=%d", response.Code, len(got), api.calls)
		})
	}
}

func TestCameraHTTPBodySizeBoundary(t *testing.T) {
	// Given: a valid JSON object padded to exactly the 16KiB limit.
	api := &cameraBoundaryAPI{}
	server, token := cameraBoundaryServer(t, "USER", api)
	body := strings.Repeat(" ", (16<<10)-len(validCameraBody)) + validCameraBody
	// When
	response := cameraBoundaryRequest(server.UserRoutes(), http.MethodPost, "/user/v1/cameras", body, token)
	// Then
	if response.Code != 201 || api.calls != 1 {
		t.Fatalf("status=%d calls=%d", response.Code, api.calls)
	}
	t.Logf("body_bytes=%d status=%d", len(body), response.Code)
}

func TestCameraHTTPRejectsNonJSONContentType(t *testing.T) {
	// Given
	api := &cameraBoundaryAPI{}
	server, token := cameraBoundaryServer(t, "USER", api)
	request := httptest.NewRequest(http.MethodPost, "/user/v1/cameras", strings.NewReader(validCameraBody))
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "text/plain")
	recorder := httptest.NewRecorder()
	// When
	server.UserRoutes().ServeHTTP(recorder, request)
	// Then
	if recorder.Code != 400 || api.calls != 0 {
		t.Fatalf("status=%d calls=%d", recorder.Code, api.calls)
	}
	t.Logf("status=%d application_calls=%d", recorder.Code, api.calls)
}
