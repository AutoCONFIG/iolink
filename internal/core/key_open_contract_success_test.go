package core_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestKeyOpenContractManagementAndFiveSignedReads(t *testing.T) {
	f := newKeyOpenFixture(t)
	var issued keyOpenIssued
	t.Run("POST_api_keys", func(t *testing.T) { issued = f.issueHTTP(t) })
	t.Run("GET_api_keys", func(t *testing.T) {
		raw := f.admin(t, keyOpenRequest{"GET", "/api-keys", "/api-keys", "", 200})
		if bytes.Contains(raw, []byte(issued.Secret)) {
			t.Error("key list exposes secret")
		}
	})
	for _, tc := range []keyOpenRequest{
		{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 200},
		{"GET", "/open/v1/ponds/{id}", "/open/v1/ponds/8301", "", 200},
		{"GET", "/open/v1/devices", "/open/v1/devices", "", 200},
		{"GET", "/open/v1/devices/{device_no}", "/open/v1/devices/m6d-allowed", "", 200},
		{"GET", "/open/v1/alarms", "/open/v1/alarms", "", 200},
	} {
		t.Run(tc.route, func(t *testing.T) {
			raw := f.signed(t, issued, tc, m6dNonce(t))
			if tc.route == "/open/v1/ponds" {
				var rows []struct {
					ID int64 `json:"id"`
				}
				platformContractDecode(t, raw, &rows)
				if len(rows) != 1 || rows[0].ID != 8301 {
					t.Error("signed pond list escaped resource scope")
				}
			}
			if tc.route == "/open/v1/devices" || tc.route == "/open/v1/alarms" {
				var rows []struct {
					DeviceNo string `json:"device_no"`
				}
				platformContractDecode(t, raw, &rows)
				if len(rows) != 1 || rows[0].DeviceNo != "m6d-allowed" {
					t.Error("signed list escaped resource scope")
				}
			}
		})
	}
	var rotated keyOpenIssued
	t.Run("POST_rotate", func(t *testing.T) {
		platformContractDecode(t, f.admin(t, keyOpenRequest{"POST", "/api-keys/{key_id}/rotate", "/api-keys/" + issued.Key.KeyID + "/rotate", "", 201}), &rotated)
	})
	if rotated.Key.KeyID == issued.Key.KeyID || rotated.Secret == issued.Secret {
		t.Error("rotation reused key credentials")
	}
	t.Run("old_key_revoked", func(t *testing.T) {
		f.signed(t, issued, keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 401}, m6dNonce(t))
	})
	t.Run("new_key_accepted", func(t *testing.T) {
		f.signed(t, rotated, keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 200}, m6dNonce(t))
	})
	t.Run("POST_revoke", func(t *testing.T) {
		f.admin(t, keyOpenRequest{"POST", "/api-keys/{key_id}/revoke", "/api-keys/" + rotated.Key.KeyID + "/revoke", "", 204})
	})
	t.Run("new_key_revoked", func(t *testing.T) {
		f.signed(t, rotated, keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 401}, m6dNonce(t))
	})
	t.Run("GET_audit", func(t *testing.T) {
		raw := f.admin(t, keyOpenRequest{"GET", "/api-keys/audit", "/api-keys/audit", "", 200})
		if bytes.Contains(raw, []byte(issued.Secret)) || bytes.Contains(raw, []byte(rotated.Secret)) {
			t.Error("key audit exposes secret")
		}
	})
}

func TestKeyOpenContractRestoreOnePhysicalOperationAndIdempotence(t *testing.T) {
	f := newKeyOpenFixture(t)
	execute(t, f.pool, `UPDATE devices SET disabled_at=now(),status='offline' WHERE device_no='m6d-allowed'`)
	for _, name := range []string{"first", "repeat"} {
		t.Run(name, func(t *testing.T) {
			raw := f.admin(t, keyOpenRequest{"POST", "/devices/{device_no}/restore", "/devices/m6d-allowed/restore", "", 204})
			assertHTTPContract(t, "license", "POST", "/devices/{device_no}/restore", f.responseStatus, f.responseContentType, raw)
			if count(t, f.pool, `SELECT count(*) FROM devices WHERE device_no='m6d-allowed' AND disabled_at IS NULL`) != 1 {
				t.Error("restore did not persist")
			}
		})
	}
}

func TestKeyOpenContractScopeOwnershipReplayAndRate(t *testing.T) {
	f := newKeyOpenFixture(t)
	issued := f.issueHTTP(t)
	for index, path := range []string{"/open/v1/ponds/8302", "/open/v1/ponds/8303", "/open/v1/devices/m6d-hidden", "/open/v1/devices/m6d-foreign"} {
		route := "/open/v1/ponds/{id}"
		if index > 1 {
			route = "/open/v1/devices/{device_no}"
		}
		t.Run(fmt.Sprintf("hidden_resource_%d", index), func(t *testing.T) { f.signed(t, issued, keyOpenRequest{"GET", route, path, "", 404}, m6dNonce(t)) })
	}
	nonce := m6dNonce(t)
	t.Run("nonce_first", func(t *testing.T) {
		f.signed(t, issued, keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 200}, nonce)
	})
	before := count(t, f.pool, "SELECT count(*) FROM api_key_nonces")
	t.Run("nonce_replay", func(t *testing.T) {
		f.signed(t, issued, keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 401}, nonce)
	})
	if count(t, f.pool, "SELECT count(*) FROM api_key_nonces") != before {
		t.Error("nonce replay changed reservations")
	}
	execute(t, f.pool, `UPDATE api_keys SET rate_tokens=0,rate_last_refill=$1 WHERE key_id=$2`, time.Now().Add(time.Hour), issued.Key.KeyID)
	nonce = m6dNonce(t)
	t.Run("rate_limit", func(t *testing.T) {
		f.signed(t, issued, keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 429}, nonce)
	})
	t.Run("throttled_nonce_replay", func(t *testing.T) {
		f.signed(t, issued, keyOpenRequest{"GET", "/open/v1/ponds", "/open/v1/ponds", "", 401}, nonce)
	})
}
