# D18 code review — frozen tree 386c7d65af727270c879fd0ab7073ad68f5ecaa6

Reviewed: 2026-10-13 13:43 UTC  
Independent reviewer: /root/d18_code_review  
Source/Git review: read-only; no implementation edits or delegation.

## Verdict

- Exact-tree verdict: **APPROVE**
- codeQualityStatus: **WATCH**
- recommendation: **APPROVE**
- blockers: **[]**
- CRITICAL: none.
- HIGH: none.
- MEDIUM: one nonblocking regression coverage gap.
- LOW: none.

This receipt applies only to tree **386c7d65af727270c879fd0ab7073ad68f5ecaa6** and does not approve any later test or source change.

## Scope and requirements

Baseline commit: d0bef4a4fa0b8212c964e819940d24c1ff5c93a2. Inspected the complete Git diff: exactly two files, 39 insertions and 16 deletions, including import regrouping:

- internal/adminapi/tenant_admin.go — blob d87ba637d4d0c3015b473b892c89feb672885987
- internal/core/platform_console_test.go — blob 705cb20c5635adcf3156392023e517bdb5235f18

Goal: close existing D18, accepting only one tenant-create JSON object with the required exact name field and no additional properties or trailing JSON, while preserving authorization, name rules, transaction/audit integrity, and accepted trailing whitespace. No new M7a functionality is in this diff.

Consulted docs/README.md and docs/CONTRIBUTING.md, then the applicable PLAN, PLAN-DETAILS, EXTENSIONS, ACCEPTANCE and IMPLEMENTED requirements. docs/api/admin-openapi.yaml:156–167 declares the platform-admin POST /tenants request as an object, name required, string length 1–128, and additionalProperties: false. D18 is registered at docs/IMPLEMENTED.md:73; its status is still pending in this reviewed tree.

Verified the export /tmp/iolink-d18-20261013/review against git ls-tree --full-tree -r for the requested tree: **1,101 tracked blobs matched their Git object IDs, zero mismatches**. The only non-blob entry is the unchanged web gitlink; frontend internals are outside this review. Unrelated root AGENTS/M7a drafts were excluded.

## Findings by severity

### CRITICAL

None.

### HIGH

None.

### MEDIUM — closed-object guard lacks a direct regression case

Reference: internal/core/platform_console_test.go:120; production guard at internal/adminapi/tenant_admin.go:77.

The only test containing both a valid name and an additional property is:

    {"name":"invalid-org","unexpected":true}

The new map[string]string decoder rejects its boolean value with a JSON UnmarshalTypeError before the key-count rule matters. Consequently all nine added malformed cases would still pass if len(req) != 1 were removed: the other cases exercise missing/exact-case name, null/array shape, or trailing content.

The present production implementation correctly rejects extra string properties, so this is not an observed correctness or security failure and is not an approval blocker. However the test does not directly lock the principal additionalProperties:false rule. Replace this case with an additional string value, for example unexpected:"extra", to make an omitted key-count guard regress visibly. Do not add a duplicate test solely to retain the boolean example.

Confirmation: inspected the actual Go 1.26.8 standard-library source at /home/yun/.local/go/src/encoding/json/decode.go:909–930, where decoding a boolean into a string records UnmarshalTypeError.

### LOW

None.

## Correctness and regression assessment

- createTenant authenticates its platform-admin requirement before decoding (tenant_admin.go:66–68), preserves the capability failure response (70–73), and performs no store call until the complete input passes (75–85).
- A typed string map plus exact name lookup rejects wrong casing, absent/null name, non-object input and unknown properties. The second decode must return io.EOF, rejecting another object, scalar or garbage while allowing whitespace.
- The same name and actor are passed into the existing CreateTenant capability. Its unchanged platform actor check, name trimming/reserved-name/length rules, authorization within a transaction, tenant insert and tenant.created audit insert remain at internal/core/platform_admin.go:42–72.
- Conflict, invalid-name, forbidden and failure status mappings are preserved. No new logging, provider dependencies, public API signatures, SDK coupling, normalization or abstractions are introduced.
- Existing positive/auth/isolation/conflict/rollback assertions were not removed or weakened. The positive request now explicitly includes trailing newline/tab/space.
- The new database assertion checks the observable persistence outcome: zero business tenants plus zero tenant.created audit records before the valid create. Counts are nonnegative, so their sum cannot conceal a partial write.
- Standard-library duplicate-member semantics are used. D18 does not add a distinct duplicate-key or body-limit requirement; these are not claimed as newly implemented defenses.

