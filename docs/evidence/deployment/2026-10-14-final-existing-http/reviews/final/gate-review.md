# Final existing HTTP gate — revised frozen candidate

recommendation: APPROVE
confidence: HIGH for the bounded current HTTP outcome
blockers: []

Candidate tree: `bbfb29c41c31549899655750e171ef2c25a6d0eb`
Base: `3635fd614c82f08654806110c2dae0c669c24ff4`
Export: `/tmp/iolink-final-existing-http-20261014-v2`
Date: 2026-10-14
Reviewer: `/root/final_existing_v2_gate_review`, independent non-editor, read-only implementation/tests/index, no delegation. Only review evidence and this report were written.
Report path: `.omo/evidence/final-existing-v2-gate-review.md`.
ULW status returned `ULW_LOOP_PLAN_MISSING`; the requested evidence fallback is used. No applicable current-scope notepad exists or was supplied.

## originalIntent

Continuously reproduce and fix existing functionality, including safe debug dumps, then publish a bounded milestone only after two independent non-editors approve the same candidate. The authoritative M0–M8/R01–R55 project scope remains unchanged. This batch covers existing API Key management, signed Open reads, restore, initialization and currently mounted camera operations. Browser acceptance is owned by the user; new M7/M8 runtime work and the License delivery work are deferred.

## desiredOutcome

Twenty distinct physical operations receive real TCP/HTTP and PostgreSQL verification: Key five, Open five, restore one, Setup two and camera seven. Responses match the current applicable contracts; authentication, roles, ownership, strict inputs, missing capabilities, storage errors and late rollback behave safely. Initialization creates only one platform ADMIN without a business organization. Camera creation/replacement honestly return 503 with no camera business writes while Availability remains absent. Correlated diagnostics expose safe error categories/SQLSTATE, without secrets or invented errors for capability absence.

## userOutcomeReview

The revised artifact satisfies this bounded outcome. Strict parsers reject the reproduced input defects before provisioning/issuance. Open authentication now preserves real PostgreSQL failures as 500 while known unauthorized/replay decisions remain 401. Vault and management capability absence return 503. Restore validates the actual captured response against both declarations and rolls back a genuine late shadow-table failure. Camera read/disable/role/scope behavior and current unavailable configuration are exercised through the real service, adapter, migrated database and TCP listener.

The old C-DIAG blocker is closed on this tree: both camera adapters classify `camera.ErrUnavailable` with a constant capability diagnostic and omit `err`. My independently executed absent-provider regression observes both user and mini surfaces, 503, unchanged business snapshot and matching server request ID with no error field. Real SQL-failure diagnostics and late camera-disable rollback still pass. The diagnostic test now checks meaningful safe-class presence/privacy/correlation instead of pinning a Go runtime error type.

This approval does not claim all 96 target operations are live, complete R02/R43/R55 or M0–M8 acceptance, real License delivery, browser acceptance, hardware, WeChat or video playback.

## Criteria and outcome

These IDs identify the supplied criteria; they do not create additional requirements.

| ID | Criterion | Result and evidence |
|---|---|---|
| C-SURFACE | Current 20 physical operations and real captured response validation | PASS. `snapshot2-focused.log` contains the 20 named top-level scenarios; the inspected source tables enumerate all five Key operations, five Open reads, restore, two Setup operations, five user-camera and two mini-camera operations. Restore is checked against admin/license using captured actual status, content type and bytes, counted once. Camera POST/PUT outcome is current 503, not claimed 201 configuration. |
| C-AUTH | Authentication, role, scope, ownership and live revocation | PASS. Current service/token fixtures, role tables, foreign/missing resource checks, membership expiry/revocation, published-signature and existing M6d negative regressions inspected. New focused matrix passes; my independent tests also preserve known authorization/storage classification. |
| C-INPUT | Strict Key/Setup duplicate/case/unknown/nested/null/trailing/UTF-8 rejection before writes; strict signed paths/queries | PASS within bounded behavior. Exact parser code and all new boundary tables inspected; real issuance and Setup rejects, zero-state changes, signed invalid path/query and nonce preservation pass. Dedicated Key raw UTF-8/nested-case/duplicate rows are a coverage NOTE below, not a proved acceptance failure. |
| C-ATOMIC | No partial business state, including actual late failure | PASS. My real relation-fault run verifies issue/rotate/revoke audit rollback, restore after device UPDATE, and camera stop-job failure after mutation. Snapshots and SQL fault probes are independent of handler outputs. |
| C-RUNTIME | Fail closed for missing capability; honest camera Availability nil and License prerequisite scope | PASS. `cmd/iolinkd/camera.go`, camera service and fixtures inspected; no fake Availability supplied. POST/PUT503/no camera business writes, absent management/vault503 and both nil-provider camera surfaces verified. |
| C-DIAG | Safe server request-ID correlation, error categories/SQLSTATE/privacy, no fabricated capability error | PASS. Own PG logs and absent-provider log below; constant injected slog messages inspected against observability redaction and request logging. Actual SQL faults produce safe categories; capability diagnostics contain no invented error. |
| C-VERIFY | Exact-source build/vet/full race, populated real DSN, focused no SKIP, Python 114, static 96/621 and ruff | PASS. Root explicitly confirmed serial session56771 terminal exit 0; I inspected the complete log and confirmed running process cwd was the pinned export. Build/vet, all 33 tested Go packages/core 471.529s passed with unchanged default timeout. Focus80051 exit 0/core 264.801s, Python 114, static 96/621 and ruff terminal exit 0 confirmed and logs inspected. My independent PG/race/TCP10 scenarios and Python 33 also pass. |
| C-RELEASE | Two independent non-editor approvals on same tree before milestone/main/tag/push, bounded status | PASS for the approval pair once this report is issued; companion independently APPROVE/WATCH on the same bbfb tree. This gate approves only bbfb. Release/status recording remains root-owned, after this pair of approvals; no deployment, staging, merge, tag or push performed by this reviewer. |

