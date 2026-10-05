package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
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
