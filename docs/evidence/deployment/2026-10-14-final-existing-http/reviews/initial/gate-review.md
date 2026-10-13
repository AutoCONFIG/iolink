# Final existing HTTP gate — initial frozen candidate

recommendation: REJECT

Reviewed tree: `4a59e953f9ad7a336babe45d90d28258fdcc072a`
Base: `3635fd614c82f08654806110c2dae0c669c24ff4`
Frozen export: `/tmp/iolink-final-existing-http-20261014`
Date: 2026-10-14
Reviewer: independent final gate; no implementation edits or delegation. Only this report was written.
ULW lookup: `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`; this report uses the requested fallback `.omo/evidence/final-existing-gate-review.md`.
Notepad: none supplied or applicable for this bounded task; no notepad completion is inferred.

## originalIntent

Continuously fix and actually test existing functionality, including safe debug-dump diagnostics, and publish a bounded milestone only after two independent non-editors approve the same snapshot. The larger M0–M8/R01–R55 scope remains authoritative. This batch concerns API Key management, signed Open reads, device restore, initialization, and currently mounted camera HTTP operations. Browser acceptance belongs to the user; License delivery acceptance and new M7/M8 runtime work are deferred.

## desiredOutcome

Twenty distinct existing physical operations are exercised through actual HTTP/TCP and real PostgreSQL. Inputs, authorization, ownership, capability absence, storage failures, rollback and safe correlated diagnostics behave as promised. Camera creation/replacement stay unavailable (503, no camera business writes) until runtime availability is assembled. Initialization creates one platform administrator without creating a business tenant. Evidence distinguishes synthetic schemas, current live handlers, historical acceptance and external limitations.

## Criteria used

These local IDs identify the success criteria in the supplied task, not additional requirements:

| ID | Supplied success criterion | Initial candidate review |
|---|---|---|
| C-SURFACE | Positive named current 20 physical operations; actual captured responses validated against current contracts | Inspected production mounting, fixtures, response helpers, success tests and completed worker evidence. Camera POST/PUT positive outcome is honest 503/fail closed, not successful video configuration. |
| C-AUTH | Negative authentication, role, scope, resource ownership and live revocation | Real current services and tokens; scope/foreign/expired/role tests inspected, storage/authentication errors independently exercised. |
| C-INPUT | Strict API Key/Setup JSON including duplicate, exact case, null/nested, trailing, invalid UTF-8; reject before write | Parsers inspected and real setup/issuance rejection tests independently passed. API Key UTF-8 and nested exact-case/duplicate rejection are supported directly by the parser but not each by a new real-PG regression row. |
| C-ATOMIC | No partial business state on rejection/storage failure; late restore and camera-disable rollback | Independent real relation-fault tests passed, including late shadow deletion and durable camera stop-intent failure. |
| C-RUNTIME | Current camera runtime absence returns 503/no camera-state write; local video License is prerequisite only | Production composition and fixture Availability remain nil; tests and evidence preserve this limit. |
| C-DIAG | Safe correlated server request IDs, safe SQLSTATE/error classes, privacy; capability absence has no fabricated error | **FAIL**: camera capability absence logs `camera.ErrUnavailable` as an `err` value. |
| C-VERIFY | Exact-source build/vet/race with real DSN, focused no SKIP, Python 114, static 96/621 separately, ruff | Focused exact-source and independent checks are complete. Full exact-source make verification was still running when this initial review was closed; no full-pass claim is made. |
| C-RELEASE | Two independent non-editor approvals on the same tree, bounded status only | Not satisfied for this tree: this review rejects it. No release or milestone approval. |

## blockers

1. **violatedCriterion: C-DIAG**
   - Observation: `cameraFailure` logs a provider-style error even when the only condition is absent camera capability. Both nil-provider branches call `cameraFailure(c, camera.ErrUnavailable)`, which then supplies `slog.Any("err", err)`. The resulting diagnostic fabricates an error classification for missing capability.
   - evidencePointer: exact-tree `internal/adminapi/cameras.go:69` and `:121`; `internal/appapi/cameras.go:80` and `:132`; `.omo/evidence/camera-capability-before.log` (both real user/mini surfaces return 503 but fail the no-fabricated-error assertion).
   - Required change: classify `camera.ErrUnavailable` at the HTTP boundary with a constant capability diagnostic and no `err` field. Preserve 503/no-write behavior and SQL-failure diagnostics. Freeze the revised source and complete its verification and fresh independent reviews.
   - The root has a draft correction outside the reviewed tree. That correction cannot approve `4a59e953` or transfer this review to another tree.

## userOutcomeReview

