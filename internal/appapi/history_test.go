package appapi

import (
	"context"
	"encoding/json"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

type capturedHistory struct {
	fakeTelemetry
	max   int
	calls int
}

func (c *capturedHistory) History(_ context.Context, _, _ string, _, _ time.Time, max int) ([]domain.MetricPoint, error) {
	c.max = max
	c.calls++
	return nil, nil
}
func TestHistoryHTTPBoundsAndUnits(t *testing.T) {
	store := &capturedHistory{}
	s := New(Config{}, Deps{Telemetry: store, Devices: fakeDevices{}}, testLogger())
	for _, tt := range []struct {
		query  string
		status int
		max    int
	}{
		{"device_no=one&metric=temperature&max_points=1", 200, 1},
		{"device_no=one&metric=ph&range=30d&max_points=200", 200, 200},
		{"device_no=one&metric=unknown", 400, 0}, {"device_no=one&metric=ph&range=bad", 400, 0},
		{"device_no=one&metric=ph&max_points=0", 400, 0}, {"device_no=one&metric=ph&max_points=201", 400, 0}, {"device_no=one&metric=ph&max_points=x", 400, 0},
	} {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest("GET", "/api/v1/water/history?"+tt.query, nil)
		c.Set("uid", int64(1))
		before := store.calls
		s.waterHistory(c)
		if recorder.Code != tt.status {
			t.Fatal(tt.query, recorder.Code)
		}
		if tt.status == 400 && store.calls != before {
			t.Fatal("invalid query reached storage")
		}
		if tt.status == 200 {
			if store.max != tt.max {
				t.Fatal("max_points not forwarded")
			}
			var body map[string]any
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if _, ok := body["unit"]; !ok {
				t.Fatal("unit missing")
			}
			if points, ok := body["points"].([]any); !ok || len(points) != 0 {
				t.Fatal("empty points not []")
			}
		}
	}
}
func TestHistoryBusinessDay(t *testing.T) {
	now := time.Date(2026, 9, 19, 17, 0, 0, 0, time.UTC)
	from, to, err := historyWindow("today", now)
	if err != nil || !from.Equal(time.Date(2026, 9, 19, 16, 0, 0, 0, time.UTC)) || !to.Equal(now) {
		t.Fatal(from, to, err)
	}
	if _, _, err = historyWindow("invalid", now); err == nil {
		t.Fatal("invalid range accepted")
	}
}
