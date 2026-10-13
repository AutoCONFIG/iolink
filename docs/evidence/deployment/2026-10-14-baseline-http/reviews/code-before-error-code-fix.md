# Original 35 HTTP operations: independent code review

Exact candidate: `dffad35ac6a5adc871621929c5a2bea22d9e724e`

Base: `4fffc4b` (v0.0.36)

Export inspected and tested: `/tmp/iolink-baseline-http-20261014`

Reviewer: `/root/latest_code_review`; no product, exported-source, fixture or staged-file edits.

codeQualityStatus: BLOCK

recommendation: REQUEST_CHANGES

## Findings

### CRITICAL

None.

### HIGH

1. **The login operation does not fail closed on unsuccessful WeChat HTTP responses, and collapses genuine upstream failures into authentication rejection.** `internal/appapi/wechat.go:85` records the HTTP status but proceeds directly to JSON decoding; lines 92-96 accept any nonempty openid with errcode zero. A non-2xx response carrying such JSON therefore becomes a successful exchange, after which `internal/appapi/server.go:209` can provision a user and issue a token. Conversely, transport/decode/provider failures all reach lines 204-207 and return 401. `docs/api/openapi.yaml:26` requires invalid/expired codes to produce 401 and unavailable WeChat to produce 502, with that response explicitly declared at line 54. This is within the audited original `/auth/login` operation. Check HTTP success before trusting the identity body, distinguish known invalid-code errors from upstream failures, and add controlled HTTP plus real-store cases proving upstream failures return 502 and create neither user nor token. This does not require real WeChat credentials. These outcomes follow directly from the inspected branches; this reviewer did not call the external WeChat service.

### MEDIUM

2. **Schema-only response checks currently certify error values that contradict the explicit common response contract.** `internal/adminapi/handlers.go:423` returns `{"error":"bad id"}` for the newly rejected nonpositive pond ID. The same class of old error strings remains in other baseline/auth negative paths. `docs/api/admin-openapi.yaml:1350`, 1356, 1362, 1368 and 1374 declare fixed invalid_request/unauthorized/forbidden/not_found/conflict values. The new validator at `scripts/validate_http_response.py:147` checks only the generic Error schema, which permits any string. I independently supplied the exact admin PUT `/ponds/{id}` 400 response with error=bad id to `validate_response`; it returned `{"valid":true,"issues":[]}`. Thus the green live schema matrix does not establish these declared error semantics. Encode the fixed values for their actual common responses, align the original 35 handlers/auth boundaries and make the regression reject mismatched values. Keep explicitly declared feature/License-specific response semantics rather than applying an indiscriminate global error enum. The test at `scripts/test_validate_http_response.py:118` currently confirms permissive generic-string behavior; that is truthful for the current schema but insufficient as the baseline contract gate.

3. **The new device query parser silently turns malformed query values into defaults.** `internal/appapi/device_list.go:19` calls `URL.Query()`, whose standard-library contract explicitly discards malformed value pairs. A malformed pair such as page=%zz can disappear before `values.Has("page")`, selecting default page 1 instead of returning 400. The API's general contract requires illegal query parameters to return 400 without silent fallback. Use the error-returning `url.ParseQuery` at this boundary and cover a malformed escaped query through TCP HTTP. The other original collection handlers should reject the same malformed input class when claiming the complete baseline matrix. I verified the actual `go doc net/url.URL.Query` behavior description; no broad query abstraction or new dependency is required.

### LOW

None beyond the documented existing limitations below.

## Independent verification and exact source

