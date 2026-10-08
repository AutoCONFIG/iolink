# Platform console independent gate review

recommendation: REJECT

Snapshot: current shared working tree atop root `febbfc5722b8aeba4bc76ddfec6a0cdf0f6b06fe`, web `b340caedd038d1c80504aeffec8de7927b2871ba`, inspected 2026-10-08 Asia/Shanghai. Includes tracked diff and all new untracked implementation/test files. Changed source manifest hash (sorted path + NUL + bytes + NUL, excludes `.tmp/`, `.omo/`, `docs/evidence/`): root `105782ecceba45ed24f64974f0739a149e60368f01690af269d62288f6bcacbe`; web `bd8da64ab3bc9a35d88bd7695634495df4f8c76c5d83ca70a9b1d19643ec7594`. Snapshot is uncommitted; no earlier gate approval applies to this delta. Reviewer changed no implementation source, tests or authoritative docs; only this mandated receipt is written.

ULW status in this thread previously returned ULW_LOOP_PLAN_MISSING; fallback evidence path used.

## originalIntent

Current supplied brief and updated authority documents require first startup to create only the platform administrator; business users self-register and wait for administrator-created organizations and manually assigned ordinary roles. Platform administrator gets platform aggregate status and identity/organization controls; business users get their own authorized dashboard. Browsers remain user-owned. Existing external API, MQTT, isolation, rollback, credential/log and stage review requirements remain.

## desiredOutcome

An empty install reaches setup, creates no business organization/account automatically, permits user registration with no grants, lets platform admin create an empty organization and assign a registered user, and shows the correct dashboard with no unauthorized tenant detail exposure. Real database and HTTP evidence must support authorization and transaction claims; skipped tests do not count as passed.

## blockers

- id: B1
  violatedCriterion: ADMIN-COLLECTION / R02.c (`docs/api/admin-openapi.yaml:5`: collections always arrays, empty results `[]`, never `null`); R04 manual organization setup outcome.
  observation: Immediately after creating a business organization and before assigning its first member, authenticated platform GET `/admin/v1/tenants/{id}/members` returns HTTP200 `null`. `ListTenantMembers` uses a nil output slice and handler serializes it directly. Frontend `getTenantMembers` invokes `data.map`, so the newly created organization's normal empty state becomes a member-loading exception.
  evidencePointer: `internal/core/tenant_members.go:26`; `internal/adminapi/tenant_admin.go:179`; `web/src/api/admin.ts:164`; `docs/api/admin-openapi.yaml:5`; independent reproduction below.
  remediation: Return a non-nil empty collection on the real endpoint and cover the newly-created organization before any membership exists; re-review the revised tree. Avoid hiding the protocol error solely by a frontend fallback.

## Independent reproduction

Real disposable PG16/Timescale `iolink-todo9-pg`; DSN read into child environment only, never printed. Temporary Go overlay outside repo inserts this assertion immediately after the existing real onboarding test's POST organization creation and before any membership write:

```go
if raw := string(call("GET", fmt.Sprintf("/tenants/%d/members", tenant.ID), admin, "", 200)); raw != "[]" {
    t.Fatalf("empty tenant members must be []; got %q", raw)
}
```

`GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated> go test -overlay <temporary overlay.json> -count=1 -v ./internal/core -run '^TestPlatformConsoleOnboardingAndIsolation$'`

```text
=== RUN TestPlatformConsoleOnboardingAndIsolation
    platform_console_test.go:123: empty tenant members must be []; got "null"
--- FAIL: TestPlatformConsoleOnboardingAndIsolation (0.76s)
FAIL git.hyhy.fun/rsplab/iolink/internal/core 0.774s
PROBE_EXIT 1
```

The endpoint is the production HTTP router/store and SQL adapter, not a fake. Existing test first assigns a member before reading this list, missing the required empty state. Overlay and temporary files removed afterward; original test/source untouched.

## userOutcomeReview

Account and business resource isolation has sound direct evidence in the inspected code: registration explicitly writes USER with no memberships; platform roles require root actor checks; business routes use tenant middleware; ordinary platform memberships are rejected; platform administrator cannot self-grant support. User sessions distinguish no-grant, platform and current tenant contexts; dashboard requests aggregate or tenant routes according to that server session. Audit/account and organization/member writes use transactions. However, the required organization creation flow includes a real empty organization whose member response fails the existing client and explicit collection contract, so current artifact does not fully meet the expected normal outcome.

