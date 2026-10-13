# Latest-reading fix: independent final gate review

recommendation: REJECT

Exact candidate tree: `6ed49166e940861740b2c9a1fee4166c0523e2ca`

Base: `0cf0b27`

Reviewed export: `/tmp/iolink-latest-20261013`

## Blockers

- violatedCriterion: C3 — preserve the external API error contract while making provider failures fail closed.
  Observation: the newly introduced management-side 500 paths emit `{"error":"telemetry query failed"}`, but the authoritative management OpenAPI `InternalError` response specifies `error=internal_error`. The new management test explicitly expects the incompatible string, so its passing result does not establish contract compliance.
  evidencePointer: candidate `internal/adminapi/handlers.go:349` and `:614`; `internal/adminapi/latest_reading_test.go:116`; `docs/api/admin-openapi.yaml:529`, `:788`, `:1352`.
  Required fix: use `internal_error` on both changed management-side provider-error paths and assert that response in the regression test. Review the resulting fixed candidate with two independent reviewers.

## Original intent and desired outcome

originalIntent: continue testing and correcting existing IoLink functions autonomously under AGENTS.md, with real integration evidence and independent double review.

desiredOutcome: existing R10/R17/R22 latest-reading surfaces include every pond device, give consistent latest results, distinguish missing/inapplicable shadows from genuine storage failures, and preserve safe contractual HTTP failures and correlated diagnostics.

This review covers that bounded software fix. It does not certify every R10/R17/R22 criterion, all project milestones, browser UX, physical devices, real WeChat, customer License acceptance, or production operations.

## Criteria and user outcome review

| Criterion | Result | Evidence |
|---|---|---|
| C1: management pond aggregation uses pond filtering and all pages, including device 201 | PASS | `internal/adminapi/latest.go`; independent real-PG/HTTP and management handler replay |
| C2: missing shadows remain successful null, unsupported products retain existing semantics, provider failures return 500 instead of stale partial success | PASS | both handler test suites; six real PostgreSQL failure surfaces in `internal/core/latest_reading_http_test.go` |
| C3: external HTTP contracts and safe error response remain intact | FAIL | management error identifier mismatch above; app `internal_error` and water `unsupported_product` remain correct |
| C4: diagnostics use injected logger, context correlation, constant messages and safe error rendering | PASS for changed paths | inspected logger plumbing; independent app replay emits request IDs and safe error categories without private provider details |
| C5: same-snapshot evidence includes positive, negative, integration, race and contract checks | PASS within stated scope | all 1,173 export blobs match candidate tree; independent targeted race and real-PG run; independent contract checker; inspected executor full verification |

## Independent checks and observed results

- Verified the export by comparing every tracked blob against `git ls-tree -rz 6ed49166e940861740b2c9a1fee4166c0523e2ca` and `git hash-object`: 1,173 matched blobs, zero mismatches. A preliminary non-NUL path parser incorrectly treated Git-quoted historical Chinese filenames as literal paths; corrected to NUL-delimited input before judging identity.
- Go environment reproduced as `go1.26.8 linux/amd64`.
- From the exact export, ran `GIN_MODE=release IOLINK_TEST_PG_DSN=<dedicated Timescale DSN> go test -race -shuffle=on -count=1 -v ./internal/adminapi ./internal/appapi ./internal/core -run 'TestPondLatestIncludesDevicesBeyondFirstPage|TestLatestHandlersDistinguishAbsenceFromProviderFailure|TestLatestHTTPDistinguishesMissingShadowFromProviderFailure|TestPondLatestReturnsNullWhenPondHasNoDevices|TestPondLatestSelectsFreshestSupportedShadowAndRejectsPartialOutage|TestLatestReadingHTTPWithLargePondAndDatabaseFailure'`: exit 0, all six top-level tests passed; real PG case ran without SKIP. The API result assertions prove pagination and failure behavior, but management response-string assertions encode the C3 defect.
- Independent app replay observed eight safe failure diagnostics: provider failure and timeout on four routes, each correlated to the server-generated request ID. Provider credentials/device identifiers did not appear in diagnostics.
- From the exact export, ran the repository venv Python with `scripts/check_contracts.py`: exit 0, 96 operations and 568 synthetic fixtures passed. This static check cannot detect the runtime error identifier discrepancy identified above.
- `git diff --check 0cf0b27 6ed49166e940861740b2c9a1fee4166c0523e2ca`: exit 0.
- `gofumpt -l` on all eight changed Go files: exit 0, no filenames printed.
- Inspected executor `snapshot-verify.log`: build, vet and shuffled race test results for 33 tested packages, no reported FAIL. This was inspected evidence, not a second independent full-suite run.

## Programming and remove-ai-slops direct pass

Consulted `omo:programming`, Go README/error-handling/testing references, and `omo:remove-ai-slops` before this direct pass.

- Error discrimination uses `errors.Is`; adapter normalizes only `pgx.ErrNoRows`, retaining actual provider failures. Context flows into the queries and logger; new code adds no goroutines, sleeps, provider SDK dependencies in domain, or new transactions.
- The new management `latest.go` extraction moves one cohesive aggregation responsibility and is reused by the existing handler; it replaces the old global first-page lookup rather than adding speculative infrastructure. Existing serializer is reused. Large legacy handler/repository modules remain pre-existing; newly added modules do not exceed the 250 pure-LOC limit.
- Tested the changed behavior through HTTP handlers, authenticated TCP HTTP servers and actual PostgreSQL. Fake consumers classify expected errors; the real DB test creates shadow state through core ingestion and then makes the shadow table genuinely unavailable.
- No deletion-only tests, prose pins, tautological self-comparisons, call-count mirroring, gratuitous production parsing/normalization, dead helpers, or redundant defensive abstractions found. The boundary-sized 201-device fixture and distinct response/error cases are meaningful. The management error-string assertion is a false-confidence issue specifically because it pins a noncontractual value; it is the C3 blocker.
- Aggregation still performs per-device shadow queries; no capacity/performance acceptance was claimed. This is a residual scope limit, not a new blocker.
- No unavailable optional lint/security tools are represented as having passed.

## Checked artifact paths

- `internal/adminapi/handlers.go`, `latest.go`, `latest_reading_test.go`
- `internal/appapi/handlers.go`, `latest_reading_test.go`
- `internal/core/repos.go`, `latest_reading_http_test.go`
- `internal/domain/domain.go`
- `docs/api/admin-openapi.yaml`, `docs/api/openapi.yaml`
- `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/ACCEPTANCE.md`, `docs/PLAN.md`
- `docs/evidence/deployment/2026-10-13-latest-readings/{README.md,admin-before.log,admin-after.log,app-before.log,app-after.log,real-pg-http.log,snapshot-verify.log,contracts.log}`

## Evidence gaps and independent code review

The evidence README is explicitly preliminary and asks for frozen-snapshot review results; this report supplies this review's candidate binding. Full-suite/contract logs were provided outside the original candidate evidence tree and were inspected at their current root paths. Browser and external-service exclusions are accurately stated and remain outside this approval scope.

At report time the final code reviewer artifact was not yet present. Root relayed the independently detected management contract mismatch; I confirmed it directly against the candidate handlers, test and authoritative OpenAPI, rather than trusting that relay. My direct programming/slop pass is recorded above. The fixed candidate needs fresh independent code and gate reports explicitly covering the same perspectives; this rejected report cannot count as an approval for a different tree.
