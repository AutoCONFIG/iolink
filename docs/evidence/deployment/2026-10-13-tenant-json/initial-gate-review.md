# D18 independent final gate review

- Date: 2026-10-13
- Reviewer: `/root/d18_gate_review`; no implementation edits, no delegated review.
- recommendation: **APPROVE**
- blockers: `[]`
- Exact reviewed candidate: tree `386c7d65af727270c879fd0ab7073ad68f5ecaa6`.
- Baseline: commit `d0bef4a4fa0b8212c964e819940d24c1ff5c93a2`.
- Runtime/source export: `/tmp/iolink-d18-20261013/review`.
- Report placement: `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`; this uses the required fallback `.omo/evidence/d18-gate-review.md`.

## originalIntent

Continue testing and fixing existing functions, closing the previously registered D18 defect in platform tenant creation. New M7a/M8 work is deferred. The prior review identified that Gin binding ignored additional object fields and consumed only the first JSON value. The intended fix must reject those malformed requests before creating a business tenant or its audit record, while preserving authorized normal creation and existing isolation/transaction behavior.

## desiredOutcome

An authenticated platform administrator can create an organization with one valid `name` object, including harmless trailing JSON whitespace. Extra fields, a second JSON value, malformed trailing bytes, and invalid request shapes return HTTP 400 without tenant or `tenant.created` writes. Existing authorization, duplicate detection, name rules, and transactional failure behavior continue working. This approval covers D18 software behavior, not full M0–M8, browser acceptance, real License delivery, WeChat, or hardware acceptance.

## Success criteria and userOutcomeReview

The following local IDs identify the existing brief/contract requirements; they do not add new requirements.

| Criterion | Authority | Review result and artifact |
|---|---|---|
| D18.C1 — Reject extra fields and trailing JSON on this existing route | `docs/IMPLEMENTED.md:73`; prior `docs/evidence/deployment/2026-10-08-platform-console/code-review.md:42`; `docs/api/admin-openapi.yaml:157` schema uses `additionalProperties: false` | PASS. Candidate `tenant_admin.go:75` parses a string-valued object, requires exactly one key and nonempty `name`, then requires EOF. The second decode rejects every further value or malformed suffix. Focused real HTTP/PG test passed independently. |
| D18.C2 — Invalid requests leave no partial tenant or audit state | Original assignment; AGENTS security/data integrity requirements | PASS. All nine invalid requests precede the SQL zero-write assertion at `platform_console_test.go:133`. Independent execution passed. Live SQL artifact contains only `accepted-org` and `tenant.created|1`. |
| D18.C3 — Valid creation and existing authorization/duplicate/transaction behavior remain | Original assignment; tenant-create contract and existing scenario | PASS. Valid object with trailing whitespace returns 201; unassigned USER returns 403; duplicate returns 409; audit-trigger failure returns 500 and leaves no tenant/user partial writes. These existing assertions remain in the executed scenario. |
| D18.C4 — Use meaningful regression, real isolated database, and contract/repository verification evidence | AGENTS verification; `docs/CONTRIBUTING.md` | PASS for this fix. Archived red run fails with actual 201 versus expected 400; green and independently repeated scenario use a nonempty dedicated DSN. Full build/vet/race log has 33 tested packages passing. Contract log explicitly describes 96 operations/568 synthetic fixtures, not live acceptance. |
| D18.C5 — Restrict source scope and avoid claiming unrelated completion | Original assignment | PASS. Baseline-to-candidate diff changes only `internal/adminapi/tenant_admin.go` and `internal/core/platform_console_test.go`; docs evidence still says in progress pending reviews. Shared AGENTS/M7a changes are excluded. |

No specific success criterion failure was found. The user receives the requested D18 rejection behavior and a real database regression, without a claim that unrelated external acceptance passed.

## Candidate identity and direct reproduction

Read the complete baseline-to-candidate diff: two files, 39 insertions and 16 deletions. Production change is confined to imports, deleting the now-unused tenant request struct, and replacing permissive binding with strict boundary decoding. Test change adds nine hostile shapes, a tenant/audit zero-write assertion, and whitespace on the valid request; prior assertions are preserved.

Candidate blob identities verified with `git ls-tree` and `git hash-object`:

- `internal/adminapi/tenant_admin.go`: `d87ba637d4d0c3015b473b892c89feb672885987`.
- `internal/core/platform_console_test.go`: `705cb20c5635adcf3156392023e517bdb5235f18`.

Additionally, independently read all recursively tracked blob entries from the candidate tree and recomputed Git blob hashes from the export: **1101 files checked, zero mismatches**. The `web` gitlink is a separate submodule entry, not a tracked blob; no frontend change belongs to this candidate.

Independently executed in the verified export with `IOLINK_TEST_PG_DSN` explicitly set to the dedicated disposable PostgreSQL/Timescale service on loopback port 33123 (credentials omitted from this report):

```text
go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestPlatformConsoleOnboardingAndIsolation$'
-test.shuffle 1791898652400744708
=== RUN   TestPlatformConsoleOnboardingAndIsolation
--- PASS: TestPlatformConsoleOnboardingAndIsolation (1.82s)
PASS
ok git.hyhy.fun/rsplab/iolink/internal/core 2.884s
exit_code: 0
```

