# Latest-reading fix: independent code review

Exact candidate: `6ed49166e940861740b2c9a1fee4166c0523e2ca`

Base: `0cf0b277ac4ce7c81eef62b1ead429cb7e99067c`

Export inspected and tested: `/tmp/iolink-latest-20261013`

Reviewer: `/root/latest_code_review`; no implementation edits or staging performed.

codeQualityStatus: WATCH

recommendation: REQUEST_CHANGES

## Findings

### CRITICAL

None.

### HIGH

None.

### MEDIUM

1. **New management-side provider failures do not use the declared API error code.** `internal/adminapi/handlers.go:348` and `internal/adminapi/handlers.go:613` return HTTP 500 with `error="telemetry query failed"`. `docs/api/admin-openapi.yaml:1352` specifies `error=internal_error` for InternalError, and the device GET declares this response. The corresponding app-side changes already use `internal_error`. These are newly introduced failure responses, so using a diagnostic sentence here adds a divergent public error value; consumers handling the declared error code cannot classify the failure consistently. `internal/adminapi/latest_reading_test.go:121` currently pins the divergent value, and my independent HTTP test replay passed that assertion. Use the contractual `internal_error` body on both new management-side failure paths and assert that value in the HTTP test. Keep diagnostic details in the injected logger.

### LOW

None.

## Verification performed

- Inspected the full production and test diff, all six candidate evidence files, the relevant R10/R17/R22 acceptance criteria, and the existing pagination, ownership and telemetry adapters. All 14 changed export blobs match the requested Git tree exactly; zero mismatches.
- From the export, independently ran `GIN_MODE=release IOLINK_TEST_PG_DSN=<dedicated instance> go test -race -shuffle=on -count=1 -v ./internal/adminapi ./internal/appapi ./internal/core -run 'Test(PondLatest|Latest)'`. Exit 0; no SKIP. Management handler scenarios, 30 app HTTP scenarios, and `TestLatestReadingHTTPWithLargePondAndDatabaseFailure` passed. Package results were adminapi 1.031s, appapi 1.059s, core 2.131s. Shuffle seeds: 1791904466422532228, 1791904466420191651, 1791904466590181211.
- The real PostgreSQL test authenticates both API consumers, stores the 201st device shadow through the real core, checks both pond summaries and the empty pond, and makes six HTTP reading surfaces fail with 500 by making shadow storage unavailable. The fake app HTTP tests additionally verify wrapped expected errors, generic-product handling, timeouts, deterministic tie selection, rejection of partial success and safe correlated diagnostic output.
- Independently ran `go vet ./internal/adminapi ./internal/appapi ./internal/core`: exit 0. `gofumpt -l` on all eight changed Go files produced no paths. `git diff --check` produced no errors. Runtime: Go 1.26.8, Linux amd64. The reused PostgreSQL container has task label `iolink.task=latest-readings-20261013`; no reviewer container was created.
- Inspected executor artifacts `docs/evidence/deployment/2026-10-13-latest-readings/{admin-before.log,app-before.log,admin-after.log,app-after.log,real-pg-http.log}`. The before artifacts show the actual truncation and swallowed-provider failures. Inspected the additional `snapshot-verify.log` and `contracts.log` supplied outside the frozen candidate: recorded build/vet/full Go race results for 33 tested packages, and 96 operations / 568 synthetic fixtures. Those are executor results, not an independently rerun full suite; synthetic contract fixtures do not detect the live error-value mismatch above.

## Required skill-perspective check

Ran the read-only `remove-ai-slops` and `programming` perspectives after loading both SKILL.md files, the Go reference, error-handling/testing guidance and the cross-language logging guidance.

- **Overfit/slop pass:** no deletion-only tests, prose pins, tautological tests, or unnecessary production parsing/normalization identified. The 201-device fixture observes the actual truncation boundary, and the provider-failure fixtures observe HTTP outcomes rather than private implementation calls. Existing HTTP consumer fakes are supplemented by real authenticated PostgreSQL integration. The error-body assertion is relevant but currently encodes the wrong contractual value, as identified in the MEDIUM finding.
- **Programming pass:** error chains use `errors.Is`, provider failures propagate rather than becoming empty successes, request context reaches adapters/logging, and constant diagnostics use the injected logger with the existing safe field rules. The new pagination helper uses the existing consumer port, filters by pond and counts the collection it actually reads; the successful latest response reuses the existing serializer. No speculative interface, dependency or unrelated feature was added.
- All four new Go files are below 250 pure LOC (48, 111, 209 and 110). Existing handlers and repos exceed that ceiling in the baseline; this bounded fix does not claim to resolve that existing structural debt. No new oversized module was added. Strict golangci-lint/nilaway checks were not part of the supplied evidence and are not claimed passed.

The public-error mismatch violates the programming perspective's observable-contract requirement. No separate remove-ai-slops violation was identified.

## Scope and limits

Authorization and repository tenant/ownership filters were retained. Success shapes, expected missing-shadow nulls, unsupported-product behavior and timestamp/device-number tie rules were retained. Pagination now visits the target pond's complete active-device collection; actual adapter SQL orders by device ID and preserves tenant filtering.

This is a review of the latest-reading fix only, not an approval of all R10/R17/R22 acceptance or the entire V1 stage. Browser interactions are user-owned. Real WeChat, hardware, final License delivery and external operational acceptance were not exercised. The candidate evidence README correctly still says final freeze/double review results are pending. No current task-specific notepad path was supplied; the candidate README, full diff and evidence artifacts were used as the scope record. `omo-agent-toolkit ulw-loop status --json` reported `ULW_LOOP_PLAN_MISSING`, so this report uses the documented fallback location.

## Blockers

- Align the two new management-side 500 bodies and their HTTP assertion with `error=internal_error`, then present the revised fixed candidate for both independent reviews.
