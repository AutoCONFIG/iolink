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
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type baselineAppResponse struct {
	status      int
	contentType string
	raw         []byte
	requestID   string
}

type baselineAppRequest struct {
	method, path, body, token string
}

type baselineAppFixture struct {
	pool                 *pgxpool.Pool
	server               *httptest.Server
	owner, viewer, empty string
	login                baselineAppResponse
	alarmID              int64
}

func baselineAppHTTP(t *testing.T, server *httptest.Server, input baselineAppRequest) baselineAppResponse {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), input.method, server.URL+"/api/v1"+input.path, strings.NewReader(input.body))
	if err != nil {
		t.Fatal("request construction failed")
	}
	req.Header.Set("Content-Type", "application/json")
	if input.token != "" {
		req.Header.Set("Authorization", "Bearer "+input.token)
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal("HTTP transport failed")
	}
	raw, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal("response read or close failed")
	}
	return baselineAppResponse{response.StatusCode, response.Header.Get("Content-Type"), raw, response.Header.Get("X-Request-ID")}
}

func (response baselineAppResponse) contract(t *testing.T, method, route string, want int) {
	t.Helper()
	t.Logf("app contract method=%s route=%s status=%d expected=%d", method, route, response.status, want)
	if response.status != want {
		t.Fatalf("app HTTP status: %s %s got=%d want=%d", method, route, response.status, want)
	}
	assertHTTPContract(t, "app", method, route, response.status, response.contentType, response.raw)
}

func newBaselineAppFixture(t *testing.T) baselineAppFixture {
	t.Helper()
	svc, pool := setup(t)
	execute(t, pool, `INSERT INTO users(id,open_id) VALUES(9001,'contract-empty'),(9002,'contract-viewer'),(9003,'contract-foreign');
 INSERT INTO tenant_memberships(tenant_id,user_id,role) SELECT id,9001,'member' FROM tenants WHERE name='test-tenant';
 INSERT INTO tenant_memberships(tenant_id,user_id,role) SELECT id,9002,'viewer' FROM tenants WHERE name='test-tenant';
 INSERT INTO farm_memberships(tenant_id,farm_id,user_id,role) SELECT id,1,9002,'viewer' FROM tenants WHERE name='test-tenant';
 INSERT INTO tenants(id,name) VALUES(9003,'foreign-contract-tenant');
 INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(9003,9003,'owner');
 INSERT INTO farms(id,owner_id,tenant_id,name) VALUES(9003,9003,9003,'foreign');
 INSERT INTO ponds(id,farm_id,name) VALUES(9003,9003,'foreign');
 INSERT INTO devices(pond_id,device_no,mqtt_username,secret_hash) SELECT 9003,'contract-foreign-device','iolink-contract-foreign',secret_hash FROM devices WHERE device_no='one'`)
	min := 4.0
	if _, err := svc.CreateRule(t.Context(), domain.AlarmRule{PondID: 1, Metric: "dissolved_oxygen", Min: &min, Level: domain.AlarmCritical, Enabled: true}); err != nil {
		t.Fatal("alarm rule fixture failed")
	}
	if err := svc.HandleEvent(sample(time.Now().UTC().Add(-time.Minute), "baseline-app", map[string]float64{"temperature": 25, "dissolved_oxygen": 3})); err != nil {
		t.Fatal("telemetry fixture failed")
	}
	var alarmID int64
	if err := pool.QueryRow(context.Background(), `SELECT id FROM alarms WHERE device_no='one'`).Scan(&alarmID); err != nil {
		t.Fatal("alarm fixture query failed")
	}
	execute(t, pool, `INSERT INTO alarms(id,device_no,pond_id,metric,current_value,threshold,level,message)
 SELECT 9003,'contract-foreign-device',9003,metric,current_value,threshold,level,message FROM alarms WHERE id=$1`, alarmID)
	api := appapi.New(appapi.Config{SecretKey: "isolated-baseline-contract-key", JWT: time.Hour}, appapi.Deps{Users: svc, Ponds: svc.Ponds(), Devices: svc.Devices(), Telemetry: svc.Telemetry(), Alarms: svc.Alarms()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	api.SetWechatExchanger(func(code string) (string, error) {
		switch code {
		case "viewer":
			return "contract-viewer", nil
		case "empty":
			return "contract-empty", nil
		case "owner":
			return "test-user", nil
		default:
			return "contract-unregistered-" + code, nil
		}
	}) // Software-only identity exchange; real WeChat is not exercised.
	server := httptest.NewServer(api.Routes())
	t.Cleanup(server.Close)
	f := baselineAppFixture{pool: pool, server: server, alarmID: alarmID}
	for _, identity := range []struct {
		code  string
		token *string
	}{{"owner", &f.owner}, {"viewer", &f.viewer}, {"empty", &f.empty}} {
		response := baselineAppHTTP(t, server, baselineAppRequest{method: http.MethodPost, path: "/auth/login", body: fmt.Sprintf(`{"code":%q}`, identity.code)})
		var auth struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(response.raw, &auth); err != nil || response.status != 200 || auth.Token == "" {
			t.Fatalf("fixture login failed status=%d", response.status)
		}
		*identity.token = auth.Token
		if identity.code == "owner" {
			f.login = response
		}
	}
	return f
}
