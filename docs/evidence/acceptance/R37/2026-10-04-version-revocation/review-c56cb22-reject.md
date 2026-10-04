# M6b code review: c56cb22

**Verdict: REJECT.** `codeQualityStatus: BLOCK`; `recommendation: REQUEST_CHANGES`.

Confidence: HIGH for the two authorization defects, reproduced against real PostgreSQL/TimescaleDB through the actual HTTP routes or public core methods. This report does not approve a later fix.

## Review binding and inputs

- Goal: independently review M6b R36.a/R37.a live authorization, role and membership revocation, isolation, atomic mutation rejection, catalog operations, alarm confirmation, and evidence traceability.
- Reviewed source: `c56cb22b00712d1cd8b2624af60e7f06636fbf24` in `/tmp/iolink-m6b-release-final`.
- Parent: `e4608328e31915a7fb3f4d5c78cb2fcf8c0fdf9b`. The parent-to-candidate diff changes only `docs/evidence/acceptance/R37/2026-10-04-m6b-auth-final/README.md`; executable code is identical.
- Branch scope: `main...c56cb22`, merge base `5c34af407e884a1c6ef6bb80daed3e15980a21db`.
- Criteria consulted: AGENTS.md instructions, `docs/README.md`, `docs/PLAN.md`, `docs/EXTENSIONS.md:17`, `docs/ACCEPTANCE.md:19`, and `docs/CONTRIBUTING.md`.
- Inspected changed authorization production code and tests, the final receipt and its revocation log, R37 historical receipts, `docs/IMPLEMENTED.md`, the acceptance manifest and gate status.
- A notepad path was not supplied; no notepad was present under this review checkout's `.omo/notepads`. Earlier reviewer verdicts were not adopted as evidence.
- `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING` in both the review checkout and root workspace. This is the required fallback report path.
- The review started with a clean c56cb22 checkout. No production source was edited by this reviewer. The owner began adding regression tests after the failing probes were delivered; the later working-tree status is explicitly recorded in `m6b-final14-probes/source-identity.txt`, and is not presented as a clean candidate snapshot.

## Independent verification

Environment: approved dedicated `iolink-todo9-pg`, local test route on port 55439; real TimescaleDB 2.30.0. Each test creates and removes its own isolated database. Credentials were supplied only through subprocess environment and are not recorded.

1. Existing focused tests: `go test -race -shuffle=on -count=1 -v ./internal/core ./internal/migrate ./internal/appapi ./internal/adminapi -run 'Test(M6b|Reviewer|AppResource|AppTelemetry|AppAlarm|TelemetryWrite|LegacyTenant|Platform|ProductionMux)'`.
   - Exit 0, zero skips. Core, migration and admin focused tests passed; appapi reported no matching tests, with the application routes exercised by core HTTP integration tests.
   - Artifact: `.omo/evidence/m6b-final14-focused.log` in the root workspace.
2. Deterministic reviewer probes via Go overlays, without changing candidate source: `go test -overlay <root>/.omo/evidence/m6b-final14-probes/overlay.json -race -shuffle=on -count=1 -v ./internal/core -run TestFinal14`.
   - Exit 1, zero skips: all five rejection scenarios failed because writes succeeded.
   - Artifact: `.omo/evidence/m6b-final14-probes/results.log`; test sources and overlay are alongside it.
3. `gofmt -l` identifies `internal/core/product_revocation_http_test.go` as unformatted. This is a style observation, not the reason for rejection.

No full `make verify` rerun or browser run is claimed by this review. Existing focused tests passing does not cover the failed scenarios below.

## Findings by severity

### CRITICAL

None found.

### HIGH

**H1 — Telemetry and both alarm confirmation routes ignore permission-version revocation at their mutation boundary.**

References: `internal/core/telemetry_authorization.go:27`, `internal/core/app_resource_scope.go:35`, `internal/core/repos.go:631`, `internal/core/admin_store.go:1042`. The version is attached by `internal/appapi/server.go:346` and `internal/adminapi/server.go:325`; the shared general mutation authorizer compares it at `internal/persistence/tenant_authorization.go:53`.

The separate telemetry authorizer loads only the live role, and the alarm SQL predicates check membership/role without comparing `TenantPermissionVersion`. If membership permission_version is incremented after middleware authenticates the request but before the core mutation executes, the old authenticated context still authorizes these writes. A same-role membership update or tenant lifecycle invalidation can revoke the old permission version while leaving the role and active flags acceptable.

The probes commit `UPDATE tenant_memberships SET permission_version=permission_version+1 WHERE tenant_id=9401 AND user_id=9402` at the same post-authentication seam used by the existing revocation tests. Actual results:

| Operation | Observed response | Persisted business state |
|---|---|---|
| POST `/api/v2/devices/v2-permission/telemetry` | 202, accepted temperature | Changed |
| POST `/admin/v1/alarms/{id}/confirm` | 204 | Changed |
| POST `/admin/v1/alarms/batch-confirm` | 200, `{"confirmed":1}` | Changed |

Evidence: `m6b-final14-probes/results.log:11`, `:18`, and `:37`; full fixtures/injection code are in the saved overlay test sources. Existing catalog tests include permission-version invalidation, but the alarm and telemetry revocation matrices omit it.

Required before approval: enforce the claimed permission version, live tenant/membership/authority and action permission at the same transaction boundary as these writes; retain resource/farm checks and atomic rejection. Add the demonstrated same-role version revocation cases to real HTTP/database regressions.

**H2 — Partial M6b contexts bypass the actor-scoped alarm write path.**

References: `internal/core/admin_store.go:951` and `:991`; compare `internal/domain/tenant_context.go:46` and the fail-closed scope detection in `internal/persistence/tenant_authorization.go:12`.

