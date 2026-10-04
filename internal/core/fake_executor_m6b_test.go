package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"git.hyhy.fun/rsplab/iolink/internal/authorization"
	"git.hyhy.fun/rsplab/iolink/internal/domain"
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
		tenantID, tenantOK := domain.TenantID(jobCtx)
		actorID, actorOK := domain.TenantUserID(jobCtx)
		if !tenantOK || !actorOK || domain.TenantRole(jobCtx) == "" {
			return domain.ErrForbidden
		}
		var role string
		err := f.pool.QueryRow(context.Background(), `SELECT tm.role FROM tenant_memberships tm JOIN tenants t ON t.id=tm.tenant_id AND t.active JOIN users u ON u.id=tm.user_id AND u.authority='USER' WHERE tm.tenant_id=$1 AND tm.user_id=$2 AND tm.active AND (tm.expires_at IS NULL OR tm.expires_at>now())`, tenantID, actorID).Scan(&role)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrForbidden
		}
		if err != nil {
			return err
		}
		if role != domain.TenantRole(jobCtx) {
			return domain.ErrForbidden
		}
		allowed, err := policy.Allow(role, "farms", "write")
		if err != nil || !allowed {
			return domain.ErrForbidden
		}
		return nil
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
}
