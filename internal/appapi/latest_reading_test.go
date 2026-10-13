package appapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/observability"
)

type latestResult struct {
	reading domain.Reading
	err     error
}

type latestTelemetry struct {
	fakeTelemetry
	results map[string]latestResult
}

func (f latestTelemetry) Latest(_ context.Context, no string) (domain.Reading, error) {
	r, ok := f.results[no]
	if !ok {
		return domain.Reading{}, domain.ErrNotFound
	}
	return r.reading, r.err
}

type latestDevices struct {
	fakeDevices
	devices []domain.Device
}

func (f latestDevices) ListByPond(context.Context, int64) ([]domain.Device, error) {
	return f.devices, nil
}

func TestLatestHTTPDistinguishesMissingShadowFromProviderFailure(t *testing.T) {
	for _, path := range []string{"/ponds", "/ponds/1", "/devices/dev-001", "/water/latest?device_no=dev-001"} {
		for _, tc := range []struct {
			name string
			err  error
			want int
		}{
			{"present", nil, http.StatusOK},
			{"missing", domain.ErrNotFound, http.StatusOK},
			{"wrapped_missing", fmt.Errorf("shadow: %w", domain.ErrNotFound), http.StatusOK},
			{"unsupported", fmt.Errorf("model: %w", domain.ErrInvalidProductModel), http.StatusOK},
			{"provider_failure", errors.New("postgres://private-user:private-password@private-host/private-database"), http.StatusInternalServerError},
			{"provider_timeout", fmt.Errorf("query: %w", context.DeadlineExceeded), http.StatusInternalServerError},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				// Given: an owned device and an explicit telemetry result.
				want := tc.want
				if tc.name == "unsupported" && strings.HasPrefix(path, "/water/latest") {
					want = http.StatusBadRequest
				}
				timestamp := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
				telemetry := latestTelemetry{results: map[string]latestResult{"dev-001": {reading: domain.Reading{DeviceNo: "dev-001", Timestamp: timestamp}, err: tc.err}}}
				// When: the authenticated client requests its latest reading.
				status, body, logs, requestID := latestHTTP(t, path, telemetry, []domain.Device{{DeviceNo: "dev-001"}})
				// Then: absence stays null, while backend errors fail safely.
				if status != want {
					t.Fatalf("status=%d want=%d body=%s", status, want, body)
				}
				if want == http.StatusInternalServerError {
					if string(body) != `{"error":"internal_error"}` {
						t.Fatalf("unsafe or unexpected failure body=%s", body)
					}
					assertLatestFailureLog(t, logs, requestID)
					return
				}
				if want == http.StatusBadRequest {
					if string(body) != `{"error":"unsupported_product"}` {
						t.Fatalf("unsupported body=%s", body)
					}
					return
				}
				var decoded any
				if err := json.Unmarshal(body, &decoded); err != nil {
					t.Fatal(err)
				}
				if items, ok := decoded.([]any); ok {
					decoded = items[0]
				}
				if item, ok := decoded.(map[string]any); ok {
					if !strings.HasPrefix(path, "/water/latest") {
						decoded = item["latest"]
					}
				}
				if tc.name == "present" {
					item, ok := decoded.(map[string]any)
					if !ok || item["device_no"] != "dev-001" || item["ts"] != timestamp.Format(time.RFC3339) {
						t.Fatalf("present latest=%v", decoded)
					}
					return
				}
				if decoded != nil {
					t.Fatalf("missing or unsupported latest=%v", decoded)
				}
			})
		}
	}
}

func TestPondLatestReturnsNullWhenPondHasNoDevices(t *testing.T) {
	for _, path := range []string{"/ponds", "/ponds/1"} {
		t.Run(path, func(t *testing.T) {
			// Given: an owned pond with no devices.
			telemetry := latestTelemetry{}
			// When: the client requests its pond summary.
			status, body, _, _ := latestHTTP(t, path, telemetry, nil)
			// Then: the empty pond retains a successful null latest reading.
			if status != http.StatusOK || !bytes.Contains(body, []byte(`"latest":null`)) || !bytes.Contains(body, []byte(`"device_count":0`)) {
				t.Fatalf("empty pond status=%d body=%s", status, body)
			}
		})
	}
}

