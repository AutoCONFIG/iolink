# M6d R41–R42 independent code-quality review — source 865e9b2

- Date: 2026-10-07 (Asia/Shanghai).
- Reviewed source: **`865e9b28f531c1402bcf96a53c8663ad0e2dab89`**.
- Reviewed web submodule: **`3c1d860e2cca0b6ed8207142f5506784f5fef452`**.
- Source baseline: `v0.0.9` / `9f46b6e549d36a6eac8c197acfbe64d2a91585a4`.
- Web baseline: `0aa7771acf0ef6322c9ac1e4c9839616646bb675`.
- `codeQualityStatus`: **BLOCK**.
- `recommendation`: **REQUEST_CHANGES**.
- `reportPath`: `docs/evidence/M6d/2026-10-07/reviews/code-865e9b2.md`.
- `notepadPath`: **none located**. `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`; the requested fallback review artifact is therefore this evidence path.

## Goal and success criteria

Review the complete M6d implementation for R41–R42: tenant/resource-scoped API Keys with one-time encrypted secrets, issue/list/rotate/revoke/audit management, live License and membership authorization, safe diagnostics, canonical HMAC signed `/open/v1` requests, an inclusive ±300-second window, durable ten-minute nonce replay protection, shared deployment-configurable rate limiting (default 60/minute and burst 10), the published API contracts, and the production management UI. Correctness requires malformed input to fail closed without partial writes, replay/scope/tenant/license/rate violations to return the documented outcomes, and secrets or signed payloads to stay out of logs and audit.

The candidate fixes the four blockers in the preceding d82aa82 review: malformed device-list input, timestamp subtraction overflow, invalid/null resource persistence, and missing rate/burst configuration. Those repairs were independently rechecked. The review below identifies one remaining high-severity replay defect and nonblocking contract/maintainability defects.

## Scope and diff binding

The complete source implementation diff was inspected from `v0.0.9` through product ancestor `21c3704197f89a91387084ed092af8e79c757eb2`; the candidate source `865e9b2` differs from that product tree only in status/evidence documentation (`git diff --name-status 21c3704..865e9b2` reports `docs/IMPLEMENTED.md`). The complete nested web diff was inspected from web baseline `0aa7771...` through `3c1d860`; the final web commit only stabilizes the live test wait relative to product web commit `85a8acd`. Thus this report is bound to the full immutable source/web SHAs above, not to later working-tree claims.

The source diff from the published baseline contains 67 paths (production, tests, migrations, contracts, deployment, and evidence); the nested web diff contains 15 paths. Commands used to collect the full diffs were:

```text
git diff v0.0.9..21c3704197f89a91387084ed092af8e79c757eb2
git -C web diff 0aa7771acf0ef6322c9ac1e4c9839616646bb675..3c1d860e2cca0b6ed8207142f5506784f5fef452
```

No product, test, or status files were edited by this reviewer; only the uniquely named review artifacts listed below were created in the locked review checkout.

## Findings by severity

### CRITICAL

None established.

### HIGH

**H1 — A valid signed request rejected with 429 can replay the same nonce later.**

- Requirement: `docs/EXTENSIONS.md:61` says a successfully authenticated nonce is stored atomically for ten minutes and R42 makes replay rejection an exit condition. The published API describes a rate-limited request as a separate 429 outcome; the signed request has already passed key, tenant, HMAC, and nonce insertion checks at that point.
- Code: `internal/core/open_auth.go:148-164` inserts the nonce inside the transaction, then returns `OpenRateLimitError` when no token is available. The deferred `tx.Rollback` removes the nonce because the rate-limit branch never commits.
- Independent real-Timescale HTTP probe: [code-865e9b2-nonce429.log](code-865e9b2-nonce429.log) first received **429**, replenished the persisted bucket, then sent the same correctly signed request and received **200**. The test fails at `m6d_persistence_http_test.go:112` with `rate-limited nonce was reusable, status=200`.
- Impact: a request that has passed authentication is not protected by the advertised replay window. The current endpoints are reads, but the failure is in the shared authentication seam and defeats the stated replay guarantee for any signed operation using it.
- Required fix: commit the nonce reservation for a valid signature even when returning 429 (without consuming a token), or move rate decision before nonce reservation and explicitly amend the contract to define 429 as unauthenticated. The former matches the current insertion order and the stated “successful authentication” rule. Add a production HTTP regression that repeats the same nonce after a 429 and requires 401.

### MEDIUM

**M1 — Internal API-key store/database failures are reported as 400.**

- `internal/adminapi/api_keys.go:39-49` maps every error other than the small sentinel set to `400 {"error":"invalid api key request"}`. The declared `/admin/v1/api-keys*` operations include a `500 InternalError` response (`docs/api/admin-openapi.yaml:1115-1173`).
- [code-865e9b2-admin-error.log](code-865e9b2-admin-error.log) drives the real authenticated admin route with a store returning `database unavailable`; the observed status is **400**, while the independent regression expected **500**.
- Fix by distinguishing typed input/validation errors from infrastructure errors and returning the stable `internal_error` 500 response for the latter. This also avoids making a transient outage look like a client defect.

**M2 — Canonical query parsing accepts invalid UTF-8 bytes.**

