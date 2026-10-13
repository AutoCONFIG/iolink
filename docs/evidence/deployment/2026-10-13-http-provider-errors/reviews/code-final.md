# Existing HTTP 500 safety: final independent code review

Exact candidate: `478f22c3396643c143f0fd9f8b9f48a272e1e701`

Base: `9c63186`

Export: `/tmp/iolink-http-errors-r2-20261014`

Reviewer: `/root/latest_code_review`; no implementation, test, evidence or staging edits. Only this review report was written.

codeQualityStatus: CLEAR

recommendation: APPROVE

## Findings

- CRITICAL: none.
- HIGH: none.
- MEDIUM: none remaining.
- LOW: none.

The manufactured-capability-error finding in `.omo/evidence/http-provider-code-review.md` is resolved. App history's missing `UserTelemetryRepo` now calls `providerUnavailable`. The new helper logs the actual capability-unavailable condition using the injected logger and request context, omits `err`, and retains HTTP 500 with the contractual `internal_error` body. Other provider failures still keep their safe error category.

## Exact final delta and independent verification

- Verified all **1,268** export blobs against the exact final tree: zero mismatches. The entire 144a-to-478f delta contains only the app history call replacement, its seven-line unavailable-capability helper, and the app diagnostic assertion update. No admin/core behavior, success shapes, authorization, 4xx/License classification, published scripts, unrelated playback draft or new business feature changes occurred in this correction.
- Inspected the final test's `json.RawMessage` field: an absent `err` produces no bytes, while `err=null`, an empty string or a fabricated category would fail the missing-capability expectation. Actual provider-failure scenarios retain the quoted safe-class expectation. This is a meaningful observable distinction rather than a relaxation of the previous assertion.
- Independently ran all 18 app TCP HTTP failure scenarios from the final export using `GIN_MODE=release go test -race -shuffle=on -count=1 -v ./internal/appapi -run TestBaselineHTTPProviderFailuresUseSafeInternalError`: exit 0, package 1.078s, shuffle seed 1791908377144810412.
- Independently replayed the exact `history_capability` scenario from the final export: exit 0, package 1.035s. Its actual dump line was ERROR, message `HTTP provider capability unavailable`, stage `http_provider`, a server-generated request ID matching the response, and **no err attribute**. The HTTP body remained `{"error":"internal_error"}`.
- From the final export ran `go vet ./internal/adminapi ./internal/appapi ./internal/core`: exit 0. `gofumpt -l` on the three delta files produced no paths. The Go diff whitespace check was clean.

The original independent review ran the unchanged management/core focus with a dedicated real PostgreSQL DSN: all 37 management scenarios and the seven real relation-failure surfaces passed, with no SKIP. The real test requires request-correlated `postgres_42P01` dump records and rejects table-name/password/JWT leakage. Those files are byte-identical in the final candidate; the final independently rerun app scenarios cover the only modified behavior.

I inspected the executor's additional `final-focused-v2.log`, outside the frozen candidate: the complete 55-scenario focus and real PostgreSQL seven-surface test passed, including the corrected no-err capability line. I also inspected `snapshot-final-verify.log`, which now contains build/vet commands and 33 tested-package race results with no failure result shown. These are executor artifacts; the release coordinator owns their terminal command-status capture. The initial `app-after.log` is an earlier iteration and is not proof of the corrected capability criterion. Final evidence metadata must distinguish that iteration from `final-focused-v2.log` and preserve the initial rejection report.

## Required skill-perspective check

The `remove-ai-slops` and `programming` checks ran. Both SKILL.md files and Go/error/testing/logging guidance were consulted during the original full review and applied to this final delta.

- No deletion-only, prose-pin, tautological or needless parsing tests identified. The fixtures observe actual HTTP behavior and dump safety; real PostgreSQL integration supplements consumer failure fakes. The RawMessage change is justified by the requirement to distinguish absence from a null/error value.
- The correction adds only the necessary boundary decision. It does not fabricate an error, emit secrets, repeat logging across propagation layers, add a new dependency/interface, or introduce production normalization. Messages and stage remain constants; actual error values use the existing redaction path. The small package-local helper follows existing logger wiring rather than adding shared indirection.
- No remaining violation of either perspective was found in the bounded final diff. New modules remain below 250 pure LOC; oversized legacy handlers remain existing debt. Strict golangci-lint/nilaway checks were not performed or claimed passed.

## Scope and residual limits

Approval applies only to this fixed existing-baseline HTTP 500 response/diagnostic substep. It does not establish the complete 35-operation R02.c matrix, the 96-operation live extension matrix, or fresh browser/WeChat/hardware/License acceptance. Existing dedicated latest-error handling is unchanged, and success/4xx ownership behavior remains outside the response-only corrections. No unrelated working-tree draft was included in this review.

The original REQUEST_CHANGES report remains preserved. No ulw-loop attempt directory is available (`ULW_LOOP_PLAN_MISSING` during review setup), so this report uses the documented fallback evidence path.

## Blockers

None for this final code review.