## Skill-perspective check

**Ran before judging tests and maintainability.** Loaded:

- /home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/remove-ai-slops/SKILL.md
- /home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/programming/SKILL.md
- programming/references/go/README.md and programming/references/go/testing.md

remove-ai-slops perspective: **no violation identified**. The tests are behavioral HTTP/real-DB tests, not deletion-only, requested-removal, prose/prompt, tautological, or implementation-constant tests. They demonstrably fail against the old handler. Required JSON parsing is at the user-input boundary and directly serves D18. No needless extraction, parsing/normalization, duplicate helper or speculative abstraction was added.

programming perspective: **the MEDIUM coverage weakness above remains**; no brittle prompt test, untyped escape hatch or needless production complexity was found. map[string]string stays at the HTTP boundary and only a string name is passed onward. Existing multi-step onboarding structure was retained rather than expanded into new abstractions. Pure LOC is 238 for tenant_admin.go and 211 for platform_console_test.go, both in the skill warning band below the 250 ceiling; a future expanding edit should consider a responsibility-based split.

The review role forbids implementation changes, so the cleanup workflow was used as a review perspective only.

## Verification and evidence inspected

Evidence root: docs/evidence/deployment/2026-10-13-tenant-json/

- README.md, source-tree.txt and environment.log: candidate, commands and environment; isolated PostgreSQL 16.15 / Timescale 2.30.2; Go 1.26.8. Independently confirmed Go version and container ownership label iolink.task=d18-20261013 with docker inspect.
- before-race.log: actual POST /tenants returned 201 versus expected 400, with FAIL. Verified red tree dddaf3065da9f66b270658697bdfd8faf06d8b68 changes only the test file: old handler blob a280261591e48afae634d1eec14b4f096726fc3a, same new test blob as this candidate.
- red-embed-setup-error.log: missing internal/web/dist setup failure; correctly retained and not counted as a behavioral reproduction or a pass.
- after-race.log: owner HTTP/PG scenario PASS. Independently replayed below.
- candidate-verify.log: inspected complete build/vet/race output; 33 tested packages report ok. The full suite was not independently rerun. Nonverbose output does not establish that every opt-in/external test ran.
- docs-tools.log and contracts.log: tooling installation completed; 96 operations / 568 synthetic request-response fixtures PASS. This is static contract evidence and does not substitute for live handler tests.
- live-unknown.http and live-trailing.http: 400 invalid_request; live-valid.http: 201 accepted-org; live-db.txt: only accepted-org and tenant.created|1. Inspected live-stderr.log shutdown records. These are executor-native service artifacts; this reviewer did not repeat the standalone binary/curl run.
- Root notepad .debug-journal.md was inspected and is consistent with the frozen source, red/green logs and exclusions.

Browser acceptance remains user-owned. Real WeChat, hardware and actual License delivery are deferred and are not counted as passed. No misleading success claim without artifact paths was found.

### Independent focused replay

Working directory: /tmp/iolink-d18-20261013/review  
Environment: IOLINK_TEST_PG_DSN explicitly set to the owned disposable instance on 127.0.0.1:33123/iolink, password omitted here. The test creates a unique database and drops it through t.Cleanup; missing DSN would skip, which did not occur.

Command:

    go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestPlatformConsoleOnboardingAndIsolation$'

Actual output excerpt:

    -test.shuffle 1791898761151614466
    === RUN   TestPlatformConsoleOnboardingAndIsolation
    --- PASS: TestPlatformConsoleOnboardingAndIsolation (1.95s)
    PASS
    ok  	git.hyhy.fun/rsplab/iolink/internal/core	3.019s

Process exit code: **0**. Result: **PASS**, no SKIP. The replay exercised the nine rejected inputs, zero tenant/audit writes, valid trailing whitespace, existing 403/409 behavior, transaction rollback and tenant isolation.

## Artifact placement

Ran omo-agent-toolkit ulw-loop status --json in the root repository. It returned ULW_LOOP_PLAN_MISSING for this session; no currentAttemptDir exists. Used the required fallback .omo/evidence/D18-code-review.md.

Approval requires no fix for the current tree; the recorded MEDIUM concern should be closed with the narrow test-input correction before claiming that the key-count guard itself has regression coverage.
