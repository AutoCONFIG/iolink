package core

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestCanonicalOpenQuery(t *testing.T) {
	got, err := CanonicalOpenQuery("b=two&a=hello%20world&a=&b=one")
	if err != nil {
		t.Fatal(err)
	}
	if got != "a=&a=hello%20world&b=one&b=two" {
		t.Fatalf("canonical query = %q", got)
	}
}

func TestCanonicalOpenPathRejectsAmbiguity(t *testing.T) {
	for _, path := range []string{"/open/v1/../ponds", "/open/v1/%2e%2e/ponds", "/open/v1/%2fponds", "/open/v1\\ponds"} {
		if _, err := CanonicalOpenPath(path); err == nil {
			t.Fatalf("accepted ambiguous path %q", path)
		}
	}
}

func TestCanonicalOpenPathKeepsEncodedSafePath(t *testing.T) {
	got, err := CanonicalOpenPath("/open/v1/ponds/%E4%B8%AD")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/open/v1/ponds/%E4%B8%AD" {
		t.Fatalf("path = %q", got)
	}
}

func TestConsumeOpenRateAllowsBurstAndRefillsAtOnePerSecond(t *testing.T) {
	now := time.Unix(1000, 0)
	tokens, last := 10.0, now
	for i := 0; i < 10; i++ {
		var retry int
		tokens, retry = consumeOpenRate(tokens, last, now)
		if retry != 0 {
			t.Fatalf("burst request %d retry=%d", i, retry)
		}
	}
	if _, retry := consumeOpenRate(tokens, last, now); retry != 1 {
		t.Fatalf("exhausted bucket retry=%d", retry)
	}
	refilled, retry := consumeOpenRate(tokens, last, now.Add(3*time.Second))
	if retry != 0 || refilled != 2 {
		t.Fatalf("refill tokens=%v retry=%d", refilled, retry)
	}
}

func TestOpenSignatureMatchesPublishedVector(t *testing.T) {
	bodyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	canonical := strings.Join([]string{"GET", "/open/v1/ponds", "a=&a=hello%20world&b=one&b=two", "1735689600", "0123456789abcdef0123456789abcdef", bodyHash}, "\n")
	mac := hmac.New(sha256.New, []byte("documented-test-secret"))
	_, _ = mac.Write([]byte(canonical))
	got := hex.EncodeToString(mac.Sum(nil))
	if got != "bcf8386ae0789327c5a6597a81eaa2f3121565d045012796e7e22bdba2a43149" {
		t.Fatalf("signature=%s", got)
	}
}
