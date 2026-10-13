# Independent code quality review — initial existing HTTP candidate

- Date: 2026-10-14.
- Reviewer: `/root/final_existing_code_review`, read-only leaf; no delegation and no implementation, test, or index edits.
- Verdict: **REJECT**.
- `codeQualityStatus`: **WATCH**.
- `recommendation`: **REQUEST_CHANGES**.
- `reportPath`: `.omo/evidence/final-existing-code-review.md`.
- Sole blocking issue: **C-DIAG**, fabricated camera capability error diagnostics in the initial candidate. Other findings below are nonblocking.

## Candidate and bounded goal

Reviewed tree: `4a59e953f9ad7a336babe45d90d28258fdcc072a`; base: `3635fd6`. I inspected the diff and `/tmp/iolink-final-existing-http-20261014`, a tracked export of that tree. An independent SHA-1 Git blob comparison checked **1,417 tracked blobs with zero mismatches**. The extra ignored `web/dist` needed for embedding was outside the reviewed tracked diff. Dirty AGENTS and playback drafts were excluded.

Goal: verify the existing five API Key operations, five signed Open reads, one device restore, two Setup operations, and seven camera operations: **20 distinct physical operations**. Restore is validated against both admin and license declarations but counted once. This review does not approve all 96 contract operations, completion of R02, R43, R55, M7/M8, the whole project, browser behavior, external hardware/WeChat, or License delivery. Camera Availability is absent in production; POST/PUT 503 and no camera business writes are the claimed behavior. Local signed License fixtures supply prerequisites only.

Authority consulted: `docs/README.md`, `docs/CONTRIBUTING.md`, relevant PLAN/PLAN-DETAILS/EXTENSIONS/ACCEPTANCE provisions, and affected version contracts. No current-scope notepad was supplied; the parent confirmed none exists. `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`, so the requested `.omo/evidence/<goal>-code-review.md` fallback is used.

Changed source/contract scope inspected:

- `docs/api/admin-openapi.yaml`, `license-openapi.yaml`, `open-openapi.yaml`.
- `internal/adminapi/api_key_input.go`, `api_keys.go`, `cameras.go`; `internal/appapi/cameras.go`.
- `internal/core/api_key_crypto.go`, `open_auth.go`; `internal/domain/api_keys.go`.
- `internal/openapi/http_errors.go`, `server.go`; `internal/setupapi/input.go`, `server.go`.
- `internal/core/camera_contract_{errors,helpers}_test.go`, `camera_contract_test.go`.
- `internal/core/key_open_contract_{boundary,capability,database,fixture,restore,success}_test.go`.
- `internal/core/setup_contract_{fixture,http}_test.go`.
- `scripts/validate_http_response.py`, `test_validate_http_response.py`.
- The initial candidate's Setup/camera evidence and README under `docs/evidence/deployment/2026-10-14-final-existing-http/` were also inspected. Later root/Key artifacts are supplemental evidence, not blobs in the pinned candidate.

The complete diff is reproducible with `git diff 3635fd6 4a59e953f9ad7a336babe45d90d28258fdcc072a`. No verdict here applies to subsequent dirty fixes or a replacement tree.

## Findings by severity

### CRITICAL

None found in the bounded diff.

### HIGH

None found in the bounded diff. The blocker below is MEDIUM because it breaches the explicit diagnostic criterion without a demonstrated access-control, persistence, or response-classification failure.

### MEDIUM

**C-DIAG — blocking: an absent camera capability is logged as an actual provider error.**

Locations in the pinned tree: `internal/adminapi/cameras.go:69` and `internal/appapi/cameras.go:80`; absent-dependency callers include admin `cameras.go:121` and app `cameras.go:132`.

The new `cameraFailure` funnels `camera.ErrUnavailable` through `slog.Any("err", err)` and the message "HTTP camera provider operation failed". When `Cameras` is nil, the handler manufactured that sentinel solely to express 503; no provider call failed. The dump therefore emits `err=*errors.errorString` for capability absence. This conflicts with the requested distinction between constant capability diagnostics and real failures. Responses remain safe 503 and fail closed; this is a diagnostic accuracy issue, not a claim of broken camera writes.

