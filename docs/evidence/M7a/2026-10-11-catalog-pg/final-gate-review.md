# M7a GB catalog persistence gate review

- reviewed snapshot: `6bee0aea1c485611cfeaef3050f237820f58bc89`
- base increment: `28f2f3eeac439d0082e10b187c9f3a5841941b93`
- follow-up fixes reviewed: `33facea90f6adb5a68f2c69fffa4d5bbcd7b7b4e` and `6bee0ae`
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

A completed catalog refresh is durable and consistent, unfinished or expired
work cannot overwrite the last known directory, duplicate starts reuse one
refresh, malformed/bounded data is rejected, concurrent same-peer starts are
serialized, and this increment does not claim a real GB/SIP runtime that is
not implemented.

## User outcome review

The fixed snapshot meets the requested software scope. `internal/gbcatalog`
contains the validation and refresh state machine; `internal/gbcatalogpg`
contains the SQL transaction and row-lock implementation; migration 014 adds
registration field checks, registered-peer uniqueness, catalog counters and
state/timestamp invariants, expiry lookup, and present-channel lookup.

The 33facea coordinator fixes reject a changed expected count after completion
and clear the staling marker when a canceled context is supplied. The 6bee0ae
adapter fixes close the remaining persistence boundaries: an active persisted
refresh with a different expected count returns `ErrInvalid`; an expired
persisted refresh atomically marks both catalog `failed` and device `stale`;
and replacement checks database `clock_timestamp()` while holding the rows,
so an old caller `ObservedAt` cannot bypass the deadline.

Source and tests show that:

- device/channel IDs, names, page sizes and total counts are bounded;
- exact duplicate pages/items are idempotent, conflicting item values fail,
  and completed items are sorted before replacement;
- refresh starts are idempotent and expected-count mismatches are rejected in
  both coordinator and PG adapter paths;
- replacement occurs outside the coordinator lock, and concurrent completion
  performs one replacement;
- expiry/stale transitions preserve the old directory and retry cleanly after
  cancellation;
- PG replacement updates channels, catalog state and device state in one
  transaction and marks missing channels not present/offline;
- no HTTP route, GB UDP/SIP runtime, camera, or playback behavior is claimed
  by this increment; `docs/IMPLEMENTED.md` still records M7a as in progress.

## Reproduced checks at exact SHA

All commands ran with `HEAD=6bee0aea1c485611cfeaef3050f237820f58bc89` and no
tracked worktree modifications.

| Check | Result | Evidence |
|---|---|---|
| `IOLINK_TEST_PG_DSN=<disposable 127.0.0.1:32904 DSN> go test -v -race -shuffle=on -count=1 ./internal/gbcatalog ./internal/gbcatalogpg ./internal/migrate` | **PASS, exit 0** | Domain, all seven PG adapter cases (including expired-stale, active mismatch, atomic replacement, and late DB deadline), and migration integration cases pass against the disposable PostgreSQL/Timescale instance. Credentials are intentionally omitted. Output: `/tmp/m7a-catalog-6bee0ae-real-pg.log`. |
| `make verify` | **PASS, exit 0** | build, vet, full race/shuffle suite. Output: `/tmp/m7a-catalog-6bee0ae-verify.log`. |
| `make verify-contracts` | **PASS, exit 0** | all six API contracts, 96 operations and 568 fixtures. Output: `/tmp/m7a-catalog-6bee0ae-contracts.log`. |
| `make docs-tools` | **PASS, exit 0** | pinned docs validators. Output: `/tmp/m7a-catalog-6bee0ae-docs-tools.log`. |
| `python3 scripts/check_architecture_manifests.py --all` | **PASS, exit 0** | 55 requirements/owners/providers/references and AGENTS scope. Output: `/tmp/m7a-catalog-6bee0ae-architecture.log`. The script has no `M7a` gate choice; `--gate M7a` is unsupported, so `--all` is the applicable command. |
| `git diff --check HEAD^ HEAD` | **PASS, exit 0** | no whitespace errors. Output: `/tmp/m7a-catalog-6bee0ae-diff.log`. |

## Direct skill perspective pass

- **remove-ai-slops:** inspected the complete diff, production code and tests
  for obvious comments, redundant defensive checks, dead code, needless
  abstractions, wrong-layer dependencies, duplicated logic, and tautological
  or implementation-mirroring tests. No success-criterion violation found.
  `internal/gbcatalog/catalog_test.go` is 296 nonblank/noncomment lines,
  exceeding the skill's 250-line maintainability heuristic; this is a note,
  not a blocker for the requested catalog behavior.
- **programming:** Go error paths are explicit; context deadlines and database
  deadline checks are bounded; the domain imports no provider SDK; SQL writes
  use explicit transaction/rollback boundaries; race, vet and full verify are
  green. No new maintenance or false-confidence issue tied to the requested
  criteria was found.

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

1. **`external_blocked`: no real GB device, SIP/UDP runtime, camera, browser or
   WeChat playback evidence.** Those capabilities are outside this increment,
   remain disabled/unimplemented, and are accurately retained as such in
   `docs/IMPLEMENTED.md`.
2. The disposable PostgreSQL/Timescale run used for this gate was isolated to
   loopback port 32904; credentials are omitted from this report. It satisfies
   the D-level persistence evidence for this snapshot.

The remaining external runtime gap does not violate the requested fixed-snapshot
catalog persistence gate, so no blocker is recorded.
