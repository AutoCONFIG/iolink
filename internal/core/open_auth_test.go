package core

import (
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
		tokens, retry = consumeOpenRate(tokens, last, now, 60, 10)
		if retry != 0 {
			t.Fatalf("burst request %d retry=%d", i, retry)
		}
	}
	if _, retry := consumeOpenRate(tokens, last, now, 60, 10); retry != 1 {
		t.Fatalf("exhausted bucket retry=%d", retry)
	}
	refilled, retry := consumeOpenRate(tokens, last, now.Add(3*time.Second), 60, 10)
	if retry != 0 || refilled != 2 {
		t.Fatalf("refill tokens=%v retry=%d", refilled, retry)
	}
}

func TestConfiguredOpenRate_refillRetryAndReducedBurst(t *testing.T) {
	now := time.Unix(1000, 0)
	if tokens, retry := consumeOpenRate(0, now, now, 30, 2); retry != 2 || tokens != 0 {
		t.Fatalf("retry=%d tokens=%v", retry, tokens)
	}
	if tokens, retry := consumeOpenRate(0, now, now.Add(2*time.Second), 30, 2); retry != 0 || tokens != 0 {
		t.Fatalf("refill retry=%d tokens=%v", retry, tokens)
	}
	if tokens, retry := consumeOpenRate(10, now, now, 30, 2); retry != 0 || tokens != 1 {
		t.Fatalf("reduced burst retry=%d tokens=%v", retry, tokens)
	}
}
