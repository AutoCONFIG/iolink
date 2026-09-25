package core_test

import (
	"encoding/json"
	"git.hyhy.fun/rsplab/iolink/internal/appapi"
	"git.hyhy.fun/rsplab/iolink/internal/event"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHistoryHTTPWithRealDatabase(t *testing.T) {
	svc, _ := setup(t)
	for i := range 10 {
		if err := svc.HandleEvent(event.Event{Kind: event.KindProperties, DeviceNo: "one", Ts: time.Now().Add(-time.Duration(i) * time.Minute), Properties: map[string]float64{"temperature": float64(20 + i)}}); err != nil {
			t.Fatal(err)
		}
	}
	api := appapi.New(appapi.Config{SecretKey: "isolated-history-integration-only", JWT: time.Hour}, appapi.Deps{Users: svc, Ponds: svc.Ponds(), Devices: svc.Devices(), Telemetry: svc.Telemetry(), Alarms: svc.Alarms()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	api.SetWechatExchanger(func(string) (string, error) { return "test-user", nil }) // explicit software-only identity provider
	server := httptest.NewServer(api.Routes())
	defer server.Close()
	login, err := http.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(`{"code":"test"}`))
	if err != nil {
		t.Fatal(err)
	}
	var auth struct {
		Token string `json:"token"`
	}
	err = json.NewDecoder(login.Body).Decode(&auth)
	login.Body.Close()
	if err != nil || auth.Token == "" {
		t.Fatal("login", err)
	}
	for _, tt := range []struct {
		query  string
		status int
	}{{"metric=temperature&range=7d&max_points=1", 200}, {"metric=ph&range=30d&max_points=200", 200}, {"metric=bad", 400}, {"metric=temperature&range=invalid", 400}, {"metric=temperature&max_points=201", 400}} {
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/water/history?device_no=one&"+tt.query, nil)
		req.Header.Set("Authorization", "Bearer "+auth.Token)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		var out struct {
			Unit   string `json:"unit"`
			Points []struct {
				Value float64 `json:"value"`
			} `json:"points"`
		}
		err = json.NewDecoder(response.Body).Decode(&out)
		response.Body.Close()
		if err != nil || response.StatusCode != tt.status {
			t.Fatal(tt.query, response.StatusCode, err)
		}
		if tt.status == 200 && strings.Contains(tt.query, "temperature") && (out.Unit != "℃" || len(out.Points) != 1 || out.Points[0].Value != 24.5) {
			t.Fatal("history shape or aggregate", out)
		}
		if tt.status == 200 && strings.Contains(tt.query, "metric=ph") && (out.Points == nil || len(out.Points) != 0 || out.Unit != "") {
			t.Fatal("empty history", out)
		}
	}
}
