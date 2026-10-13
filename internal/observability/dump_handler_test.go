package observability

import (
	"bytes"
	"strings"
	"testing"
)

func TestDumpFailureFallsBackToStderrEvenBelowConsoleLevel(t *testing.T) {
	// Given: an error-only console and a dump writer that fails with a sensitive error.
	var stderr bytes.Buffer
	cfg := retentionConfig(t.TempDir())
	cfg.Level = "error"
	log, sink, err := New(cfg, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	sink.mu.Lock()
	if err := sink.file.Close(); err != nil {
		sink.mu.Unlock()
		t.Fatal(err)
	}
	sink.file = failingFile{}
	sink.mu.Unlock()
	// When: a debug record cannot be persisted.
	log.Debug("fallback detail", "count", 12)
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	// Then: readiness fails and the record reaches stderr without raw error contents.
	text := stderr.String()
	if sink.Healthy() || !strings.Contains(text, "readiness disabled") || !strings.Contains(text, "fallback detail") || strings.Contains(text, "disk full secret") {
		t.Fatal("failed debug dump was not safely observable", text)
	}
}

func TestStderrFailureDisablesReadinessButKeepsPersistedRecord(t *testing.T) {
	for _, short := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "short write"}[short], func(t *testing.T) {
			// Given: a healthy dump file with a broken console mirror.
			log, sink, err := New(retentionConfig(t.TempDir()), failingFile{short: short})
			if err != nil {
				t.Fatal(err)
			}
			// When: an info event reaches the mirror.
			log.Info("persisted despite console failure", "count", 19)
			if err := sink.Close(); err != nil {
				t.Fatal(err)
			}
			// Then: readiness reflects the failure while the file retains the event.
			if sink.Healthy() || !strings.Contains(readDump(t, sink.path), "persisted despite console failure") {
				t.Fatal("console failure was hidden or lost persisted record")
			}
		})
	}
}
