# M6b final gate review — 0335438

recommendation: **APPROVE**

confidence: **HIGH** for R36.a/R37.a software scope and the corrected authorization defects.

reviewedSha: `03354381b5954087d82d0d3f807cad9993b94b8d`

reviewedTree: `96f613ba6fb128b7ecd6b9fb39eb0129083a6c9b`

webSha: `e84092326d9979e066c611e2be8eb6b9736e09f3`

blockers: **[]**

No candidate source, tracked artifact, status, or commit was changed by this reviewer. The report and reproduction logs are in the original workspace, outside `/tmp/iolink-m6b-release-final`. `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`; this uses the required `.omo/evidence/<goal>-gate-review.md` fallback.

## Original intent and desired outcome

originalIntent: Deliver the M6b tenant/RBAC foundation for the resources that already exist, preserve the external API/MQTT and business semantics, reject cross-tenant and unauthorized operations, invalidate revoked identities and permissions, and provide reproducible real database and UI evidence. The user requires positive and error paths and two independent approvals of the same final snapshot before main/tag/push.

desiredOutcome: Tenant owner/admin can use all of their tenant's existing business resources even without a farm assignment; member/viewer/support remain within authorized farm scope and action permissions; platform admins have no ordinary tenant business access; expiring support is explicit and audited. Previously authenticated contexts cannot commit mutations after the tested revocations. Failed operations leave no partial business state. Future R36.b/R37.b resources and external acceptance remain deferred.

userOutcomeReview: **PASS for R36.a/R37.a software scope.** The artifact matches that outcome. Its status is still pending final double review, so this single review does not certify R55 or complete the external/future scope. No specific stated success criterion failure was found.

## Criteria checked

| Criterion | Direct artifact/reproduction assessment |
|---|---|
| R36.a existing-resource isolation, tenant membership and fixture backfill | Tenant/farm filters and parent tenant checks retained. Real HTTP/database tests cover same-tenant assigned/unassigned resources, foreign tenant rejection, inactive tenant behavior, legacy reconciliation and migration failure rollback. |
| R37.a role matrix and platform/support boundary | Owner/admin positives without farm assignment, lower-role farm scope, viewer confirmation rejection, legacy platform-owner rejection, expiring/revoked support, audit behavior and token rejection passed in the independent focused run. |
| R37.a old-token/permission revocation and fake execution | Both middleware paths carry actor and permission version. Shared mutation authorizer, telemetry authorizer, and alarm confirmation predicates check current permission version. Fake executor uses the production `AuthorizeTenantWrite` seam and rejects committed role/version revocation before side effects. |
| Common negative gate and AGENTS security/integrity | Corrected telemetry returns 403; singular/batch alarm version revocation returns 404; incomplete scoped alarm contexts return `ErrForbidden`. Persisted telemetry/sensor/shadow/alarm/outbox snapshots stay identical. Farm/member revocation returns 403 with unchanged target state. Inspected multi-row writes, membership/audit and product/device/shadow/audit operations retain explicit transaction boundaries. |
| Verification/evidence requirement | The full `make verify` log and command/exit metadata are now bound to executable `ef6510f`; `git diff --quiet ef6510f..0335438 -- ':!docs'` exits 0. All referenced final logs exist and terminate with exit 0. Independent exact-SHA database reproduction exits 0 with zero skips. Contract and architecture checks were independently reproduced. |
| Scope and phase completion | `docs/IMPLEMENTED.md` and `docs/evidence/rebuild/gates/M6b.json` say verification passed but final double review pending. Future resource and applicable external acceptance are explicitly deferred/`external_blocked`, not marked passed. |

## Independent verification

Environment recorded at `.omo/evidence/m6b-final15-gate/identity.json`: Go 1.26.8 linux/amd64, Node v24.21.0, npm 11.19.0, dedicated `iolink-todo9-pg` image `sha256:75d58b53f3337a6babd3a1cce5e503f2300bdede62dd7d743651deb1a95b6d76`, test connection on local port 55439. Real PostgreSQL 16.15 / TimescaleDB 2.30.0 is recorded by the candidate receipt and verified through the fixture logs. DSN supplied only through process environment; not preserved in reproduction artifacts.

