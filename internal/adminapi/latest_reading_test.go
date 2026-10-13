package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

type latestStore struct {
	*fakeStore
	devices  []domain.Device
	queryErr error
}

func (s *latestStore) ListPonds(context.Context) ([]domain.Pond, error) {
	return []domain.Pond{{ID: 1, FarmID: 1, Name: "pond"}}, nil
}

func (s *latestStore) ListDevicesPage(_ context.Context, q domain.DeviceListQuery) (domain.DevicePage, error) {
	if s.queryErr != nil {
		return domain.DevicePage{}, s.queryErr
	}
	devices := make([]domain.Device, 0)
	for _, d := range s.devices {
		if q.PondID == 0 || q.PondID == d.PondID {
			devices = append(devices, d)
		}
	}
	end := min(q.Offset+q.Limit, len(devices))
	return domain.DevicePage{Items: devices[min(q.Offset, end):end], Total: len(devices)}, nil
}

type latestTelemetry struct {
	readings map[string]domain.Reading
	err      error
}

func (r latestTelemetry) Latest(_ context.Context, no string) (domain.Reading, error) {
	if r.err != nil {
		return domain.Reading{}, r.err
	}
	if rd, ok := r.readings[no]; ok {
		return rd, nil
	}
	return domain.Reading{}, fmt.Errorf("shadow absent: %w", domain.ErrNotFound)
}

func (latestTelemetry) History(context.Context, string, string, time.Time, time.Time, int) ([]domain.MetricPoint, error) {
	return nil, nil
}

func TestPondLatestIncludesDevicesBeyondFirstPage(t *testing.T) {
	// Given: 201 pond devices, the newest shadow on the final page.
	devices := make([]domain.Device, 201)
	for i := range devices {
		devices[i] = domain.Device{ID: int64(i + 1), PondID: 1, DeviceNo: fmt.Sprintf("device-%03d", i)}
	}
	store := &latestStore{fakeStore: &fakeStore{}, devices: devices}
	telemetry := latestTelemetry{readings: map[string]domain.Reading{
		"device-200": {DeviceNo: "device-200", PondID: 1, Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
	}}
	s := New(Config{}, Deps{Store: store, Telemetry: telemetry})
	r := gin.New()
	r.GET("/ponds", s.listPonds)
	// When: the real handler renders pond status.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ponds", nil))
	// Then: the count and latest include the final page.
	var body []struct {
		DeviceCount int `json:"device_count"`
		Latest      *struct {
			DeviceNo string `json:"device_no"`
		} `json:"latest"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(body) != 1 || body[0].DeviceCount != 201 || body[0].Latest == nil || body[0].Latest.DeviceNo != "device-200" {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestLatestHandlersDistinguishAbsenceFromProviderFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"no shadow", fmt.Errorf("wrapped: %w", domain.ErrNotFound), 200},
		{"generic product", domain.ErrInvalidProductModel, 200},
		{"provider failure", errors.New("sensitive provider failure"), 500},
	} {
		for _, endpoint := range []string{"ponds", "device"} {
			t.Run(tc.name+"/"+endpoint, func(t *testing.T) {
				// Given: an owned device and one classified telemetry outcome.
				d := domain.Device{ID: 1, PondID: 1, DeviceNo: "device"}
				store := &latestStore{fakeStore: &fakeStore{devices: map[string]domain.Device{"device": d}}, devices: []domain.Device{d}}
				s := New(Config{}, Deps{Store: store, Telemetry: latestTelemetry{err: tc.err}})
				r := gin.New()
				r.GET("/ponds", s.listPonds)
				r.GET("/device/:device_no", s.getDevice)
				path := "/ponds"
				if endpoint == "device" {
					path = "/device/device"
				}
				// When: the real handler reads the shadow.
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				// Then: failures have an error response, expected absences are null.
				if w.Code != tc.want {
					t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
				}
				if tc.want == 500 && w.Body.String() != `{"error":"internal_error"}` {
					t.Fatalf("unsafe error body=%s", w.Body.String())
				}
			})
		}
	}
}
