package observability

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentPruningAndWritingPreservesCompleteRecentRecords(t *testing.T) {
	// Given: a live dump and one writer object reused for its entire lifecycle.
	log, sink, err := New(retentionConfig(t.TempDir()), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	writer := sink.file
	errors := make(chan error, 20)
	var wg sync.WaitGroup
	// When: regular writes overlap repeated retention sweeps.
	for i := 0; i < 4; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				log.Debug("recent concurrent record", "count", j)
			}
		})
	}
	wg.Go(func() {
		for i := 0; i < 20; i++ {
			if err := sink.cleanup(time.Now()); err != nil {
				errors <- err
			}
		}
	})
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	// Then: no records are lost or torn, and sweeps never allocate another lumberjack worker.
	if writer != sink.file {
		t.Fatal("cleanup replaced the rotation writer")
	}
	raw := readDump(t, sink.path)
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 400 {
		t.Fatalf("lost recent records: %d", len(lines))
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Fatal("torn JSON record", line)
		}
	}
}

func TestBackupLimitIgnoresUnknownAndSymlinkEntries(t *testing.T) {
	// Given: excess real backup files and matching symbolic links.
	dir := t.TempDir()
	now := time.Now()
	recent := dumpRecord(now, "recent")
	for _, timestamp := range []string{"2026-10-10T00-00-00.000", "2026-10-11T00-00-00.000", "2026-10-12T00-00-00.000"} {
		writeDump(t, filepath.Join(dir, "current", "iolinkd-"+timestamp+".jsonl"), recent)
	}
	outside := filepath.Join(t.TempDir(), "target.jsonl")
	writeDump(t, outside, "untouched\n")
	link := filepath.Join(dir, "current/iolinkd-2020-01-01T00-00-00.000.jsonl")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	// When: startup enforces two backups synchronously.
	_, sink, err := New(retentionConfig(dir), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	// Then: the oldest real backup is removed, the link and newest backups are intact.
	if _, err := os.Stat(filepath.Join(dir, "current/iolinkd-2026-10-10T00-00-00.000.jsonl")); !os.IsNotExist(err) {
		t.Fatal("oldest excess backup retained", err)
	}
	for _, timestamp := range []string{"2026-10-11T00-00-00.000", "2026-10-12T00-00-00.000"} {
		if readDump(t, filepath.Join(dir, "current", "iolinkd-"+timestamp+".jsonl")) != recent {
			t.Fatal("recent backup altered")
		}
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("backup limiting removed a symlink", err)
	}
	if readDump(t, outside) != "untouched\n" {
		t.Fatal("backup limiting changed symlink target")
	}
}

func TestMalformedStartupDumpFailsClosedWithoutPartialRewrite(t *testing.T) {
	// Given: a stale record followed by a record exceeding the bounded scanner limit.
	dir := t.TempDir()
	path := filepath.Join(dir, "current/iolinkd.jsonl")
	original := dumpRecord(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), "stale") + strings.Repeat("x", 2*1024*1024+1)
	writeDump(t, path, original)
	// When: startup retention cannot complete safely.
	_, _, err := New(retentionConfig(dir), io.Discard)
	// Then: no sink is returned as ready, and existing contents remain intact.
	if err == nil || readDump(t, path) != original {
		t.Fatal("startup reported success or partially rewrote dump", err)
	}
}

func TestReopenedWriterEnforcesBackupsAtExactSizeBoundary(t *testing.T) {
	// Given: the first write after startup exactly meets lumberjack's rotation boundary.
	dir := t.TempDir()
	active := filepath.Join(dir, "current/iolinkd.jsonl")
	writeDump(t, active, strings.Repeat("x", 1024*1024-3))
	recent := dumpRecord(time.Now(), "recent")
	for _, timestamp := range []string{"2026-10-10T00-00-00.000", "2026-10-11T00-00-00.000"} {
		writeDump(t, filepath.Join(dir, "current", "iolinkd-"+timestamp+".jsonl"), recent)
	}
	_, sink, err := New(retentionConfig(dir), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	// When: reopening triggers rotation at equality, not only above the size limit.
	if _, err := sink.Write([]byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	// Then: the file contains only the new record, and backup count is still bounded.
	if readDump(t, active) != "{}\n" {
		t.Fatal("reopen did not rotate at exact size boundary")
	}
	entries, err := os.ReadDir(filepath.Dir(active))
	if err != nil || len(entries) != 3 {
		t.Fatal("exact boundary bypassed backup cap", entries, err)
	}
}
