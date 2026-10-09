# User API split bounded gate review

- recommendation: APPROVE
- reviewed snapshot: `d39aded..2e7205d` (root HEAD `2e7205d`; web submodule `953335e`)
- scope: route separation, ordinary-user-only guards, frontend API prefixes/proxy, tests/evidence, and `docs/IMPLEMENTED.md`
- production code edited by reviewer: none

## originalIntent

Separate platform administration and ordinary business-user HTTP surfaces. Expose ordinary-user business resources on `/user/v1`, keep platform controls on `/admin/v1`, preserve the existing `/admin/v1` business paths for compatibility, and route the web client accordingly. Platform administrators must not use the ordinary-user login/business surface.

## desiredOutcome

A deployed request to `/user/v1` reaches the same business handlers with tenant authorization, platform-only endpoints are absent there, ordinary users can use the business console, platform controls remain on `/admin/v1`, unknown API paths return JSON 404, and the change is recorded with reproducible evidence.

## userOutcomeReview

APPROVED. The root mux mounts `/user/v1/` separately; `UserRoutes` mounts business handlers plus ordinary-user guards, while `Routes` retains the admin surface and compatibility business routes. The user route login rejects ADMIN accounts with 403, and authenticated platform admins are rejected by `userAccountRequired`. Frontend business calls use `userHttp` (`/user/v1`), platform calls use `http` (`/admin/v1`), registration uses `/user/v1/register`, and Vite proxies `/user/v1`. SPA fallback recognizes `/user/v1` as JSON API space.

The focused reviewer reproduction passed:
`go test -race -shuffle=on ./internal/adminapi ./cmd/iolinkd -count=1`
with both packages passing. The checked deployment evidence reports the same focused route/auth test, full Go unit suite, 58 web tests, web typecheck/build, 74-operation contract verification, and the unknown-API JSON 404 check as PASS.

## checked artifact paths

- `git diff --find-renames d39aded..HEAD`
- `internal/adminapi/server.go`
- `internal/adminapi/auth.go`
- `internal/adminapi/user_routes.go`
- `internal/adminapi/server_test.go`
- `cmd/iolinkd/main.go`
- `cmd/iolinkd/main_test.go`
- `web/src/api/http.ts`
- `web/src/api/admin.ts`
- `web/src/api/platform.ts`
- `web/vite.config.ts`
- `web/src/stores/auth.ts`
- `web/src/components/VersionLabel.vue`
- `docs/evidence/deployment/2026-10-09-user-api-split/README.md`
- `docs/IMPLEMENTED.md`
- `docs/FRONTEND-HANDOVER.md`
- `docs/README.md`
- `.github/workflows/ci.yml`
- `Dockerfile`

## criterion audit

| Criterion | Result | Evidence pointer |
|---|---|---|
| C1: platform and ordinary-user route separation | PASS | `internal/adminapi/server.go:143-240`; `cmd/iolinkd/main.go:341-343`; `internal/adminapi/server_test.go:315-335` |
| C2: ordinary-user-only login and business guards | PASS | `internal/adminapi/auth.go:30-34, 77-80`; `internal/adminapi/user_routes.go:8-15`; `internal/adminapi/server_test.go:337-362` |
| C3: frontend API prefixes and dev proxy | PASS | `web/src/api/http.ts:4-30`; `web/src/api/admin.ts`; `web/src/api/platform.ts:5-18`; `web/vite.config.ts:10-16`; deployment evidence README “Covered behavior” |
| C4: compatibility admin business paths and API fallback | PASS | `internal/adminapi/server.go:143-150, 169-240`; `cmd/iolinkd/main.go:398-406`; `cmd/iolinkd/main_test.go:37-45`; deployment evidence README “Checks” |
| C5: reproducible verification and implementation record | PASS | `docs/evidence/deployment/2026-10-09-user-api-split/README.md`; `docs/IMPLEMENTED.md:S6`; focused test reproduction above |

## remove-ai-slops and programming pass

I inspected the bounded diff directly. New helpers have two real callers (route mounting/client creation), tests assert externally observable HTTP behavior, and no test is deletion-only, tautological, implementation-mirroring, or derived from the output under test. I found no new broad exception handling, dead code, debug output, needless normalization, or boundary leak in the changed Go/TypeScript. Changed production modules remain below the 250 pure-LOC limit (the larger test and pre-existing main files were not newly split by this change). No new untyped public escape hatch or unchecked error path was introduced in the reviewed lines.

## blockers

[]

## exact evidence gaps

- The deployment evidence explicitly leaves external browser visual acceptance to the user; no browser screenshot/manual UI run is claimed. This is recorded as an external follow-up and is not a blocker for the bounded HTTP route/prefix criteria reviewed here.
- No ulw-loop plan exists for this task, so the required attempt-directory report location was unavailable; this report is written to the requested fallback path.