The inspected artifact largely achieves the bounded outcome: real service/database fixtures test the mounted operations; SQL faults stop masquerading as authentication failures; the API Key vault absence is a typed capability failure; strict parsing closes the reproduced duplicate/case/UTF-8 initialization failures; restore uses captured status/header/bytes for both contracts; camera writes stay honestly unavailable. It is not ready to publish because one requested diagnostic behavior fails. No whole R02, complete R43, License delivery, real WeChat/device/video/browser acceptance or R55 completion is implied.

## Independent reproductions

1. Identity: a NUL-safe `git ls-tree -rz 4a59e953f9ad7a336babe45d90d28258fdcc072a` read followed by Git-blob SHA-1 calculation over every exported tracked file returned `tracked_blobs=1417 mismatches=0`. This excludes dirty AGENTS/playback work from the reviewed source. Ignored `web/dist` content copied for Go embed is a separately disclosed prerequisite, not a tracked-source claim. An initial non-NUL filename reader misinterpreted quoted Unicode paths; the corrected check established identity.
2. From the frozen export, with populated dedicated real-PG `IOLINK_TEST_PG_DSN`, the repository contracts Python path, and `GIN_MODE=release`:

   ```sh
   go test -race -shuffle=on -count=1 -v ./internal/core \
     -run '^Test(KeyOpenContract(Database|Invalid|Missing|RestoreLate)|SetupContracts|CameraContractProvider)'
   ```

   Tool session `27261`, exit 0, `PASS`, core `110.168s`, shuffle seed `1791917051693603511`. Nine top-level tests, no SKIP. The run covered 13 actual Key/Open relation faults, 12 issuance rejections, missing vault/management capabilities, setup success and storage faults plus 16 setup rejections, six camera SQL faults with rollback/privacy/correlation, and late restore storage rollback. This command ran against **4a59e953 only**; it does not certify the subsequent correction.
3. From the same export: repository contracts Python `-m pytest -q scripts/test_validate_http_response.py`; tool session `51911`, exit 0, `33 passed in 3.28s`. The command checks the validator's fixed contracts and safe response diagnostics; it is distinct from actual handler acceptance.

## Checked artifacts and source

Authority: `docs/README.md`, `docs/CONTRIBUTING.md`, scoped rows/status from `docs/PLAN.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`; original supplied task and repository instructions.

Changed production/contracts inspected against base:

- `internal/adminapi/api_key_input.go`, `api_keys.go`, `cameras.go`; `internal/appapi/cameras.go`.
- `internal/core/api_key_crypto.go`, `open_auth.go`; `internal/domain/api_keys.go`.
- `internal/openapi/http_errors.go`, `server.go`; `internal/setupapi/input.go`, `server.go`.
- `scripts/validate_http_response.py`, `scripts/test_validate_http_response.py`.
- `docs/api/admin-openapi.yaml`, `license-openapi.yaml`, `open-openapi.yaml`.

Every new test in the exact diff inspected:

- `internal/core/camera_contract_errors_test.go`, `camera_contract_helpers_test.go`, `camera_contract_test.go`.
- `internal/core/key_open_contract_boundary_test.go`, `key_open_contract_capability_test.go`, `key_open_contract_database_test.go`, `key_open_contract_fixture_test.go`, `key_open_contract_restore_test.go`, `key_open_contract_success_test.go`.
- `internal/core/setup_contract_fixture_test.go`, `setup_contract_http_test.go`.

Related existing production/test paths inspected: `internal/adminapi/api_key_resource_input.go`, `api_key_resource_input_test.go`; `internal/core/api_keys.go`; `internal/camera/service.go`; `cmd/iolinkd/camera.go`; `internal/observability/redaction.go`; `internal/operations/request_logging.go`; `internal/testdb/testdb.go`; existing M6d input/signing/resource tests; actual response-contract helper.

Evidence inspected:

- `docs/evidence/deployment/2026-10-14-final-existing-http/README.md` and `camera/README.md`.
- `camera/initial.log`, `diagnostic-before.log`, `after.log`, `http-unit-after.log`, Go/Python version logs.
- `setup/input-before.log`, `utf8-before.log`, `shared-edit-build.log`, `final.log`.
- `.omo/evidence/key-open-contract/README.md`, `source-sha256.log`, `auth-sql-before.log`, `vault-before.log`, `restore-actual-status-before-schema.log`, `final-real-pg.log`, `final-http-unit.log`, `final-contracts.log`.
- `.omo/evidence/camera-capability-before.log` and the root's unfrozen capability correction diff.
- `docs/evidence/deployment/2026-10-14-final-existing-http/snapshot-focused.log`, `snapshot-python.log`, `snapshot-contracts.log`, `snapshot-ruff.log`, and incomplete `snapshot-verify.log`.

