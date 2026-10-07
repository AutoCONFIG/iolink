package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/license"
)

type fakeAuth struct {
	principal domain.OpenPrincipal
	err       error
}

func (f fakeAuth) AuthenticateOpen(context.Context, domain.OpenRequest, time.Time) (domain.OpenPrincipal, error) {
	return f.principal, f.err
}

type fakeResources struct{}

func (fakeResources) ListOpenPonds(context.Context, int64, domain.APIKeyResourceScope) ([]domain.Pond, error) {
	return []domain.Pond{{ID: 7}}, nil
}
func (fakeResources) ListOpenDevices(context.Context, int64, domain.APIKeyResourceScope) ([]domain.Device, error) {
	return nil, nil
}
func (fakeResources) ListOpenAlarms(context.Context, int64, domain.APIKeyResourceScope) ([]domain.Alarm, error) {
	return nil, nil
}

func TestRoutesScopeAndAuth(t *testing.T) {
	s := New(Deps{Auth: fakeAuth{principal: domain.OpenPrincipal{TenantID: 1, Scopes: []string{"ponds:read"}}}, Resources: fakeResources{}})
	ts := httptest.NewServer(s.Routes())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/open/v1/ponds")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	resp.Body.Close()
	resp, err = http.Get(ts.URL + "/open/v1/devices")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("scope status=%d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestRoutesFailClosedWhenOpenAPIFeatureUnavailable(t *testing.T) {
	s := New(Deps{Auth: fakeAuth{err: license.ErrUnavailable}, Resources: fakeResources{}})
	ts := httptest.NewServer(s.Routes())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/open/v1/ponds")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestRoutesRateLimitIncludesRetryAfter(t *testing.T) {
	s := New(Deps{Auth: fakeAuth{err: &core.OpenRateLimitError{RetryAfter: 3}}, Resources: fakeResources{}})
	ts := httptest.NewServer(s.Routes())
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/open/v1/ponds")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || resp.Header.Get("Retry-After") != "3" {
		t.Fatalf("status=%d retry=%q", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
}

func TestRoutesRejectOversizedBody(t *testing.T) {
	s := New(Deps{Auth: fakeAuth{principal: domain.OpenPrincipal{TenantID: 1, Scopes: []string{"ponds:read"}}}, Resources: fakeResources{}})
	for _, size := range []int{4 << 20, (4 << 20) + 1} {
		req := httptest.NewRequest(http.MethodGet, "/open/v1/ponds", strings.NewReader(strings.Repeat("x", size)))
		rec := httptest.NewRecorder()
		s.Routes().ServeHTTP(rec, req)
		want := http.StatusOK
		if size > 4<<20 {
			want = http.StatusRequestEntityTooLarge
		}
		if rec.Code != want {
			t.Fatalf("body bytes=%d status=%d want=%d", size, rec.Code, want)
		}
	}
}
