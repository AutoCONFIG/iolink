package observability

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func dumpRecord(at time.Time, message string) string {
	return fmt.Sprintf("{\"time\":%q,\"msg\":%q}\n", at.Format(time.RFC3339Nano), message)
}
func writeDump(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
func readDump(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
func retentionConfig(dir string) Config {
	return Config{Dir: dir, Version: "current", Level: "info", MaxMB: 1, Backups: 2, Retention: time.Hour, CleanupInterval: time.Hour}
}

func TestRetentionPrunesRecordsInAllDumpLocations(t *testing.T) {
	// Given: active, real lumberjack backups, and prior versions with misleading mtimes.
	dir := t.TempDir()
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	cutoff := now.Add(-time.Hour)
	for _, path := range []string{"current/iolinkd.jsonl", "current/iolinkd-2026-10-13T09-00-00.000.jsonl", "previous/iolinkd.jsonl", "previous/iolinkd-2026-10-13T09-00-00.000.jsonl"} {
		full := filepath.Join(dir, path)
		writeDump(t, full, dumpRecord(cutoff.Add(-time.Nanosecond), "old")+dumpRecord(cutoff, "boundary")+dumpRecord(now, "recent")+"malformed\n")
		if err := os.Chtimes(full, now.Add(-24*time.Hour), now.Add(-24*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	// When: the sweep receives a deterministic cutoff.
	sink := &Sink{file: failingFile{}, rootDir: dir, path: filepath.Join(dir, "current/iolinkd.jsonl"), retention: time.Hour, stderr: io.Discard}
	if err := sink.cleanup(now); err != nil {
		t.Fatal(err)
	}
	// Then: record times decide retention; equality and unknown records survive.
	for _, path := range []string{"current/iolinkd.jsonl", "current/iolinkd-2026-10-13T09-00-00.000.jsonl", "previous/iolinkd.jsonl", "previous/iolinkd-2026-10-13T09-00-00.000.jsonl"} {
		got := readDump(t, filepath.Join(dir, path))
		if strings.Contains(got, `"old"`) || !strings.Contains(got, `"boundary"`) || !strings.Contains(got, `"recent"`) || !strings.Contains(got, "malformed") {
			t.Fatalf("incorrect retained records in %s: %s", path, got)
		}
		info, err := os.Stat(filepath.Join(dir, path))
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("unsafe permission: %v %v", info, err)
		}
	}
}

func TestStartupPrunesStaleRecordsAndRemovesEmptyOldDumps(t *testing.T) {
	// Given: stale records from this version and an earlier version before startup.
	dir := t.TempDir()
	stale := dumpRecord(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "expired")
	active := filepath.Join(dir, "current/iolinkd.jsonl")
	old := filepath.Join(dir, "previous/iolinkd.jsonl")
	writeDump(t, active, stale)
	writeDump(t, old, stale)
	// When: a new sink starts, before any event is emitted.
	log, sink, err := New(retentionConfig(dir), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Error(err)
		}
	}()
	// Then: startup cleaned the active file and removed the empty older dump.
	if got := readDump(t, active); got != "" {
		t.Fatalf("startup kept stale records: %s", got)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("empty old dump retained: %v", err)
	}
	log.Debug("after cleanup", "count", 7)
	if !strings.Contains(readDump(t, active), "after cleanup") {
		t.Fatal("writer did not reopen after pruning")
	}
}

func TestRetentionLeavesUnrelatedFilesAndSymlinks(t *testing.T) {
	// Given: non-dump files, a dump-shaped symlink, and a version symlink.
	dir := t.TempDir()
	outside := t.TempDir()
	stale := dumpRecord(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "untouched")
	target := filepath.Join(outside, "iolinkd.jsonl")
	writeDump(t, target, stale)
	for _, name := range []string{"notes.jsonl", "iolinkd.jsonl.cleanup", "iolinkd-not-a-date.jsonl"} {
		writeDump(t, filepath.Join(dir, "current", name), stale)
	}
	link := filepath.Join(dir, "current/iolinkd-2020-01-01T00-00-00.000.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked-version")); err != nil {
		t.Fatal(err)
	}
	// When: startup performs retention and backup limiting.
	_, sink, err := New(retentionConfig(dir), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	// Then: unrelated content and symlink entries remain unchanged.
	if readDump(t, target) != stale {
		t.Fatal("symlink target changed")
	}
	for _, name := range []string{"notes.jsonl", "iolinkd.jsonl.cleanup", "iolinkd-not-a-date.jsonl"} {
		if readDump(t, filepath.Join(dir, "current", name)) != stale {
			t.Fatal("unrelated file changed", name)
		}
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("dump symlink removed", err)
	}
}

func TestCleanupFailureDisablesReadinessAndPreservesOriginal(t *testing.T) {
	// Given: a record beyond the bounded reader size; invalid input must fail closed.
	dir := t.TempDir()
	_, sink, err := New(retentionConfig(dir), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Error(err)
		}
	}()
	var stderr bytes.Buffer
	sink.stderr = &stderr
	original := strings.Repeat("x", 2*1024*1024+1)
	writeDump(t, sink.path, original)
	// When: cleanup cannot safely parse the stream.
	err = sink.cleanup(time.Now())
	// Then: no partial rewrite and only the constant safe failure reaches stderr.
	if err == nil || sink.Healthy() || readDump(t, sink.path) != original || !strings.Contains(stderr.String(), "readiness disabled") {
		t.Fatal("cleanup did not fail closed", err)
	}
	entries, err := os.ReadDir(filepath.Dir(sink.path))
	if err != nil || len(entries) != 1 {
		t.Fatal("cleanup leaked a temporary file", entries, err)
	}
}
