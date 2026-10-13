# Original HTTP baseline: final bounded code review

Exact candidate: `91926edee43fb1d05e3ea32e914f44149259d5c7`

Base: `4fffc4b` (v0.0.36)

Export inspected and tested: `/tmp/iolink-baseline-http-r2-20261014`

Reviewer: `/root/latest_code_review`; no implementation, fixture, exported-source or staging edits. Only this report was written.

codeQualityStatus: WATCH

recommendation: APPROVE

**Approval scope:** the actual original-35 positive workflow and named negative cases in this evidence bundle, together with closure of the three findings from the dffad35 review. This is not approval of every possible baseline fault, the whole R02.c/R02 requirement, or the later platform matrix. The known remaining issues below are part of that distinction.

## Findings and disposition

### CRITICAL

None in the approved fix scope.

### HIGH

The previous fail-open WeChat finding is resolved. The adapter rejects non-2xx before parsing identity data, wraps the known invalid-code sentinel for code 40029, and the login boundary returns unauthorized for that case and upstream_error/502 for other failed exchanges. No user provisioning occurs on those error branches.

### MEDIUM

The two previous blocking findings are resolved:

- The common response components now encode their already documented machine codes with enum constraints. The validator rejects bad id for a common 400 response, and the controllers/auth boundaries use the declared common values. Generic Error and separately scoped responses were not replaced by a global enum.
- The new app parser checks ParseQuery errors; the four affected app query surfaces and six original admin collection surfaces reject malformed escapes. This no longer treats the specific malformed-query class in the rejection report as a missing/default parameter.

**Known follow-up scope, not closed by this approval:**

- `internal/adminapi/auth.go:132` and `internal/appapi/server.go:320` still combine token-version storage errors with revocation into 401; app authority failures at line 326 also become 401. The coordinator reports an actual relation-failure reproduction in the next platform matrix. This preexisting shared-auth fault classification is explicitly pending; this candidate's green named failure cases do not establish that all provider failures produce 500.
- `internal/adminapi/handlers.go:318` and line 781 still treat an explicitly empty optional farm_id/pond_id as absent through the raw != empty condition. ParseQuery handles malformed encoding but does not establish correct handling of every otherwise decoded illegal value. Empty optional numeric filters remain a further input-coverage item; no claim of a complete negative-input matrix is made.

### LOW

The disclosed Python strict type-check limitations remain; see the skill check below. No new low-severity blocker identified.

## Exact final source and independent checks

- Verified all **1,331** export blobs against the exact candidate tree: zero mismatches. Inspected the full final delta against dffad35, including common-response schemas, handler/auth changes, query helpers, the WeChat adapter, shared non-camera auth response mapping, regression fixtures and archived initial rejections. SQL/domain changes already reviewed in the original candidate are unchanged. Unrelated playback, user AGENTS and next-platform drafts are excluded.
- Independently replayed five final-export real PostgreSQL/TCP test groups: management rejected requests, six admin malformed-query cases, app rejected requests including four malformed-query cases, all seven WeChat upstream failures, and the existing role/farm-scope matrix. Command used `go test -race -shuffle=on -count=1 -v ./internal/core` with the dedicated DSN and explicit contracts Python. Exit 0, core **38.367s**, shuffle seed 1791912522872973370. The HTTP schema validator came from this exact export.
- The seven independently observed WeChat results were: invalid code 200->401; busy provider 200->502; HTTP 503 carrying openid->502; HTTP 400 carrying invalid-code JSON->502; malformed JSON 200->502; missing openid 200->502; refused TCP provider 0->502. Every case verified unchanged user count, no token-bearing success body, safe completion diagnostics and response/downstream request-ID correlation. This directly covers the prior fail-open case without real WeChat credentials.
- Independently ran the final-export validator suite: **29 passed in 2.95s**, exit 0. Repeated the exact counterexample: common admin PUT `/ponds/{id}` 400 with bad id is now invalid with keyword enum and schema path properties/error/enum; invalid_request is valid. Private values are absent from validator diagnostics.
- Independently ran final-export `go vet ./internal/adminapi ./internal/appapi ./internal/core`: exit 0. `gofumpt -l` on final-delta Go files produced no paths; product/schema/script diff whitespace checks were clean.

The original independent review also ran all 35 positive operations, device filtering, disabled-rule activation and batch rollback against real PostgreSQL. Their unchanged business/SQL code remains supported by those results; the final executor additionally replayed the complete current suite on this final source.

## Final executor evidence and required wait

I waited for the coordinator's terminal result before approving. It reported `final-snapshot-verify.log` run 95867 exit 0, with **33 tested Go packages**, core 232.874s, build/vet/race; focus run 8464 exit 0; Python/static run 68247 exit 0, 110 tests and 96 operations / 584 synthetic fixtures. I inspected the final log files: full results contain 33 ok lines and no FAIL/DATA RACE; the focus contains 28 top-level passes and no SKIP/FAIL. These are executor observations, not an independent rerun of the entire Go suite.

The earlier snapshot-* results, selector/prerequisite failures and both initial rejection reports remain archived as pre-fix observations. They are not labeled final-source approval. The README identifies the unchanged web gitlink/dist prerequisite and does not claim browser acceptance.

## Required skill-perspective check

Both `remove-ai-slops` and `programming` checks ran. Their SKILL.md files and Python/Go/error/testing/logging guidance were consulted in the original full review and applied again to the final correction.

- **Tests:** no deletion-only, prose pin, tautological or implementation-removal tests identified. Existing role counts were retained and adapted to DevicePage; old active-alarm fixtures explicitly preserve Enabled:true, and disabled-rule behavior is independently tested. Updated WeChat diagnostic expectations reflect the deliberate non-2xx precedence, with additional HTTP-200 invalid/malformed cases retained. New tests observe live HTTP, real-store state and safe logs rather than mirroring a private helper implementation.
- **Production:** the query boundary uses the standard error-returning parser, not a hand-written parser. Strict JSON processing remains at the untrusted boundary. Common machine values are schema-consumed constraints. The WeChat guard is a necessary trust-boundary decision and preserves actual failures; no dependency, speculative provider interface, SDK leak into domain or unrelated business feature was added. Camera-specific auth envelopes retain their original branch; only the shared non-camera response path is normalized.
- **Static Python:** the recorded twelve basedpyright diagnostics remain explicitly not passed. Existing resolve/dynamic validator typing and required exhaustive unreachable branches were not hidden with casts or ignores. Typed input/result models and runtime adapters plus privacy regressions mitigate the observed boundary risks. No separate actual runtime/privacy defect attributable to those diagnostics was found, and a broad dependency/stub rewrite is not required for this bounded approval.

The three reported programming/boundary violations have been fixed for their observed classes. Known shared-auth/empty-filter follow-ups and existing oversized handler/store debt keep codeQualityStatus at WATCH. They do not disappear because this substep is approved.

## Residual acceptance limits

V1d and whole R02 remain ongoing. The complete 96-operation live matrix, all possible failure/input combinations, real WeChat credentials/hardware, License delivery, browser interaction, large recovery and long capacity are not certified here. Future work must use fresh source evidence and its own independent approval rather than inheriting this result as complete project acceptance.

No ulw-loop attempt directory is available (`ULW_LOOP_PLAN_MISSING` during setup), so this report uses the documented fallback path. The initial code review is preserved in `.omo/evidence/baseline-http-code-review.md` and its archived evidence copy.

## Blockers

None for the stated bounded fixes and actual test set. The pending acceptance scope and follow-up findings above must remain visible in the implementation ledger.