## Independently reproduced verification

- `npm test --prefix web`: PASS, 8 files / 58 tests.
- `npm run typecheck --prefix web`: PASS.
- `make verify-contracts`: PASS, 74 operations / 366 synthetic fixtures. Static pass does not establish live collection responses.
- `python3 scripts/check_architecture_manifests.py --all`: PASS, 55 requirements.
- Real HTTP/SQL empty organization overlay: FAIL as above, not skipped.

## Checked executor evidence

Directory `docs/evidence/deployment/2026-10-08-platform-console/`: README.md, go-final.log, go-verify-final.log, focused.log, contracts-final.log, architecture.log, web-tests-final.log, web-build-final.log, docker-smoke.log, docker-build.log and earlier counterparts. Go full logs list success, focused log lists actual onboarding/CLI/setup/owner-boundary tests, frontend has 58 test/build success including vue-tsc, contract/static counts match direct reproduction. No SKIP presented as successful acceptance found.

Docker smoke file contains only a prose PASS summary and dedicated temporary DB note, no individual commands, HTTP/SQL captures, image identity or restart outputs. It does not independently reproduce every Docker claim. This is an evidence NOTE here; B1 is a directly proven functional criterion failure. No browser, external WeChat/hardware, public TLS or tenant-scoped license acceptance inferred.

## Direct programming / remove-ai-slops pass

Both skills were consulted earlier in this continuous review session; their Go/TS and overfit/slop criteria applied directly to all changed/new code and tests. No code changes made for cleanup.

- Registration and role/session parsing are at actual input boundaries; no speculative parser/normalizer layers or provider SDK entering domain. Shared existing authorization and transaction seams reused; extracted auth, tenant administration and member files represent coherent responsibilities.
- Real SQL onboarding tests include forbidden writes, failed-audit rollback, inactive tenant and isolation assertions. No deletion-only/removal-only tests or tautological output-derived expected values found. Small fake/spy API tests cover meaningful input/error mapping; existing/new literal JSON/prose render checks introduce some maintenance coupling but fail no success criterion independently.
- Dashboard SSR tests assert branch-specific headings while mounted data loading never runs; they demonstrate rendered branch choice only, not permission-safe network dispatch or actual dashboard values. Direct production branch inspection and real API tests complement them. Missing empty-member integration case caused concrete false confidence (B1).
- Existing untyped JWT casts, ignored upgrade error, broad UI catches and oversized handlers remain maintenance notes from existing patterns; no unrelated refactor gate inferred. New readonly/interface style preference issues remain notes.
- No independent current code-review report in this evidence directory was supplied during this review. Directory inspected; direct skill pass supplies requested criterion coverage, so missing report alone is not a blocker.

## Checked artifacts and exact gaps

Authority: AGENTS.md, docs/README.md, CONTRIBUTING.md, PLAN.md, PLAN-DETAILS.md, EXTENSIONS.md, ACCEPTANCE.md R04/R14/R36/R37/R40, DEPLOY.md, IMPLEMENTED.md, admin-openapi.yaml. Source: full root/web tracked diffs plus new auth.go, platform_users.go, registration.go, session.go, tenant_admin.go; core/platform_admin.go, registration.go, tenant_members.go, platform_console_test.go, users.go; domain/registration.go; platform/setup.go/setup_test.go; adminapi/server.go/handlers.go/server_test.go; platform auth/observability/persistence tenant authorization; core admin_store.go; scripts/install-offline.sh/check_contracts.py; web platform API, auth store/router/layout/dashboard/register/pending/tenant views, tests and API contract adapters.

No dedicated notepad or frozen final manifest supplied; hashes above bind the read working tree. Root and web are uncommitted, so remote publication claims for this delta not independently established. S5 table truthfully remains “正在双审”; IMPLEMENTED introductory “已完成软件核验” is premature against pending reviews and should be reconciled once review concludes. Existing S4 approval applies only its earlier snapshot. Browser visual QA remains expressly user-owned. Full backend and Docker were inspected from executor evidence, while narrow real DB failure and web/static gates reproduced independently. No successful count is accepted as proof of B1's untested empty state.