The real test uses `httptest.NewServer`, actual login/authentication and service wiring, migrated PostgreSQL storage, and a fresh test database. `internal/testdb/testdb.go` creates a uniquely named database and drops it in `t.Cleanup`; this run did not take the missing-DSN skip branch. Full suite was not repeated because the recorded successful candidate verification plus the directly reproduced affected scenario resolves this change's concrete risk.

## Direct programming and remove-ai-slops pass

Consulted `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/programming/SKILL.md`, its Go README and testing/error-handling/data-modeling references, and `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/remove-ai-slops/SKILL.md`. Applied their review criteria directly; no cleanup was performed because this assignment is read-only.

- Excessive/useless tests: no new suite, fixture subsystem, or helper abstraction. Nine inputs represent distinct boundary classes inside the existing real onboarding scenario; the zero-write assertion observes the security/persistence outcome.
- Deletion-only/removal-pin tests: none. Tests send requests and inspect HTTP/SQL behavior rather than checking that a type, phrase, import, or call was removed.
- Tautology/implementation mirroring: none. Expected status values and the independently queried database count do not derive from the decoder implementation. Actual red evidence confirms the test can fail for the reported bug.
- Unnecessary extraction/parsing/normalization: none. The standard-library decoder is necessary at the HTTP trust boundary because the old Gin decoder permits the documented defect. No parser is extracted, no new normalization is added, and application name handling is preserved.
- Boundary/typed error/context practice: request parsing remains in the HTTP adapter, every decoder error leads to 400, request context still reaches the existing store, and sentinel business errors retain `errors.Is` dispatch. No provider SDK or new production side effect is introduced.
- Obvious comments, dead code, duplicate wrappers, speculative abstractions, hidden costs: none introduced. Deleting the unused request struct is appropriate. Production module has 238 pure LOC and test module 211, below the consulted 250 LOC ceiling.
- Existing long scenario style is retained. Named per-input subtests could improve failure diagnosis because the helper reports the route rather than the failing body, but this is a NOTE: it does not violate D18 behavior or evidence criteria. No unrelated refactor is justified for this fix.
- Strict skill tool suggestions absent from the repository evidence (nilaway, expanded lint bundles, etc.) are not newly imposed D18 success criteria. Repository-required build/vet/race and contract evidence were inspected; actual affected HTTP/PG behavior was reproduced.

## Code review report coverage

At the time of this receipt, the concurrent independent code-review lane's planned `docs/evidence/deployment/2026-10-13-tenant-json/code-review.md` did not yet exist. Read the evidence directory and prior originating code review instead; the direct pass above explicitly covers both requested skill perspectives and all specified overfit/slop classes. Missing concurrent report is therefore not a D18 behavior blocker under the gate instructions. The root must obtain the other independent same-tree terminal approval before marking the milestone complete. This receipt is one of the two reviews, not a claim that both have finished.

## Checked artifact paths

Authoritative context:

- `docs/README.md`, `docs/CONTRIBUTING.md`.
- Relevant D18/tenant/platform entries in `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`.
- `docs/api/admin-openapi.yaml` tenant-create request/response contract.
- `docs/evidence/deployment/2026-10-08-platform-console/code-review.md` originating defect.
- `.debug-journal.md` owner notepad.

Candidate sources and wiring (read from the verified export):

- `internal/adminapi/tenant_admin.go`, `internal/core/platform_console_test.go`.
- `internal/adminapi/server.go` route registration; `internal/core/platform_admin.go` existing validation and atomic tenant/audit transaction; `internal/testdb/testdb.go` isolation and cleanup.

Executor evidence directory `docs/evidence/deployment/2026-10-13-tenant-json/`:

- `README.md` (scope, command/environment record and manual QA matrix).
- `source-tree.txt`, `environment.log`.
- `before-race.log`, `after-race.log`, `red-embed-setup-error.log`.
- `candidate-verify.log`, `contracts.log`, `docs-tools.log`.
- `live-unknown.http`, `live-trailing.http`, `live-valid.http`, `live-db.txt`, `live-stderr.log`.

Inspected raw response artifacts: unknown and trailing cases return 400 with `{"error":"invalid_request"}`, valid case returns 201 with active tenant `accepted-org`. Live server log has corresponding generated request IDs and shutdown entries. It does not expose request bodies, credentials or JWTs. Initial missing-embed failure remains explicitly failed, never counted as verification.

## Exact evidence gaps and notes

1. Concurrent D18 code-review report unavailable when this receipt was written. Direct skill-perspective coverage supports this independent approval; second same-tree approval remains a root completion prerequisite.
2. Full `make verify` log is nonverbose and does not individually list opt-in skips or show the DSN assignment. Its successful package outputs support the recorded full run, while this review's explicitly configured, verbose, real HTTP/PG run proves the affected integration actually executed. No claim that every opt-in/external test passed.
3. Live artifacts retain responses/DB/logs, but not a full executable server/curl command transcript with every config flag. The README records the binary build, ports, service and scenario. Direct real HTTP/PG reproduction independently supports D18; no unsupported native launch detail is asserted here.
4. No browser/UI change; browser acceptance belongs to the user. Real License delivery, WeChat and hardware were not executed and remain outside this D18 approval. No broad R55/full-release approval is granted.

Every gap above is bounded to what the artifacts actually support; none demonstrates failure of a stated D18 success criterion.
