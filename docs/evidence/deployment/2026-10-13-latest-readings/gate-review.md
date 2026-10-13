# Latest-reading fix: final independent gate review

recommendation: APPROVE

blockers: []

Exact candidate tree: `8edf585a14ea1b532aab9e692e05cda96067658e`

Base: `0cf0b277ac4ce7c81eef62b1ead429cb7e99067c`

Reviewed export: `/tmp/iolink-latest-r2-20261013`

Reviewer: `/root/latest_gate_review`, independent and nonediting. Only this report was written; no production/test edits or staging performed.

## Findings and resolved blocker

No blocking finding remains in this candidate's bounded latest-reading fix. The C3 rejection in `.omo/evidence/latest-gate-review.md` is resolved: both newly introduced management-side 500 responses now emit `{"error":"internal_error"}`, as declared by the `InternalError` response referenced by `GET /ponds` and `GET /devices/{device_no}`. The HTTP test now checks that contractual value. Internal diagnostic messages remain constant and are not exposed as public error identifiers.

## Original intent and desired outcome

originalIntent: autonomously test and repair existing IoLink functionality under AGENTS.md, with integration evidence and two independent reviews before publishing each milestone.

desiredOutcome: for the existing R10/R17/R22 latest-reading surfaces, include all pond devices in count/latest selection, preserve missing/inapplicable-shadow semantics, propagate actual provider failures as safe contractual 500 responses, and record correlated diagnostics.

userOutcomeReview: the exact fixed snapshot satisfies this bounded outcome. Independent authenticated HTTP and real PostgreSQL replay observes the 201st device in both summaries, expected empty results, and 500 responses on all six existing latest-reading surfaces when the shadow table is unavailable. The corrected management error code is independently verified against the authoritative API document and exercised by the final handler replay.

## Acceptance matrix

| Criterion | Result | Checked evidence |
|---|---|---|
| C1: pond-scoped management pagination covers every active device, including device 201 | PASS | `internal/adminapi/latest.go`; independent management and real-PG HTTP replay |
| C2: absence/inapplicability remain null, unsupported water product remains 400, actual provider failures cannot return stale partial success | PASS | independent final app classification/tie/empty/outage tests and real-PG six-surface failure scenario |
| C3: changed public failures preserve the authoritative safe API error code | PASS | final management handlers `:348`, `:613`, test `:121`; `docs/api/admin-openapi.yaml:529`, `:788`, `:1352` |
| C4: injected context-aware diagnostics retain safe categories and generated correlation ID | PASS | direct logger/caller inspection and independent final app HTTP log assertions |
| C5: exact snapshot is verified with positive/negative, real integration, race and contract evidence | PASS in bounded scope | all 1,173 export blobs match; independent final focused race/PG and contract replay; inspected final executor full-suite log |

## Exact identity and observed checks

- Compared every tracked blob of `8edf585a14ea1b532aab9e692e05cda96067658e` to the exported filesystem using NUL-delimited Git tree entries and `git hash-object`: 1,173 matched blobs, zero mismatches.
- Inspected the entire delta from the previously reviewed tree `6ed49166e940861740b2c9a1fee4166c0523e2ca`: only three string replacements in two files, consisting of the two management failure bodies and their response assertion. The aggregation, app handlers, core adapter and real integration test are unchanged. The original full production/test diff and caller inspection is recorded in `.omo/evidence/latest-gate-review.md` and was consulted for this final pass.
- Independently ran from the final export:

```text
GIN_MODE=release IOLINK_TEST_PG_DSN=<dedicated Timescale DSN> go test -race -shuffle=on -count=1 -v ./internal/adminapi ./internal/appapi ./internal/core -run 'TestPondLatestIncludesDevicesBeyondFirstPage|TestLatestHandlersDistinguishAbsenceFromProviderFailure|TestLatestHTTPDistinguishesMissingShadowFromProviderFailure|TestPondLatestReturnsNullWhenPondHasNoDevices|TestPondLatestSelectsFreshestSupportedShadowAndRejectsPartialOutage|TestLatestReadingHTTPWithLargePondAndDatabaseFailure'
```

  Exit 0; management 1.039s, app 1.072s, core 2.652s. Shuffle seeds: 1791904801764380737, 1791904801765370321, 1791904801911770536. All six top-level tests passed. The real PostgreSQL test ran without SKIP in 1.57s. The eight app provider-failure/timeout diagnostics retained safe error categories and nonempty matching request IDs.
