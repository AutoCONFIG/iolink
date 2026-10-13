# GB catalog validation gate review — 2026-10-13

recommendation: APPROVE

blockers: []

Reviewer: independent gate lane `/root/gbcatalog_release_gate_review`; no source edits, staging, delegation, database mutation, or release operations. This review artifact is the only file written by this lane.

## Scope and original intent

originalIntent: The supplied original-request summary asks for testing existing functions following the debug dump release. The bounded implementation goal is closing the 11 calendar-dependent GB catalog test failures without changing production behavior or weakening tests. It does not claim completing M7a, R55, browser acceptance, actual License delivery, WeChat, or hardware acceptance.

desiredOutcome: Existing GB catalog tests remain meaningful after 2026-10-11, their normal and error cases pass, and the frozen release candidate has recorded whole-repository regression and contract checks with honest limits.

Exact baseline commit: `9c5dba929cf705c0cbee307876b5c300d127ba37`.

Exact reviewed candidate **tree**, not commit: `52c1266384b5cdc6a77ef6ef8e454ac9f882dae5`.

Frozen export: `/tmp/iolink-gbcatalog-20261013/review`.

Web gitlink: `39299dfb0d7771690117a2765fb766a9b715f554`.

`git diff baseline candidate` contains exactly one replacement in `internal/gbcatalog/catalog_test.go:88`: fixed `time.Date(2026, 10, 11, 2, 0, 0, 0, time.UTC)` becomes `time.Now().UTC()`. All production blobs, test assertions, fake-clock advancement, and deadline checks are unchanged. An independent audit compared all 1,084 tracked exported blobs to `git ls-tree -r` / `git hash-object`: zero missing or different files. The gitlink was checked separately; frontend/browser behavior is outside this test-only diff. `git diff --check` passes.

The main workspace contains unrelated AGENTS and M7a playback drafts; the export audit and one-line tree diff establish their exclusion. Approval applies to the named candidate, not those drafts.

## Criteria and user outcome review

The following IDs normalize the supplied assignment's success conditions; they are not new repository requirements.

| Criterion | Direct review and evidence | Outcome |
| --- | --- | --- |
| GBC-1: close the 11 fixed-date failures | Independently ran all 11 candidate tests with race/shuffle/count=1, exit 0. Replaced only the fixture through an overlay against the frozen export and independently reproduced all 11 failures, exit 1. | PASS |
| GBC-2: preserve production deadlines, expiry and assertions | Exact one-line diff; `validation.go` still uses `context.WithDeadline`; fake store still requires a future real deadline; expiry still uses `fakeClock.Advance`; stale preservation, cancellation, duplicate/conflict, invalid input, concurrency, store failure assertions remain. | PASS |
| GBC-3: verify bounded candidate regression and contracts, with real DB evidence | `candidate-verify.log` records build/vet/all-package race output with no failing package. `invocation.md` identifies the nonempty dedicated DSN invocation. `catalog-pg-race.log` explicitly shows seven real database tests PASS and no SKIP. Contract output is 96 target operations / 568 synthetic fixtures, accurately described as synthetic. | PASS |
| GBC-4: report exclusions and failed/skipped checks honestly | README preserves failed export-order run and excludes it from success; browser opt-in, actual License delivery, WeChat/hardware excluded; v0.0.31 CI JSON identifies baseline image success and skipped build/integration jobs. | PASS |
| GBC-5: independent same-candidate review | This lane did not edit implementation and independently inspected the exact tree. This is one explicit APPROVE receipt; the release owner must pair it with the other independent lane's same-tree verdict before marking completion. | This lane PASS; dual-receipt closure belongs to owner |

userOutcomeReview: The bounded defect is closed by a change at the shared test fixture. Because production passes an absolute fake-clock-derived deadline to the real Go context timer, a historical fixed date is already expired before the store executes. Anchoring each fixture to the current time gives the existing ten-second deadline its intended meaning. The fake clock remains frozen and explicitly advanced afterward. Independent old/new runs show the difference is observable behavior, not a removed check. Whole-repository results support regression confidence; they do not establish unnamed external end-to-end acceptance. No specific stated success criterion fails.

## Independent reproduction

Candidate command, in frozen export:

```text
go test -race -shuffle=on -count=1 -v ./internal/gbcatalog
shuffle seed 1791896507314462095
11 PASS, exit 0
```

Old fixture command: Python `subprocess.run` fed an overlay JSON through `/dev/stdin` to the same Go command with `-overlay /dev/stdin`, mapping the **export's** absolute `internal/gbcatalog/catalog_test.go` path to `/tmp/iolink-gbcatalog-20261013/catalog_test.old.go`.

```text
shuffle seed 1791896618915273818
11 FAIL, exit 1
```

