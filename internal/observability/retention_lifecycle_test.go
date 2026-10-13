package observability

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestPeriodicCleanupUsesConfiguredIntervalAndStopsOnClose(t *testing.T) {
	// Given: a real file and a controlled clock; no wall-clock waits.
	dir := t.TempDir()
	path := filepath.Join(dir, "current/iolinkd.jsonl")
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		original := dumpRecord(now, "expires after first interval")
		writeDump(t, path, original)
		sink := &Sink{file: failingFile{}, rootDir: dir, path: path, retention: 30 * time.Minute, stderr: io.Discard, stop: make(chan struct{}), done: make(chan struct{})}
		// When: the configured hourly timer fires.
		go sink.cleanupLoop(time.Hour)
		synctest.Wait()
		time.Sleep(59 * time.Minute)
		synctest.Wait()
		if readDump(t, path) != original {
			t.Fatal("cleanup ran before configured interval")
		}
		time.Sleep(time.Minute)
		synctest.Wait()
		// Then: expired records are gone and Close joins the scheduler.
		if readDump(t, path) != "" {
			t.Fatal("periodic cleanup did not remove expired record")
		}
		if err := sink.Close(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-sink.done:
		default:
			t.Fatal("Close left cleanup goroutine running")
		}
		if err := sink.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestConcurrentCloseAndWritesCannotReopenClosedSink(t *testing.T) {
	// Given: a live sink, writers, and concurrent closers.
	_, sink, err := New(retentionConfig(t.TempDir()), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	failures := make(chan error, 32)
	var wg sync.WaitGroup
	// When: all callers race at the lifecycle boundary.
	for i := 0; i < 16; i++ {
		wg.Go(func() {
			<-start
			for j := 0; j < 100; j++ {
				if _, err := sink.Write([]byte("{}\n")); err != nil && !errors.Is(err, os.ErrClosed) {
					failures <- err
					return
				}
			}
		})
		wg.Go(func() {
			<-start
			if err := sink.Close(); err != nil {
				failures <- err
			}
		})
	}
	close(start)
	wg.Wait()
	close(failures)
	// Then: no panic or unexpected error; subsequent writes stay closed.
	for err := range failures {
		t.Error(err)
	}
	if _, err := sink.Write([]byte("late\n")); !errors.Is(err, os.ErrClosed) {
		t.Fatal("closed sink reopened", err)
	}
	select {
	case <-sink.done:
	default:
		t.Fatal("scheduler still running")
	}
}

func TestNewRejectsVersionEscapeAndSymlinkedActivePaths(t *testing.T) {
	// Given: escaping versions and active paths that redirect to another location.
	for _, version := range []string{"..", ".", "../outside", "/absolute", "v/child", "v/"} {
		t.Run(version, func(t *testing.T) {
			cfg := retentionConfig(t.TempDir())
			cfg.Version = version
			// When / Then: invalid versions fail before opening a dump.
			if _, _, err := New(cfg, io.Discard); err == nil {
				t.Fatal("escaping version accepted")
			}
		})
	}
	for _, directoryLink := range []bool{false, true} {
		t.Run(map[bool]string{false: "file link", true: "directory link"}[directoryLink], func(t *testing.T) {
			dir := t.TempDir()
			outside := t.TempDir()
			target := filepath.Join(outside, "iolinkd.jsonl")
			writeDump(t, target, "untouched\n")
			if directoryLink {
				if err := os.Symlink(outside, filepath.Join(dir, "current")); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(filepath.Join(dir, "current"), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(dir, "current/iolinkd.jsonl")); err != nil {
					t.Fatal(err)
				}
			}
			// When / Then: startup rejects the redirect without changing its target.
			if _, _, err := New(retentionConfig(dir), io.Discard); err == nil {
				t.Fatal("symlinked active path accepted")
			}
			if readDump(t, target) != "untouched\n" {
				t.Fatal("redirect target changed")
			}
		})
	}
}

func TestDurationEnvironmentIsAppliedAndInvalidDurationsRejected(t *testing.T) {
	// Given: distinct configured durations and invalid edge cases.
	env := map[string]string{"IOLINK_LOG_RETENTION": "90m", "IOLINK_LOG_CLEANUP_INTERVAL": "2h", "IOLINK_VERSION": "v-test"}
	// When: parsing the environment boundary.
	cfg, err := FromEnv(func(key string) string { return env[key] })
	// Then: explicit durations reach configuration without fallback.
	if err != nil || cfg.Retention != 90*time.Minute || cfg.CleanupInterval != 2*time.Hour || cfg.Version != "v-test" {
		t.Fatal("duration configuration ignored", cfg, err)
	}
	for _, key := range []string{"IOLINK_LOG_RETENTION", "IOLINK_LOG_CLEANUP_INTERVAL"} {
		for _, raw := range []string{"0", "-1h", "bogus", "999999999999999h"} {
			if _, err := FromEnv(func(k string) string {
				if k == key {
					return raw
				}
				return ""
			}); err == nil {
				t.Fatalf("accepted %s=%s", key, raw)
			}
		}
	}
	defaultCfg, err := FromEnv(func(string) string { return "" })
	if err != nil || defaultCfg.Version != "dev" || defaultCfg.Retention != 6*time.Hour || defaultCfg.CleanupInterval != 6*time.Hour {
		t.Fatal("defaults invalid", defaultCfg, err)
	}
	for _, bad := range []Config{{Dir: t.TempDir(), Version: "v", Level: "info", MaxMB: 1, Backups: 2, Retention: -time.Hour}, {Dir: t.TempDir(), Version: "v", Level: "info", MaxMB: 1, Backups: 2, CleanupInterval: -time.Hour}} {
		if _, _, err := New(bad, io.Discard); err == nil {
			t.Fatal("invalid direct config accepted")
		}
	}
}

func TestDumpRetainsDebugAndConsoleUsesConfiguredLevelWithGroupedAttrs(t *testing.T) {
	// Given: a warn console and a detailed dump with grouped logger attributes.
	dir := t.TempDir()
	var stderr bytes.Buffer
	cfg := retentionConfig(dir)
	cfg.Level = "warn"
	log, sink, err := New(cfg, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	// When: scoped debug and warning events are emitted.
	scoped := log.WithGroup("stage").With("count", 17)
	scoped.Debug("debug detail", "bytes", 23)
	scoped.Warn("warning detail", "count", 19)
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	// Then: dump gets both, console only warning; grouped safe fields survive.
	dump := readDump(t, filepath.Join(dir, "current/iolinkd.jsonl"))
	if !strings.Contains(dump, "debug detail") || !strings.Contains(dump, "warning detail") || !strings.Contains(dump, `"stage"`) || !strings.Contains(dump, `"count":17`) || strings.Contains(stderr.String(), "debug detail") || !strings.Contains(stderr.String(), "warning detail") {
		t.Fatal("dump/console handler behavior incorrect", dump, stderr.String())
	}
}
