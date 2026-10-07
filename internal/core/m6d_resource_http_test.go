package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestM6dResourceHTTP_filtersListsAndDetails(t *testing.T) {
	// Given: two farms in one tenant plus a foreign tenant, served by real HTTP and Timescale.
	f := newM6dHTTPFixture(t)
	for _, scope := range []struct {
		name      string
		resources domain.APIKeyResourceScope
	}{
		{"farm", domain.APIKeyResourceScope{FarmIDs: []int64{8301}}},
		{"pond", domain.APIKeyResourceScope{PondIDs: []int64{8301}}},
		{"device", domain.APIKeyResourceScope{DeviceNos: []string{"m6d-allowed"}}},
	} {
		key, secret := f.issue(t, []string{"ponds:read", "devices:read", "alarms:read"}, scope.resources)
		for _, path := range []string{"/open/v1/ponds", "/open/v1/devices", "/open/v1/alarms"} {
			t.Run(scope.name+path, func(t *testing.T) {
				// When: a limited key reads a resource list.
				status, _, body := m6dSignedHTTP(t, f.server, key, secret, path, m6dNonce(t))
				// Then: only the permitted resource is visible.
				if status != http.StatusOK {
					t.Fatalf("status=%d", status)
				}
				var rows []struct {
					ID       int64  `json:"id"`
					PondID   int64  `json:"pond_id"`
					DeviceNo string `json:"device_no"`
				}
				if err := json.Unmarshal(body, &rows); err != nil {
					t.Fatal(err)
				}
				if len(rows) != 1 {
					t.Fatalf("rows=%d want 1", len(rows))
				}
				if path == "/open/v1/ponds" && rows[0].ID != 8301 {
					t.Fatal("unexpected pond")
				}
				if path != "/open/v1/ponds" && rows[0].DeviceNo != "m6d-allowed" {
					t.Fatal("unexpected device")
				}
			})
		}
		for _, path := range []string{"/open/v1/ponds/8302", "/open/v1/ponds/8303", "/open/v1/devices/m6d-hidden", "/open/v1/devices/m6d-foreign"} {
			t.Run(scope.name+path, func(t *testing.T) {
				status, _, _ := m6dSignedHTTP(t, f.server, key, secret, path, m6dNonce(t))
				if status != http.StatusNotFound {
					t.Fatalf("status=%d want 404", status)
				}
			})
		}
	}
}

func TestM6dDeviceResourceHTTP_filtersDevicesAndAlarms(t *testing.T) {
	f := newM6dHTTPFixture(t)
	key, secret := f.issue(t, []string{"devices:read", "alarms:read"}, domain.APIKeyResourceScope{DeviceNos: []string{"m6d-allowed"}})
	for _, path := range []string{"/open/v1/devices", "/open/v1/alarms"} {
		t.Run(path, func(t *testing.T) {
			status, _, body := m6dSignedHTTP(t, f.server, key, secret, path, m6dNonce(t))
			var rows []struct {
				DeviceNo string `json:"device_no"`
			}
			if err := json.Unmarshal(body, &rows); err != nil {
				t.Fatal(err)
			}
			if status != http.StatusOK || len(rows) != 1 || rows[0].DeviceNo != "m6d-allowed" {
				t.Fatal("device scope not enforced")
			}
		})
	}
}

func TestM6dSignedHTTP_rejectsScopeTenantFeatureAndReplays(t *testing.T) {
	for _, scenario := range []struct {
		name string
		want int
	}{
		{"scope", http.StatusForbidden}, {"inactive tenant", http.StatusUnauthorized},
		{"missing feature", http.StatusForbidden}, {"replay", http.StatusUnauthorized},
		{"exhausted rate", http.StatusTooManyRequests},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Given: a valid signed key with a specific denial condition.
			f := newM6dHTTPFixture(t)
			scopes := []string{"ponds:read"}
			if scenario.name == "scope" {
				scopes = []string{"devices:read"}
			}
			key, secret := f.issue(t, scopes, domain.APIKeyResourceScope{})
			nonce := m6dNonce(t)
			switch scenario.name {
			case "inactive tenant":
				if _, err := f.pool.Exec(t.Context(), `UPDATE tenants SET active=false WHERE id=8301`); err != nil {
					t.Fatal(err)
				}
			case "missing feature":
				installTestLicense(t, f.pool, f.svc, 100)
			case "replay":
				status, _, _ := m6dSignedHTTP(t, f.server, key, secret, "/open/v1/ponds", nonce)
				if status != http.StatusOK {
					t.Fatalf("first request=%d", status)
				}
			case "exhausted rate":
				if _, err := f.pool.Exec(t.Context(), `UPDATE api_keys SET rate_tokens=0,rate_last_refill=now()+interval '1 hour' WHERE key_id=$1`, key.KeyID); err != nil {
					t.Fatal(err)
				}
			}
			// When: the real signed HTTP endpoint is called.
			status, headers, _ := m6dSignedHTTP(t, f.server, key, secret, "/open/v1/ponds", nonce)
			// Then: fail closed and expose retry information for rate rejection.
			if status != scenario.want {
				t.Fatalf("status=%d want=%d", status, scenario.want)
			}
			if scenario.want == http.StatusTooManyRequests {
				retry, err := strconv.Atoi(headers.Get("Retry-After"))
				if err != nil || retry < 1 {
					t.Fatal("missing positive Retry-After")
				}
			}
		})
	}
}

func TestM6dAdministration_rejectsStaleMembershipWithoutPartialWrite(t *testing.T) {
	// Given: previously authorized context whose membership has been revoked.
	f := newM6dHTTPFixture(t)
	if _, err := f.pool.Exec(context.Background(), `UPDATE tenant_memberships SET active=false WHERE tenant_id=8301 AND user_id=8301`); err != nil {
		t.Fatal(err)
	}
	// When: application use case attempts issuance with the old context.
	_, _, err := f.svc.IssueAPIKey(f.owner, 8301, "revoked", []string{"ponds:read"}, domain.APIKeyResourceScope{}, 8301)
	// Then: current database authorization wins, with no key/audit rows.
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("revoked membership error=%v", err)
	}
	var count int
	if err := f.pool.QueryRow(t.Context(), `SELECT count(*) FROM api_keys`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("unauthorized key persisted")
	}
}
