package core_test

import (
	"context"
	"errors"
	"testing"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"git.hyhy.fun/rsplab/iolink/internal/persistence"
)

type fakeTenantJob struct {
	ctx context.Context
	run func(context.Context) error
}

type fakeTenantExecutor struct {
	jobs []fakeTenantJob
}

func (e *fakeTenantExecutor) Enqueue(ctx context.Context, run func(context.Context) error) {
	e.jobs = append(e.jobs, fakeTenantJob{ctx: ctx, run: run})
}

func (e *fakeTenantExecutor) RunNext(authorize func(context.Context) error) error {
	job := e.jobs[0]
	e.jobs = e.jobs[1:]
	if err := authorize(job.ctx); err != nil {
		return err
	}
	return job.run(job.ctx)
}

func TestM6bFakeExecutorPropagatesScopeAndRejectsRevocation(t *testing.T) {
	f := newTelemetryPermissionFixture(t)
	policy, err := authorization.New()
	if err != nil {
		t.Fatal(err)
	}
	ctx := telemetryActor("admin", 9402)
	f.role(t, "admin", true)
	check := func(jobCtx context.Context) error {
		tx, err := f.pool.Begin(jobCtx)
		if err != nil {
			return err
		}
		defer tx.Rollback(jobCtx)
		return persistence.AuthorizeTenantWrite(jobCtx, tx, policy, "farms", "write")
	}

	t.Run("preserves tenant actor context", func(t *testing.T) {
		var observedTenant, observedActor int64
		var observedRole string
		executor := fakeTenantExecutor{}
		executor.Enqueue(ctx, func(jobCtx context.Context) error {
			observedTenant, _ = domain.TenantID(jobCtx)
			observedActor, _ = domain.TenantUserID(jobCtx)
			observedRole = domain.TenantRole(jobCtx)
			return nil
		})
		if err := executor.RunNext(check); err != nil {
			t.Fatal(err)
		}
		if observedTenant != 9401 || observedActor != 9402 || observedRole != "admin" {
			t.Fatalf("scope tenant=%d actor=%d role=%q", observedTenant, observedActor, observedRole)
		}
	})

	t.Run("rejects committed role revocation before side effect", func(t *testing.T) {
		f.role(t, "admin", true)
		executor := fakeTenantExecutor{}
		runs := 0
		executor.Enqueue(ctx, func(context.Context) error {
			runs++
			return nil
		})
		f.role(t, "viewer", true)
		if err := executor.RunNext(check); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("revoked executor result=%v", err)
		}
		if runs != 0 {
			t.Fatalf("revoked job executed %d times", runs)
		}
	})

	t.Run("rejects committed permission version revocation before side effect", func(t *testing.T) {
		f.role(t, "admin", true)
		var version int64
		if err := f.pool.QueryRow(ctx, `SELECT permission_version FROM tenant_memberships WHERE tenant_id=9401 AND user_id=9402`).Scan(&version); err != nil {
			t.Fatal(err)
		}
		jobCtx := domain.WithTenantPermissionVersion(ctx, version)
		executor := fakeTenantExecutor{}
		runs := 0
		executor.Enqueue(jobCtx, func(context.Context) error {
			runs++
			return nil
		})
		if _, err := f.pool.Exec(ctx, `UPDATE tenant_memberships SET permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402`); err != nil {
			t.Fatal(err)
		}
		if err := executor.RunNext(check); !errors.Is(err, domain.ErrForbidden) || runs != 0 {
			t.Fatalf("revoked executor result=%v runs=%d", err, runs)
		}
	})
}
