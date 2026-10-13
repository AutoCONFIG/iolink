# D18 fresh release gate review

- Date: 2026-10-13
- Reviewer: `/root/d18_release_gate_review`; independent, no implementation/Git edits, no delegated review.
- recommendation: **APPROVE**
- blockers: `[]`
- Exact final source tree: `8e856090c601c4900b9b6a60b68363868ff1f319`.
- Baseline: `d0bef4a4fa0b8212c964e819940d24c1ff5c93a2`.
- Verified export: `/tmp/iolink-d18-20261013/final`.
- Notepad: `.debug-journal.md`.
- Placement: `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`; no `currentAttemptDir` exists. This is the required `.omo/evidence/<goal>-gate-review.md` fallback for goal D18.

## originalIntent

The user wants testing and fixes to existing functionality. This assignment closes registered defect D18: platform tenant creation used Gin binding that ignored additional properties and consumed only the first JSON value. New M7a/M8 work is deferred; browser acceptance belongs to the user; actual License delivery, WeChat and hardware acceptance remain deferred. The intended fix rejects malformed tenant creation requests before persistence, preserving normal creation, authorization, business name rules and transaction behavior.

## desiredOutcome

An authenticated platform administrator submits one object containing a valid `name` string and receives 201. Trailing JSON whitespace is accepted. Additional fields, subsequent JSON values, malformed trailing bytes and invalid shapes receive 400 without business tenant or `tenant.created` audit writes. Existing 403 authorization, 409 duplicate behavior, business name validation, rollback and tenant isolation continue to work.

## userOutcomeReview

The following criterion IDs label the stated brief and existing contract; they add no new requirements.

| Criterion | Authority | Result and evidence |
| --- | --- | --- |
| D18.C1 — Reject extra fields and trailing JSON on the existing tenant-create route | `docs/IMPLEMENTED.md:73`, originating `docs/evidence/deployment/2026-10-08-platform-console/code-review.md:42`, `docs/api/admin-openapi.yaml:157` POST /tenants closed object | PASS. `internal/adminapi/tenant_admin.go:75` uses a typed string map, exact one-key/nonempty `name` check, then requires EOF. Direct real HTTP/PG replay passed. Final extra-string regression catches removal of the key-count guard according to `closed-key-mutation.log`. |
| D18.C2 — Rejected input leaves no partial business tenant or audit state | Assignment; AGENTS safety/data integrity requirements | PASS. Nine rejected requests precede the real SQL zero-write assertion at `internal/core/platform_console_test.go:133`; direct execution passed. Native live DB artifact shows only the accepted organization and one audit event. |
| D18.C3 — Preserve normal creation and prior authentication, business name, conflict, transaction and isolation behavior | Assignment; existing route contract and onboarding scenario | PASS. Direct execution included valid trailing whitespace/201, unauthorized USER/403, duplicate/409, reserved name/400, audit-trigger failure/500 with no partial tenant/user rows, membership and resource isolation. Prior assertions remain intact in the diff. |
| D18.C4 — Meaningful regression, real isolated database execution and repository/contract evidence | AGENTS verification; `docs/CONTRIBUTING.md` | PASS for D18. Original and final old-handler logs fail with 201 versus 400; final mutation log distinguishes the closed-key rule. Direct race/shuffle replay used explicit dedicated DSN and did not skip. Initial unchanged-production full build/vet/race log and static contract log were read; final affected integration was reproduced. |
| D18.C5 — Keep fix scoped and avoid unrelated acceptance claims | Assignment | PASS. Baseline-to-final source diff contains only two files, 39 additions/16 deletions. Initial-to-final changes only one test input value; production blob is identical. Evidence README still says in progress pending final reviews; no browser/external/full M0–M8 acceptance is claimed. |

No stated D18 criterion fails. This approval is one independent review receipt for the final tree; root must archive the second nonediting, same-tree explicit approval before recording milestone completion.

## Source identity and independent reproduction

Read the complete baseline-to-final diff and surrounding production/test flow. Recomputed Git blob SHA-1 from every recursively tracked blob in the final export: **1,101 checked; zero mismatches**. The unchanged `web` gitlink is a nonblob submodule entry, not included in that count. Verified changed file identities against `git ls-tree` and `final-source-tree.txt`:

