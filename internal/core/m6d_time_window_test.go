package core_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
)

func TestM6dTimestampWindow_signedBoundsAndExtremes(t *testing.T) {
	// Given: an issued key and a fixed authentication clock, with valid signatures for each timestamp.
	f := newM6dHTTPFixture(t)
	key, secretText := f.issue(t, []string{"ponds:read"}, domain.APIKeyResourceScope{})
	secret, err := base64.RawURLEncoding.DecodeString(secretText)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, scenario := range []struct {
		name      string
		timestamp int64
		accepted  bool
	}{
		{"past bound", now.Unix() - 300, true},
		{"future bound", now.Unix() + 300, true},
		{"now", now.Unix(), true},
		{"too old", now.Unix() - 301, false},
		{"too new", now.Unix() + 301, false},
		{"zero", 0, false},
		{"negative", -1, false},
		{"minimum", math.MinInt64, false},
		{"maximum", math.MaxInt64, false},
		{"overflow delta", now.Unix() + math.MinInt64, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			nonce := m6dNonce(t)
			bodyHash := sha256.Sum256(nil)
			canonical := strings.Join([]string{"GET", "/open/v1/ponds", "", strconv.FormatInt(scenario.timestamp, 10), nonce, hex.EncodeToString(bodyHash[:])}, "\n")
			mac := hmac.New(sha256.New, secret)
			if _, err := mac.Write([]byte(canonical)); err != nil {
				t.Fatal(err)
			}
			request := domain.OpenRequest{KeyID: key.KeyID, Method: "GET", Path: "/open/v1/ponds", Timestamp: scenario.timestamp, Nonce: nonce, Signature: hex.EncodeToString(mac.Sum(nil))}
			// When: the production verifier authenticates the correctly signed request.
			_, err := f.svc.AuthenticateOpen(t.Context(), request, now)
			// Then: the timestamp alone decides whether the request is inside the inclusive window.
			if scenario.accepted && err != nil {
				t.Fatalf("valid timestamp rejected: %v", err)
			}
			if !scenario.accepted && !errors.Is(err, core.ErrOpenUnauthorized) {
				t.Fatalf("out-of-window timestamp error=%v", err)
			}
		})
	}
}
