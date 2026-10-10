package main

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"github.com/golang-jwt/jwt/v5"
)

func TestCameraHTTPRealPGAuthenticationAndEndpointSeparation(t *testing.T) {
	// Given: authenticated production surfaces with an ordinary platform administrator and an unassigned user.
	f := newCameraHTTPFixture(t)
	owner := f.login(t, 1701, false)
	mini := f.login(t, 1701, true)
	unassigned := f.login(t, 1708, false)
	// When: platform ADMIN attempts business login.
	f.request(t, "", http.MethodPost, "/user/v1/login", `{"username":"camera-1707","password":"camera-fixture-password"}`, 403)
	// Then: no ADMIN camera surface exists and signed identities remain separated.
	f.request(t, owner, http.MethodGet, "/admin/v1/cameras", "", 404)
	f.request(t, unassigned, http.MethodGet, "/user/v1/cameras", "", 403)
	f.request(t, owner, http.MethodGet, "/api/v1/cameras", "", 401)
	f.request(t, mini, http.MethodGet, "/user/v1/cameras", "", 401)
	for _, prefix := range []string{"/user/v1", "/api/v1"} {
		f.request(t, "invalid", http.MethodGet, prefix+"/cameras", "", 401)
		claimID := "aid"
		key := platform.DeriveAdminKey(cameraHTTPRoot)
		if prefix == "/api/v1" {
			claimID = "uid"
			key = platform.DeriveAppKey(cameraHTTPRoot)
		}
		for _, expired := range []bool{false, true} {
			claims := jwt.MapClaims{claimID: 1701, "ver": 0, "exp": time.Now().Add(time.Hour).Unix()}
			want := 403
			if expired {
				claims["exp"] = time.Now().Add(-time.Hour).Unix()
				want = 401
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			f.request(t, token, http.MethodGet, prefix+"/cameras", "", want)
		}
	}
	f.request(t, mini, http.MethodPost, "/api/v1/cameras", cameraHTTPConfig, 404)
	f.request(t, owner, http.MethodPost, "/user/v1/cameras/1701/playback", "", 404)
}

func TestCameraHTTPRealPGPlatformAdminMiniCameraForbidden(t *testing.T) {
	f := newCameraHTTPFixture(t)
	token := f.adminMiniToken(t)
	var before int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM video_cameras`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/cameras", "/api/v1/cameras/1701"} {
		raw := f.request(t, token, http.MethodGet, path, "", http.StatusForbidden)
		if string(raw) != `{"code":"forbidden","message":"forbidden"}` {
			t.Fatalf("path=%s response=%s", path, raw)
		}
	}
	var after int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM video_cameras`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("camera count changed from %d to %d", before, after)
	}
}

func TestCameraHTTPRealPGPaginationFiltersBeforeCursor(t *testing.T) {
	// Given: owner and viewer have different camera visibility.
	f := newCameraHTTPFixture(t)
	for _, mini := range []bool{false, true} {
		prefix := "/user/v1"
		if mini {
			prefix = "/api/v1"
		}
		owner := f.login(t, 1701, mini)
		viewer := f.login(t, 1704, mini)
		// When: paginating and requesting malformed or undeclared query values.
		raw := f.request(t, owner, http.MethodGet, prefix+"/cameras?limit=1", "", 200)
		// Then: only visible next camera contributes the continuation cursor.
		if !strings.Contains(string(raw), `"next_after_id":1701`) || strings.Contains(string(raw), `"id":1702`) {
			t.Fatalf("page=%s", raw)
		}
		raw = f.request(t, owner, http.MethodGet, prefix+"/cameras?limit=1&after_id=1701", "", 200)
		if !strings.Contains(string(raw), `"id":1702`) || strings.Contains(string(raw), "next_after_id") {
			t.Fatalf("page=%s", raw)
		}
		raw = f.request(t, viewer, http.MethodGet, prefix+"/cameras?limit=1", "", 200)
		if strings.Contains(string(raw), "next_after_id") {
			t.Fatalf("hidden count leaked: %s", raw)
		}
		for _, query := range []string{"limit=0", "limit=101", "after_id=0", "limit=1&limit=2", "farm_id=1702", "tenant_id=1702", "after_id=invalid"} {
			f.request(t, owner, http.MethodGet, prefix+"/cameras?"+query, "", 400)
		}
		f.request(t, owner, http.MethodGet, prefix+"/cameras/1701?limit=1", "", 400)
	}
}

func TestCameraHTTPRealPGMissingLicenseKeyFailsClosed(t *testing.T) {
	// Given: current video License with absent verification key in composition.
	f := newCameraHTTPFixture(t, false)
	f.installLicense(t, []string{"video"}, false)
	owner := f.login(t, 1701, false)
	// When: owner creates or replaces a source.
	f.request(t, owner, http.MethodPost, "/user/v1/cameras", cameraHTTPConfig, 503)
	f.request(t, owner, http.MethodPut, "/user/v1/cameras/1701", cameraHTTPConfig, 503)
	// Then: metadata reading and logical disable remain available.
	f.request(t, owner, http.MethodGet, "/user/v1/cameras/1701", "", 200)
	f.request(t, owner, http.MethodDelete, "/user/v1/cameras/1701", "", 204)
}

func TestCameraHTTPRealPGResourceHidingPrecedesLicenseClockDenial(t *testing.T) {
	// Given: the current License clock is latched unavailable for configuration.
	f := newCameraHTTPFixture(t)
	f.installLicense(t, []string{"video"}, false)
	owner := f.login(t, 1701, false)
	if _, err := f.pool.Exec(t.Context(), `UPDATE license_clock SET clock_error=true`); err != nil {
		t.Fatal(err)
	}
	// When: a manager configures a permitted or a foreign resource.
	f.request(t, owner, http.MethodPost, "/user/v1/cameras", cameraHTTPConfig, 403)
	foreign := strings.Replace(cameraHTTPConfig, `"pond_id":1701`, `"pond_id":1703`, 1)
	// Then: hidden resources return404 regardless of License state.
	f.request(t, owner, http.MethodPost, "/user/v1/cameras", foreign, 404)
	f.request(t, owner, http.MethodPut, "/user/v1/cameras/1703", cameraHTTPConfig, 404)
}
