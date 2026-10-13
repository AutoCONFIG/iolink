# Todo 6 independent gate review

recommendation: APPROVE
reviewed_at: 2026-09-27
manifest_digest: ef5bf5cff614339c255d2846a7631af042a45417bce16186f2a02c3cdd2ab6e2
blockers: []

## originalIntent

Deliver the R28-R29 notification delivery path: commit alarm intent and notification outbox atomically, deliver after commit through a durable leased worker, re-check the current owner and subscription, classify provider outcomes safely, recover expired work after restart, and keep the WeChat provider explicitly disabled when credentials are absent. Real WeChat delivery must remain `external_blocked` without credentials, consent, and a real device.

## desiredOutcome

The repository contains a buildable notification application port, PostgreSQL persistence adapter and migration, WeChat HTTP adapter, bounded shutdown wiring, meaningful unit/integration coverage, and evidence that distinguishes software verification from unavailable external delivery. Todo 6 may be marked complete only after this review and the independent code review approve the same worktree.

## userOutcomeReview

The implementation satisfies the software behavior reviewed here. `internal/core/alarm.go` inserts `alarms` and `notification_outbox` in the same transaction. `internal/persistence/notifications.go` claims with `FOR UPDATE SKIP LOCKED`, leases rows, reclaims expired processing rows, resolves the current owner, explicit `wechat_subscribed` consent, and active tenant membership, excludes `internal-admin`, and rejects stale lease finishes. `internal/notifications/worker.go` sends outside the claim transaction, bounds concurrency and delivery time, records disabled/permanent/retryable/unknown outcomes, and applies the 1/2/4/8/16 minute retry schedule with six total attempts. `internal/wechat/sender.go` has a synchronized token cache, one expiry refresh, configurable field mapping, fail-closed duplicate mapping, retryable token transport/decode/temporary-provider outcomes, non-2xx/provider-error handling, and a disabled configuration path. `cmd/iolinkd/main.go` starts the worker and waits for bounded shutdown. The revised evidence now includes captured verbose DSN-backed notification output and explicitly distinguishes no-DSN skips from passes.

The recorded real-database evidence in `docs/evidence/rebuild/notifications.txt` and `.omo/evidence/todo6-raw-qa.log` covers migration, alarm/outbox atomicity, claim/lease, owner transfer, disabled owner, retry, duplicate worker, unknown result, persistence, and the WeChat adapter. The raw evidence correctly labels real WeChat delivery `external_blocked`; it does not claim a receipt that was not available.

The local gate rerun independently passed focused notification, WeChat, persistence, and command tests, including the new retryable token and explicit subscription fixtures, `go vet` on the repository, compile sweep, formatting, and `git diff --check`. The PostgreSQL worker cases explicitly skip without a DSN in this reviewer process. Full `make verify` reached the package tests but the sandbox rejected `httptest`'s IPv6 listener (`listen tcp6 [::1]:0: socket: operation not permitted`); this environment limitation is recorded in the raw QA artifact and is not treated as a pass. Docker access was also unavailable in this reviewer process, so the claimed dedicated-container run was checked from its raw evidence rather than rerun here.

## remove-ai-slops and programming pass

The changed notification source files are small and responsibility-specific (worker 173 pure lines, persistence 73, WeChat sender 200); no oversized new module, dead provider wrapper, broad catch, untyped escape hatch, or provider SDK import was found. The sender and store interfaces are narrow seams used for the real HTTP and PostgreSQL tests. The tests assert observable delivery state and provider outcomes rather than snapshots, deletion-only behavior, or constants derived from the implementation. No criterion-blocking AI-slop or programming-practice issue was found.

## checked artifact paths

- `internal/core/alarm.go`
- `internal/core/pipeline.go`
- `internal/migrate/sql/003_ingestion.sql`
- `internal/migrate/sql/006_notification_delivery.sql`
- `internal/notifications/worker.go`
- `internal/notifications/worker_integration_test.go`
- `internal/persistence/notifications.go`
- `internal/wechat/sender.go`
- `internal/wechat/sender_test.go`
- `cmd/iolinkd/main.go`
- `docs/ACCEPTANCE.md` (R28-R29)
- `docs/PLAN-DETAILS.md` (notification ownership and outbox rules)
- `.omo/plans/modular-maintainability-refactor.md` (Todo 6)
- `docs/evidence/rebuild/notifications.txt`
- `.omo/evidence/todo6-raw-qa.log`

## verification evidence

- `GOCACHE=/tmp/iolink-review-gocache go test ./internal/notifications ./internal/wechat ./internal/persistence ./cmd/iolinkd -count=1`: PASS.
- `GOCACHE=/tmp/iolink-review-gocache go test ./... -run '^$' -count=1`: PASS compile sweep.
- `GOCACHE=/tmp/iolink-review-gocache go vet ./...`: PASS.
- `GOCACHE=/tmp/iolink-review-gocache go test ./internal/notifications ./internal/wechat -race -shuffle=on -count=1`: PASS; PostgreSQL worker cases explicitly skipped without a DSN in this reviewer environment.
- `git diff --check`: PASS.
- Prior dedicated-container and real-DSN commands are recorded in `.omo/evidence/todo6-raw-qa.log`; external WeChat is explicitly `external_blocked`.

## exact evidence gaps

There is no live WeChat credential, consent, or device receipt, so R28's external delivery portion remains `external_blocked`. The reviewer process could not access Docker or the recorded PostgreSQL endpoint, but the revised raw artifact contains captured verbose DSN-backed output for the notification integration cases, and the no-DSN local runs are explicitly labeled unit-only/skipped. The earlier overlay probe is stale and targeted the pre-fix token and mapping implementation; it is not evidence against the current tree.

The digest covers the 106 existing regular paths from `git ls-files -co --exclude-standard -z`, excluding `.omo/` and `docs/evidence/`; each record is sorted `relative-path + NUL + sha256(file bytes) + LF`. The deleted `internal/core/notify.go` is absent from the digest because it is not a filesystem path in the reviewed tree.