1. `GOFLAGS=-buildvcs=false GIN_MODE=release IOLINK_TEST_PG_DSN=<redacted> go test -race -shuffle=on -count=1 -v ./internal/core ./internal/migrate ./internal/adminapi ./internal/appapi -run 'Test(M6b|Reviewer|AppResource|AppTelemetry|AppAlarm|TelemetrySubmission|TelemetryHTTP|LegacyTenant|Platform|ProductionMux)'`
   - Exact `0335438` source header; exit 0; 29 top-level tests / 249 PASS markers; **zero SKIP and zero FAIL**. Counts are supplementary: test bodies and actual rejection/positive/state assertions were inspected.
   - `.omo/evidence/m6b-final15-gate/focused.log`.
   - Reproduced telemetry version rejection: 403, unchanged=true. Reproduced singular and batch alarm version rejection: 404, unchanged=true. All 20 partial-context open/already-confirmed operation cases: forbidden, unchanged=true. Farm/member role, membership, tenant and authority revocation: 403 and unchanged target.
2. `.venv/contracts/bin/python scripts/check_contracts.py`
   - Exit 0, 53 operations / 245 synthetic fixtures. `.omo/evidence/m6b-final15-gate/contracts.log`. Synthetic contract validation is not presented as live HTTP proof; live HTTP proof is above.
3. `.venv/contracts/bin/python scripts/check_architecture_manifests.py --all`
   - Exit 0, 55 requirements. `.omo/evidence/m6b-final15-gate/architecture.log`.
4. `gofmt -l <all branch-changed internal Go files>`
   - Exit 0, empty file list. `.omo/evidence/m6b-final15-gate/gofmt.log`.
5. `git diff --check c56cb22...0335438` passes, final candidate tracked status remains clean. No independent full-suite or fresh browser run is claimed by this review.

The committed full verification artifacts were read and checked: build/vet/full race+shuffle suite, docs tooling, contracts, architecture, focused real database and revocation runs, frontend units/typecheck/build, and 16 browser scenarios all contain command headers and exit 0. The verbose focused and revocation logs contain zero skipped tests. The quiet full-suite log alone cannot expose individual skips; the actual real-database focused runs and independent reproduction resolve the relevant database gate.

## Direct programming / remove-ai-slops pass

Consulted both skill files plus the Go overview and testing reference. Applied their perspectives directly to the branch diff, production authorization paths and changed tests; prior success reports were not adopted as proof.

- No deletion-only tests, tests pinning a requested removal, prose/text pins, tautological assertions, or implementation-mirroring tests were found. Real HTTP/database rejection tests compare business state, and new version/partial-context cases have red/green artifacts. Existing positive owner/admin cases prevent a blanket-denial implementation from satisfying the matrix.
- Corrected the former fake-executor false-confidence concern: its authorization callback now invokes production `persistence.AuthorizeTenantWrite`, not a separate test-local role implementation. It proves context propagation and pre-execution rejection; it makes no claim about future real jobs.
- No unnecessary production extraction, parsing or normalization, speculative plugin layer, broad catch, provider SDK dependency in domain/application, new unsafe logging, or untyped domain escape hatch was introduced. The shared SQL resource predicate has multiple callers and implements the required manager/farm distinction. Interpolated role values are restricted literals; actor/version values are typed integers.
- Boundary authorization and transaction checks are load-bearing security checks, not redundant interior validation. The already-confirmed alarm query provides the public idempotency outcome rather than redundant verification of a destructive action.
- Oversized existing files and permissive farm/member error assertions remain notes below. They do not prove a stated M6b success criterion failure.

Code-review coverage consultation: the inspected `.omo/evidence/m6b-final14-code-review.md` explicitly covers programming/remove-ai-slops, deletion-only/removal-only/tautological/implementation-mirroring tests, unnecessary production extraction/parsing/normalization, fake-executor relevance and oversized files. That report rejects the older c56cb22 snapshot and is not used as a current approval. The requested fresh `.omo/evidence/m6b-final-code-review.md` was not present when this report was finalized; my direct pass supplies the required perspective check. The owner must still collect its terminal exact-SHA approval to satisfy the separate two-review release condition.

## Notes and exact evidence gaps

