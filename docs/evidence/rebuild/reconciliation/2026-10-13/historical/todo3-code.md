# Todo 3 final code review v3

## Verdict

`PASS`

`codeQualityStatus: WATCH`

`recommendation: APPROVE`

No CRITICAL or HIGH finding remains in the frozen snapshot. The WATCH status records non-blocking maintainability notes below; they do not prevent Todo 3 approval.

## Scope and review inputs

- Goal: PostgreSQL/Timescale persistence foundation for Todo 3, including transactional migrations, tenant ownership and attribution, telemetry snapshots, alarm/outbox/job integrity, bootstrap audit scope, and R13/R20 negative paths.
- Snapshot: exact shared worktree reviewed after the ConfirmByUser, CreateFarm membership, and audit-constraint fixes. Source snapshot digest from the reviewed `internal/` and `cmd/` tree: `601a1183ed47e5d31f9cea6c879bc0eea480b40474b0fb3910e567ca2fd030ef`.
- Authoritative documents: `AGENTS.md`, `docs/README.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/ACCEPTANCE.md`, `docs/CONTRIBUTING.md`, and `.omo/plans/modular-maintainability-refactor.md`.
- Changed implementation/test files reviewed: `internal/core/admin_store.go`, `internal/core/ingestion_test.go`, `internal/core/m2_integration_test.go`, `internal/core/pipeline.go`, `internal/core/repos.go`, `internal/migrate/migrate.go`, `internal/migrate/migrate_test.go`, `internal/migrate/sql/005_persistence_foundation.sql`, `internal/migrate/sql_foundation_test.go`, `internal/persistence/postgres.go`, `internal/persistence/postgres_test.go`, `internal/platform/bootstrap.go`, and `internal/platform/bootstrap_test.go`.
- Documentation/guidance changes were also checked for scope consistency; no implementation claim was taken from prose in place of runtime evidence.
- Skill perspective: `remove-ai-slops` and `programming` were loaded. No new provider import, untyped escape hatch, broad error suppression, needless production parsing, or boundary abstraction was introduced by this Todo 3 diff.

## Findings

### CRITICAL

None.

### HIGH

None.

### MEDIUM

- `internal/migrate/migrate.go:114-164` stores and refuses a `dirty` marker but does not automatically write `dirty=true` during an in-flight migration. The runner's transaction makes an interrupted DDL/history write roll back, so rerunning from the last committed version is the implemented recovery model. The independent probe verifies manual dirty refusal and recovery; future migration tooling should keep this atomic-versus-recorded-dirty contract explicit.
- `internal/persistence/postgres.go:13,59-62` declares `ErrNotOwned` but `ReadTelemetrySnapshot` returns the driver's `pgx.ErrNoRows` directly. Current callers can map the no-row result to their 404 policy, and the real cross-tenant probe passes, but the typed authorization sentinel is unused and leaves policy implicit for future callers. This is a maintainability item for the ownership/API wave, not a Todo 3 correctness blocker.
- `internal/persistence/postgres_test.go`, `internal/platform/bootstrap_test.go`, and `internal/migrate/sql_foundation_test.go` contain SQL/constant shape assertions. They are implementation-sensitive and would be weak in isolation. The current gate also has real PostgreSQL tests and the independent runtime probes in `.omo/evidence/todo3-review-probes-v3/`, so these supplemental checks do not create false acceptance evidence. Replacing them with behavior-only tests can be considered in a later cleanup.

### LOW

- `internal/core/admin_store.go:79-92` chooses the reserved system tenant for a new owner with no active membership and rejects multiple active memberships. This is fail-closed and covered by the owner red-green test; M6b must replace the implicit default selection when tenant switching becomes an exposed feature.

## Positive verification

- `go build ./...`, `go vet ./...`, and `go test ./... -count=1` pass in `.omo/evidence/todo3-final-go-v4.log`.
- `go test -race -shuffle=on -count=1 ./internal/migrate ./internal/persistence ./internal/platform ./internal/core` passes in `.omo/evidence/todo3-final-race-shuffle.log`, including alarm concurrency, tenant attribution, rollback, user confirmation idempotency, ownership, and MQTT persistence scenarios.
- The real PostgreSQL/Timescale package run is recorded in `.omo/evidence/todo3-real-go-test-final-v2.log`; the dedicated-container owner red-green result is in `.omo/evidence/todo3-owner-membership-red-green.txt`.
- Independent fresh-container probes passed in `.omo/evidence/todo3-review-probes-v3.log`: repeated owner confirmation preserves the first `confirmed_at`; foreign and missing alarms remain not-found; `Within` commits or rolls back tenants/jobs/outbox atomically; dirty history is refused and recovers after operator clearing; and actual jobs/outbox/audit constraints reject duplicate, null, blank, invalid-state, invalid-attempt, and invalid-foreign-key rows.
- `internal/core/repos.go:211-231` now returns success only for the current owner's already-confirmed alarm and keeps `domain.ErrNotFound` for missing/foreign alarms. `internal/core/ingestion_test.go` and the independent probe cover repeated confirmation, foreign access, missing IDs, and timestamp preservation.
- `internal/core/admin_store.go:64-107` performs owner validation, default membership creation, farm insertion, and commit in one transaction; multiple active memberships return `domain.ErrConflict`. `TestCreateFarmAssignsDefaultTenantMembership` passes against the real database.
- `internal/migrate/sql/005_persistence_foundation.sql:65-90` installs the audit checks and actor foreign key on the pre-existing 002 table before enforcing non-null tenant ownership. The real audit constraint probe exercises the installed constraints rather than only reading SQL text.
- `internal/core/pipeline.go:37-94` locks the device, resolves device to pond to farm to tenant, writes tenant-attributed sensor/shadow rows, commits alarm/outbox intent, and runs notifier side effects after commit.
- `internal/persistence/postgres.go:49-62` requires farm tenant equality, active owner membership, matching non-null shadow tenant, and an enabled device for telemetry snapshots.

## Evidence limitations

Hardware, WeChat, and deployed-customer upgrade acceptance remain outside Todo 3 or explicitly `external_blocked` in the authoritative scope. No such external acceptance is counted as passed. The Docker socket is unavailable to some review shells, but the dedicated container endpoint was reachable for the recorded independent probes and the real-provider logs redact credentials.

## Reviewer conclusion

The prior tenant-attribution, fixture masking, user confirmation idempotency, owner membership, and audit-constraint blockers are resolved in this exact snapshot. The remaining notes are non-blocking follow-up items for the later ownership/API and multi-tenant waves. Todo 3 is approved.