Artifact: `.omo/evidence/camera-capability-before.log` records actual authenticated TCP/PG requests on both user and mini surfaces returning 503 but failing the no-fabricated-error diagnostic assertion; terminal FAIL, core 6.918s. I inspected the added diagnostic scenario and confirmed the finding independently against the pinned source. That later scenario is not part of the 4a59 tree and was executed by the root, not by this reviewer.

Required before approval: emit a constant, correlated capability diagnostic without an invented `err` field for `camera.ErrUnavailable`, preserve the safe 503/no-write behavior, and rerun both surfaces plus actual SQL-failure correlation/rollback on the newly frozen tree. Root has started a fix outside this candidate; it cannot make the old tree approved.

**C-TEST — nonblocking: the camera SQL diagnostic assertion pins a Go runtime implementation type.**

Location: `internal/core/camera_contract_errors_test.go:61` and `:68`.

The SQL/correlation/rollback test is meaningful, but requiring the exact diagnostic class `*errors.errorString` overconstrains the Go error representation. A safe typed or wrapped camera error could preserve response, privacy, correlation, and rollback while failing this predicate. This is implementation coupling under the programming/remove-ai-slops perspectives. Prefer the stated safe-category/privacy/correlation contract rather than that runtime type name. Root's subsequent change addresses this, but it is outside 4a59.

### LOW

**L-INPUT — missing direct regression cases for part of the new Key parser.** `internal/core/key_open_contract_boundary_test.go:11` includes unknown/nested/null/duplicate top-level/case/trailing inputs, but no raw invalid UTF-8 or nested duplicate/case-variant keys. `internal/adminapi/api_key_input.go:19` and its recursive exact-key scan correctly guard those cases by inspection; Setup does have a raw invalid UTF-8 TCP case. Extend the existing behavioral table when touching this boundary again. No demonstrated acceptance bug was found.

**L-EVIDENCE — one environment artifact is not colocated with the copied camera report.** `docs/evidence/deployment/2026-10-14-final-existing-http/camera/README.md:15` names `postgres-environment.log`, which exists in `.omo/evidence/camera-contract/` but is absent from the candidate's copied camera directory. I inspected it: PostgreSQL 16.15 / Timescale 2.30.2. Copy or explicitly link it in final release evidence. Passing scenario logs themselves have concrete existing artifact paths.

## Independent verification and inspected evidence

Independently executed from the exact tracked export, with `GIN_MODE=release`, the provided isolated `IOLINK_TEST_PG_DSN` on loopback 33127, and absolute `IOLINK_CONTRACT_PYTHON` pointing to the main repository's prepared Python environment:

```sh
go test -race -shuffle=on -count=1 -v ./internal/core \
  -run '^Test(KeyOpenContract(InvalidIssuanceHasNoKeyOrAuditWrites|MissingVaultFailsUnavailableWithoutWrites|DatabaseFailuresAreInternalAtomicAndDiagnosed|ScopeOwnershipReplayAndRate|RestoreLateStorageFailureRollsBackAndDiagnoses)|SetupContracts(RejectInvalidRequestsWithoutProvisioning|StorageFailuresStayClosed)|CameraContract(ProviderFailureHasSafeCorrelatedDumpAndRollback|CurrentReadDisableAndUnavailableConfiguration))$'
```

- **Exit 0**, core **109.570s**; nine top-level tests, 132 total PASS records, zero FAIL or SKIP. Artifact: `.omo/evidence/final-existing-code-review-pg.log`. Tests use real TCP listeners, production services/adapters and per-fixture migrated disposable databases, not a mock DB. Assertions observe response schemas, authorization, persisted state, actual relation faults, late rollback, nonce/replay/rate behavior and safe correlated dumps. Per-test cleanup drops the owned test databases; the shared parent-owned container is left for parent cleanup.
- Independently ran `python -m pytest -q scripts/test_validate_http_response.py`: **exit 0, 33 passed in 3.13s**. Artifact: `.omo/evidence/final-existing-code-review-validator.log`.
- Independently ran `git diff --check` against the pinned tree: clean. All changed Go files are `gofmt` clean. All changed Go/Python files are below 250 pure LOC; the existing Python test module is in the warning band at 235.

