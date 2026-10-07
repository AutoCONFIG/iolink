# M6d R41–R42 independent code review — d82aa82

- Date: 2026-10-07, Asia/Shanghai.
- Reviewed source: **`d82aa8249e70302ee70e8cacd5cef15177b31ef5`**.
- Reviewed web: **`207dcfa751545e11a63a3a013da7e60552fecd75`**.
- Source baseline: `v0.0.9` / `9f46b6e549d36a6eac8c197acfbe64d2a91585a4`.
- Web baseline: `0aa7771acf0ef6322c9ac1e4c9839616646bb675`.
- Frozen executor test source: `b4b93359976c0529c8f52f50b02a968e3f99750e`; independently verified that its production, test, script, contract, and web paths are identical to the reviewed source. The intervening commit archives evidence and pending status.
- `codeQualityStatus`: **BLOCK**.
- `recommendation`: **REQUEST_CHANGES**.
- `reportPath`: **`.omo/evidence/m6d-d82aa82-code-review.md`**.
- Delivery copy: **`docs/evidence/M6d/2026-10-07/reviews/code-d82aa82.md`**.

**This candidate is not approved.** Focused existing checks pass, but independent adversarial probes reproduce resource grant widening, an overflowing timestamp check, and invalid resource grants that the web client cannot load. Later executor fixes in the shared checkout are outside this verdict.

## Goal, scope, and authority

Deliver the complete M6d production change from v0.0.9: tenant-scoped Key management, a one-time 32-byte encrypted secret, immediate rotation/revocation, scope/resource authorization, sanitized diagnostics/audit, canonical HMAC signing, a ±300-second timestamp window, durable atomic nonce protection and shared 60/minute burst-10 throttling, License guards, third-party documentation, and working management UI. Applicable cross-stage checks are R36.b/R37.b/R39.b.

Read `AGENTS.md`, `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/PLAN.md`, M6b/M6c/M6d in `docs/EXTENSIONS.md`, and the negative-gate and R36/R37/R39/R41/R42 criteria in `docs/ACCEPTANCE.md`. Reviewed all changed production source, migration 012, relevant neighbors, tests, composition/contract/checker changes, and the complete web diff. The immutable change manifest is [code-d82aa82-changed-files.txt](code-d82aa82-changed-files.txt); production/contract diff is [code-d82aa82-source.diff](code-d82aa82-source.diff), with the complete nested web diff in [code-d82aa82-web.diff](code-d82aa82-web.diff). Redis research adds no runtime dependency and is explicitly future work.

Used detached, locked source/web worktrees. No product code was edited, no agents were spawned, and the user's `.github/workflows/ci.yml` and `.tmp/` were preserved. The ULW status command returned `ULW_LOOP_PLAN_MISSING`, so the required fallback report path is used. No notepad path was supplied or located under `.omo`. Pending manifests and the absence of this review's approval are not circular blockers.

## Findings by severity

### CRITICAL

None established.

### HIGH

**H1 — Malformed device input still becomes a tenant-wide grant.**

- Requirement: R41 resource scope; `web/DESIGN.md:47` explicitly says erroneous resource input must block submission without expanding permissions. The operation-boundary safety rule also requires malformed input to fail closed.
- Code: `web/src/views/SystemView.vue:57`, `:58`; `web/src/api/admin.ts:89`; `internal/core/open_resources.go:23` and `:57`.
- Unlike numeric IDs, `parseDeviceNos` splits, trims, and discards empty entries. Nonempty `,` and `, ,` both become `[]`. With the other resource inputs blank, issuance sends an unrestricted resource set.
- Independently executed the exact function body from web207dcfa: [parser evidence](code-d82aa82-device-parser.log). Passed that exact empty result to real admin HTTP issuance, then made correctly signed requests against real core/Timescale. Both `/open/v1/ponds/8302` and `/open/v1/devices/m6d-hidden` returned **200**, exposing resources outside the intended device restriction: [adversarial evidence](code-d82aa82-adversarial.log), `TestReviewM6dMalformedDeviceGrant`.
- Fix before approval: accept deliberate blank input separately; reject every empty/invalid token in nonblank comma input before issuance, preserve the input, and explain the error. Add a production UI regression that establishes no issuance request or persisted key for comma-only and mixed invalid device input. Keep valid device-only filtering.

