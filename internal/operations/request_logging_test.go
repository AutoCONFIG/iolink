package operations

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/observability"
	"github.com/gin-gonic/gin"
)

func TestRequestLogsCorrelateWithoutUntrustedContent(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		status     int
		panic      bool
	}{
		{"success", "/devices/raw-secret?token=query-secret", 200, false},
		{"denied", "/devices/raw-secret?token=query-secret", 403, false},
		{"failure", "/devices/raw-secret?token=query-secret", 500, false},
		{"panic", "/devices/raw-secret?token=query-secret", 500, true},
		{"unmatched", "/raw-secret?token=query-secret", 404, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			log, err := observability.Console(observability.Config{Level: "debug"}, &out)
			if err != nil {
				t.Fatal(err)
			}
			router := gin.New()
			router.Use(RequestLogging(log))
			router.POST("/devices/:device_no", func(c *gin.Context) {
				if tc.panic {
					panic("panic-secret")
				}
				c.Status(tc.status)
			})
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader("body-secret"))
			req.Header.Set("Authorization", "Bearer jwt-secret")
			req.Header.Set("X-Request-ID", "spoofed-secret")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status=%d", rec.Code)
			}
			for _, secret := range []string{"raw-secret", "query-secret", "body-secret", "jwt-secret", "spoofed-secret", "panic-secret"} {
				if strings.Contains(out.String(), secret) {
					t.Fatalf("leaked %s", secret)
				}
			}
			lines := strings.Split(strings.TrimSpace(out.String()), "\n")
			var record struct {
				RequestID string `json:"request_id"`
				Route     string `json:"route"`
				Status    int    `json:"status"`
				Level     string `json:"level"`
			}
			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &record); err != nil {
				t.Fatal(err)
			}
			if record.RequestID == "" || record.RequestID != rec.Header().Get("X-Request-ID") || record.Status != tc.status {
				t.Fatalf("missing correlation: %+v", record)
			}
			if tc.status == 500 && record.Level != "ERROR" {
				t.Fatal("failure log level", record.Level)
			}
			if tc.name == "unmatched" && record.Route != "unmatched" {
				t.Fatal("raw unmatched path logged")
			}
		})
	}
}