- `internal/adminapi/tenant_admin.go`: `d87ba637d4d0c3015b473b892c89feb672885987`.
- `internal/core/platform_console_test.go`: `2e5c5ca86794c499202fdf75023c50d9f704f1d6`.

`git diff --exit-code 386c7d65af727270c879fd0ab7073ad68f5ecaa6 8e856090c601c4900b9b6a60b68363868ff1f319 -- internal/adminapi/tenant_admin.go` returned 0. The only initial-to-final diff changes `unexpected:true` to `unexpected:"extra"` in the existing malformed table. Initial reviews of tree `386c7d6` were inspected as context, never counted as final-tree approvals.

Independently executed in the verified final export with `IOLINK_TEST_PG_DSN` explicitly set to the dedicated disposable instance at `127.0.0.1:33123/iolink` (credentials omitted from this report):

```text
go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestPlatformConsoleOnboardingAndIsolation$'
-test.shuffle 1791899715495757908
=== RUN   TestPlatformConsoleOnboardingAndIsolation
--- PASS: TestPlatformConsoleOnboardingAndIsolation (1.82s)
PASS
ok git.hyhy.fun/rsplab/iolink/internal/core 2.884s
exit_code: 0
```

No SKIP occurred. Confirmed live container ownership `/iolink-d18-pg-20261013`, label `iolink.task=d18-20261013`, status running. `internal/testdb/testdb.go` creates a uniquely named database and drops it in `t.Cleanup`; this run had no cleanup error. The test uses real Gin HTTP via `httptest.NewServer`, actual login and authorization, migrations, real service wiring and PostgreSQL persistence. It resolves the changed behavior directly. Full suite was not repeated, as requested.

The store call still occurs only after both decoding checks. Auth/capability checks precede parsing. `internal/core/platform_admin.go:42` is unchanged: actor authorization, trimming/name rules, tenant insert and audit insert remain within the same transaction. Errors retain `errors.Is` mappings. Route search shows only the platform route registers this handler; there is no unpatched sibling route using this shared handler.

## Direct programming and remove-ai-slops pass

Loaded and applied `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/remove-ai-slops/SKILL.md`, `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/programming/SKILL.md`, Go README, testing, error-handling and data-modeling references. Used them as read-only review perspectives; no cleanup workflow or implementation edits were performed.

- **Excessive/useless tests:** none introduced. Nine input classes exercise the changed HTTP trust boundary inside the existing scenario. There is no additional fixture framework, test suite, sleep or abstraction. Database assertions observe the requested no-write outcome. Their two nonnegative counts cannot cancel to hide a write.
- **Deletion-only/requested-removal tests:** none. No source text, removed type, phrase, import or call is pinned. Tests send requests and inspect HTTP/SQL results.
- **Tautology/implementation mirroring:** none. Expected status and SQL absence are independently specified outcomes, not calculated from production decoding. Archived old-handler and key-count mutation failures demonstrate distinguishability. Changing the extra field to a string closes the initial test weakness without a duplicate case.
- **Unnecessary production extraction/parsing/normalization:** none. Standard-library decoding is required at the HTTP boundary to enforce the existing closed-object/one-value behavior. It is not extracted into a single-use helper; business name normalization remains unchanged. The string map stays local to the adapter, and only the name string crosses the existing store seam; no `any` escape hatch appears.
- **Comments/dead code/defense:** no new obvious comments or debug leftovers. Removed request struct is unused after the decoder change. Input error checks are load-bearing boundary checks. Error handling, request context and capability fail-closed behavior are retained.
- **Complexity/abstraction/boundaries/duplication/performance:** no new speculative interfaces, wrappers, dependencies, provider SDK use, domain/provider coupling, duplicated parser or costly loop. The few early returns implement the existing route contract. Handler and tests follow the established surrounding pattern.
- **Programming maintenance notes:** the existing narrative scenario could name its malformed rows for clearer failures; helper failure output currently identifies the route/status rather than the body. This is a NOTE, not a D18 criterion failure. Broader skill suggestions for code generation, domain smart constructors or additional lint tools are not an invitation to refactor unchanged project architecture in this narrow fix.

No slop or programming finding creates false confidence, maintenance burden or scope drift that violates a stated D18 criterion.

## Code review report coverage