func TestPondLatestSelectsFreshestSupportedShadowAndRejectsPartialOutage(t *testing.T) {
	now := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for _, path := range []string{"/ponds", "/ponds/1"} {
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/outage=%v", path, fail), func(t *testing.T) {
				// Given: mixed products and tied freshest shadows.
				results := map[string]latestResult{
					"empty":   {err: domain.ErrNotFound},
					"generic": {err: domain.ErrInvalidProductModel},
					"older":   {reading: domain.Reading{DeviceNo: "older", Timestamp: now.Add(-time.Hour)}},
					"z":       {reading: domain.Reading{DeviceNo: "z", Timestamp: now}},
					"a":       {reading: domain.Reading{DeviceNo: "a", Timestamp: now}},
				}
				if fail {
					results["a"] = latestResult{err: errors.New("backend unavailable")}
				}
				devices := []domain.Device{{DeviceNo: "empty"}, {DeviceNo: "generic"}, {DeviceNo: "older"}, {DeviceNo: "z"}, {DeviceNo: "a"}}
				// When: the client requests the pond aggregate.
				status, body, _, _ := latestHTTP(t, path, latestTelemetry{results: results}, devices)
				// Then: a partial backend failure cannot return a stale success.
				if fail {
					if status != http.StatusInternalServerError {
						t.Fatalf("outage status=%d body=%s", status, body)
					}
					return
				}
				if status != http.StatusOK {
					t.Fatalf("status=%d body=%s", status, body)
				}
				var item map[string]any
				if path == "/ponds" {
					var items []map[string]any
					if err := json.Unmarshal(body, &items); err != nil || len(items) != 1 {
						t.Fatalf("items=%v err=%v", items, err)
					}
					item = items[0]
				} else if err := json.Unmarshal(body, &item); err != nil {
					t.Fatal(err)
				}
				if item["latest"].(map[string]any)["device_no"] != "a" || item["device_count"] != float64(5) {
					t.Fatalf("aggregate=%v", item)
				}
			})
		}
	}
}

func latestHTTP(t *testing.T, path string, telemetry latestTelemetry, devices []domain.Device) (int, []byte, []byte, string) {
	t.Helper()
	var logs bytes.Buffer
	logger, err := observability.Console(observability.Config{Level: "debug"}, &logs)
	if err != nil {
		t.Fatal(err)
	}
	s := New(Config{SecretKey: "test-key", JWT: time.Hour}, Deps{
		Ponds:   &fakeRepos{ponds: []domain.Pond{{ID: 1, Name: "test-pond"}}},
		Devices: latestDevices{devices: devices}, Telemetry: telemetry, Alarms: &fakeAlarms{}, Users: fakeTenantUsers{},
	}, logger)
	token, err := s.signToken(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Routes())
	defer ts.Close()
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1"+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body, logs.Bytes(), resp.Header.Get("X-Request-ID")
}

func assertLatestFailureLog(t *testing.T, logs []byte, requestID string) {
	t.Helper()
	if strings.Contains(string(logs), "private-") || strings.Contains(string(logs), "dev-001") {
		t.Fatalf("sensitive details in logs: %s", logs)
	}
	for _, line := range bytes.Split(bytes.TrimSpace(logs), []byte{'\n'}) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatal(err)
		}
		if record["stage"] == "telemetry_latest" && record["level"] == "ERROR" {
			if requestID == "" || record["request_id"] != requestID || record["err"] == nil {
				t.Fatalf("uncorrelated failure record=%v", record)
			}
			t.Logf("correlated safe telemetry diagnostic: %s", line)
			return
		}
	}
	t.Fatalf("missing telemetry error diagnostic: %s", logs)
}