**H2 — Timestamp subtraction overflow bypasses the ±300-second window.**

- Requirement: R42 and `docs/EXTENSIONS.md:61`, stale/out-of-window signed requests must be rejected.
- Code: `internal/core/open_auth.go:98`.
- Both `now.Unix()-req.Timestamp` and `req.Timestamp-now.Unix()` use int64 subtraction. For `req.Timestamp = now.Unix()+math.MinInt64`, both differences overflow to negative values and neither exceeds 300.
- Independently generated correct signatures for each timestamp. ±300 accepted and ±301 rejected, but delta **−9223372036854775808 authenticated successfully**. This is a correctly signed request, so signature failure does not mask the time-window bug: [adversarial evidence](code-d82aa82-adversarial.log), `TestReviewM6dTimestampOverflow`.
- Fix before approval: use an overflow-safe timestamp-window comparison. Cover correctly re-signed ±300, ±301, zero, negative, and int64-extreme timestamps through production authentication/HTTP. Do not change a timestamp while retaining the signature for a different timestamp.

**H3 — Issuance persists contract-invalid resource values and breaks key-list loading.**

- Requirement: R41 resource management; ACCEPTANCE negative gate N requires invalid input 400. The versioned `APIKeyResourceScope` requires positive farm/pond IDs and nonempty device strings (`docs/api/admin-openapi.yaml:1775`). Arrays are not nullable.
- Code: `internal/core/api_keys.go:21`, `:38`; `internal/adminapi/api_keys.go:95`, `:107`; `web/src/domain/api-key.ts:4`, `:61`.
- Validation checks the resource array lengths but not their elements. JSON decoding also turns explicit null arrays into nil slices. Independently reproduced real admin HTTP **201 plus one persisted key** for `farm_ids:[0]`, `farm_ids:[-1]`, `pond_ids:[0]`, `device_nos:[""]`, `farm_ids:null`, and `device_nos:null`: [adversarial evidence](code-d82aa82-adversarial.log), `TestReviewM6dInvalidResourceHTTP`.
- The first four produce resource values rejected by the exact current `parseAPIKeys` response boundary. A single such persisted key rejects the whole list, preventing the UI from managing otherwise valid keys: [response-boundary evidence](code-d82aa82-resource-response.log). Null arrays additionally erase an attempted resource dimension and become unrestricted. Severity is HIGH for these demonstrated management and grant-semantics failures.
- Fix before approval: parse the concrete request shape at the HTTP boundary, reject explicit null arrays, and enforce the positive-ID/nonempty-device item contract before committing issuance. Invalid input must leave no Key/audit state. Test real handler status/persistence and the resulting normal list/UI behavior. Ownership/existence validation was not independently reproduced as an additional defect in this review.

### MEDIUM

**M1 — Some retained tests still offer misleading or redundant coverage.**

- `internal/core/open_auth_test.go:59` computes standard-library HMAC over copied constants without calling production signing/authentication; corrupting production canonical assembly leaves it green. The new production-verifier test at `internal/core/m6d_persistence_http_test.go:69` supplies the meaningful vector proof.
- `internal/core/m6d_open_api_test.go:73` and `:78` change the timestamp without recomputing the signature. Removing the timestamp guard still returns unauthorized for those altered signatures. The old body case at `:109` also changes nonce and body while reusing the original signature. The new body-only test repairs body coverage, but the time-window gap led to H2.
- The demo browser smoke at `web/e2e/m6d.spec.ts:10` is named as a signed-management-contract check while asserting a canned demo secret and empty logs. It is only a demo UI smoke; `m6d-live.spec.ts` is the actual management/API evidence.
- Remove redundant stdlib-only coverage and rename or replace misleading cases with independently distinguishable behavior. MEDIUM by default for useless/false-confidence tests; H2 records the demonstrated correctness failure separately.

**M2 — Redundant parsing and an unused activation branch add avoidable production complexity.**

