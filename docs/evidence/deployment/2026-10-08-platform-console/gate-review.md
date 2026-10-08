# Platform console final independent gate review

recommendation: APPROVE
blockers: []

Reviewed current shared working tree, 2026-10-08 Asia/Shanghai. This supersedes prior REJECT only for the repaired working tree. No implementation source, test or authority doc was changed by reviewer. Mandatory receipt only.

## originalIntent

First initialization creates only platform administrator. Business users register themselves; administrator creates organizations and assigns ordinary roles manually. Platform admin gets aggregate platform status and account/organization controls; users get their authorized tenant/business dashboard. Browser appearance is user-owned; no browser acceptance claimed.

## desiredOutcome

Reliable empty setup and registration, manual organization and first-member assignment with valid empty states, server-enforced platform/tenant boundaries, no partial writes on failure, and automated/real database/Docker evidence appropriate to the authorized software scope.

## userOutcomeReview

APPROVE for current software delta. The original gate blocker ADMIN-COLLECTION/R02.c is CLOSED: `internal/core/tenant_members.go:26` now initializes a non-nil empty slice, and production HTTP returns `[]` for the new organization's member list before any grant. Frontend's existing `.map` consumes this expected shape. Minimal repair fixes the provider result instead of masking an inconsistent protocol response in the client.

The directly inspected full delta keeps registration authority fixed to USER with no organization grants, separates platform summary routes from tenant business routes, revalidates current token/membership/role at HTTP boundaries, rejects ordinary memberships for platform admins, and forbids platform self-grant of support. Creation, registration and membership writes have explicit transactions and audit rollback. Frontend resolves server session before selecting dashboard/navigation; unassigned users reach pending state. No discovered remaining failure is tied to a stated success criterion.

## Independent red/green evidence

Initial independent overlay against original production handler + real isolated Timescale failed: `empty tenant members must be []; got "null"`, exit1. The executor also added the actual test before production repair; inspected `docs/evidence/deployment/2026-10-08-platform-console/empty-members-before.log` confirms FAIL with `empty tenant members must be an array: null`.

Revised test `internal/core/platform_console_test.go:123` GETs the member endpoint directly after tenant creation and before any membership write, requiring HTTP200 `[]`. It then continues registration, assignment, tenant dashboard filtering, aggregate role checks, failed-audit rollback and inactive-tenant denial.

Independent run, DSN held in child environment and omitted:

`GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated iolink-todo9-pg> go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestPlatformConsoleOnboardingAndIsolation$'`

```text
-test.shuffle 1791474777565718162
=== RUN TestPlatformConsoleOnboardingAndIsolation
--- PASS: TestPlatformConsoleOnboardingAndIsolation (1.98s)
PASS
ok git.hyhy.fun/rsplab/iolink/internal/core 3.047s
```

No SKIP. Fixture creates/drops its own random DB on dedicated PG16/Timescale instance.

Independent checks immediately preceding this minimal repair: `npm test --prefix web` PASS (8 files/58 tests); `npm run typecheck --prefix web` PASS; `make verify-contracts` PASS (74 operations/366 fixtures); `python3 scripts/check_architecture_manifests.py --all` PASS (55 requirements). Web/contracts were not changed by the two-line DB/test repair. Subsequently the platform-admin member role dropdown was narrowed to owner/admin/member/viewer; tenant-admin support remains available, matching `SetTenantMember` server authorization. The final CLI positive assertion additionally verifies the specified operator ADMIN exists and its password authenticates; it has observable initialization value beyond checking absence of business data. Executor full-Go/backend/web-build logs were directly inspected; no skipped-as-passed claim accepted.

## Direct programming / remove-ai-slops pass

Applied the skills already consulted in this continuous review to complete original delta and narrow repair. New empty-state assertion is an observable HTTP contract test and independently failed before repair; it is not a deletion-only/removal-pin test, tautology or production-implementation mirror. Existing true SQL onboarding/rollback/authorization tests have behavior value. Parsing remains at HTTP response/input boundaries; no extra normalization/extraction layer or speculative abstraction was introduced. Transactions/providers/typed errors/logger patterns remain intact. Existing SSR heading assertions only prove branch rendering, not mounted network behavior; real API integration and inspected dispatch supplement them. Existing ignored password upgrade error, large inherited handlers, broad UI catches and literal JSON ordering remain maintenance NOTES, without stated user-criterion failure.

Current independent code-review report was not supplied/found under the platform-console evidence locations at latest receipt write time (the first-run report belongs to the previous setup scope). Evidence directory read and direct programming/slop pass supplied here; absence of a simultaneous reviewer receipt is not itself a role blocker. This gate receipt alone does not say the required two-review completion has already occurred.

## Checked artifact paths

Authority: AGENTS.md; docs/README.md; CONTRIBUTING.md; PLAN.md; PLAN-DETAILS.md; EXTENSIONS.md; ACCEPTANCE.md R04/R14/R36/R37/R40; DEPLOY.md; IMPLEMENTED.md; docs/api/admin-openapi.yaml.

Full tracked root/web diffs and all untracked new implementation/test files, notably adminapi auth.go/platform_users.go/registration.go/session.go/tenant_admin.go/server.go/handlers.go/server_test.go; core platform_admin.go/platform_console_test.go/registration.go/tenant_members.go/users.go/admin_store.go; domain registration.go; platform setup.go/setup_test.go; persistence tenant_authorization.go; scripts/install-offline.sh/check_contracts.py; web platform API, HTTP client, auth store, router, layout, Dashboard/Tenants/Register/Pending/System/Login views, console-session/platform-api tests.

