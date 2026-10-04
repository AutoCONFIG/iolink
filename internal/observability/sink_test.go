package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type failingFile struct{ short bool }

func (f failingFile) Write(p []byte) (int, error) {
	if f.short {
		return len(p) - 1, nil
	}
	return 0, errors.New("disk full secret")
}
func (failingFile) Close() error { return nil }

func TestSinkFailureDegradesReadinessAndRetainsStderr(t *testing.T) {
	for _, short := range []bool{false, true} {
		var stderr bytes.Buffer
		sink := &Sink{file: failingFile{short}, stderr: &stderr}
		_, err := sink.Write([]byte("safe event\n"))
		if err == nil || sink.Healthy() || !strings.Contains(stderr.String(), "safe event") || !strings.Contains(stderr.String(), "readiness disabled") {
			t.Fatalf("failure not observable: %v", err)
		}
		if strings.Contains(stderr.String(), "disk full secret") {
			t.Fatal("raw write error leaked")
		}
	}
}

func TestSinkClosedCannotReopenFile(t *testing.T) {
	dir := t.TempDir()
	_, sink, err := New(Config{Dir: dir, Level: "info", MaxMB: 1, Backups: 2}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = sink.Write([]byte("late event\n"))
	if !errors.Is(err, os.ErrClosed) {
		t.Fatal("closed sink accepted write", err)
	}
}

func TestConcurrentLoggingProducesCompleteJSONAndSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Dir: dir, Level: "debug", MaxMB: 1, Backups: 2}
	log, sink, err := New(cfg, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Go(func() {
			for i := 0; i < 100; i++ {
				log.Debug("concurrent event", "count", i)
			}
		})
	}
	wg.Wait()
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	log, sink, err = New(cfg, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("restarted")
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "iolinkd.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 801 {
		t.Fatalf("lost events: %d", len(lines))
	}
	for _, line := range lines {
		var record map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
	}
}

func TestErrorsKeepSafeClassificationWithoutValues(t *testing.T) {
	for _, tc := range []struct {
		err   error
		class string
	}{
		{fmt.Errorf("query secret: %w", &pgconn.PgError{Code: "23505", Detail: "password secret"}), "postgres_23505"},
		{fmt.Errorf("secret: %w", context.DeadlineExceeded), "deadline_exceeded"},
		{&os.PathError{Op: "open", Path: "secret-path", Err: os.ErrPermission}, "filesystem_error"},
	} {
		if got := errorClass(tc.err); got != tc.class {
			t.Fatalf("class=%s", got)
		}
	}
}

func TestInvalidLogPathFailsStartup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := New(Config{Dir: path, Level: "info", MaxMB: 1, Backups: 2}, io.Discard)
	if err == nil {
		t.Fatal("invalid directory accepted")
	}
}
