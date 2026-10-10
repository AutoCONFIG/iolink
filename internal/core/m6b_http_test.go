package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/adminapi"
	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	corepkg "git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/migrate"
	"git.hyhy.fun/rsplab/iolink/internal/platform"
	"git.hyhy.fun/rsplab/iolink/internal/testdb"
)

func TestM6bAdminHTTPAssignedFarmReadsAndBatchConfirmation(t *testing.T) {
	p := testdb.New(t)
	ctx := context.Background()
	if err := migrate.Up(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Exec(ctx, `INSERT INTO users(id,open_id,username,password_hash) VALUES(802,'http-actor','scoped-user',$1)`, platform.HashPassword("M6b-http-password!")); err != nil {
		t.Fatal(err)
	}
	_, err := p.Exec(ctx, `INSERT INTO users(id,open_id) VALUES(801,'http-owner');
		INSERT INTO tenants(id,name) VALUES(801,'http-scope');
		INSERT INTO tenant_memberships(tenant_id,user_id,role,expires_at) VALUES
		(801,801,'owner',NULL),(801,802,'member',now()+interval '1 hour');
		INSERT INTO farms(id,tenant_id,owner_id,name) VALUES(801,801,801,'visible'),(802,801,801,'hidden');
		INSERT INTO ponds(id,farm_id,name) VALUES(801,801,'visible'),(802,802,'hidden');
		INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role) VALUES(801,801,802,'member');
		INSERT INTO devices(pond_id,device_no,secret_hash) VALUES(801,'http-visible','hash'),(802,'http-hidden','hash');
		INSERT INTO alarms(device_no,pond_id,metric,current_value,threshold,level) VALUES
		('http-visible',801,'temperature',30,25,'warning'),('http-hidden',802,'temperature',30,25,'warning')`)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	svc, err := corepkg.NewWithPolicy(ctx, p, slog.New(slog.NewTextHandler(io.Discard, nil)), policy)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(adminapi.New(adminapi.Config{SecretKey: strings.Repeat("s", 32), JWT: time.Hour}, adminapi.Deps{
		Store: svc, Telemetry: svc.Telemetry(), Catalog: svc.Products(), Policy: policy,
	}).Routes())
	defer server.Close()
	var visibleID, hiddenID int64
	if err := p.QueryRow(ctx, `SELECT id FROM alarms WHERE device_no='http-visible'`).Scan(&visibleID); err != nil {
		t.Fatal(err)
	}
	if err := p.QueryRow(ctx, `SELECT id FROM alarms WHERE device_no='http-hidden'`).Scan(&hiddenID); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"member", "viewer", "support"} {
		t.Run(role, func(t *testing.T) {
			if _, err := p.Exec(ctx, `UPDATE tenant_memberships SET role=$1,permission_version=permission_version+1 WHERE user_id=802`, role); err != nil {
				t.Fatal(err)
			}
			if _, err := p.Exec(ctx, `UPDATE alarms SET confirmed_at=NULL`); err != nil {
				t.Fatal(err)
			}
			var login struct{ Token string }
			m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/admin/v1/login", `{"username":"scoped-user","password":"M6b-http-password!"}`, http.StatusOK, &login)
			if login.Token == "" {
				t.Fatal("login returned no token")
			}
			for _, path := range []string{"farms", "ponds", "devices", "alarms"} {
				type resourceRow struct {
					ID       int64  `json:"id"`
					DeviceNo string `json:"device_no"`
				}
				var rows []resourceRow
				if path == "devices" {
					var page struct {
						List     []resourceRow `json:"list"`
						Total    int64         `json:"total"`
						Page     int           `json:"page"`
						PageSize int           `json:"page_size"`
					}
					m6bHTTPRequest(t, server.URL, login.Token, http.MethodGet, "/admin/v1/"+path, "", http.StatusOK, &page)
					if page.Total != 1 || page.Page != 1 || page.PageSize != 20 {
						t.Fatalf("device page included hidden resources or invalid defaults: %+v", page)
					}
					rows = page.List
				} else {
					m6bHTTPRequest(t, server.URL, login.Token, http.MethodGet, "/admin/v1/"+path, "", http.StatusOK, &rows)
				}
				if len(rows) != 1 {
					t.Fatalf("%s returned %d rows, want assigned farm only", path, len(rows))
				}
				if (path == "farms" || path == "ponds") && rows[0].ID != 801 {
					t.Fatalf("%s returned hidden resource %+v", path, rows[0])
				}
				if (path == "devices" || path == "alarms") && rows[0].DeviceNo != "http-visible" {
					t.Fatalf("%s returned hidden device %+v", path, rows[0])
				}
			}
			var stats domain.Stats
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodGet, "/admin/v1/stats", "", http.StatusOK, &stats)
			if stats.DevicesTotal != 1 || stats.Offline != 1 || stats.OpenAlarms != 1 {
				t.Fatalf("statistics included hidden resources: %+v", stats)
			}
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodGet, "/admin/v1/devices/http-hidden", "", http.StatusNotFound, nil)
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodGet, "/admin/v1/devices/http-hidden/product", "", http.StatusNotFound, nil)
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodGet, "/admin/v1/farms/801/members", "", http.StatusForbidden, nil)
			batch := fmt.Sprintf(`{"ids":[%d,%d]}`, visibleID, hiddenID)
			if role == "viewer" {
				m6bHTTPRequest(t, server.URL, login.Token, http.MethodPost, "/admin/v1/alarms/batch-confirm", batch, http.StatusForbidden, nil)
				return
			}
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodPost, "/admin/v1/alarms/batch-confirm", batch, http.StatusNotFound, nil)
			var changed int
			if err := p.QueryRow(ctx, `SELECT count(*) FROM alarms WHERE confirmed_at IS NOT NULL`).Scan(&changed); err != nil || changed != 0 {
				t.Fatalf("rejected batch partially changed alarms: count=%d err=%v", changed, err)
			}
			var result struct{ Confirmed int64 }
			batch = fmt.Sprintf(`{"ids":[%d]}`, visibleID)
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodPost, "/admin/v1/alarms/batch-confirm", batch, http.StatusOK, &result)
			if result.Confirmed != 1 {
				t.Fatalf("confirmed=%d, want 1", result.Confirmed)
			}
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodPost, "/admin/v1/alarms/batch-confirm", batch, http.StatusOK, &result)
			if result.Confirmed != 0 {
				t.Fatalf("duplicate confirmed=%d, want 0", result.Confirmed)
			}
			if _, err := p.Exec(ctx, `UPDATE alarms SET confirmed_at=NULL WHERE id=$1`, visibleID); err != nil {
				t.Fatal(err)
			}
			results := make(chan int64, 2)
			errors := make(chan error, 2)
			var wg sync.WaitGroup
			actorCtx := domain.WithTenantUserID(domain.WithTenantRole(domain.WithTenantID(ctx, 801), role), 802)
			for range 2 {
				wg.Go(func() {
					n, err := svc.BatchConfirmByActor(actorCtx, []int64{visibleID}, 802)
					results <- n
					errors <- err
				})
			}
			wg.Wait()
			close(results)
			close(errors)
			for err := range errors {
				if err != nil {
					t.Fatalf("concurrent duplicate confirmation: %v", err)
				}
			}
			var total int64
			for n := range results {
				total += n
			}
			if total != 1 {
				t.Fatalf("concurrent confirmations changed %d rows, want 1", total)
			}
		})
	}
	for _, role := range []string{"owner", "admin"} {
		t.Run("tenant_manager_without_farm_assignment_"+role, func(t *testing.T) {
			if _, err := p.Exec(ctx, `UPDATE tenant_memberships SET role=$1,permission_version=permission_version+1 WHERE user_id=802`, role); err != nil {
				t.Fatal(err)
			}
			if _, err := p.Exec(ctx, `DELETE FROM farm_memberships WHERE user_id=802; UPDATE alarms SET confirmed_at=NULL`); err != nil {
				t.Fatal(err)
			}
			var login struct{ Token string }
			m6bHTTPRequest(t, server.URL, "", http.MethodPost, "/admin/v1/login", `{"username":"scoped-user","password":"M6b-http-password!"}`, http.StatusOK, &login)
			batch := fmt.Sprintf(`{"ids":[%d,%d]}`, visibleID, hiddenID)
			var result struct{ Confirmed int64 }
			m6bHTTPRequest(t, server.URL, login.Token, http.MethodPost, "/admin/v1/alarms/batch-confirm", batch, http.StatusOK, &result)
			if result.Confirmed != 2 {
				t.Fatalf("tenant manager role=%s confirmed=%d, want 2", role, result.Confirmed)
			}
			var confirmed int
			if err := p.QueryRow(ctx, `SELECT count(*) FROM alarms WHERE id IN ($1,$2) AND confirmed_at IS NOT NULL`, visibleID, hiddenID).Scan(&confirmed); err != nil || confirmed != 2 {
				t.Fatalf("tenant manager role=%s confirmed rows=%d err=%v", role, confirmed, err)
			}
		})
	}
}

func m6bHTTPRequest(t *testing.T, baseURL, token, method, path, body string, want int, output any) {
	t.Helper()
	req, err := http.NewRequest(method, baseURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != want {
		t.Fatalf("%s %s status=%d want=%d", method, path, response.StatusCode, want)
	}
	if output != nil {
		if err := json.NewDecoder(response.Body).Decode(output); err != nil {
			t.Fatal(err)
		}
	}
}
