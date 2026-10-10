# M7a GB catalog persistence code-quality review

- `codeQualityStatus`: WATCH
- `recommendation`: APPROVE
- `snapshotSHA`: `6bee0aea1c485611cfeaef3050f237820f58bc89`
- `reviewScope`: `internal/gbcatalog`, `internal/gbcatalogpg`, `internal/migrate/sql/014_video_catalog.sql`, and their tests/evidence; unrelated HTTP, SIP, and media changes were excluded
- `blockers`: `[]`
- `external_blocked`: real PostgreSQL/Timescale integration was unavailable because `IOLINK_TEST_PG_DSN` was not set; those tests were reported as skipped, not passed
- `ulwStatus`: `ULW_LOOP_PLAN_MISSING`; no current attempt directory or notepad was available, so this fallback report path was used

## Goal and success criteria

Review the GB catalog refresh persistence increment for same-SN idempotency and conflict handling, transactional replacement and stale transitions, tenant/device scoping, deadline enforcement, bounded page aggregation, maintainable ports, and truthful tests/evidence. The design requires a 10-second refresh deadline, deduplication and complete `SumNum` aggregation before replacing the directory, and stale marking without overwriting the old directory.

## Input and diff inspected

The exact snapshot and cumulative scoped diff were inspected with:

```text
git rev-parse HEAD
git diff 28f2f3e..6bee0ae -- internal/gbcatalog internal/gbcatalogpg internal/migrate/sql/014_video_catalog.sql
git diff --check 28f2f3e..6bee0ae -- internal/gbcatalog internal/gbcatalogpg internal/migrate/sql/014_video_catalog.sql
```

The final commit is `6bee0ae` (`fix: harden GB catalog persistence deadlines`), based on `28f2f3e` and `33facea`. The final fix adds the active-refresh expected-count comparison, atomically sets `video_gb_devices.catalog_state='stale'` when an expired collecting row is discovered, and compares the persisted deadline with `clock_timestamp()` before a replacement. I read `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/design/M7a-video.md`, `docs/ACCEPTANCE.md`, and the checked-in catalog receipts under `docs/evidence/M7a/2026-10-11-catalog-pg/` as untrusted source material.

## Skill perspective check

This check ran before judging maintainability and test relevance. I explicitly loaded and consulted `omo:remove-ai-slops` and `omo:programming` (Go guidance). The overfit/slop pass found no deletion-only tests, tests that merely prove a requested removal, tautological assertions, untyped escape hatches, provider-SDK leakage, broad catch-all handling, or needless production parsing/normalization beyond the catalog boundary. The medium findings below are the remaining oversized test module, unbounded process-local result maps, and one page-validation coverage gap.

## Findings by severity

### CRITICAL

None.

### HIGH

None. The final snapshot closes the previously identified PG boundary issues: active same-SN refreshes with a different expected count are rejected (`internal/gbcatalogpg/store.go:44-47`), an expired collecting row fails the catalog and marks the device stale in one transaction (`internal/gbcatalogpg/store.go:59-70`), and a replacement is rejected when either the supplied observation or the database clock is past the persisted deadline (`internal/gbcatalogpg/store.go:98-109`). Coordinator completion count mismatch and canceled stale-mark retry are covered in `internal/gbcatalog/catalog.go:61-67` and `:222-255`.

### MEDIUM

1. **The coordinator accepts a non-empty `SumNum` with an empty page.** `internal/gbcatalog/validation.go:23-37` returns true for `SumNum > 0` and `len(Items) == 0`. The GB XML boundary rejects that combination, and the design says a nonzero total cannot carry an empty page. A direct caller can nevertheless create a pending refresh that cannot complete until expiry. Add the same guard and a focused regression before wiring additional untrusted page callers. This is a bounded correctness/coverage gap, not a current directory-corruption path.

2. **Completed and stale refresh maps never evict entries.** `internal/gbcatalog/catalog.go:20-24,187-189,241-245` retains one value for every device/SN forever. SN values can increase over a long-lived process, so a registered peer can cause unbounded heap growth even though each pending page is capped at 1000 items. Add TTL/size eviction or move terminal-state lookup to durable state before production-scale wiring. This is a resource/maintenance risk for this provider-neutral increment.

3. **The catalog unit-test module exceeds the remove-ai-slops 250 pure-LOC ceiling.** `internal/gbcatalog/catalog_test.go` measures 296 pure non-comment lines and combines aggregation, duplicate/conflict, deadline, concurrency, and store-failure seams. Split by behavior before the next substantial increment to keep review and failure isolation manageable. `internal/gbcatalog/catalog.go` is 242 pure lines (warning band); avoid adding another responsibility there.

### LOW

