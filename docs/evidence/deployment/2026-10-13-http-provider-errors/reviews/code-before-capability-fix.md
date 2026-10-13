# Existing HTTP 500 safety: independent code review

Exact candidate: `144a9abf4f4d1ada23b6c11febe3ff7577df7f49`

Base: `9c63186`

Export inspected and tested: `/tmp/iolink-http-errors-20261014`

Reviewer: `/root/latest_code_review`; no implementation, fixture, evidence or staging edits.

codeQualityStatus: WATCH

recommendation: REQUEST_CHANGES

## Findings

### CRITICAL

None.

### HIGH

None.

### MEDIUM

1. **The app history capability branch manufactures a provider error instead of recording the actual missing capability.** `internal/appapi/handlers.go:255` creates `errors.New("history authorization unavailable")` when the `UserTelemetryRepo` assertion fails, then calls `providerFailure`. No provider returned that error. The resulting diagnostic claims an operation failure with `err="*errors.errorString"`, rather than the observed unavailable capability, contrary to the stated goal and evidence README. `internal/appapi/provider_error_test.go:181` requires that error class even for `history_capability`, so the test currently endorses the wrong category. The corresponding management capability branches already use `providerUnavailable`, emit a constant capability-unavailable diagnostic and omit `err`. Use the equivalent app boundary path and verify that the missing-capability scenario has no manufactured error attribute while preserving HTTP 500/internal_error and request correlation.

### LOW

None.

## Independent verification

- Verified all **1,268** export blobs against the exact candidate tree: zero mismatches. Inspected the complete ten-file Go diff, new consumer fixtures/tests and six frozen evidence files. The candidate excludes unrelated playback changes; app server changes contain only the two login-error hunks.
- From the export independently ran `GIN_MODE=release IOLINK_TEST_PG_DSN=<dedicated instance> go test -race -shuffle=on -count=1 -v ./internal/adminapi ./internal/appapi ./internal/core -run 'TestBaseline.*HTTPProvider'`: exit 0, no SKIP. All 37 management and 18 app HTTP fault scenarios passed their current assertions. The real PostgreSQL test passed in 0.89s, core package 1.961s; app package 1.067s. The real test creates its own database via testdb, authenticates both APIs, exercises seven relation-failure surfaces and requires `postgres_42P01` diagnostics linked to returned server request IDs.
- The independent app replay also confirmed the identified `history_capability` scenario passes the current erroneous `*errors.errorString` expectation. The safety defect is diagnostic truthfulness, not failure to return the public contract response.
- Independently ran `go vet ./internal/adminapi ./internal/appapi ./internal/core`: exit 0. `gofumpt -l` on all ten changed Go files produced no paths. Product diff whitespace checks were clean.
- Inspected `admin-before.log` and `app-before.log`: the prior handlers expose the poisoned test provider URI or return bespoke error values, and preexisting stable-500 branches lack the required diagnostic. The values are controlled test markers. Inspected the corresponding after logs and `real-pg-http.log`, which corroborate the observed HTTP results.
- Inspected the executor's additional `contracts.log`: 96 target operations / 568 synthetic fixtures passed. The outside-candidate full `snapshot-verify.log` was still being populated when inspected, so this report does not claim a completed full final suite. The release coordinator must capture its terminal result before completing the milestone.

## Required skill-perspective check

Loaded and applied the `remove-ai-slops` and `programming` perspectives, with the Go/error/testing/logging references previously consulted in this review session.

- **Test relevance/overfit pass:** no deletion-only, prose-pin or tautological tests identified. Tests send actual TCP HTTP requests through routing/authentication and observe the public error contract and safe dump output. Per-method consumer fakes isolate failure decisions; the actual PostgreSQL relation-failure test establishes that real adapter errors reach the boundary. Poisoned URI, request/body/query markers and tokens are adversarial inputs with meaningful leak assertions. Exact `internal_error` bodies are machine-consumed contractual values, not prose pins.
- **Production quality pass:** the small package-local helpers centralize the actual HTTP error-handling decision and use the existing injected logger/context. They do not introduce a provider SDK, speculative interface, dependency or additional parsing. Similar helper bodies in admin/app packages follow their existing separate logger wiring and avoid a new shared abstraction. Existing successful output, 4xx/License mappings and dedicated latest-read diagnostic paths are unchanged; failures do not newly execute mutations or continue after response handling.
- All six new Go files are below 250 pure LOC (18, 140, 153, 12, 161, 119). Existing large handler files remain baseline debt. Strict golangci-lint/nilaway checks were not supplied or performed and are not claimed passed.

The manufactured app capability error violates the programming perspective's accurate boundary/error modeling requirement. Its test is relevant but currently asserts the wrong required outcome. No separate slop or unnecessary-production-complexity blocker was identified.

## Scope and limits

This approval request covers existing baseline 500 response/logging behavior only. It is not a complete R02.c handler matrix, a 35-operation acceptance, or a fresh extension/License/WeChat/hardware/browser acceptance. Other untouched handlers and their response mappings are outside this fix. Browser testing remains user-owned. The evidence README correctly leaves final full checks and independent double approval pending.

No dedicated current notepad was supplied; the fixed diff, scoped README and referenced artifacts served as the task record. `omo-agent-toolkit ulw-loop status --json` reports `ULW_LOOP_PLAN_MISSING`, so this report uses the documented fallback path.

## Blockers

- Record app history's unavailable capability without manufacturing a provider error, update that scenario's diagnostic expectation, then present one revised fixed snapshot for both independent reviews.