## Independent reproductions

Environment: Linux amd64, repository Go 1.26.8 / Python 3.14.6 evidence; populated `IOLINK_TEST_PG_DSN` points to the root-owned isolated PostgreSQL 16.15 / Timescale 2.30.2 endpoint on loopback33127. `GIN_MODE=release`; absolute `IOLINK_CONTRACT_PYTHON` points to the repository's prepared contracts environment. No production credentials are used. Fixtures create/drop owned migrated databases; shared container cleanup belongs to root.

1. NUL-safe Git blob verification independently checked **1,437 tracked blobs, zero mismatches**, twice. All changed Go files are gofmt clean; every changed Go/Python source/test is under 250 pure LOC, largest Python test 235. Evidence: `.omo/evidence/final-existing-v2-gate-review-identity.log`. Dirty AGENTS and playback drafts are excluded. Ignored copied `web/dist` is disclosed Go-embed prerequisite, outside the tracked source diff.

2. From the exact export, real PG/TCP/race/shuffle uncached subset:

   ```sh
   go test -race -shuffle=on -count=1 -v ./internal/core \
     -run '^Test(KeyOpenContract(InvalidIssuanceHasNoKeyOrAuditWrites|DatabaseFailuresAreInternalAtomicAndDiagnosed|RestoreLateStorageFailureRollsBackAndDiagnoses|SignedInputBoundsAndNonceState|MissingKeyManagementPortRejectsWithoutWrites)|SetupContracts(RejectInvalidRequestsWithoutProvisioning|StorageFailuresStayClosed)|CameraContract(ProviderFailureHasSafeCorrelatedDumpAndRollback|CurrentReadDisableAndUnavailableConfiguration|MissingProviderHasConstantCorrelatedDiagnostic))$'
   ```

   Session24174 **exit 0**, core 140.766s; nine top-level scenarios / 121 PASS records, zero FAIL/SKIP. Evidence: `.omo/evidence/final-existing-v2-gate-review-pg.log`. The extra `MissingProviderHasConstantCorrelatedDiagnostic` selector did not name an existing test; the current absent-provider test was explicitly run separately by exact name below. No coverage is inferred from that unmatched alternative.

3. Exact current diagnostic regression:

   ```sh
   go test -race -shuffle=on -count=1 -v ./internal/core \
     -run '^TestCameraContractAbsentProviderLogsCapabilityWithoutFabricatedError$'
   ```

   Session72234 **exit 0**, core 11.135s, both real user/mini surfaces pass. Evidence: `.omo/evidence/final-existing-v2-gate-review-capability.log`.

4. From the same export, prepared contracts Python `-m pytest -q scripts/test_validate_http_response.py`: session75085 **exit 0**, **33 passed in 2.77s**. Evidence: `.omo/evidence/final-existing-v2-gate-review-validator.log`. This verifies the fixed selector/validator, distinct from live handlers.

5. `git diff --check 3635fd6 bbfb29c41c31549899655750e171ef2c25a6d0eb` passed. All shared parser/error helper callers were inspected, including both camera adapters and Open authentication/resource funnels.

## Exact full verification and failed attempts

