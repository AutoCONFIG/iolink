# M6b independent review history

Latest review status: pending. No earlier approval is reused after source changes or the platform-admin owner fix.

## 2026-09-30, before Casbin correction

Reviewed main tracked-diff digest: `64bd9988078991c7bdf6006009927f1612a6a4f26f0974e7fa071e34c803501a`.

- `/root/m6b_final_review_code_r6`: REJECT. Required Casbin policy was absent; direct core farm-member listing did not authorize the caller. Both were corrected with a policy port/adapter and live membership checks.
- `/root/m6b_final_review_gate_r6`: APPROVE for R36.a/R37.a. This approval became stale after the subsequent corrections.
- Dedicated code/gate reviewer role invocations failed because their configured model was unsupported by the account. They produced no valid review verdict.

## 2026-09-30, after Casbin correction

Reviewed main tracked-diff digest: `c0a62c08ad7587d10ea47f0fcec4e0d4a7c6709192d6750dc0a3fefbbde53745`.

- `/root/m6b_policy_review_a`: REJECT. Operator batch alarm confirmation was classified as an ordinary write action. Corrected the action mapping and added atomic actor-scoped batch confirmation with real HTTP/database and concurrent-idempotency tests.
- `/root/m6b_policy_review_b`: REJECT, claiming assigned-farm reads were absent. The cited list methods call `tenantFilter`, whose existing role/actor branch adds an active `farm_memberships` predicate. The finding was not reproducible. Added real admin HTTP tests with two same-tenant farms, covering member/viewer/support lists, hidden-device and product-assignment rejection, and filtered statistics. Those tests passed against TimescaleDB; no filter was removed or weakened.

## 2026-10-01 verification snapshot

Latest source also mounts the already implemented application V2 routes in the production mux. `TestProductionMuxMountsBothApplicationAPIVersions` checks the actual mux helper against the application HTTP handler.

Raw final verification output is in `2026-10-01-go.txt`, `2026-10-01-http.txt`, and `2026-10-01-contracts.txt`. Two fresh independent reviewers must approve the same final source snapshot before M6b completion and release.

## 2026-10-01 final release review

Reviewed final main tracked-diff digest: `d28c8429fd9d38deabc517e91593c5d711f25b51049638e02a10be3125ec6d9b`; untracked source/evidence digest: `7456c57dde12ae0d8732e0ee35c3d8985288518cebef88a782e6044f37a0233a`.

- `/root/m6b_release_review_a`: APPROVE. Traced tenant filtering, Casbin injection, JWT revalidation, actor-scoped batch confirmation, production V2 mux, and current evidence.
- `/root/m6b_release_review_b`: APPROVE. Re-ran real Timescale migration rollback/backfill and HTTP member/viewer/support isolation plus atomic batch confirmation; no blocker found.

These two approvals referred to the pre-fix final worktree. They are stale/rejected because the platform-admin legacy-owner boundary bug was then fixed in source commit `1c9f1eaf95053f9409cfd784f5481cdddb7f1c12`. The new regression evidence is [`2026-10-01-platform-owner-fix/`](2026-10-01-platform-owner-fix/). Fresh independent double review on the exact release snapshot is required before R36/R37 software status or the M6b gate can be marked passed.

## 2026-10-01 platform-admin owner fix

- Fix commit: `1c9f1eaf95053f9409cfd784f5481cdddb7f1c12`; web submodule: `c54237e9d1a424bc9cb0b3f700014e2e966cf2cc`.
- The previous approvals are stale/rejected due to the platform-admin legacy-owner bug. Raw red/green regression commands and outputs are recorded in [`2026-10-01-platform-owner-fix/`](2026-10-01-platform-owner-fix/).
- Status: fixed, pending fresh independent double review on this exact snapshot; no R36/R37 final software gate pass is claimed.

## 2026-10-01 re-review of snapshot 764c5eb

- Snapshot reviewed: `764c5eb01dcbee0d015bf7153ded74f3881ba07f`.
- `/root/m6b_review_764_retry1`: REQUEST_CHANGES. Evidence in `.omo/evidence/m6b-review-764-retry1-viewer-repro.log` shows owner and viewer `POST /api/v2/devices/review-device/telemetry` both returned HTTP 202; viewer increased telemetry rows from 1 to 2 and changed shadow temperature from the owner control value 20 to 25.
- `/root/m6b_review_764_a` and `/root/m6b_review_764_b`: inconclusive because API returned 429; neither is an approval.
- Current state: D13 is open. R36/R37 software status is pending (`not_run` in the requirement manifest); previous approvals and prior passing checks remain historical evidence for their own source snapshots only. M6b gate is pending until D13 is fixed, fresh relevant verification completes, and two independent reviewers approve the same exact post-fix snapshot.

## 2026-10-03 D15 support-expiry timezone fix

- Source candidate commit: `62c0ecab0e5aa32fc0ad7227a7d8d4ff45ca1b9a`; web submodule candidate: `483cb2cecc4a368fdf6fc3406fa682207b514aea`.
- D15 targeted Playwright evidence covers Asia/Shanghai, UTC, and America/New_York in `2026-10-03-v2-timezone/`. D13 was already fixed in `96de9abc5cafdb7bf574e431f21686ecb7ca625f`; stale earlier wording incorrectly called it open. R36/R37/M6b remained pending final assembled-snapshot review.

## 2026-10-03 candidate ce23 review and corrections

- Candidate `ce23acdeb9d450b48649f0279e1f6c3b02afc3f2` was rejected by two independent reviewers: unchanged support expiry lost seconds (D16), and a tenant administrator without farm membership could not access same-tenant app resources (D17). Neither earlier approval can be reused.
- D16 fix: web `e84092326d9979e066c611e2be8eb6b9736e09f3`, assembled in parent `8197c590dfaa1eb1bdd3b7c7606f5b95c05896da`; evidence in [`../../rebuild/M6b-D16/`](../../rebuild/M6b-D16/).
- D17 fix: `62acecee62160d48efcd48f0e161a927ad583ef6`, integrated as `a96b234`; evidence in [`2026-10-03-d17-app-scope/`](2026-10-03-d17-app-scope/).
- Fresh verification and review of the combined candidate are pending. No current software-gate pass is claimed.
