# D18 final independent code review

Date: 2026-10-13. Reviewer: `/root/d18_release_code_review`. Read-only review; the reviewer did not participate in implementation or change source/Git state. Only this report and review evidence files were written.

**Verdict: APPROVE** for final tree `8e856090c601c4900b9b6a60b68363868ff1f319` against baseline `d0bef4a4fa0b8212c964e819940d24c1ff5c93a2`.

- `codeQualityStatus`: **CLEAR**
- `recommendation`: **APPROVE**
- `reportPath`: `.omo/evidence/D18-final-code-review.md`
- `blockers`: **[]**

`omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`. This final review uses the required `.omo/evidence/<goal>-code-review.md` fallback with goal `D18-final`, preserving the historical initial D18 review.

## Goal and reviewed inputs

Close the existing platform tenant creation request to an object containing only a nonempty `name` string, reject any trailing JSON or garbage before database writes, and retain legitimate trailing whitespace, authorization, name rules, conflict behavior, and atomic tenant/audit writes. This is D18, within the existing HTTP contract and the common negative gate in `docs/ACCEPTANCE.md:24`; it does not implement a new M7/M8 feature.

Consulted `docs/README.md`, `docs/CONTRIBUTING.md`, relevant plan/acceptance/state sections, and `docs/api/admin-openapi.yaml:158`–`178`. The POST request schema requires `name`, a string of length 1–128, and `additionalProperties: false`. The state ledger still records D18 as pending; it does not prematurely claim final approval.

Changed files are exactly `internal/adminapi/tenant_admin.go` and `internal/core/platform_console_test.go`. The full baseline diff is captured at `.omo/evidence/D18-final-code-review.diff`. The final delta from initial tree `386c7d65af727270c879fd0ab7073ad68f5ecaa6` is exactly the extra-field fixture changing from boolean `true` to string `"extra"`; production is identical. Inputs also included `.debug-journal.md` and `docs/evidence/deployment/2026-10-13-tenant-json/README.md` with its referenced artifacts.

The frozen export `/tmp/iolink-d18-20261013/final` matches all 1,101 tracked blobs in the final tree, with zero mismatches. The `web` gitlink was excluded from file hashing. Identity evidence is `.omo/evidence/D18-final-code-review-identity.log`; the reviewed Go blobs are `d87ba637d4d0c3015b473b892c89feb672885987` and `2e5c5ca86794c499202fdf75023c50d9f704f1d6`.

## Findings by severity

- **CRITICAL:** None.
- **HIGH:** None.
- **MEDIUM:** None.
- **LOW:** None.

## Correctness, scope, and maintenance assessment

At `internal/adminapi/tenant_admin.go:75`–`85`, the standard decoder parses into a local `map[string]string`. The decode error, key count, and exact `name` lookup reject incorrect top-level shapes, wrong value types, missing/incorrect-case keys, and extra keys. The second decode must return EOF, so a second value or garbage is rejected while trailing whitespace remains valid. All rejection paths return before `CreateTenant`.

Platform-admin and capability checks remain before parsing. Existing store/error behavior remains intact. Inspection of `internal/core/platform_admin.go:42`–`73` confirms name rules and reauthorization still precede the tenant insert, and tenant plus audit writes still share one transaction. The handler passes only the name string to that existing seam. No provider coupling, speculative helper, dependency, normalization, logging exposure, or external-contract change was introduced.

At `internal/core/platform_console_test.go:119`–`137`, the nine negative request classes execute through a real HTTP server and real PostgreSQL/Timescale storage. The SQL assertion checks the required absence of both business tenants and tenant-created audit rows. The accepted request exercises whitespace after the object. Existing 403, 409, reserved-name, rollback, membership, and resource-isolation checks remain intact. This extends the repository's existing onboarding scenario with observable behavior rather than adding duplicate test infrastructure.

## Required skill-perspective check

**Ran both checks** after reading `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/remove-ai-slops/SKILL.md` and `programming/SKILL.md`, plus programming's Go README and testing reference.

- **remove-ai-slops:** No diff violation found. The new tests are neither deletion-only nor tests of a requested removal, tautologies, prose pins, or mirrors of implementation constants. HTTP status and database side effects are independently observed. Parsing and EOF enforcement are necessary at this untrusted HTTP boundary and directly implement the goal; no unnecessary production extraction or normalization exists.
- **programming:** No diff violation found. `map[string]string` is typed and stays local to the HTTP boundary; no bare `any`/`interface{}` value is passed into the application. No new escape hatch, needless abstraction, brittle prompt assertion, or implementation-mirroring test exists. The amended fixture is capable of failing for the closed-key regression.

Measured pure LOC: handler 238 and test file 211, both below the 250 ceiling and in the skill's warning band. Future growth should consider separating the specific boundary responsibility; a refactor is not required to correct this bounded change. Repository verification is the established build/vet/race pipeline; this review does not claim execution of an unconfigured golangci/nilaway/goleak bundle.

## Verification and evidence

Independently executed, from the verified final export, with a nonempty DSN pointing to the dedicated loopback `127.0.0.1:33123/iolink` Timescale instance:

```text
go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestPlatformConsoleOnboardingAndIsolation$'
```

**PASS, exit 0, no skip**, test 1.76s / package 2.831s. Complete command, environment description with credentials omitted, output, and exit code: `.omo/evidence/D18-final-code-review-replay.log`. The fixture creates and cleans its own test database, and the HTTP server uses an ephemeral port. `git diff --check` for the baseline/final trees also returned exit 0.

Inspected the root's permanent evidence rather than assuming the summary was accurate:

- `final-race.log`: final owner scenario PASS.
- `final-old-handler-race.log`: final tests against the old handler fail with actual 201 versus expected 400.
- `closed-key-mutation.log`: removing only the key-count guard fails with actual 201 versus expected 400. Independently inspected both overlay maps and verified the old source exactly equals baseline and the mutant exactly equals final with only `len(req) != 1` removed; details at `.omo/evidence/D18-final-code-review-overlay-identity.log`.
- `candidate-verify.log`: initial candidate full build/vet/race log shows 33 tested packages passing. This is earlier-snapshot evidence; production is identical and the one changed test fixture was independently replayed on the final tree. Nonverbose output is not used to claim that every opt-in test ran.
- `contracts.log`: 96 operations / 568 synthetic fixtures PASS, explicitly distinguished from live handler acceptance.
- `live-unknown.http`, `live-trailing.http`, `live-valid.http`, and `live-db.txt`: initial candidate native service responses are 400/400/201, with only `accepted-org` and one `tenant.created` audit row.

All root log names above are under `docs/evidence/deployment/2026-10-13-tenant-json/`. Native curl and the full suite were not rerun by this reviewer; their source identity and artifacts were inspected. Browser acceptance belongs to the user, and actual License/WeChat/hardware acceptance is not claimed here. This is one independent approval of the final D18 tree; the project still requires its second independent final approval before recording completion.
