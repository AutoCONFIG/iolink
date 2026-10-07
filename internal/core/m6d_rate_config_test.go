package core_test

import (
	"net/http"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestM6dConfiguredRate_initialBurstRotationAndHTTPRetry(t *testing.T) {
	f := newM6dHTTPFixture(t)
	if err := f.svc.ConfigureOpenRateLimit(30, 2); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ConfigureOpenRateLimit(0, 2); err == nil {
		t.Fatal("invalid rate accepted")
	}
	key, secret := f.issue(t, []string{"ponds:read"}, domain.APIKeyResourceScope{})
	var tokens float64
	if err := f.pool.QueryRow(t.Context(), `SELECT rate_tokens FROM api_keys WHERE key_id=$1`, key.KeyID).Scan(&tokens); err != nil || tokens != 2 {
		t.Fatalf("initial burst tokens=%v error=%v", tokens, err)
	}
	if _, err := f.pool.Exec(t.Context(), `UPDATE api_keys SET rate_tokens=0,rate_last_refill=now()+interval '1 hour' WHERE key_id=$1`, key.KeyID); err != nil {
		t.Fatal(err)
	}
	status, headers, _ := m6dSignedHTTP(t, f.server, key, secret, "/open/v1/ponds", m6dNonce(t))
	if status != http.StatusTooManyRequests || headers.Get("Retry-After") != "2" {
		t.Fatalf("configured throttle status=%d retry=%s", status, headers.Get("Retry-After"))
	}
	rotated, _, err := f.svc.RotateAPIKey(f.owner, 8301, 8301, key.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(t.Context(), `SELECT rate_tokens FROM api_keys WHERE key_id=$1`, rotated.KeyID).Scan(&tokens); err != nil || tokens != 2 {
		t.Fatalf("rotation burst tokens=%v error=%v", tokens, err)
	}
}
