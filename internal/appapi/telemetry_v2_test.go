package appapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type fakeGenericTelemetry struct{ result domain.TelemetryV2Result }
type capturingGenericTelemetry struct{ fakeGenericTelemetry }

func (f fakeGenericTelemetry) Latest(context.Context, string) (domain.Reading, error) {
	return domain.Reading{}, nil
}
func (f fakeGenericTelemetry) History(context.Context, string, string, time.Time, time.Time, int) ([]domain.MetricPoint, error) {
	return nil, nil
}
func (f fakeGenericTelemetry) HistoryForUser(context.Context, string, int64, string, time.Time, time.Time, int) ([]domain.MetricPoint, error) {
	return nil, nil
}
func (f fakeGenericTelemetry) SubmitTelemetry(context.Context, string, int64, time.Time, map[string]json.RawMessage) (domain.TelemetryV2Result, error) {
	return f.result, nil
}
func (f fakeGenericTelemetry) HistoryV2ForUser(context.Context, string, int64, string, time.Time, time.Time, int) ([]domain.TelemetryHistoryPoint, string, error) {
	return []domain.TelemetryHistoryPoint{}, "℃", nil
}
func (f fakeGenericTelemetry) ModelLatestForUser(context.Context, string, int64) (domain.DeviceModelLatest, error) {
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	return domain.DeviceModelLatest{DeviceNo: "d1", ProductID: 3, ModelVersion: 2, Timestamp: &now, Fields: []domain.ModelField{{Identifier: "mode", Type: "string", Readable: true}}, Properties: map[string]json.RawMessage{"mode": json.RawMessage(`"auto"`)}}, nil
}

func (f capturingGenericTelemetry) SubmitTelemetry(_ context.Context, _ string, _ int64, _ time.Time, properties map[string]json.RawMessage) (domain.TelemetryV2Result, error) {
	if len(properties) == 0 {
		return domain.TelemetryV2Result{}, nil
	}
	return domain.TelemetryV2Result{DeviceNo: "d1", ModelVersion: 1}, nil
}

func TestSubmitTelemetryV2ReturnsAcceptedResult(t *testing.T) {
	s := New(Config{}, Deps{Telemetry: fakeGenericTelemetry{result: domain.TelemetryV2Result{DeviceNo: "d1", ModelVersion: 2, Accepted: []string{"temperature"}}}}, testLogger())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v2/devices/d1/telemetry", bytesReader(`{"ts":"2026-09-28T00:00:00Z","properties":{"temperature":1}}`))
	c.Params = gin.Params{{Key: "device_no", Value: "d1"}}
	c.Set("uid", int64(1))
	s.submitTelemetryV2(c)
	if recorder.Code != 202 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestTelemetryHistoryV2RejectsBadMetric(t *testing.T) {
	s := New(Config{}, Deps{Telemetry: fakeGenericTelemetry{}}, testLogger())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/v2/devices/d1/history?metric=bad.metric", nil)
	c.Params = gin.Params{{Key: "device_no", Value: "d1"}}
	c.Set("uid", int64(1))
	s.telemetryHistoryV2(c)
	if recorder.Code != 400 {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestSubmitTelemetryV2RejectsEmptyProperties(t *testing.T) {
	s := New(Config{}, Deps{Telemetry: capturingGenericTelemetry{}}, testLogger())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/api/v2/devices/d1/telemetry", bytesReader(`{"ts":"2026-09-28T00:00:00Z","properties":{}}`))
	c.Params = gin.Params{{Key: "device_no", Value: "d1"}}
	c.Set("uid", int64(1))
	s.submitTelemetryV2(c)
	if recorder.Code != 400 {
		t.Fatalf("status=%d", recorder.Code)
	}
}

func TestModelLatestV2ReturnsTypedDynamicDetail(t *testing.T) {
	s := New(Config{}, Deps{Telemetry: fakeGenericTelemetry{}}, testLogger())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/api/v2/devices/d1/model/latest", nil)
	c.Params = gin.Params{{Key: "device_no", Value: "d1"}}
	c.Set("uid", int64(1))
	s.modelLatestV2(c)
	if recorder.Code != 200 {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body domain.DeviceModelLatest
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.ModelVersion != 2 || string(body.Properties["mode"]) != `"auto"` {
		t.Fatalf("body=%s err=%v", recorder.Body.String(), err)
	}
}
