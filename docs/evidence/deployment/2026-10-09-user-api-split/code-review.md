# User API split quick code review

- reviewed snapshot: root `c38f881` -> `1606bb9` -> `2e7205d`; web submodule `953335e`
- scope: route split, authorization guards, frontend API prefixes, tests, and deployment evidence
- production edits by reviewer: none
- codeQualityStatus: WATCH
- recommendation: APPROVE

## Verification

I inspected the exact root diff and the web submodule commit. The root mux mounts `/user/v1/` separately, `UserRoutes` uses the ordinary-account guard, and `/user/v1/login` rejects `ADMIN` accounts before issuing a token (`internal/adminapi/server.go:142-240`, `internal/adminapi/auth.go:30-80`, `internal/adminapi/user_routes.go:8-15`). Tenant authorization and path permission handling remain on the shared handlers (`internal/adminapi/auth.go:174-212`). The SPA fallback and route registration cover `/user/v1` (`cmd/iolinkd/main.go:339-405`).

The web commit routes business calls through `userHttp` and keeps platform calls on `http` (`web/src/api/http.ts:3-30`, `web/src/api/admin.ts`, `web/src/api/platform.ts:5-39`), and the Vite proxy includes `/user/v1`. The focused Go tests cover user routes, platform-only absence, admin rejection, tenant-context denial, and API fallback (`internal/adminapi/server_test.go:312-365`, `cmd/iolinkd/main_test.go:37-45`). The deployment evidence records the focused Go race run, full Go suite, isolated Timescale checks, 58 web tests, web typecheck/build, and contract checks at `docs/evidence/deployment/2026-10-09-user-api-split/README.md`.

## Findings

### CRITICAL

None.

### HIGH

None.

### MEDIUM

1. `web/tests/platform-api.test.ts:6-11` does not provide a `localStorage` stub or set the platform flag before calling `getSession()`. In Vitest's default Node environment, `getSession()` can reject at the `localStorage.getItem` access before the malformed-session schema is exercised, so the rejection assertion can pass vacuously. The test suite also does not directly pin the `http`/`userHttp` selection for `getSession` or the broad set of migrated business calls. This is a test-confidence gap; the production route mapping was checked directly and the supplied 58-test/typecheck/build evidence passes.

### LOW

None.

## Skill perspective

The `remove-ai-slops` and `programming` skills were explicitly loaded. The diff has no deletion-only tests, debug output, broad exception handling, dead helper, needless production parsing, or untyped escape hatch introduced by this snapshot. The MEDIUM item above is the only tautological/under-tested path found. No CRITICAL or HIGH finding remains.

## Blockers

[]