`ConfirmAlarm` and `BatchConfirm` choose the authenticated path only when `TenantRole(ctx)` is nonempty. A context containing a tenant ID but no authenticated actor or role instead reaches the legacy write path. The new shared writer correctly distinguishes partial tenant context from an entirely unscoped internal call, but these alarm entrypoints do not.

Both public core methods successfully confirmed an alarm using only `domain.WithTenantID(context.Background(), 9401)`: nil error and changed alarm snapshot. Evidence: `m6b-final14-probes/results.log:26` and `:30`. This is a core boundary failure; this review does not claim the HTTP authentication middleware accepts an incomplete context.

Required before approval: whenever any M6b scope is present, require the complete authenticated actor/role context and use the authorized path. Keep genuinely unscoped bootstrap behavior distinct. Add regressions for both singular and batch public entrypoints.

**H3 — The final receipt's full verification PASS claims lack artifacts bound to its stated executable source.**

References: `docs/evidence/acceptance/R37/2026-10-04-m6b-auth-final/README.md:13` through its verification bullets and `:20`.

The c56cb22 receipt attributes `make verify`, docs-tools, contract and architecture PASS results to e460832 but references only `logs/revocation.log`. That log contains the focused test output and no command header, full-suite output, or source identity. The available earlier committed receipt/verification metadata are bound to `7ea4790`, before later executable authorization changes; the worktree copy in `.omo/evidence/m6b-final/verification.json` is also bound to that earlier source. Its old farm/member revocation output even records HTTP 500 responses, so it cannot certify the current mapping. The owner confirmed that no later full verification artifact is being used as c56cb22 proof.

Required before approval: preserve the actual commands, source/web identities, environment, exit codes, no-skip checks, and raw output for the fixed candidate; link each claimed final PASS to its artifact. The independent focused run above is valid evidence for its exact command, not proof of the receipt's full-suite claims. Misleading success output without the required artifact paths is a review blocker.

### MEDIUM

**M1 — The fake executor's revocation result exercises a test-local authorizer rather than the production authorization seam.**

Reference: `internal/core/fake_executor_m6b_test.go:44`.

The fake captures and passes context, but its `check` closure implements its own database role/tenant checks. Removing the production `AuthorizeTenantWrite` check or introducing the demonstrated version omission would leave this test green. Fake coverage is explicitly required for future executor context propagation; its revocation half should call the shared production authorization boundary so it can detect a regression there. No future real job support is claimed by this report. This is MEDIUM test relevance/false-confidence debt, not an additional demonstrated runtime failure.

**M2 — Changed files continue growing beyond both skills' 250 pure-LOC ceiling.**

References: `internal/adminapi/handlers.go:1`, `internal/core/admin_store.go:1`, `internal/core/repos.go:1`, `internal/persistence/products.go:1`.

Measured pure LOC, excluding blank and // comment lines: handlers 1007 → 1075; admin_store 930 → 1042; repos 648 → 652; products 378 → 402. These files were already oversized on main and have no size waiver. This is inherited maintainability debt that the diff increases; it is recorded as MEDIUM, without making an unrelated broad refactor a new approval blocker.

### LOW

**L1 — New catalog revocation test is not gofmt formatted.**

Reference: `internal/core/product_revocation_http_test.go:18`. `gofmt -l` reports the file. Normalize its formatting when repairing the required tests.

## Required skill-perspective check

Ran. Explicitly read/consulted:

- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.13/skills/remove-ai-slops/SKILL.md`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.13/skills/programming/SKILL.md`
- The programming Go overview and testing reference.

Applied the overfit/slop pass to both production changes and changed tests. No deletion-only tests, removal-only assertions, brittle prompt/prose pins, implementation-constant mirror tests, or newly introduced untyped domain escape hatches were found. No unnecessary production data extraction/parsing/normalization was introduced by the authorization diff. The SQL scope builder is required for the actual manager/farm behavior; its role variants are restricted literals and numeric actor interpolation cannot inject SQL.

The diff violates both perspectives through authorization logic split across inconsistent seams (H1) and oversized touched files (M2). The test-local executor authorization proof is weak under both test relevance perspectives (M1). Most added tests use real isolated database state, actual HTTP handlers, and before/after business-state assertions and are relevant to the goal. The general farm/member and catalog transaction changes are appropriate boundary checks rather than needless interior validation.

## Other audit observations

- General farm/pond/device/rule and catalog writes acquire live tenant, actor membership and authority locks in their transaction; role/permission mismatches return typed errors. Product assignment includes device/shadow/audit changes in one transaction.
- The inspected changed admin mutation handlers now map `ErrForbidden` to 403. Existing farm/member tests only require status >=400; their final log shows 403 but those assertions would still accept a future 500 regression.
- Existing focused tests cover manager access without farm assignment, foreign-tenant resource rejection, lower-role farm scope, singular/batch alarm rejection, and batch atomicity. Those checks passed independently on the candidate.
- `docs/IMPLEMENTED.md` and the M6b gate remain pending rather than declaring the phase completed. R36.b/R37.b future resources and applicable external acceptance remain deferred.
- Parent-to-candidate executable identity is sound, but changing the receipt's source SHA alone does not bind unrecorded full verification runs to it.

## Blockers

1. Fix mutation-time permission-version enforcement for telemetry and singular/batch alarm confirmation; demonstrate 4xx rejection with unchanged state in the reproduced HTTP cases.
2. Reject incomplete M6b contexts at singular/batch core alarm write entrypoints.
3. Replace unsupported final PASS claims with source-bound, linked raw verification artifacts for the fixed snapshot.

This report is a rejection of c56cb22 only. A new snapshot requires fresh verification and review.
