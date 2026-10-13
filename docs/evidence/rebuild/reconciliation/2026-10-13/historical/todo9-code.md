# Todo 9 / M5 independent code review

codeQualityStatus: WATCH
recommendation: APPROVE
reviewDate: 2026-09-28
reviewer: independent code-quality reviewer (no Todo 9 production edits)
reviewTree: current shared working tree
reportPath: .omo/evidence/todo9-independent-code-review.md
notepadPath: N/A; no ulw-loop plan exists
blockers: []

## Goal and success criteria

Todo 9 delivers the M5 operations and notification release gate for R28-R33: health/readiness/metrics and bounded shutdown, fail-closed deployment boundaries, durable notification delivery, reproducible full-database backup/restore, migration/release safety, capacity smoke checks, and CI/runner status that distinguishes software pass, integration pass/failure, and unavailable external inputs. Real WeChat, public TLS/firewall, 100 GiB recovery-time, timed release interruption, and sustained 500-device/90-day capacity evidence must remain explicitly `external_blocked` when unavailable.

## Reviewed scope

I consulted `docs/README.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/ACCEPTANCE.md`, `docs/DEPLOY.md`, `docs/IMPLEMENTED.md`, `docs/CONTRIBUTING.md`, the Todo plan, the M5 acceptance/evidence artifacts, and the current operational source. Reviewed implementation includes `deploy/backup.sh`, `deploy/restore.sh`, `scripts/run_external_case.py`, `scripts/capacity_smoke.py`, their tests, `internal/operations`, `internal/notifications`, migration/persistence additions, CI and deployment files, and the M5 evidence receipts.

The restore flow checks the explicit isolated marker and `iolink_restore_*` database name, checksum and required sidecars, PostgreSQL and TimescaleDB provider versions, and numeric backup migration metadata before invoking `timescaledb_pre_restore()` or `pg_restore`. It checks the target migration count before restore, permitting only an empty/nonexistent migration table or a count matching the backup. After restore it checks migration count and business/time-series/shadow/alarm counts and min/max timestamps, public index count, and Timescale retention-job count against the backup summary.

The runner now records the actual integration command result: no DSN reports `integration: skipped_external_blocked`; a successful DSN-backed integration reports `integration: passed`; a failing integration reports `integration: failed` and returns the nonzero command status. `scripts/test_run_external_case.py` pins the previously missing failure behavior with a fake `make` executable.

## Findings

### CRITICAL

None.

### HIGH

None. The prior runner-status blocker is closed in the current tree, and restore migration/provider mismatch guards execute before `pg_restore`.

### MEDIUM

1. Real PostgreSQL/Timescale, external WeChat, public TLS/firewall, 100 GiB RTO, timed release interruption, and sustained 500-device/90-day/disk-budget exercises are unavailable in this review environment. The checked-in evidence and acceptance map retain these portions as `external_blocked`; they are not treated as software passes here.
2. The runner regression test directly covers the failed integration state. The no-DSN skipped state and successful integration state were additionally reproduced by direct invocations/evidence, but are not each separately encoded in the focused test file. This is a small maintainability watch item and does not weaken the current gate classification.

### LOW

None affecting the Todo 9 software decision.

## Skill-perspective check

This check ran. I explicitly consulted `omo:remove-ai-slops` and `omo:programming` before judging the production code and tests. I found no deletion-only tests, requested-removal tests, tautological self-comparisons, prose pins, unnecessary production parsing/normalization, broad catches, provider SDK leaks, untyped escape hatches, or needless abstraction introduced by the M5 changes. The tests assert observable runner, capacity, health, notification, and migration behavior. The reviewed new source does not trigger the 250 pure-LOC oversized-module rule.

## Verification evidence

- `python3 -m pytest -q scripts/test_run_external_case.py scripts/test_capacity_smoke.py scripts/test_check_architecture_manifests.py`: **PASS**, 32 tests.
- `GOCACHE=/tmp/iolink-todo9-review-gocache go test -race -shuffle=on ./internal/operations ./internal/platform ./internal/notifications ./internal/wechat -count=1`: **PASS**.
- `GOCACHE=/tmp/iolink-todo9-review-gocache go test -run '^$' ./... -count=1`: **PASS**, repository compile sweep.
- `bash -n deploy/backup.sh deploy/restore.sh` and `git diff --check`: **PASS**.
- `python3 scripts/check_architecture_manifests.py --gate M5`: **PASS**.
- No-DSN runner invocation: `software=passed`, `integration=skipped_external_blocked`, `external=external_blocked`, exit 0.
- DSN-present fake integration failure probe: `software=failed`, `integration=failed`, exit 7; the focused regression test reproduces this behavior.
- `deploy/restore.sh` fake-tool probes for migration-count mismatch, PostgreSQL-version mismatch, and Timescale-version mismatch each exited nonzero before the fake `pg_restore` marker was created. The recorded pinned Timescale replay in `docs/evidence/acceptance/R31/README.md` reaches `restore verified` after the checks.
- `python3 scripts/run_external_case.py --case R28-R33 --provider reference --mode external`: correctly reports `external_blocked` and exits 0.

## Recommendation

**APPROVE** Todo 9/M5 for the software scope. Keep the documented R28, R30, R31, R32, and R33 real-environment portions explicitly `external_blocked` until their required credentials, hosts, recovery-size, interruption, and sustained-load evidence exist.