- All **1,312** exported blobs match the requested Git tree; zero mismatches. Inspected the complete production diff, SQL paths, DTO/input helpers, public login projection, scoped pagination, new schema CLI and tests. Unrelated playback/AGENTS edits are excluded; app server contains only the selected login changes.
- Independently ran the final-export validator tests with the repository contracts Python: **29 passed in 4.94s**, exit 0. These include real stdin invocation, rejection of non-finite values, body-presence inconsistency, invalid content/media/selection, private additional properties and timestamp format failures without echoing secret markers.
- Independently ran `go vet ./internal/adminapi ./internal/appapi ./internal/core` from the export: exit 0. `gofumpt -l` on the changed production Go files produced no paths.
- Independently ran five real PostgreSQL/TCP scenarios with `go test -race -shuffle=on -count=1 -v ./internal/core`: the 25-operation management positive workflow, ten-operation app positive workflow, app device filters, disabled-rule activation, and missing-resource batch rollback. Exit 0, core 17.152s, shuffle seed 1791911223715069496, no SKIP. The contract helper used the exact exported scripts and explicit `IOLINK_CONTRACT_PYTHON`.
- Inspected the operator-provided `snapshot-verify.log`, `snapshot-focused.log`, `snapshot-python.log` and `snapshot-contracts.log` outside the frozen candidate. Full build/vet/race recorded 33 tested packages, the focus recorded 21 top-level TestBaseline results plus the resource-error focus and no SKIP, Python recorded 110 passes, and static validation recorded 96 operations / 584 synthetic fixtures. The coordinator reported terminal exit 0 for these runs before this review conclusion. Earlier prerequisite/selector failures are retained and are not counted as successful product evidence. This report is not an independent rerun of the entire Go suite.

## Required skill-perspective check

Loaded and applied `remove-ai-slops` and `programming`, including the Python/Go references and error/testing/logging guidance, before judging the code and tests.

- **Test relevance/overfit:** no deletion-only, natural-language prompt/prose pin or tautological test identified. The core scenarios use authenticated TCP HTTP, real storage and actual ingestion; rejected writes compare persisted state, the batch verifies transaction rollback, and the disabled rule verifies actual alarm behavior. Adapting device assertions to DevicePage preserves and strengthens the existing role-scope check. Adding Enabled:true to old alarm fixtures explicitly retains their original intended active-rule behavior; the assertions were not removed. The remaining error-code and upstream gaps are false confidence in acceptance breadth, as described in the findings, not evidence that the positive tests are useless.
- **Production scope/complexity:** strict key/null/duplicate/trailing-object parsing is at the untrusted boundary and is needed to enforce the stated input goal. The extra raw JSON pass addresses duplicate/case ambiguity not handled by ordinary Go struct decoding. Owner replacement correctly distinguishes absent from nullable; the pond update checks the already scoped pond and its original farm inside one transaction before modification. The new page port is a narrow consumer capability, and its parameterized SQL retains active membership, role/farm and explicit tenant filtering. No provider SDK leaks into domain/application and no speculative dependency or parser was added.
- **Static Python diagnostics:** the twelve recorded basedpyright diagnostics are not a passed gate. They arise from the existing untyped resolve helper, dynamically constructed SDK validator interfaces, and required exhaustive unreachable branches. The new code has typed input/result models, runtime adapters around contract/error metadata, no cast/type-ignore suppression, and safe stdout tests. I found no separate concrete privacy/runtime failure attributable to those diagnostics and do not require dependency upgrades or a broad stub/refactor project for this fix. Their unresolved static coverage remains an explicitly disclosed limitation; the real blocking defects are the observable contract/boundary failures above.

No independent slop or gratuitous-production-complexity blocker was found. The programming perspective rejects the upstream fail-open behavior and silent invalid-query fallback. Oversized existing handler/store files remain baseline debt; no claim of broad architectural cleanup or strict tooling compliance is made.

## Scope and limits

The good SQL, DTO, positive workflow and rollback results remain valid pre-revision evidence. They do not make the original 35-operation negative semantics complete. The full 96-operation live matrix, complete R02/M0-M8 acceptance, actual WeChat credentials/devices/License delivery, browser interaction, large recovery and long capacity remain outside this approval. The browser is user-owned. No source/binary/frontend identity is retroactively changed by these results.

No current task-specific notepad was supplied; the candidate README, full diff and cited evidence were used as the task record. `omo-agent-toolkit ulw-loop status --json` reports `ULW_LOOP_PLAN_MISSING`, so this report uses the documented fallback location.

## Blockers

- Fail closed and classify WeChat invalid-code versus upstream failures with no failed-login provisioning.
- Align and actually validate the common error values for the original baseline response scope.
- Reject malformed query decoding rather than silently defaulting dropped parameters.
- Freeze the revised source and obtain both independent reviews on that exact snapshot.
