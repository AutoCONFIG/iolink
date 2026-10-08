# M4 mini-program demo code review

## Verdict

- `codeQualityStatus`: `BLOCK`
- `recommendation`: `REQUEST_CHANGES`
- reviewed snapshot: `2bf4205d613da8611c9b24f05e2340026b551144`
- review date: 2026-10-08 (Asia/Shanghai)
- reviewer: independent code review agent

The candidate is close to the requested small browser demonstration, but its real login HTTP path is broken. The high-severity finding below must be fixed and retested before this snapshot can receive an independent approval.

## Goal and success criteria

The goal is the M4 software demonstration boundary: six page entries, a typed `/api/v1` client boundary, alarm loading/confirmation and subscription demonstration, and explicit classification of real WeChat/device checks as `external_blocked`. The project instructions require positive and error paths, contract checks, available integration evidence, and a review against the same frozen tree. The software demonstration is allowed to stay small; it must still work through its actual HTTP boundary.

## Scope reviewed

The complete candidate diff is reproducible with:

```text
git show --format=fuller --no-ext-diff 2bf4205d613da8611c9b24f05e2340026b551144 --
```

It changes 15 paths: `docs/evidence/M4/2026-10-08/README.md`, the three R25-R27 evidence READMEs, `web-mini/DESIGN.md`, the generated `web-mini/dist` assets and index, `web-mini/src/api.ts`, `web-mini/src/domain/alarms.ts`, `web-mini/src/pages/AlarmsPage.vue`, `web-mini/src/types.ts`, and `web-mini/tests/domain.test.ts`. Existing user changes in `.github/workflows/ci.yml` and `.tmp/` were not touched.

## Verification reproduced

All commands below were run against the reviewed tree; the only worktree changes after the run remain the pre-existing user changes.

| Check | Result |
|---|---|
| `npm test --prefix web-mini` | PASS, 1 file / 15 tests |
| `npm run typecheck --prefix web-mini` | PASS |
| `npm run build --prefix web-mini` | PASS, Vite production build |
| `python3 scripts/run_external_case.py --case R25-R27 --provider wechat --mode software` | PASS, `software=passed`, `external=external_blocked` |
| `python3 scripts/check_architecture_manifests.py --all` | PASS, 55 requirements |
| `make verify-contracts` | PASS, 67 operations / 334 fixtures |
| `git diff --check` | PASS |
| `test -f scripts/run_external_case.py` | PASS |

Evidence inspected: [candidate README](../README.md), [R25 evidence](../../../../acceptance/R25/README.md), [R26 evidence](../../../../acceptance/R26/README.md), [R27 evidence](../../../../acceptance/R27/README.md), [the older rebuild record](../../../rebuild/m4-mini-program.txt), and the current API contract at `docs/api/openapi.yaml:309-377,590-762`. No real WeChat credential, HTTPS app domain, qualification, or device is available; the external portion remains correctly `external_blocked`.

## Findings

### HIGH

#### H1. Login JSON is sent without `Content-Type: application/json`

- Location: `web-mini/src/api.ts:38-45` and `web-mini/src/api.ts:60-62`.
- `createFetchRequest` creates headers with only `Accept` and `Authorization`. `ApiClient.login` passes `JSON.stringify({ code })` as the POST body but never sets a JSON content type.
- The browser/Fetch contract gives a string body the default `text/plain;charset=UTF-8` content type. Reproduction with Node's `Request` API reports exactly `text/plain;charset=UTF-8` for the same `RequestInit`.
- The server route uses `c.ShouldBindJSON(&req)` at `internal/appapi/server.go:187-190`, so the real `/api/v1/auth/login` request is not bound as JSON and returns 400 before code2session. This breaks the observable login boundary required by R25.
- The current test at `web-mini/tests/domain.test.ts:42-47` injects a fake generic `RequestFn`; it never observes request headers and therefore stays green while the real transport is unusable.

Required before approval: set the JSON content type at the request boundary (while preserving caller headers), add a regression assertion for the outgoing login headers/body, and exercise the request against the server contract or an HTTP-level test that uses the real request builder.

### MEDIUM

#### M1. Successful JSON is cast to every requested type without parsing

- Location: `web-mini/src/api.ts:36-56`.
- `JSON.parse` only proves that the response is syntactically JSON; `return body as T` lets an object, array, or missing fields cross the untrusted HTTP boundary as `WaterLatest`, `AlarmSummary[]`, or `LoginExchange`.
- The new tests cover malformed JSON, but not a syntactically valid response with the wrong shape. This conflicts with the programming skill's parse-at-boundary rule and can make `AlarmsPage` fail during rendering if a successful response is malformed.
- This is not a reason to expand the demo into a large framework, but the boundary needs a narrow runtime shape check (or a documented typed decoder) for the response types it exposes.

#### M2. Error and empty states can be shown at the same time

- Location: `web-mini/src/pages/AlarmsPage.vue:56-59`.
- When `alarms()` rejects on the initial load, `error` is set and `alarms` remains empty. The independent error paragraph renders, then `v-else-if="!alarms.length"` also renders `当前没有报警`.
- A user who is offline or unauthorized is told both that loading failed and that there are no alarms, which is a misleading empty state. The evidence claims complete error/empty handling, so the component should make these states exclusive and add a component-level regression.

#### M3. The new component behavior is not locked by tests

- `web-mini/tests/domain.test.ts:50-64` covers the latest type, transport errors, 204, and the pure pending-count helper, but it never mounts `AlarmsPage` or drives loading, empty, error, confirmation, and the three subscription button outcomes.
- The M4 README records a browser observation, but no screenshot or browser trace artifact is stored under `docs/evidence/M4/2026-10-08/`; the report says it was recorded in a session. For the project's comprehensive-test requirement, the visible component paths need either executable component/browser evidence with a path or a narrower documented scope.

## remove-ai-slops and programming perspective

I explicitly consulted `omo:remove-ai-slops` and `omo:programming` before judging maintainability and test relevance. The changed tests are not deletion-only tests, prompt/prose pins, or tautologies; they assert transport errors, response values, and the pending alarm projection. The production diff does contain the untyped `body as T` escape hatch called out in M1. The one-line methods and compact Vue template are consistent with this small demo's existing style; no oversized-module or needless abstraction finding was added.

## Residual assumptions

- Real WeChat `wx.login`, code2session, subscription dialogs, HTTPS, qualification, and device rendering remain `external_blocked`; this review does not treat the adapters or browser buttons as real WeChat acceptance.
- `docs/IMPLEMENTED.md` intentionally still leaves TODO 8 pending until two independent reviewers approve a corrected final snapshot; that status is not counted as a finding here.

## Final recommendation

`REQUEST_CHANGES`. Fix H1, add the transport regression, and rerun the listed checks on the new frozen tree. Re-review M1-M3 during the next pass; no approval should be recorded for snapshot `2bf4205` while the real login request cannot satisfy the server's JSON boundary.
