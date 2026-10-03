package observability

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStructuredFileLoggingLevelsAndRedaction(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer
	logger, sink, err := New(Config{Dir: dir, Level: "info", MaxMB: 1, Backups: 2}, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	logger.Debug("hidden", "secret", "jwt-value")
	logger.Info("visible", "device", "pond-device-7", "password", "pw-value", "err", errors.New("database password=pw-value"))
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "iolinkd.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"hidden", "jwt-value", "pw-value", "pond-device-7"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("sensitive or debug value leaked: %q", forbidden)
		}
	}
	if !strings.Contains(text, `"msg":"visible"`) || !strings.Contains(text, "device_hash") {
		t.Fatalf("structured event missing: %s", text)
	}
}

func TestStructuredFileLoggingRotates(t *testing.T) {
	dir := t.TempDir()
	logger, sink, err := New(Config{Dir: dir, Level: "debug", MaxMB: 1, Backups: 2}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 30000; i++ {
		logger.Debug("event", "fields", strings.Repeat("x", 100))
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2 {
		t.Fatalf("rotation missing; files=%v", entries)
	}
}

func TestLogConfigRejectsInvalidBounds(t *testing.T) {
	for _, env := range []map[string]string{{"IOLINK_LOG_LEVEL": "trace"}, {"IOLINK_LOG_MAX_MB": "0"}, {"IOLINK_LOG_BACKUPS": "21"}} {
		cfg, err := FromEnv(func(key string) string { return env[key] })
		if err == nil && env["IOLINK_LOG_LEVEL"] != "" {
			_, _, err = New(cfg, &bytes.Buffer{})
		}
		if err == nil {
			t.Fatalf("accepted invalid env: %#v", env)
		}
	}
}