1. **Receipts are not an exact-SHA audit trail.** The checked-in `gbcatalogpg-race.log` and `migrate.log` contain short `ok` summaries without the commit SHA or command line, and `vet.log` is empty. This review compensates with fresh exact-SHA commands recorded below; future receipts should include the SHA, environment, command, and explicit skipped/pass result.

2. **`received_pages` is persisted as `GREATEST(received_pages,1)`.** `internal/gbcatalogpg/store.go:125` records only that at least one aggregated snapshot existed; the coordinator port does not carry a page count. This is acceptable for the current replacement contract, but the field should either be documented as a lower bound or populated when page-count observability becomes a requirement.

## Criterion review

- **Same-SN idempotency and conflict:** The coordinator serializes pending/completed keys and rejects changed expected counts. The PG store locks the device and catalog row, reuses an active row only when the count matches, and refuses completed/failed rows. Concurrent same-SN starts therefore produce one catalog row.
- **Aggregation and SumNum:** Pages are deduplicated by channel ID, conflicting duplicates fail, aggregate count cannot exceed the expected count, and replacement runs only at exact completion. The `SumNum > 0`/empty-page validation gap is recorded above.
- **Atomicity:** `StartRefresh`, expired-row stale transition, `ReplaceCatalog`, and `MarkStale` each use one transaction with rollback on every error. Replacement deletes/rebuilds catalog items, updates channel presence, completes the catalog, and marks the device ready before commit. No old-directory mutation occurs on validation or deadline failure.
- **Tenant/device scope:** `video_gb_devices.device_id` is globally unique; catalog and channel rows carry tenant IDs and the `(device_id,tenant_id)` foreign key. The store resolves the tenant from the locked device and never trusts a caller-supplied tenant. Replacement joins the requested device ID to its catalog row and uses that locked row's tenant/device keys for all writes.
- **Deadline/stale:** Coordinator calls are bounded by the refresh deadline; expired in-memory refreshes mark stale without replacing the old directory. The PG adapter rejects an expired row at start, atomically fails it and marks the device stale, and uses the database clock to reject late replacements even when a caller supplies an old observation timestamp.
- **Schema:** Migration 014 adds registration invariants, a registered-peer uniqueness index, catalog counters/timestamps/state checks, and lookup indexes. Complete rows require an expected count and exact received count; failed rows require `failed_at`; collecting rows cannot carry terminal timestamps.
- **Ports/logging:** `CatalogStore` is a narrow domain port implemented by `gbcatalogpg`; no provider SDK or logging was introduced in the scoped diff. No secret, request body, or telemetry payload is logged.
- **Scope:** The increment stays within catalog coordination, PostgreSQL persistence, migration 014, and tests. SIP transport, UDP, RTP/PS, camera integration, and HTTP parent behavior are outside this review.

## Fresh verification at the exact SHA

- `git rev-parse HEAD`: **PASS**, `6bee0aea1c485611cfeaef3050f237820f58bc89`.
- `go test -race -shuffle=on -count=1 -v ./internal/gbcatalog ./internal/gbcatalogpg ./internal/migrate`: **PASS** for all unit/static tests. All PG-backed catalog and migration tests explicitly **SKIP** with `integration: set IOLINK_TEST_PG_DSN to a disposable Timescale instance`.
- `go test -race -shuffle=on -count=1 ./...`: **PASS** for the full Go suite; PG-backed tests remain skipped for the same missing DSN.
- `go vet ./...`: **PASS**.
- `test -z "$(gofmt -d internal/gbcatalog/*.go internal/gbcatalogpg/*.go)"`: **PASS**.
- `git diff --check 28f2f3e..6bee0ae -- internal/gbcatalog internal/gbcatalogpg internal/migrate/sql/014_video_catalog.sql`: **PASS**.
- `python3 scripts/check_architecture_manifests.py`: **PASS** (`55 requirements, owners, providers, references, and AGENTS.md scope`).
- `make verify-contracts`: **PASS** (`96 target operations, 568 synthetic request/response fixtures`).
- `make docs-tools`: **PASS** (dependencies already satisfied; no documentation-tool errors).

## External and deferred evidence

The real PostgreSQL/Timescale path, rollback behavior under injected SQL failures, concurrent database transactions, migration idempotence against a disposable database, and the actual GB device/catalog UDP chain could not be exercised here because `IOLINK_TEST_PG_DSN` and external devices/services were unavailable. These remain `external_blocked`; no skipped test is counted as a pass. The fresh unit/full/race/vet/static results establish the coordinator and compile-time/provider-boundary behavior only.

## Final recommendation

**APPROVE** the GB catalog persistence snapshot at `6bee0aea1c485611cfeaef3050f237820f58bc89`, with `blockers=[]`. Carry the three medium follow-ups and the explicit real-Postgres `external_blocked` status into the next increment.
