# Independent code review: tenant permission UI

- Verdict: **APPROVE**
- codeQualityStatus: **CLEAR**
- recommendation: **APPROVE**
- blockers: **none**
- Reviewed Web SHA: `f62f66347f6bd7d7e5dbd3ec3a6e7a83fbf13350`
- Compared Web baseline: `7bd9523abff5494806d0971fecb6ecae6ec67cce`
- Frozen source worktree: `/tmp/iolink-permission-review-20261011`
- Backend context SHA: `0d2168d6b980ba0a3f67f418e2d4afa77d8ec7aa`
- Reviewer: independent permission_code_review agent; did not participate in edits.
- Review date: 2026-10-11, Asia/Shanghai.
- Source files were not changed. This report is the only reviewer artifact written.
- No notepad path was supplied; no `.omo/notepads` directory was present.
- `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`; this report uses the required fallback path.

## Goal and success criteria

Align tenant UI actions with existing server RBAC while preserving read paths: owner/admin manage farms, ponds, devices, rules and products; member/viewer/support read; owner/admin/member/support confirm alarms; viewer cannot confirm. Platform administrators remain restricted to platform operations. Include a guard against product-binding Enter requests. No backend, API or database changes.

Authority consulted: root `docs/README.md`, `docs/CONTRIBUTING.md`, relevant R23/R24/R37 clauses in `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md` and `docs/ACCEPTANCE.md`, plus the supplied task scope.

## Findings by severity

- **CRITICAL:** none.
- **HIGH:** none.
- **MEDIUM:** none.
- **LOW:** none requiring a change for this goal.

## Inspection and correctness

Inspected all 11 changed files and the entire baseline-to-candidate diff: DESIGN.md; e2e/tenant-permissions.spec.ts; both Playwright configs; src/domain/tenantPermissions.ts; AlarmsView, DevicesView, PondsView, ProductsView, RulesView; tests/domain.test.ts.

- `src/domain/tenantPermissions.ts:3` permits management only for owner/admin; `:7` additionally permits member/support alarm confirmation and denies empty, missing and unknown roles. This matches root `internal/authorization/policy.go:33` and the server request action mapping in `internal/adminapi/auth.go:196`.
- Farm/pond write guards precede validation, busy-state changes and API calls at `src/views/PondsView.vue:33` and `:48`. Their toolbar and dialogs are conditional, while list/filter/pond display behavior is retained.
- Device registration, restore, disable and move are guarded at `src/views/DevicesView.vue:77`, `:81`, `:90`, `:94`, `:99`. Details, telemetry, filtering, pagination and the product/model read link remain available.
- Rule creation is guarded at `src/views/RulesView.vue:30` and `:34`; reading is unchanged.
- Product create, model create, publish and assignment guard before writes at `src/views/ProductsView.vue:123`, `:129`, `:148`, `:167`, `:187`, `:203`. The binding input/select are disabled for read roles; the Enter handler still calls the guarded assign method (`:203`, template `:229`).
- Alarm confirmation uses a separate capability at `src/views/AlarmsView.vue:11` and guards the shared single/batch method at `:28`. Selection and both confirmation entry points are hidden for viewers, while alarm data/filter/read paths remain.
- Traced all callers of the affected API write exports in `src/api/admin.ts`; no sibling caller in the UI was omitted.
- Existing `src/stores/auth.ts` resolves the validated server session before protected routes mount. `src/router/index.ts:49` redirects platform administrators from tenant routes; server `internal/adminapi/user_routes.go:10` also rejects platform actors. The UI capabilities do not grant platform or backend authority.
- No API adapter, backend policy, database, dependency, telemetry payload or logging changes appear in the diff.

## Skill-perspective check

**Ran before judging tests and maintainability.** Loaded:
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/programming/SKILL.md`
- `programming/references/typescript/README.md`

No goal-relevant violation of either perspective found. The new unit assertions encode the independently inspected server role contract, rather than reading implementation constants. Browser assertions pair forbidden-action absence with retained reads, permitted management forms, real frontend HTTP requests, server rejection feedback and alarm confirmation outcomes. They are not deletion-only, tautological, prompt-prose or implementation-mirroring tests. The archived baseline failure demonstrates sensitivity to the original viewer defect.

The shared capability seam has multiple actual callers. No untyped escape hatch, speculative abstraction, production parsing/normalization, extra data extraction, broad catch or redundant validation was added. Event guards are needed in addition to presentation checks to cover alternate invocation paths.

Measured changed source/test files: helper 8 pure lines; AlarmsView 41; DevicesView 141; PondsView 76; ProductsView 217; RulesView 51; domain tests 103; permission E2E 118. ProductsView is within the 200–250 warning band and retains its existing layout/field parsing. Future growth warrants a responsibility split; this scoped fix does not require one.

## Independent verification

Ran in the frozen review worktree:

```text
$ npm test -- --run
> iolink-admin-ui@0.1.0 test
> vitest run tests --run


 RUN  v5.0.1 /tmp/iolink-permission-review-20261011


 Test Files  10 passed (10)
      Tests  67 passed (67)
   Start at  00:08:58
   Duration  1.44s (tests 68%, transform 19%, import 12%, worker 1%)
exit_code=0

$ ./node_modules/.bin/vue-tsc --noEmit --incremental false
(no diagnostics)
exit_code=0

$ git diff --check 7bd9523abff5494806d0971fecb6ecae6ec67cce f62f66347f6bd7d7e5dbd3ec3a6e7a83fbf13350
(no diagnostics)
exit_code=0

$ git diff --exit-code f62f66347f6bd7d7e5dbd3ec3a6e7a83fbf13350
(no tracked source changes)
exit_code=0
```

Final worktree HEAD was still the reviewed full SHA. The only untracked entry was the pre-existing node_modules symlink to the root Web dependency directory.

## Inspected executor artifacts

Under `/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/docs/evidence/tenant-permission-ui/2026-10-11/`:
- `README.md`: scope, frozen SHAs, environment, command/evidence mapping, limits.
- `baseline-regression.log`: old viewer farm-button assertion expected 0, received 1.
- `unit.log`: 10 files / 67 tests passed.
- `build.log`: vue-tsc and Vite production build completed.
- `permissions.log`: 10 role/error scenarios passed with the non-demo configuration.
- `admin-pages.log`: 5 existing demo scenarios passed.
- `backend.log`: adminapi and authorization tests passed with cache hits; no database result asserted.
- Opened `browser/viewer-ponds.png` and `browser/viewer-device-config.png`: visible reads and the readonly message, management controls absent. The other four role screenshot paths were inventoried.

Browser suites were not rerun during this code review, per the explicit port coordination constraint. Their source, fixtures, logs and representative artifacts were inspected; browser outcomes above are attributed to supplied evidence. The dedicated suite uses one worker, strictPort and reuseExistingServer=false, so it requires exclusive port 5188 and fails startup if occupied.

Lint/security scanners: N/A for this bounded review; no configured lint/security script is present in the Web manifest. User-owned visual acceptance remains pending. Real PostgreSQL/Timescale,现场 account reproduction and deployment were not run or claimed. These do not block this pure Web correction.

## Result

Approve this exact Web snapshot for the stated permission UI correction. No concrete blocker remains. This is one independent review receipt; the repository requirement for a second independent approval still applies.