1. **Maintenance note:** branch-touched production files remain oversized: `internal/adminapi/handlers.go` 1075 pure LOC, `internal/core/admin_store.go` 1042, `internal/core/repos.go` 652, `internal/persistence/products.go` 402; server files also exceed 250. This is inherited and increased maintenance burden under both skills. No stated R36.a/R37.a acceptance criterion imposes a LOC ceiling, so it is not an approval blocker.
2. **Test precision note:** `internal/core/tenant_write_revocation_http_test.go` still accepts any status >=400 rather than exactly 403 and logs a hardcoded historical `source=9120469`. Actual current results are 403; the reproduction log's top-level exact-SHA header establishes current identity. Tightening assertions/source labels would improve future regression signal but no current runtime criterion fails.
3. **Artifact-format note:** branch-wide diff-check findings in preserved historical raw D16 browser snippets/logs are not source-code whitespace and do not fail a named M6b criterion. The fresh correction diff is clean. The receipt's `git diff --check` is only a working-tree check, not a branch-wide claim.
4. **UI proof limit:** 16 E2E scenarios use the demo adapter; they establish frontend role/expiry persistence and UI behavior, not API/database integration. The final receipt says so. I inspected the actual browser test code, the source-bound raw E2E output, the retained D16 expiry JSON artifacts (30 files preserving exactly the whole/fractional original or edited UTC instant), and the edited/reloaded screenshot. Backend behavior is independently established by real HTTP/Timescale tests. No fresh manual browser execution is claimed here.
5. **Inputs absent:** no notepad path was supplied and none was found in this review checkout. The only supplied manual QA matrix, `.omo/evidence/m6b-final-764/m6b-final-764-manual-qa.md`, belongs to historical 764c5eb and is not treated as current proof. The final source-bound receipt/scenario output and independent audit support current completion; no requirement for a newly named matrix artifact is stated.
6. **Release evidence still pending:** the second independent terminal exact-SHA report was not yet available to this reviewer. This report is one independent approval, not permission to bypass the user's two-approval release gate. R36.b/R37.b, real hardware/WeChat/public TLS/capacity and full R55 acceptance are outside this approval and remain deferred or externally blocked.

## Checked artifact paths

All relative candidate paths below resolve beneath `/tmp/iolink-m6b-release-final` unless explicitly marked root workspace.

- `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/PLAN.md`, `docs/EXTENSIONS.md:17`, `docs/ACCEPTANCE.md:9`, `docs/ACCEPTANCE.md:19`, `docs/IMPLEMENTED.md`, `docs/evidence/rebuild/gates/M6b.json`.
- Branch production diff and affected mutation bodies: `internal/adminapi/handlers.go`, `internal/adminapi/products.go`, `internal/adminapi/server.go`, `internal/appapi/server.go`, `internal/appapi/telemetry_v2.go`, `internal/authorization/policy.go`, `internal/core/admin_store.go`, `internal/core/app_resource_scope.go`, `internal/core/telemetry_authorization.go`, `internal/core/tenant_write_auth.go`, `internal/core/repos.go`, `internal/core/service.go`, `internal/core/product_model.go`, `internal/core/users.go`, `internal/persistence/products.go`, `internal/persistence/tenant_authorization.go`, `internal/domain/tenant_context.go`, `internal/domain/platform_context.go`.
- Changed regression fixtures/cases: `internal/core/alarm_partial_context_test.go`, `alarm_revocation_http_test.go`, `alarm_batch_revocation_test.go`, `telemetry_version_revocation_http_test.go`, `telemetry_permission_fixture_test.go`, `telemetry_permission_test.go`, `telemetry_permission_http_test.go`, `product_revocation_http_test.go`, `tenant_write_revocation_http_test.go`, `fake_executor_m6b_test.go`, `app_resource_scope_test.go`, `app_resource_scope_http_test.go`, `app_resource_scope_write_http_test.go`, `m6b_tenant_test.go`, `m6b_http_test.go`; branch fixture diffs in `m2_integration_test.go`, `m6a_telemetry_integration_test.go`, and `internal/migrate/platform_owner_test.go`.
- `docs/evidence/acceptance/R37/2026-10-04-version-revocation/README.md`, `verification.json`, `red.log`, `red-alarm.log`, `green.log`, `fixture-failure.log`, `fixture-green.log`, `reviewer-probe-red.log`, `review-c56cb22-reject.md`; **all eleven referenced `logs/*.log` files**.
- Executed runner source `docs/evidence/acceptance/R37/2026-10-04-m6b-revocation/verify.mjs`; historical candidate receipt `docs/evidence/acceptance/R37/2026-10-04-m6b-auth-final/README.md`; R36/R37 receipt maps; D17 receipt `docs/evidence/acceptance/R37/2026-10-03-d17-app-scope/README.md`.
- `web/e2e/admin-pages.spec.ts`, `web/e2e/support-expiry.spec.ts`, `docs/evidence/rebuild/M6b-D16/README.md`, JSON and screenshot artifacts under its `support-browser-results/`, specifically `support-expiry-支持到期：Asia-Shanghai-上海时间显示、编辑、保存和重载保持同一瞬间/edited-reloaded.png`.
- Root workspace `.omo/evidence/m6b-final14-code-review.md`, `.omo/evidence/m6b-final-764/m6b-final-764-manual-qa.md`, `.omo/evidence/m6b-final15-gate/{identity.json,focused.log,contracts.log,architecture.log,gofmt.log}`.

**Final verdict: APPROVE exact 03354381b5954087d82d0d3f807cad9993b94b8d for R36.a/R37.a software scope.**
