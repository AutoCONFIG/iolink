# M7a GB catalog persistence gate review

- reviewed snapshot: `b3c98c4d0127530a6fd8f9f9935f51fe7c1889d8`
- parent snapshot: `6bee0aea1c485611cfeaef3050f237820f58bc89`
- reviewer: independent read-only gate reviewer (`/root/m7a_gb_gate_review_final2`)
- reviewed: 2026-10-11 (Asia/Shanghai)
- recommendation: **APPROVE**
- blockers: `[]`

## Original intent

Implement the M7a GB28181 catalog persistence seam: validate and aggregate
paged catalogs by device/SN, deduplicate idempotent pages, reject conflicting
items and inconsistent expected counts, enforce the bounded refresh window,
atomically replace the persistent directory on completion, preserve the old
directory on expiry, and add the 014 migration constraints/indexes. Keep the
domain port provider-neutral and perform PostgreSQL writes transactionally.

## Desired outcome

A completed refresh is durable and consistent, incomplete or expired work
cannot overwrite the last known directory, duplicate starts reuse one refresh,
malformed/bounded data is rejected, concurrent starts serialize safely, and a
nonzero catalog total cannot be represented by an empty page. This increment
does not claim a real GB/SIP runtime or playback path.

## User outcome review

The fixed snapshot meets this software scope. `internal/gbcatalog` owns the
validation and refresh state machine; `internal/gbcatalogpg` owns SQL
transactions and row locking; migration 014 adds registration checks, peer
uniqueness, catalog state/timestamp/count invariants, expiry lookup, and
present-channel lookup.

The new b3c98c4 validation rule rejects `SumNum > 0` with zero page items while
retaining a valid zero-item page only when `SumNum == 0`; a regression test
covers the rejection. Earlier fixes remain present: changed expected counts
are rejected in coordinator and PG paths, expired persisted refreshes atomically
mark catalog failed and device stale, database clock checks prevent an old
caller timestamp bypass, and canceled stale cleanup is retryable.

The implementation and tests demonstrate bounded IDs/names/counts, duplicate
and conflict handling, sorted atomic replacement, lock-free provider calls,
concurrent single completion, stale preservation, missing-channel offline
state, and rollback-safe SQL updates. No HTTP route, GB UDP/SIP runtime,
camera, or playback behavior is claimed; `docs/IMPLEMENTED.md` keeps M7a in
progress.

## Reproduced checks at exact SHA

All commands ran with `HEAD=b3c98c4d0127530a6fd8f9f9935f51fe7c1889d8` and no
tracked worktree modifications.

| Check | Result | Evidence |
|---|---|---|
| `IOLINK_TEST_PG_DSN=<disposable loopback DSN> go test -v -race -shuffle=on -count=1 ./internal/gbcatalog ./internal/gbcatalogpg ./internal/migrate` | **PASS, exit 0** | Domain tests (including nonzero empty-page rejection), all PG adapter tests (active mismatch, expired stale, late DB deadline, atomic replacement, constraints), and migration integration tests pass against the disposable PostgreSQL/Timescale instance on loopback port 32904. Credentials omitted. Output: `/tmp/m7a-catalog-b3c98c4-real-pg.log`. |
| `IOLINK_TEST_PG_DSN=<disposable loopback DSN> make verify` | **PASS, exit 0** | Build, vet and full race/shuffle suite; PG-backed packages passed. Output: `/tmp/m7a-catalog-b3c98c4-verify.log`. |
| `make verify-contracts` | **PASS, exit 0** | Six API contracts, 96 operations, 568 fixtures. Output: `/tmp/m7a-catalog-b3c98c4-contracts.log`. |
| `make docs-tools` | **PASS, exit 0** | Pinned documentation validators. Output: `/tmp/m7a-catalog-b3c98c4-docs-tools.log`. |
| `python3 scripts/check_architecture_manifests.py --all` | **PASS, exit 0** | 55 requirements/owners/providers/references and AGENTS scope. Output: `/tmp/m7a-catalog-b3c98c4-architecture.log`. The checker has no `M7a` gate choice; `--all` is the applicable invocation. |
| `git diff --check HEAD^ HEAD` | **PASS, exit 0** | No whitespace errors. Output: `/tmp/m7a-catalog-b3c98c4-diff.log`. |

## Direct skill perspective pass

- **remove-ai-slops:** reviewed the complete diff, production code and tests
  for obvious comments, redundant defensive checks, dead code, needless
  abstractions, wrong-layer dependencies, duplicated logic, and tautological
  or implementation-mirroring tests. The added rejection test is meaningful
  and fails if the new boundary is removed. No criterion-violating slop was
  found. `catalog_test.go` remains 296 pure lines, and terminal refresh maps
  are bounded by the current process lifetime; these are maintainability notes,
  not blockers for the requested persistence behavior.
- **programming:** Go errors and context deadlines are explicit; provider
  dependencies remain behind the narrow `CatalogStore` port; SQL writes have
  explicit transaction/rollback boundaries; race, vet and full verify pass.
  No new maintenance or false-confidence issue tied to a stated criterion was
  found.

## Checked artifact paths

- `docs/README.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`
- `docs/design/M7a-video.md` (catalog requirements at lines 168–170)
- `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`, `docs/api/video-openapi.yaml`
- `internal/gbcatalog/contracts.go`
- `internal/gbcatalog/validation.go`
- `internal/gbcatalog/catalog.go`
- `internal/gbcatalog/catalog_test.go`
- `internal/gbcatalogpg/store.go`
- `internal/gbcatalogpg/store_test.go`
- `internal/migrate/sql/013_video_foundation.sql`
- `internal/migrate/sql/014_video_catalog.sql`
- `internal/testdb/testdb.go`
- `scripts/check_architecture_manifests.py`

## Evidence gaps and external status

1. **`external_blocked`:** no real GB device, SIP/UDP runtime, camera,
   browser, or WeChat playback evidence. Those capabilities are outside this
   increment, remain disabled/unimplemented, and are accurately retained in
   `docs/IMPLEMENTED.md`.
2. The disposable PostgreSQL/Timescale test was isolated to loopback port
   32904; credentials are intentionally omitted. It satisfies the D-level
   persistence evidence for this snapshot.

The remaining external runtime gap does not violate the requested fixed
catalog persistence gate, so no blocker is recorded.