Executor directory `docs/evidence/deployment/2026-10-08-platform-console/`: README.md; go-final.log; go-verify-final.log; focused.log; contracts-final.log; architecture.log; web-tests-final.log; web-build-final.log; docker-smoke.log; docker-build.log; earlier corresponding outputs; empty-members-before.log; empty-members-after.log; final-onboarding.log. Previous `.omo/evidence/platform-console-gate-review.md` retains original rejection/probe evidence.

## Exact evidence gaps / notes

- Re-inspected completed executor `empty-members-after.log`: core 89.040s, adminapi 10.555s, platform 1.707s, all PASS. `final-onboarding.log` records both `TestPlatformConsoleOnboardingAndIsolation` (1.67s) and `TestCLISetupCreatesOnlyPlatformAdmin` (0.65s), total3.385s, PASS with no SKIP. These are executor runs; the separately reproduced onboarding run remains above. Final `web-tests-final.log` records 8 files/58 tests PASS at23:55:03; `web-build-final.log` starts `vue-tsc -b && vite build` and ends successful Vite build (4.25s). No additional full-suite rerun by this reviewer was needed for the role option and six-line CLI assertion changes.
- Docker smoke log has only PASS summary/environment prose, without step-level commands/HTTP/SQL captures, restart output or image binding. It is supporting executor evidence, not independently reproduced Docker acceptance by this reviewer. Prior Docker build log was inspected. This note creates no newly named hardening/scenario gate.
- Commit identity rechecked 2026-10-09: root implementation/API/tests/docs contract committed locally at `2c8a6a91e7ab03f8c575011ecac0259ddcb60560`, web at `1327ff1cbb09b2b78aee1c9173b44081368d9d9c`. Every root manifest path except status-only `docs/IMPLEMENTED.md` equals the root commit blob byte-for-byte. IMPLEMENTED remains the separately reviewed pending status delta. The original full working-tree hashes are unchanged, so no implementation retest is required. Remote publication/CI not asserted for this delta.
- No dedicated notepad provided; README plus linked logs supply QA matrix. S5 table remains in review; introductory IMPLEMENTED completion prose should align after two explicit same-tree receipts.
- Browser, external WeChat/hardware, customer offline machine, public TLS and tenant-scoped License not accepted by this software review.

## Bound source identity

Hash algorithm: sorted changed/new source paths relative to each explicit diff_base, path + NUL + file bytes + NUL; excludes `.tmp/`, `.omo/`, `docs/evidence/`.

```json
{
  "root": {
    "head": "2c8a6a91e7ab03f8c575011ecac0259ddcb60560",
    "diff_base": "febbfc5722b8aeba4bc76ddfec6a0cdf0f6b06fe",
    "sha256_changed_sources": "a1450970e3bfc0001edf146323e7d4ff63878bfedf599a8d011bce079456c783",
    "paths": [
      "cmd/iolinkd/license_cli_test.go",
      "docs/ACCEPTANCE.md",
      "docs/DEPLOY.md",
      "docs/EXTENSIONS.md",
      "docs/IMPLEMENTED.md",
      "docs/PLAN-DETAILS.md",
      "docs/api/admin-openapi.yaml",
      "internal/adminapi/auth.go",
      "internal/adminapi/handlers.go",
      "internal/adminapi/platform_users.go",
      "internal/adminapi/registration.go",
      "internal/adminapi/server.go",
      "internal/adminapi/server_test.go",
      "internal/adminapi/session.go",
      "internal/adminapi/tenant_admin.go",
      "internal/core/m2_integration_test.go",
      "internal/core/platform_admin.go",
      "internal/core/platform_console_test.go",
      "internal/core/registration.go",
      "internal/core/tenant_members.go",
      "internal/core/users.go",
      "internal/domain/registration.go",
      "internal/platform/setup.go",
      "internal/platform/setup_test.go",
      "scripts/check_contracts.py",
      "scripts/install-offline.sh"
    ],
    "sha256_committed_manifest_excluding_status_doc": "870c88a81eb03254dacb1a02907d15e8d0c7a0cd791e15c2def310f848348bc8",
    "uncommitted_status_doc": "docs/IMPLEMENTED.md"
  },
  "web": {
    "head": "1327ff1cbb09b2b78aee1c9173b44081368d9d9c",
    "diff_base": "b340caedd038d1c80504aeffec8de7927b2871ba",
    "sha256_changed_sources": "dcbeb8e9c6967a07217a2d9077f993b675ea0d7e0af4f34de6d4ada7cf5afb43",
    "paths": [
      "DESIGN.md",
      "src/api/admin.ts",
      "src/api/http.ts",
      "src/api/platform.ts",
      "src/layouts/AdminLayout.vue",
      "src/router/index.ts",
      "src/stores/auth.ts",
      "src/types/api.ts",
      "src/views/DashboardView.vue",
      "src/views/LoginView.vue",
      "src/views/PendingView.vue",
      "src/views/RegisterView.vue",
      "src/views/SystemView.vue",
      "src/views/TenantsView.vue",
      "tests/console-session.test.ts",
      "tests/platform-api.test.ts"
    ]
  }
}
```