- `internal/core/open_auth.go:32-54` unescapes query components and passes arbitrary bytes to `rfc3986`; it never checks `utf8.ValidString`. `x=%FF` is therefore canonicalized as `x=%FF` and can be signed and authenticated, although the contract requires UTF-8 RFC3986 encoding (`docs/EXTENSIONS.md:59`).
- [code-865e9b2-query-utf8.log](code-865e9b2-query-utf8.log) contains a focused production-function test failure: `invalid UTF-8 accepted as "x=%FF"`.
- Fix by rejecting invalid UTF-8 immediately after unescaping (and apply the same boundary rule to path components if paths are intended to be UTF-8), with a regression for malformed byte sequences.

**M3 — Demo-mode API-key loading uses an unchecked TypeScript assertion.**

- `web/src/api/admin.ts:73-76` returns `JSON.parse(...) as APIKey[]` for localStorage data. LocalStorage is untrusted and this bypasses the Zod response boundary used by the real API; malformed stored data can reach the table as if it were a valid key list.
- The loaded programming perspective forbids assertion escapes and requires parse-at-boundary. Parse demo data with the existing schema, handle malformed storage explicitly, and return a typed value without `as APIKey[]`.

**M4 — The demo E2E test pins implementation constants rather than a meaningful behavior seam.**

- `web/e2e/m6d.spec.ts:10-21` asserts the literal `demo-secret-shown-once` and that the demo audit is empty. These assertions stay green when the demo workflow is broken in other ways and merely restate the canned adapter values. The real production management scenario is covered separately by `web/e2e/m6d-live.spec.ts` and the live browser evidence.
- This is a remove-ai-slops test-quality issue, not a production security failure. Replace it with an observable demo lifecycle assertion or keep the demo smoke limited to navigation/form behavior without pinning canned data.

**M5 — New use-case wiring relies on a long positional parameter list.**

- `internal/adminapi/server.go:122-133` and `internal/core/api_keys.go:56` pass six unrelated values to `IssueAPIKey` (context, tenant, name, scopes, resources, actor). This is harder to review at call sites and makes future additions easy to mis-order.
- A small issue command/value object would make the boundary clearer. No runtime failure was established, so this remains a maintainability note.

### LOW

No additional low-severity correctness issue was required for the verdict. Existing bundle-size and evidence-log formatting warnings do not alter the M6d behavior.

## Skill-perspective and slop review

This check **ran** before judging maintainability or test relevance. I loaded and consulted `omo:programming/SKILL.md`, its Go and TypeScript references, `omo:remove-ai-slops/SKILL.md`, and the `omo:review-work` evidence rules.

- H1 violates the programming/data-integrity perspective because a valid boundary decision leaves replay state rolled back.
- M1 violates typed error mapping and stable HTTP contract handling.
- M2 violates parse-at-boundary for the documented UTF-8 wire grammar.
- M3 violates the TypeScript no-assertion and Zod-boundary rules.
- M4 is an implementation-mirroring test under the slop/overfit checklist.
- M5 is the long-parameter-list complexity smell; it is nonblocking.
- No deletion-only test, prompt/prose pin, tautological production assertion, unsafe provider abstraction, or unrelated production data extraction was found. The strict resource parsing, response schemas, encryption, and signed-byte canonicalization are required boundary work rather than unnecessary normalization.

## Verification and evidence audit

| Verification | Result | Evidence |
|---|---|---|
| Focused real DB/core/openapi tests with race and shuffle, including signed timestamp extremes, resource ownership, concurrent nonce, configured rate, body tamper, restart state, and published vector | **PASS**; the browser test was skipped because this focused command did not set `IOLINK_M6D_BROWSER=1`, and that skip is not counted as a pass | [code-865e9b2-focused-go.log](code-865e9b2-focused-go.log) |
| Independent web unit tests, typecheck, and production build | **PASS**: 45 tests; typecheck/build exit 0 | [code-865e9b2-web.log](code-865e9b2-web.log) |
| Retained full Go race/shuffle/build/vet run and real browser-enabled candidate run | **PASS** for the frozen product source `21c3704`; no skipped result is counted as passing | [final-2-go-verify.log](../logs/final-2-go-verify.log), [README](../README.md) |
| Retained contracts, architecture, existing browser regression, and Docker build | **PASS**: 67 operations / 334 synthetic fixtures, 55 architecture requirements, 17 browser scenarios, image build | [final-06d2486-contracts-web-docker.log](../logs/final-06d2486-contracts-web-docker.log), [review-fixes-final-web.log](../logs/review-fixes-final-web.log) |
| Valid signed request after 429 reuses nonce | **FAIL**; observed 429 then 200 for the same nonce | [code-865e9b2-nonce429.log](code-865e9b2-nonce429.log) |
| Invalid UTF-8 canonical query | **FAIL**; `%FF` accepted | [code-865e9b2-query-utf8.log](code-865e9b2-query-utf8.log) |
| Internal API-key store error mapping | **FAIL**; observed 400 instead of declared 500 | [code-865e9b2-admin-error.log](code-865e9b2-admin-error.log) |

The retained real-browser evidence covers issue, one-time display disappearance, rotation, revocation, safe audit, malformed/null/foreign resource rejection, scope selection, and 375/768/1440 layouts. It does not cover replay after 429, invalid UTF-8, or infrastructure error mapping; the three new probe artifacts supply those missing observations. External issuer, WeChat, hardware, and later-stage acceptance remain `external_blocked` by scope.

## Blockers required before approval

1. **H1 / R42 replay guarantee:** ensure a correctly signed request rejected with 429 cannot reuse its nonce during the ten-minute window; add and pass the real HTTP regression.

M1–M5 should be corrected in the same follow-up where practical, but only H1 is a HIGH blocker under this review. Any CRITICAL or HIGH finding requires `REQUEST_CHANGES`; this report does not approve the snapshot.