Read `.omo/evidence/D18-code-review.md` and archived `initial-code-review.md`. They explicitly include programming/remove-ai-slops perspectives and cover excessive/useless, deletion-only, requested-removal, prose/prompt, tautological, implementation-pinning tests and unnecessary parsing/normalization/extraction. They identify the original boolean-extra coverage weakness; final source and mutation evidence address that exact concern. Their approval is bound to the initial tree and does not approve this final tree.

The fresh concurrent final code lane report `docs/evidence/deployment/2026-10-13-tenant-json/code-review.md` was not yet present at this receipt. Inspected the evidence directory before judgment. My direct final-tree pass above supports criterion completion under the gate instructions; report absence is not independently a D18 behavior failure. Root confirms it will obtain/archive the other same-tree terminal approval before completion. Do not treat this section as a second approval.

## Checked artifact paths

Authoritative/context paths:

- `docs/README.md`, `docs/CONTRIBUTING.md`; relevant platform/tenant/R01/R02/R36/R37/R55 sections in `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`.
- `docs/api/admin-openapi.yaml`, `docs/evidence/deployment/2026-10-08-platform-console/code-review.md`.
- `.debug-journal.md`, `.omo/evidence/D18-code-review.md`, `.omo/evidence/d18-gate-review.md` (initial approval context only).

Final verified export sources:

- `/tmp/iolink-d18-20261013/final/internal/adminapi/tenant_admin.go`.
- `/tmp/iolink-d18-20261013/final/internal/core/platform_console_test.go`.
- `/tmp/iolink-d18-20261013/final/internal/adminapi/server.go`.
- `/tmp/iolink-d18-20261013/final/internal/core/platform_admin.go`.
- `/tmp/iolink-d18-20261013/final/internal/testdb/testdb.go`.

Executor directory `docs/evidence/deployment/2026-10-13-tenant-json/`:

- `README.md` (brief, environment, commands and manual QA matrix).
- `source-tree.txt`, `final-source-tree.txt`, `environment.log`.
- `before-race.log`, `after-race.log`, `red-embed-setup-error.log`, `final-race.log`, `final-old-handler-race.log`, `closed-key-mutation.log`.
- `candidate-verify.log`, `contracts.log`, `docs-tools.log`.
- `live-unknown.http`, `live-trailing.http`, `live-valid.http`, `live-db.txt`, `live-stderr.log`.
- `initial-code-review.md`, `initial-gate-review.md`.

Raw native response artifacts show extra input 400, trailing object 400 and valid 201/accepted-org; native SQL shows only accepted-org and `tenant.created|1`. Matching server-generated request IDs appear in the diagnostic log with constant messages/route templates and shutdown records. No request body/JWT/secret was logged in these artifacts. Static contracts explicitly report 96 operations/568 **synthetic** fixtures; they do not certify live behavior. Initial setup failure due to missing embedded frontend is clearly retained as failure, not counted as red reproduction or PASS.

## Exact evidence gaps / notes

1. Fresh concurrent final code report absent at this receipt. Direct final pass is complete; this receipt cannot fulfill both independent approvals alone.
2. Full-suite and static-contract evidence belongs to the initial tree. Verified production is byte-identical; final delta changes only a test fixture. Final actual affected race scenario was independently reproduced. No full final-suite replay is claimed.
3. Full `candidate-verify.log` is nonverbose and does not show its DSN assignment or enumerate optional skips. README records explicit DSN; independently configured focused integration and native artifacts establish D18 real database execution. No claim that every opt-in/external test ran.
4. Final build/vet success is recorded in README/tool execution history, not a separate raw build/vet file. Initial full build/vet raw log is present and final production is unchanged. Final focused replay builds the relevant packages. This bounded recording gap does not demonstrate a stated criterion failure.
5. Native server evidence retains responses/SQL/diagnostics and README command summary, not a complete server/curl transcript of every flag. I independently reproduced the requested HTTP/DB behavior through the final real server route in the owner test; I did not rerun the standalone binary/curl launch.
6. Browser, actual License delivery, WeChat and hardware were not executed. They are outside D18 approval and remain deferred/external-blocked where applicable.

No gap above proves failure of a stated D18 criterion. No broad R55 or full M0–M8 release approval is granted by this receipt.