Final authoritative full log: `docs/evidence/deployment/2026-10-14-final-existing-http/snapshot2-serial-verify.log`. It shows `go build ./...`, `go vet ./...`, and `go test -race -shuffle=on ./... -count=1`; 33 packages report ok, nine report no test files, zero failure lines, core 471.529s. Root confirmed session56771 terminal exit 0 and unchanged bbfb/index. I independently observed the process cwd as the exact export.

Earlier `snapshot-verify.log` (old4a59) and `snapshot2-verify.log` (bbfb concurrent run) **failed** on the package's cumulative 10-minute timeout. Revised concurrent log lines64–67 show the active M6b scenario had run 9s, its subtest 1s, with core 600.104s/terminal failure. Neither is a full-pass artifact. Quiet serial retry used the same source and unchanged default timeout and passed. The observed result supports completion of the stated full gate; it does not establish a general suite performance guarantee.

`snapshot2-focused.log`:20 top-level scenarios / 247 PASS records, zero FAIL/SKIP; core 264.801s. `snapshot2-python.log`:114 passed. `snapshot2-contracts.log`:96 target operations/621 synthetic fixtures. `snapshot2-ruff.log`:all checks passed/two files already formatted. Root confirmed terminal exit 0 for each; I inspected their completed outputs. Counts supplement the source/scenario checks, never replace them.

## Direct programming/remove-ai-slops review

Consulted installed `programming/SKILL.md`, Go/Python indexes, Go testing/error handling, Python error handling/type patterns, and `remove-ai-slops/SKILL.md`. Applied the criteria directly to diff, production code and all new tests. Read-only/leaf scope supersedes the skill's editing/delegation workflow; no cleanup or new tests performed.

- Existence/deletion ladder: input parsers enforce required exact-key/duplicate/UTF-8 behavior that ordinary struct decoding does not supply. Narrow HTTP error funnels serve real classification/logging seams. Fixed Python selector cases reuse the established validator. No speculative parser framework/provider abstraction/production extraction was found.
- Useless/excessive/deletion-only/requested-removal-only tests: none found. The number of fixtures follows actual adversarial and storage cases; relation renames cause genuine PostgreSQL faults rather than configuring mocks to return the expected error.
- Tautologies/prose pins/output-derived expectations: none found. Contract fixtures are independent fixed local schemas. Captured status/content/body are used, and before/after persisted business state is compared separately. Published-vector regressions provide an independent signing oracle.
- Implementation mirroring/overfit: the old camera runtime-type pin is removed. New predicate asserts safe normalized class presence, actual correlation and privacy, allowing implementation changes. Lifecycle sequences are ordered nonparallel integration narratives; they observe credential invalidation, idempotence and persistence.
- Defensive parsing/normalization: required HTTP trust-boundary checks remain bounded by request size and return typed request values. Typed domain authorization/replay/capability sentinels and wrapped SQL errors avoid string discrimination and preserve classification. No unnecessary production normalization added.
- Boundaries/logging: no domain/application provider SDK dependency introduced. Constant messages, injected loggers and approved stage/err fields inspected. Observability sanitizes actual errors; raw body/header/query/URL/credentials/RTSP values are not newly logged. Real response bodies travel to the six fixed local specs via stdin only, not sensitive argv or diagnostics.
- Dead code/duplication/complexity/performance/size: no criterion failure found. Separate small Setup and Key parsers have different typed/body requirements; forcing a universal abstraction is not required. All changed source/test modules are below 250 pure LOC.
- Gates: repository Go build/vet/race, actual PG/TCP regressions, Python and configured ruff/contract gates passed. Historical strict Python typecheck still has12 diagnostics and is **not PASS**. Skill-recommended LSP/golangci/nilaway and separate security scan were not configured/run here (N/A); they are not added as unstated repository acceptance criteria.

## Checked artifact paths

Authority: `docs/README.md`, `CONTRIBUTING.md`, scoped `PLAN.md`, `EXTENSIONS.md`, `ACCEPTANCE.md`, `IMPLEMENTED.md`, original supplied task.

Exact changed production/contracts:
- `docs/api/admin-openapi.yaml`, `license-openapi.yaml`, `open-openapi.yaml`.
- `internal/adminapi/api_key_input.go`, `api_keys.go`, `cameras.go`; `internal/appapi/cameras.go`.
- `internal/core/api_key_crypto.go`, `open_auth.go`; `internal/domain/api_keys.go`.
- `internal/openapi/http_errors.go`, `server.go`; `internal/setupapi/input.go`, `server.go`.
- `scripts/validate_http_response.py`, `scripts/test_validate_http_response.py`.

