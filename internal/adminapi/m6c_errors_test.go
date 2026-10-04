package adminapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
)

func TestLicenseImport_returnsContractErrorsWithoutDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"expired", license.ErrExpired, 400, "license_expired"},
		{"not before", license.ErrNotBefore, 400, "license_not_before"},
		{"audit failure", errors.New("database password=private"), 500, "internal_error"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hash := platform.HashPassword("admin123")
			store := &fakeStore{admin: &domain.User{ID: 9, Username: strptr("admin"), PasswordHash: &hash, Authority: "ADMIN"}, importErr: tt.err}
			policy, err := authorization.New()
			if err != nil {
				t.Fatal(err)
			}
			ts := httptest.NewServer(New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{Store: store, Policy: policy}).Routes())
			defer ts.Close()
			token := adminLogin(t, ts)
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/v1/license", strings.NewReader(`{"payload_b64":"YQ==","signature_b64":"Yg=="}`))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			var got map[string]string
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tt.status || got["error"] != tt.code || len(got) != 1 {
				t.Fatalf("status=%d body=%v", resp.StatusCode, got)
			}
		})
	}
}

func TestRegisterDevice_rejectsMalformedBodyBeforeCreatingDevice(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	token := adminLogin(t, ts)
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/admin/v1/devices", strings.NewReader(`{"pond_id":"bad"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var got map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 400 || got["error"] != "invalid_request" {
		t.Fatalf("status=%d body=%v", resp.StatusCode, got)
	}
}
