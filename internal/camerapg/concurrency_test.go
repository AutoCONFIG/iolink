package camerapg_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"git.hyhy.fun/rsplab/iolink/internal/camera"
	"git.hyhy.fun/rsplab/iolink/internal/camerapg"
	"git.hyhy.fun/rsplab/iolink/internal/core"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReplace_whenConcurrentSourceChanges(t *testing.T) {
	// Given
	f := newFixture(t)
	addSession(t, f.pool)
	cfg := configuration(t, 101, true)
	start := make(chan struct{})
	results := make(chan camera.Camera, 2)
	failures := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			<-start
			got, err := f.service.Replace(actor(1, "owner"), 101, cfg)
			results <- got
			failures <- err
		})
	}
	// When
	close(start)
	workers.Wait()
	close(results)
	close(failures)
	// Then
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[int64]bool{}
	for result := range results {
		seen[result.SourceVersion] = true
	}
	if !seen[2] || !seen[3] {
		t.Fatal("source changes lost a version")
	}
	var version int64
	var jobs, audits int
	if err := f.pool.QueryRow(t.Context(), `SELECT source_version,(SELECT count(*) FROM jobs),(SELECT count(*) FROM audit_events WHERE action='camera.replaced') FROM video_cameras WHERE id=101`).Scan(&version, &jobs, &audits); err != nil || version != 3 || jobs != 1 || audits != 2 {
		t.Fatalf("version=%d jobs=%d audits=%d error=%v", version, jobs, audits, err)
	}
}

func TestDisable_whenConcurrentRepeatedRequests(t *testing.T) {
	// Given
	f := newFixture(t)
	addSession(t, f.pool)
	start := make(chan struct{})
	failures := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() { <-start; failures <- f.service.Disable(actor(1, "owner"), 101) })
	}
	// When
	close(start)
	workers.Wait()
	close(failures)
	// Then
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var jobs, audits int
	if err := f.pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM jobs),(SELECT count(*) FROM audit_events WHERE action='camera.disabled')`).Scan(&jobs, &audits); err != nil || jobs != 1 || audits != 1 {
		t.Fatalf("jobs=%d audits=%d error=%v", jobs, audits, err)
	}
}

func TestCreate_whenPoolHasOneConnection(t *testing.T) {
	// Given
	f := newFixture(t)
	config := f.pool.Config()
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal("pool construction failed")
	}
	defer pool.Close()
	f.deps.Store = camerapg.New(pool)
	f.deps.LicenseClock = core.NewLicenseService(pool, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s := camera.New(f.deps)
	ctx, cancel := context.WithTimeout(actor(1, "owner"), 5*time.Second)
	defer cancel()
	// When
	got, err := s.Create(ctx, configuration(t, 101, true))
	// Then
	if err != nil || got.ID <= 0 {
		t.Fatalf("id=%d error=%v", got.ID, err)
	}
}

func TestCreate_whenMembershipRevokedBeforeCommit(t *testing.T) {
	// Given
	f := newFixture(t)
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	if _, err = tx.Exec(t.Context(), `UPDATE tenant_memberships SET active=false,permission_version=permission_version+1 WHERE tenant_id=101 AND user_id=1`); err != nil {
		t.Fatal(err)
	}
	cfg := configuration(t, 101, true)
	done := make(chan error, 1)
	started := make(chan struct{})
	go func() { close(started); _, err := f.service.Create(actor(1, "owner"), cfg); done <- err }()
	<-started
	// When
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	err = <-done
	// Then
	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("error=%v", err)
	}
	var count int
	if err = f.pool.QueryRow(t.Context(), `SELECT count(*) FROM video_cameras`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("count=%d error=%v", count, err)
	}
}