- `web/src/domain/api-key.ts:62`, `:66` parse every list item via the array schema, then invoke item functions that parse that already typed item again (`:32`, `:45`). A typed mapper or schema transform can map once after the boundary parse.
- `internal/core/api_keys.go:188` accepts an `active` boolean, but its sole caller passes false (`:128`). The `revoked_at=NULL`/`api_key.activated` path at `:198`, `:210` is unused and outside the delivered revoke/rotate API.
- This is not required hardening, parsing, or abstraction for R41/R42. Simplify the mapping and retain only the required revoke behavior. MEDIUM quality notes; no additional runtime failure was established from these two cases.

**M3 — The documented deployment adjustment for rate/burst is absent.**

- `docs/EXTENSIONS.md:61` specifies deployment-configurable throttling. `internal/core/open_auth.go:169` always refills at 1 token/second and caps at 10; migration `012_open_api.sql:13` initializes 10. No rate/burst configuration or composition input exists in `internal/platform/config.go` or the new wiring.
- The required default 60/minute burst10 is tested and works; changing persisted tokens cannot configure a refill rate or burst because the next call uses the constants again.
- Resolve this explicit design gap before claiming the complete M6d design delivered: provide validated deployment configuration with current defaults, wire it through composition, and cover nondefault consumption/refill/retry behavior. MEDIUM functional omission, not a bypass of the default limiter.

### LOW

None beyond the nonblocking quality notes above. Existing large composition/admin files were not escalated simply for inherited size. Evidence logs contain trailing whitespace; the production/contract and web diffs pass `git diff --check`.

## Earlier rejection fixes independently rechecked

| Earlier item | Current result |
|---|---|
| Numeric resource IDs silently discarded | Fixed for numeric UI inputs; H1 shows the sibling device input remains unsafe; H3 shows the server boundary remains incomplete |
| Unsigned body tail accepted | `http.MaxBytesReader` rejects 4 MiB+1 with 413; focused current route test passes |
| Device-only pond scope ignored | Device-only, pond-only, and farm-only real list/detail tests pass; SQL now intersects the device restriction |
| Stale admitted Key mutation authority | Mutations call live authorization within their transaction; independent stale rotate/revoke probe rejects both and preserves active Key/audit counts |
| Six absent contract operations | All admin Key list/create/rotate/revoke/audit and open list/detail operations are declared; 67-operation/333-fixture check passes |
| Open diagnostics bypass injected logger | Composition passes its logger; open server forwards it to the same request-logging middleware; route-template/server-ID fields remain safe |
| No scope selector/display | Supported checkbox selection and granted scope display exist; live browser evidence asserts the selected payload and displayed scope |
| Empty list `null` | Current list accumulators return allocated empty slices; new-tenant live HTTP/browser checks and typed empty-list tests exist |
| Weak vector/body/restart/UI proof | Production vector/body-only/persisted nonce/rate and real management UI tests are added; M1 identifies remaining weak cases |
| Response coercion/casts in real API mapping | Concrete Zod parsers reject wrong/missing values; correct omitted `revoked_at` is accepted; H3 exposes malformed values still persisted by the server |

Independent issue denials for member/viewer/support return 403. The overlay's first support fixture omitted its required expiry and failed a database CHECK; it was corrected and all three denial rows passed. That fixture failure is retained transparently and is not a product finding.

## Skill-perspective check

**Ran before judging test relevance or maintainability.** Loaded `programming/SKILL.md` and `remove-ai-slops/SKILL.md` under `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.19/skills/`; consulted Go/TypeScript programming references and the testing/error/data-modeling criteria. Applied `review-work` evidence and goal checks inline as a leaf reviewer, with no extra agents or concurrent browser run.

The diff violates these perspectives in H1/H3 (untrusted input normalized or admitted into invalid/broader state), M1 (tests without the named production seam), and M2 (validation duplicated after typed parsing and unused generality). No deletion-only test, removal-pinning test, tautological production-call assertion, prose/prompt pin, or unrelated production payload extraction was found. Required HTTP/API response parsing, RFC3986 canonicalization, and secret encryption are appropriate boundaries. New runtime Go modules are below 250 pure LOC: Key use cases 205, open authentication 191, open HTTP 172; the new integration lifecycle test has 233.

