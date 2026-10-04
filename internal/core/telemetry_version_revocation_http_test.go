package core_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

type revokingTelemetry struct {
	domain.TelemetryRepo
	domain.GenericTelemetryRepo
	revoke  func() error
	invoked bool
}

func (r *revokingTelemetry) SubmitTelemetry(ctx context.Context, device string, user int64, ts time.Time, properties map[string]json.RawMessage) (domain.TelemetryV2Result, error) {
	r.invoked = true
	if err := r.revoke(); err != nil {
		return domain.TelemetryV2Result{}, err
	}
	return r.GenericTelemetryRepo.SubmitTelemetry(ctx, device, user, ts, properties)
}

func TestM6bTelemetryRejectsVersionRevocationAfterAuthentication(t *testing.T) {
	f := newTelemetryPermissionFixture(t)
	f.role(t, "admin", true)
	repo := &revokingTelemetry{TelemetryRepo: f.svc.Telemetry(), GenericTelemetryRepo: f.repo, revoke: func() error {
		_, err := f.pool.Exec(context.Background(), `UPDATE tenant_memberships SET permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`)
		return err
	}}
	previous := appapi.WechatExchanger
	appapi.WechatExchanger = func(string) (string, error) { return "v2-actor", nil }
	t.Cleanup(func() { appapi.WechatExchanger = previous })
	server := httptest.NewServer(appapi.New(appapi.Config{SecretKey: strings.Repeat("v", 32), JWT: time.Hour}, appapi.Deps{Users: f.svc, Telemetry: repo}, slog.New(slog.NewTextHandler(io.Discard, nil))).Routes())
	defer server.Close()
	var login struct{ Token string }
	m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/api/v1/auth/login", `{"code":"fixture"}`, 200, &login)
	before := f.snapshot(t)
	status, response := telemetryHTTPPost(t, server.URL, login.Token, `{"ts":"2026-10-01T00:00:00Z","properties":{"temperature":25}}`)
	after := f.snapshot(t)
	t.Logf("revoked_after_authentication=%t status=%d unchanged=%t response=%s", repo.invoked, status, before == after, response)
	if !repo.invoked || status != http.StatusForbidden || before != after {
		t.Fatalf("stale authenticated context committed telemetry: status=%d changed=%t", status, before != after)
	}
}