Supplemental inspected final-process artifacts:

- `.omo/evidence/key-open-contract/final-real-pg.log`: terminal PASS, core 83.409s; associated README labels historical RED/fixture-error experiments correctly. `final-http-unit.log` and `final-contracts.log` have completed package/static outputs.
- `docs/evidence/deployment/2026-10-14-final-existing-http/setup/final.log`: terminal PASS for core 19.132s and setupapi 6.069s; inspected earlier input/UTF-8 RED reproduction files separately.
- `docs/evidence/deployment/2026-10-14-final-existing-http/camera/after.log`: terminal PASS, core 39.915s; `http-unit-after.log` completed admin/app package passes. `diagnostic-before.log` reproduces the earlier missing-correlation failure and is not presented as a pass.
- `snapshot-python.log`: 114 passed; `snapshot-contracts.log`: 96 operations/621 synthetic fixtures; `snapshot-ruff.log`: all checks passed/two files formatted. These counts are static/unit checks, not added live coverage.

The root's full pinned-source `make verify` and focused combined runs were still ongoing at this review cutoff. They are **not** counted as passing evidence here, and full release gates must be completed on the replacement candidate. Historical `.omo/evidence/live-response-validator/boundary-typecheck.log` has **12 errors**; this review does not claim strict Python type checking passed. Those existing unknown library types/assert-never diagnostics are outside the narrow selector extension. Strict golangci/nilaway/LSP gates were not configured or independently run; no dedicated new security scanner was configured (N/A). Go race execution/compilation and repository contract checks provide the stated, narrower evidence.

## Skill-perspective and overfit/slop review

**Ran directly before judging maintainability or test relevance:** `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/programming/SKILL.md`, its Go/Python references, and `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/remove-ai-slops/SKILL.md`. Additional error/logging guidance was consulted. The leaf/read-only assignment prohibits delegation or removal edits; I applied the full review criteria directly rather than claiming the skill's editing workflow or delegated cleanup ran.

- Deletion-only/removal-only tests: none. The diff adds behavior/contract regressions; it does not introduce assertions that merely certify a requested deletion.
- Tautologies, output-derived expectations and prose/prompt pins: none found in the new tests. Fixed schema selector fixtures exercise route/status selection; fixture values and machine error codes are external contract inputs, not prose pins.
- Implementation-mirroring assertions: **C-TEST** is the one concrete concern. Most new tests would fail for actual malformed-input acceptance, cross-tenant exposure, stale membership, partial transactions, replay/rate errors, missing diagnostics, secret disclosure or incorrect schemas.
- Excessive/useless tests: none found beyond the overconstrained predicate. The shared lifecycle sequences cover issuance/rotation/revocation or initialization/one-time closure and are appropriate integration narratives. SQL table renames genuinely fail PostgreSQL operations; they are not mocks returning the expected error.
- Production extraction/parsing/normalization: no needless new layer found. Exact/duplicate/case checks and UTF-8 parsing are bounded HTTP trust-boundary work required by the goal; only typed requests reach services. Existing domain resource checks are unchanged. No speculative provider/plugin abstraction, new provider SDK dependency, or interior revalidation was introduced.
- Typed errors and safety: Open unauthorized/replay errors move to domain sentinels while core aliases preserve existing callers. Real query errors remain wrapped and distinguishable from genuine 401 failures. Vault absence maps to 503 before authentication writes. Capability diagnostics are correct in the new Key/Open paths but violate the criterion in the initial camera paths (**C-DIAG**).
- Strict perspective conclusion: the diff is otherwise scoped and maintainable, with **C-TEST** violating the test-coupling perspective and **C-DIAG** violating accurate capability/error-boundary logging. No newly introduced domain-level untyped escape hatch or needless production complexity was found. Existing broad typing/strict-toolchain limitations are reported, not represented as clean strict gates.

## Required disposition

`blockers`: **C-DIAG** only. Freeze the corrected candidate, finish its complete verification, and obtain fresh independent reviews of the same tree before recording this bounded milestone or publishing it. No commit, tag, push, stage completion, or whole-project approval was performed by this reviewer.
