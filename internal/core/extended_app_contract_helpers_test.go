package core_test

import (
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
)

type extendedAppFixture struct {
	telemetryPermissionFixture
	server    *httptest.Server
	token     string
	validBody string
}

type extendedAppRequest struct {
	method, route, path, body string
	status                    int
}

func newExtendedAppFixture(t *testing.T) extendedAppFixture {
	t.Helper()
	f := newTelemetryPermissionFixture(t)
	f.role(t, "admin", true)
	execute(t, f.pool, `INSERT INTO tenant_memberships(tenant_id,user_id,role) VALUES(9402,9402,'viewer')`)
	var productID int64
	if err := f.pool.QueryRow(t.Context(), `INSERT INTO products(tenant_id,name) VALUES(9401,'extended-contract-product') RETURNING id`).Scan(&productID); err != nil {
		t.Fatal("product fixture failed")
	}
	schema := `{"fields":[{"identifier":"temperature","type":"number","unit":"℃","minimum":0,"maximum":50,"readable":true,"writable":false,"nullable":false},{"identifier":"mode","type":"string","enum_values":["auto","manual"],"unit":"","readable":true,"writable":false,"nullable":false},{"identifier":"secret","type":"string","unit":"","readable":false,"writable":false,"nullable":true}]}`
	execute(t, f.pool, `INSERT INTO product_models(product_id,version,schema,published_at) VALUES($1,1,$2::jsonb,now()),($1,2,$2::jsonb,NULL)`, productID, schema)
	execute(t, f.pool, `UPDATE devices SET product_id=$1,model_version=CASE WHEN device_no='v2-hidden' THEN 2 ELSE 1 END WHERE device_no IN ('v2-permission','v2-hidden')`, productID)
	api := appapi.New(appapi.Config{SecretKey: "isolated-extended-app-contract", JWT: time.Hour}, appapi.Deps{Users: f.svc, Ponds: f.svc.Ponds(), Devices: f.svc.Devices(), Telemetry: f.svc.Telemetry(), Alarms: f.svc.Alarms()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	api.SetWechatExchanger(func(string) (string, error) { return "v2-actor", nil })
	server := httptest.NewServer(api.Routes())
	t.Cleanup(server.Close)
	var auth struct{ Token string }
	response := baselineAppHTTP(t, server, baselineAppRequest{method: "POST", path: "/auth/login", body: `{"code":"local-fixture"}`})
	if err := json.Unmarshal(response.raw, &auth); err != nil || response.status != 200 || auth.Token == "" {
		t.Fatal("extended fixture login failed")
	}
	body := fmt.Sprintf(`{"ts":%q,"properties":{"temperature":25,"mode":"auto","secret":"hidden-fixture"}}`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano))
	return extendedAppFixture{f, server, auth.Token, body}
}

func (f extendedAppFixture) request(t *testing.T, tc extendedAppRequest) baselineAppResponse {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), tc.method, f.server.URL+tc.path, strings.NewReader(tc.body))
	if err != nil {
		t.Fatal("extended request construction failed")
	}
	req.Header.Set("Content-Type", "application/json")
	if f.token != "" {
		req.Header.Set("Authorization", "Bearer "+f.token)
	}
	response, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal("extended HTTP transport failed")
	}
	raw, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatal("extended response read or close failed")
	}
	return baselineAppResponse{response.StatusCode, response.Header.Get("Content-Type"), raw, response.Header.Get("X-Request-ID")}
}

func (f extendedAppFixture) checked(t *testing.T, tc extendedAppRequest) baselineAppResponse {
	t.Helper()
	response := f.request(t, tc)
	response.contract(t, tc.method, tc.route, tc.status)
	return response
}
