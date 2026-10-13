package core_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestCameraContractAbsentProviderLogsCapabilityWithoutFabricatedError(t *testing.T) {
	for _, path := range []string{"/user/v1/cameras", "/api/v1/cameras"} {
		t.Run(path, func(t *testing.T) {
			// Given: real authentication and storage, with the camera capability absent.
			f := newCameraContractFixtureWithProvider(t, false)
			before := f.snapshot(t)
			token := f.user
			if strings.HasPrefix(path, "/api/") {
				token = f.mini
			}
			tc := cameraContractRequest{"GET", path, path, "", 503}
			// When: an authenticated request reaches the absent capability.
			response := f.request(t, tc, token)
			// Then: failure is closed and the diagnostic reports absence without an invented error.
			assertCameraContract(t, tc, response)
			if before != f.snapshot(t) {
				t.Fatal("absent camera capability mutated state")
			}
			raw, err := os.ReadFile(f.dumpPath)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
				var record map[string]any
				if err := json.Unmarshal([]byte(line), &record); err != nil {
					t.Fatal(err)
				}
				if record["request_id"] == response.requestID && record["stage"] == "http_provider" {
					found = true
					if _, exists := record["err"]; exists {
						t.Error("absent capability diagnostic fabricated an error")
					}
				}
			}
			if !found || response.requestID == "" {
				t.Error("absent camera capability lacks correlated diagnostic")
			}
		})
	}
}