- Independently ran repository venv Python `scripts/check_contracts.py` from the final export: exit 0, all 96 target operations / 568 synthetic fixtures passed. Runtime response-code compatibility was checked directly as well; static fixture counts alone do not establish it.
- `git diff --check 0cf0b27 8edf585a14ea1b532aab9e692e05cda96067658e`: exit 0. `gofumpt -l` on the two final-delta Go files emitted no filenames. All eight original changed Go files had passed formatting inspection in the original review; their remaining contents are unchanged.
- Inspected `final-focused.log`: final executor authenticated handler and real-PG focused tests passed, including core 2.188s.
- Inspected `snapshot-final-verify.log` after it contained all 33 tested Go package results, including core 139.659s, build/vet commands and the complete race-test command; no FAIL was present. The coordinator subsequently reported the terminal `write_stdin` result for session 28551 as exit 0 and confirmed the staged tree still equals the reviewed candidate. This is executor evidence, not a second independently executed full suite. The coordinator must preserve that terminal result and source binding in the evidence README.

## Programming and remove-ai-slops checks

Both skills were loaded and applied directly during this review, including the Go README/error-handling/testing references. The full original diff was reviewed using these perspectives; the final error-code delta was assessed again.

- Tests observe HTTP status/JSON/latest results and actual PostgreSQL failure behavior. The 201-device fixture is a real pagination boundary. Missing, wrapped-missing, generic product, timeout, partial outage and deterministic tie cases exercise different consumer outcomes. No deletion-only, prose-pin, tautological or implementation-removal tests found.
- The corrected error-body assertion pins a declared machine-consumed contract value; it no longer encodes the former mismatch. No unnecessary production parsing, normalization, abstraction or dependency was introduced by the fix.
- Known sentinel errors are classified using `errors.Is`, only `pgx.ErrNoRows` is normalized to absence, and other provider errors remain failures. Context and existing authorization/ownership filters remain in the call chain. New diagnostics use injected loggers and the existing redaction handler.
- The new management extraction is cohesive, reuses the existing serializer and consumer port, and replaces the faulty global first-page lookup. No new oversized module was introduced. Large legacy modules and per-device shadow-query cost are residual pre-existing structural/capacity limits outside this acceptance claim.
- Consulted `.omo/evidence/latest-code-final.md`: it explicitly approves this same exact tree and records both required skill perspectives, overfit/slop checks, test relevance, and unavailable strict-linter checks. This report coverage was confirmed after my independent direct pass; it did not substitute for that pass.

## Checked artifact paths and limits

Checked final candidate production/test paths: `internal/adminapi/{handlers.go,latest.go,latest_reading_test.go}`, `internal/appapi/{handlers.go,latest_reading_test.go}`, `internal/core/{repos.go,latest_reading_http_test.go}`, `internal/domain/domain.go`; relevant API contract references in `docs/api/admin-openapi.yaml` and `docs/api/openapi.yaml`.

Checked executor evidence directory `docs/evidence/deployment/2026-10-13-latest-readings/`: `README.md`, `admin-before.log`, `admin-after.log`, `app-before.log`, `app-after.log`, `real-pg-http.log`, `contracts.log`, `snapshot-verify.log`, `final-focused.log`, `snapshot-final-verify.log`. Additional final logs and reports are outside the original candidate tree and must be archived with their source binding before the milestone completion record is published.

The earlier rejected reports remain historical and do not count as approval. This report approves only the fixed latest-reading tree. Separate operations/foundation QA, all of V1, all R10/R17/R22 criteria and all project milestones are not certified here. Browser interaction is user-owned; real WeChat, physical hardware, customer License delivery and external operations were not tested. No optional lint/security tool is claimed passed. No new test container was created by this reviewer; the task-owned dedicated PG instance was reused.

`omo-agent-toolkit ulw-loop status --json` previously returned `ULW_LOOP_PLAN_MISSING`, so this report uses the specified fallback evidence location.