Worker terminal outputs checked: Key/Open final core83.409s and package race tests; camera final core39.915s and HTTP packages; setup/core/bootstrap final outputs. Reproduction failures are retained as RED, not reclassified as passes. The restore audit-fault experiment is correctly described as a wrong fixture assumption; the final test instead exercises the genuine late shadow deletion dependency. Root static/Python outputs say 114 passed, 96 operations/621 synthetic fixtures and ruff pass. Synthetic counts do not establish extra live HTTP coverage.

## Direct programming and remove-ai-slops pass

Consulted both installed SKILL.md files, Go/Python reference indexes, Go testing/error-handling and cross-language logging criteria. Applied the review criteria directly; no cleanup, test changes or scope expansion performed.

- Excessive/useless tests: no deletion-only or requested-removal-only test was introduced. The larger test volume follows the requested actual HTTP/SQL/error-state matrix. Separate fixtures for relation faults prove storage behavior rather than counting assertions as acceptance.
- Tautology/implementation mirroring: captured real response status and bytes are checked against fixed local schemas; state before/after is compared independently. Signature helpers have existing published-vector tests. No test derives an expected business result from the output under test.
- Overfit note: the original camera SQL diagnostic assertion pins `*errors.errorString`, coupling to Go implementation naming. This is a maintenance note rather than an additional success-criterion blocker; actual privacy/correlation and normalized diagnostic presence are the desired behavior. The unfrozen root correction removes that coupling.
- Parsing/extraction/normalization: the new small JSON parsers enforce exact-key, duplicate and UTF-8 behavior that ordinary struct decoding does not enforce. They stay at input boundaries and return typed request values. Error helpers operate at HTTP decision boundaries; no new domain/provider dependency or unnecessary production abstraction was introduced. Distinct setup and API Key parsing needs do not justify a speculative universal parser.
- Errors/logging: typed authentication/replay/capability sentinels and `%w` preserve genuine PostgreSQL failures; injected loggers use constant messages and observability fields. C-DIAG above is the concrete exception. No new plaintext SQL/body/header/query/credential logging found.
- Size/maintenance: every changed Go/Python source/test is below 250 pure lines (largest measured: Python test235, admin camera203). No dead code or speculative feature work was found in the exact diff.
- Notes: the Key/Open business digest omits encrypted secret/nonce and shadow rows, so it proves only its named key metadata/audit/device subset; it must not be described as a complete database snapshot. Some success narratives share state across ordered subtests, but they run deterministically without parallel subtests and represent a real lifecycle.
- New Go behavior has real TCP/PG/race regressions. Python extends an exhaustive fixed `Literal`/match dispatch. Strict Python typecheck remains known unpassed (12 existing diagnostics); no strict-typecheck approval is made. Unconfigured skill-recommended linters/security tools are not substituted for or added as new repository acceptance criteria.

## Companion review and exact evidence gaps

Companion old-tree `.omo/evidence/final-existing-code-review.md` was read after it was written. It explicitly records programming and remove-ai-slops direct perspectives, deletion/removal-only, tautology/prose pins, implementation mirroring, excessive/useless tests, and production extraction/parsing/normalization coverage. It independently rejects the same C-DIAG blocker; runtime-type pinning and narrower input-regression gaps are nonblocking notes. Its referenced PG9 and validator33 logs were inspected, as was the separate environment artifact `.omo/evidence/camera-contract/postgres-environment.log` (PG16.15/Timescale2.30.2). The companion's coverage supplements, and does not replace, my direct pass. Its report correctly limits approval to neither the old nor replacement tree.

- Full `make verify` exact-tree terminal result was pending at closure; the partial log is not a passed build/vet/full-race gate.
- No same-tree pair of APPROVE conclusions exists; this REJECT prevents approval.
- No applicable notepad was supplied; none is fabricated.
- API Key invalid UTF-8/nested-case and duplicate-nested fields lack dedicated new real-PG rows, although the reviewed exact-key/UTF-8 parser rejects them. This is a NOTE on regression coverage, not a criterion failure proven in the artifact.
- The copied camera README names a PostgreSQL environment log that is held in `.omo/evidence/camera-contract/`, rather than colocated with the copied README. That actual artifact was inspected; final release evidence should link or colocate it. This is an evidence navigation NOTE, not missing environment proof.
- External/browser/License delivery/new video runtime and complete R02/R55 evidence are outside this bounded candidate and remain unproven/deferred.

Final initial-candidate status: **REJECT**. A fresh candidate and fresh reviews are required after the capability diagnostic correction.