## Commands and inspected artifacts

Environment independently checked: Go1.26.8, Node24.21.0, PostgreSQL16.15, TimescaleDB2.30.0, Linux amd64. Dedicated DB endpoint `127.0.0.1:55439`; credentials, JWTs, auth headers, secrets, and real response bodies were not printed. Each integration test used an isolated create/drop database. See [environment record](code-d82aa82-environment.log).

| Verification | Actual outcome and artifact |
|---|---|
| `GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated> go test -race -shuffle=on -count=1 ./internal/core ./internal/openapi -run 'TestM6d(Issue\|Concurrent\|Body\|Restart\|Published\|Resource\|Device\|Signed\|Administration)\|TestCanonicalOpen\|TestConsumeOpenRate\|TestOpenSignature\|TestRoutes' -v` | PASS: core6.817s, openapi1.135s; no integration skips. First run lacked embedded frontend in the fresh review tree and failed setup; after building/embedding, rerun passed. [Complete log](code-d82aa82-go.log) |
| `npm ci --prefix web`; `npm test --prefix web -- --run`; `npm run typecheck --prefix web`; `npm run build --prefix web`; `make embed-front` | PASS:44 tests, typecheck/build/embed; existing Rollup/bundle warnings. [Log](code-d82aa82-web.log) |
| Contract checker, architecture checker `--all`, production/contract diff check and nested web diff check | PASS:67 operations/333 synthetic fixtures;55 requirements; both selected diffs clean. [Log](code-d82aa82-contracts.log) |
| `GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated> go test -overlay <review-overlay> -race -count=1 ./internal/core -run '^TestReviewM6d' -v` | FAIL: malformed device grant, six malformed resource requests, timestamp overflow; PASS stale rotation/revocation and member/viewer/support issuance denials after fixture repair. [Log](code-d82aa82-adversarial.log); reproducible [probe source](code-d82aa82-probe.go.txt) |
| Exact source parser and response parser through Node24 | Device comma input becomes []; contract-invalid resource values reject the complete Key list. [Device parser](code-d82aa82-device-parser.log), [response parser](code-d82aa82-resource-response.log) |

Audited the frozen executor logs themselves: [full Go](../logs/frozen-go-verify.log), [contracts/web/Docker](../logs/frozen-contracts-web-docker.log), [real browser](../logs/browser-live.log), [candidate browser](../logs/candidate-browser.log), and [focused HTTP](../logs/http-final.log). They show actual successful runs, including the production management scenario and17 separate regression scenarios; synthetic contract fixtures do not verify live request validation. The frozen full-Go log has the core91.454s result reported in the matrix. Reusing that full-suite evidence does not override the independent failed probes.

Opened all four frozen screenshots (`key-issued-375.png`, `key-issued-768.png`, `key-issued-1440.png`, `audit.png`) under `../browser/m6d-live-real-tenant-owner-a6b29-evokes-and-reads-safe-audit/`. They show the Secret alert masked, selected scopes, a created Key, and the three management audit actions. These screenshots and the actual live test support the covered management flow; they do not cover comma-only device input, timestamp extremes, or invalid persisted resources. No concurrent browser was launched by this reviewer and no external-provider acceptance is claimed.

## Blockers required before approval

1. **H1 / R41:** Reject malformed nonblank device input without issuing an unrestricted Key; prove through the production management path.
2. **H2 / R42:** Reject out-of-window correctly signed timestamps without integer overflow; add actual signed boundary/extreme coverage.
3. **H3 / R41 + negative gate N:** Reject contract-invalid resource items and null arrays with400 and no persistence; keep the returned Key list consumable by the UI.
4. **M3 / EXTENSIONS M6d:** Implement the explicitly documented deployment rate/burst adjustment, or obtain an authoritative requirement amendment before declaring the complete design delivered.

M1/M2 are medium quality follow-ups; the three reproduced HIGH findings require REQUEST_CHANGES regardless. Freeze a new source/web snapshot after fixes and run fresh independent reviews. This report never approves uncommitted or later changes.