Old fixture hash `d32ddada64f164dd544c970fc721116522e0156e` independently matches the baseline blob. Candidate test blob is `5995746b1dad521f4390f244bfaa861368a92add`. Root's supplied before-overlay maps the main workspace path and was run there; an unmodified use of that overlay in the export does not switch the fixture. The corrected export mapping above establishes the independent regression reproduction. A first check using the root mapping in the export was green, correctly indicating that no replacement occurred; it is not treated as old-fixture reproduction evidence.

These independent commands and outputs are retained in this lane's tool transcript. The owner's archived `before-race.log` and `after-race.log` provide durable matching old/new results. No additional assertion-pinning test was introduced.

## Direct remove-ai-slops and programming pass

Consulted `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/remove-ai-slops/SKILL.md`, `programming/SKILL.md`, and `programming/references/go/README.md`.

Directly checked the diff, complete changed test file, and production coordinator, contracts, and validation. No excessive/useless new tests, deletion-only tests, tests verifying removal, tautological assertions, implementation-mirroring additions, new parsing/normalization/extraction, speculative helpers, production abstraction, error masking, dependency changes, or logging changes. Existing tests observe catalog results, conflicts, replacement count, deadlines, stale state, cancellation and concurrency. The old-fixture failure / candidate success experiment gives a meaningful regression oracle without a test that merely pins `time.Now`.

The shared fixture is the correct scope for these eleven callers. Clock injection into the coordinator remains intact; no wall-clock read was added to production domain logic. Typed errors and context/cancel ownership remain unchanged. No maintenance burden, false confidence or scope drift was introduced by the diff.

Nonblocking inherited notes: `catalog_test.go` is 297 pure LOC, unchanged in size, exceeding the skills' module-size preference. Existing real deadline coupling leaves tests subject to a process stall exceeding the ten-second deadline; this is visible in the unchanged production/fake-store contract. Neither note violates the bounded assignment, and unrelated refactoring/hardening is not required for approval.

Code-review report coverage: no completed same-candidate code-review report existed in the evidence directory when this lane inspected it; the owner said that independent lane was still running. This lane's direct pass explicitly covers both requested skill perspectives and all requested overfit/slop classes. Missing second-lane prose is not treated as evidence of a code defect or substituted with an unverified success claim. The owner must archive the second independent verdict before declaring dual-review completion.

## Checked artifacts

Paths below are relative to the repository unless absolute:

- `docs/README.md`, `docs/CONTRIBUTING.md`; relevant status/exit criteria in `docs/PLAN.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`.
- `.debug-journal.md` (local notepad): GB catalog hypotheses, unchanged-production experiment, export-order failure, candidate/DB ownership, invocation and scope limits.
- Exact baseline/candidate Git objects; frozen export tracked-blob audit; `internal/gbcatalog/{catalog_test.go,catalog.go,contracts.go,validation.go}`.
- Frozen export `Makefile`, `internal/testdb/testdb.go`, and `internal/gbcatalogpg/*test.go` context. DB helper skips only when DSN is absent; the seven named supplemental tests use the real helper and migrations, including old-observed-at deadline rejection.
- Every supplied file under `docs/evidence/deployment/2026-10-13-gbcatalog-validation/`: `README.md`, `before-race.log`, `after-race.log`, `candidate-verify.log`, `contracts.log`, `docs-tools.log`, `environment.log`, `source-tree.txt`, `export-order-error.log`, `v0.0.31-ci.json`, `v0.0.31-jobs.json`, `invocation.md`, `catalog-pg-race.log`.
- `/tmp/iolink-gbcatalog-20261013/overlay.json`, `catalog_test.old.go` and matching baseline blob.

Environment artifacts identify Go 1.26.8, PostgreSQL 16.15, Timescale 2.30.2 and dedicated container `iolink-validation-pg-20261013` labelled `iolink.task=gbcatalog-20261013`. No production database is represented as test evidence.

## Exact evidence gaps and limits

1. Full-suite output is nonverbose and does not individually enumerate skipped tests. `invocation.md` records the dedicated nonempty DSN, and supplemental verbose catalog persistence results prove this affected adapter's seven real DB cases. This does not prove every unnamed optional external check ran; no such success is claimed.
2. Browser QA is user-owned and opt-in; actual License delivery, WeChat and hardware acceptance are not present. These are correctly excluded and are not requirements of this one-line fixture closure.
3. No completed second independent report was available at this lane's cutoff. This report supplies only this lane's approval, not a claim that dual review has already finished.
4. The original broader user message was supplied as a summary, not a full transcript. The verdict follows the explicit bounded assignment and checked journal/evidence scope, and makes no global M0–M8/R55 completion claim.
5. No active ulw-loop plan exists: `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`. Therefore this report uses the required fallback `.omo/evidence/gbcatalog-validation-gate-review.md`.

No blockers with a violated criterion and evidence pointer were found.
