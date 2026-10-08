# M4 mini-program API demonstration code review

## Verdict

- `codeQualityStatus`: `WATCH`
- `recommendation`: `APPROVE`
- reviewed snapshot: `2b66812fd0d6e64609c1bc3fccbccccf71a4f99d`
- review date: 2026-10-08 (Asia/Shanghai)
- reviewer: independent code review agent
- blockers: `[]`

The current tree implements the intentionally small M4 software surface: six navigable pages, a shared API demonstration form, a typed fetch boundary, runtime response parsing, alarm loading/confirmation, and browser-only subscription outcomes. Real WeChat login, device execution, the native subscription dialog, HTTPS, credentials, and qualification remain explicitly `external_blocked`. The earlier login `Content-Type` defect and alarm error/empty-state overlap are fixed in this snapshot.

## Scope reviewed

The complete candidate change is the `2b66812` commit on top of `8c6eb8b`. It contains:

- `web-mini/App.vue` and `web-mini/src/ApiDemo.vue` for the six-page API demonstration surface.
- `web-mini/src/api.ts`, `web-mini/src/response-schemas.ts`, and `web-mini/src/types.ts` for the HTTP and response boundary.
- `web-mini/src/pages/AlarmsPage.vue` and `web-mini/src/pages/RealtimePage.vue` for alarm and model/latest flows.
- `web-mini/tests/api.test.ts`, `web-mini/tests/pages.test.ts`, and the updated `web-mini/tests/domain.test.ts`.
- `web-mini/package.json`, the lockfile, Vite test configuration, and regenerated `web-mini/dist` assets.
- M4 evidence updates in `docs/evidence/M4/2026-10-08/README.md` and `docs/evidence/acceptance/R26/README.md`.

The pre-existing user changes in `.github/workflows/ci.yml` and `.tmp/` were not inspected as candidate changes and were not modified.

## Verification reproduced

All commands were run against the reviewed working tree after `npm ci --prefix web-mini`:

| Check | Result |
|---|---|
| `npm ci --prefix web-mini` | PASS; 128 packages installed, audit reported 0 vulnerabilities (npm emitted a deprecation warning for transitive `glob@10.5.0`) |
| `npm test --prefix web-mini` | PASS; 3 files, 39 tests |
| `npm run typecheck --prefix web-mini` | PASS |
| `npm run build --prefix web-mini` | PASS; Vite produced the checked-in `dist` assets |
| `npm audit --prefix web-mini --omit=optional` | PASS; 0 vulnerabilities |
| `python3 scripts/run_external_case.py --case R25-R27 --provider wechat --mode software` | PASS; `software=passed`, `integration=not_required`, `external=external_blocked` |
| `python3 scripts/check_architecture_manifests.py --all` | PASS; 55 requirements |
| `make verify-contracts` | PASS; 67 operations and 334 fixtures |
| `git diff --check HEAD` (excluding the user-owned `.github`/`.tmp` paths) | PASS |
| local Vite server and `curl http://127.0.0.1:5175/` | PASS; served the app entry and module entrypoint |

The three Vitest files include a real ephemeral Node HTTP server fixture (`web-mini/tests/api.test.ts:9-51`), HTTP status/error cases, malformed successful payload cases, 204 confirmation, and browser component interactions (`web-mini/tests/pages.test.ts:14-105`). No test is deletion-only, a prompt/prose pin, or a tautological assertion of an implementation constant.

## Findings

### CRITICAL

None.

### HIGH

None.

### MEDIUM

#### M1. Transport catches are intentionally broad and unannotated

- Location: `web-mini/src/api.ts:47-54`.
- Both `catch` blocks convert every thrown value into a user-facing transport/JSON error. This is safe for the expected `fetch` and `JSON.parse` boundary failures and is covered by tests, but it does not follow the repository's TypeScript guidance to narrow caught `unknown` values or rethrow unexpected programmer errors. A future adapter bug could be reported as a generic network failure.
- This is a maintainability observation for the lightweight demo, not a criterion-level defect. A production follow-up should narrow `SyntaxError` and known transport errors while preserving the safe message policy.

#### M2. Runtime schemas and exported interfaces are maintained separately

- Locations: `web-mini/src/response-schemas.ts:8-39` and `web-mini/src/types.ts:6-42`.
- Zod parsing is the correct boundary choice and prevents the previous `body as T` problem, but each payload shape is duplicated as a manually written interface. A future field change can update one side and leave the other side out of sync. The current typecheck and malformed-payload tests do not detect a semantic drift that still satisfies both duplicate definitions.
- For a production client, derive response types from the schemas (or add a contract generation check). The duplication is acceptable for this explicitly small demonstration and does not block approval.

#### M3. The demo's page dispatch uses an open-ended `if` chain

- Location: `web-mini/src/ApiDemo.vue:31-43`.
- The `page` union is handled as login, home/ponds, realtime, or a catch-all history branch. Adding a new non-alarm page without updating this branch would silently call history. A `switch` with an exhaustive assertion would make future page additions fail at typecheck time.
- The existing six-page contract is tested and the current behavior is correct; this is a bounded maintainability risk, not a blocker for the requested API demonstration.

### LOW

#### L1. Test fixture uses a type assertion for Node response headers

- Location: `web-mini/tests/api.test.ts:36` (`response.headers as Record<string, string>`).
- This is test-only and typecheck-clean, but it is an unverified assertion in a project that otherwise favors strict boundary types. A small conversion helper could preserve the same fixture behavior without the assertion.

#### L2. npm install reports a transitive deprecation warning

- `npm ci` reports `glob@10.5.0` as deprecated through the test utility dependency chain; `npm audit --omit=optional` reports zero vulnerabilities. This is dependency hygiene to revisit when the testing stack is next refreshed, not a current security finding.

#### L3. Browser QA screenshots are referenced but not stored as repository artifacts

- `docs/evidence/M4/2026-10-08/README.md` records the browser observations, while no screenshot/trace path is committed under the evidence directory. Executable component tests and the local Vite smoke check provide reproducible coverage; the missing screenshot artifact limits independent visual replay only.

## Skill perspective checks

I explicitly loaded and applied `omo:remove-ai-slops` and `omo:programming`, including the TypeScript data-modeling, error-handling, and type-pattern references, before judging this diff.

- The remove-ai-slops pass found no deletion-only tests, tests that merely pin the requested removal, prompt/prose tests, debug output, dead production path, oversized module, or speculative provider abstraction. The API tests assert observable HTTP and response behavior, and the page tests exercise rendered outcomes.
- The programming pass confirms that the new HTTP boundary uses Zod parsing, avoids `any` and suppression directives, adds the required JSON content type, applies a timeout, and maps 401/403/404 without exposing server internals. The medium observations above are the remaining deviations: broad catches, duplicated schema/interface declarations, and non-exhaustive page dispatch. The test fixture assertion is the low finding L1.

## External classification and residual risk

The software runner correctly reports `external_blocked`; no real AppID/Secret, HTTPS app domain, WeChat qualification, native subscription prompt, or test device was available. The browser simulation is not evidence of real WeChat behavior. `docs/IMPLEMENTED.md` remains pending for TODO 8 until the required independent gate review and bookkeeping are completed; that status is not a code-quality blocker in this review.

## Recommendation

`APPROVE` the `2b66812` candidate for the requested lightweight M4 API demonstration. Keep M1-M3 and L1-L2 in the maintenance backlog before treating this client as a production mini-program implementation. A separate independent gate review must still approve this exact snapshot before updating the milestone status or publishing a tag.