All new exact-tree tests:
- `internal/core/camera_contract_capability_test.go`, `camera_contract_errors_test.go`, `camera_contract_helpers_test.go`, `camera_contract_test.go`.
- `internal/core/key_open_contract_boundary_test.go`, `key_open_contract_capability_test.go`, `key_open_contract_database_test.go`, `key_open_contract_fixture_test.go`, `key_open_contract_restore_test.go`, `key_open_contract_success_test.go`.
- `internal/core/setup_contract_fixture_test.go`, `setup_contract_http_test.go`.

Related seams: `internal/core/http_contract_helpers_test.go`, existing `m6d_*test.go` behavior/vector/rate tests, `internal/adminapi/api_key_resource_input.go`, `internal/camera/service.go`, `cmd/iolinkd/camera.go`, `internal/observability/redaction.go`, `internal/operations/request_logging.go`.

Executor/manual HTTP matrix and output artifacts:
- `docs/evidence/deployment/2026-10-14-final-existing-http/README.md`, `key-open/README.md`, `camera/README.md`.
- All copied `camera/*.log`: initial/after, diagnostic-before, capability-before/after, HTTP packages, Go/Python/PG environment.
- Copied `key-open/*.log`: final real PG/HTTP/static, source hashes, input/auth/database/vault/open mapping/status/restore intermediate reproductions. Their terminal statuses inspected; intermediate RED logs are not release passes.
- `setup/input-before.log`, `utf8-before.log`, `shared-edit-build.log`, `final.log`.
- Both initial `snapshot-*.log` and revised `snapshot2-*.log`, plus `snapshot2-serial-verify.log`; old/current failed full attempts explicitly separated above.
- Old rejected `.omo/evidence/final-existing-code-review.md` and `final-existing-gate-review.md`; no old verdict is transferred.
- `.omo/evidence/live-response-validator/boundary-typecheck.log` (12 errors, not passed).
- Own identity/PG/capability/validator artifacts listed above.

## Companion independent review

Inspected `.omo/evidence/final-existing-v2-code-review.md` in full after it was written. It explicitly approves **the same bbfb Git tree**, records `blockers: []` and WATCH for two LOW coverage/navigation notes, and independently compares all 1,437 blobs with zero mismatches. I also inspected its referenced artifacts:

- `.omo/evidence/final-existing-v2-code-review-pg.log`: four actual PG/TCP/race scenarios, core 32.545s, zero FAIL/SKIP; capability absence, camera SQL/late rollback, 13 Key/Open faults and late restore rollback.
- `final-existing-v2-code-review-http.log`: four HTTP packages pass.
- `final-existing-v2-code-review-python.log`:33 validator tests pass.
- `final-existing-v2-code-review-contracts.log`:96/621 synthetic checks pass.

The report explicitly documents its own direct programming/remove-ai-slops pass and coverage of deletion-only/requested-removal-only tests, tautology/prose pins, output-derived expectations, excessive/useless tests, implementation mirroring, and unnecessary production extraction/parsing/normalization. Its evidence supports the stated conclusions. That perspective coverage supplements and does not replace my independent direct pass. Both reports retain failed full runs as failures and strict Python typing as unpassed. This report provides the second independent non-editor approval for the exact source candidate; later implementation/test changes are not covered.

## Exact evidence gaps and nonblocking notes

- No blocker remains against a stated bounded criterion.
- Key invalid raw UTF-8, nested exact-case and duplicate-nested fields lack dedicated new real-PG rows. The exact parser directly rejects these classes; broader new regression coverage is a NOTE, not a demonstrated criterion failure.
- The Key README refers to Go/Python version files not colocated in its copied directory. Actual environment proof exists in the copied camera version/environment files and was inspected; this is navigation debt, not missing environment proof.
- The camera README's original `after.log` discussion describes the historical exact runtime class. Root README, current test source, capability-after and exact final logs supersede it; release documentation should keep that distinction clear.
- Key state digest covers named key metadata/audit count/device state, omitting encrypted secret/nonce and shadow rows; no complete database-snapshot claim is made. Camera snapshot covers cameras/streams/sessions/stop jobs/camera audits. License clock observation is an explicitly separate prerequisite and not a camera-business-write claim.
- Strict Python typecheck remains unpassed; no strict-toolchain completion claim. Browser/external video/device/WeChat/License delivery, complete R02/R43/R55 and new M7/M8 work remain outside this approval.
- No applicable notepad was supplied; none is invented. Release operations and resource cleanup remain root-owned.

Final recommendation: **APPROVE** for the exact bounded bbfb source candidate.
