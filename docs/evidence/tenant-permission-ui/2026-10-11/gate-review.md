# Tenant permission UI independent gate review

recommendation: APPROVE
blockers: []
reviewer: permission_gate_review (independent; no implementation edits or child reviewers)
reviewedWebSHA: f62f66347f6bd7d7e5dbd3ec3a6e7a83fbf13350
webBaseSHA: 7bd9523abff5494806d0971fecb6ecae6ec67cce
backendSHA: 0d2168d6b980ba0a3f67f418e2d4afa77d8ec7aa
reviewWorktree: /tmp/iolink-permission-review-20261011
date: 2026-10-11 Asia/Shanghai

## originalIntent

A business user could click create farm and receive `tenant action forbidden`. The requested maintenance fix aligns the existing frontend with server authorization, preserves owner/admin operations and resource reading, preserves member/support alarm confirmation, and keeps platform administration separated. It does not grant new privileges or provision default tenants. Scope is this Web patch before v0.0.18 publication, not a fresh whole-stage R23/R24/R37 acceptance.

## desiredOutcome

Users see actions permitted for their current tenant role. Non-managers retain business resource lists, details and telemetry, see a read-only explanation, and cannot submit management requests through these controls. Owner/admin retain management controls and useful rejected-write feedback. Owner/admin/member/support can confirm alarms; viewer cannot. Server authentication, resource ownership and authorization remain the final boundary.

## userOutcomeReview

PASS for the scoped fix. I independently inspected the entire 11-file diff, each changed production seam, auth/session restoration, tenant routing, HTTP client call sites, backend policy and middleware. The shared helper denies management except for owner/admin and denies alarm confirmation except for owner/admin/member/support, including unknown/empty roles. Ponds guards farm/pond saves and hides both create dialogs; Devices guards register/move/disable/restore and keeps reading; Rules guards creation; Products guards create/model/publish/assignment and disables binding inputs; Alarms guards both confirmation paths and hides selection/actions for viewer. Permitted branches retain their original API calls and error handling. No backend, schema, account provisioning, API contract or dependency change exists in this diff.

| Criterion | Direct evidence | Result |
|---|---|---|
| R23 scoped business interaction | PondsView.vue:32, DevicesView.vue:76, RulesView.vue:29; permission E2E read-only cases across member/viewer/support, owner/admin form and server rejection cases | PASS |
| R24 frontend construction | Independent vue-tsc and VITE_APP_VERSION=v0.0.18 production build, exit 0; generated permission chunk and changed view bundles present | PASS for frontend build; no new deployment claim |
| R37.a role alignment | internal/authorization/policy.go:33-51; internal/adminapi/auth.go:172-212; tenantPermissions.ts:1-11; five-role browser alarm cases | PASS |
| Preserve resource reading and legitimate management | Browser resource reads/detail/telemetry and management entry cases; unchanged permitted API branches inspected; existing admin-pages evidence/test inspected | PASS |
| No authorization widening/default tenants; platform separation | Only Web files changed; unchanged router platform redirect and server tenant middleware inspected | PASS |

## Independent commands and actual results

All Web commands ran in the dedicated review worktree, whose HEAD was checked before and after execution. Source remained unchanged; node_modules is an existing untracked symlink. No real user credentials were used.

- `npm test -- --run`: exit 0; 10 files, 67 tests passed; 1.06 seconds.
- `npm run typecheck`: exit 0; vue-tsc passed.
- `npx playwright test --config playwright.permissions.config.ts`: exit 0; 10 tests passed in 16.7 seconds, Chrome, one worker, VITE_DEMO_MODE=false, HTTP fixtures. Port 5188 was checked free and reserved with the parent before the run. Actual UI rendering, input/click paths and request monitoring ran against the frozen source. Three read-only roles issued zero management writes; owner/admin farm rejection left the dialog available; four alarm-confirming roles sent one confirmation each; viewer controls were absent.
- `VITE_APP_VERSION=v0.0.18 npm run build`: exit 0; 1825 modules transformed; built in 4.93 seconds. Third-party Zod PURE annotation and existing 500 kB chunk warnings observed. No changed dependency/build optimization requirement was claimed.
- `go test -count=1 ./internal/authorization ./internal/adminapi`: exit 0; authorization 0.003 seconds, adminapi 4.723 seconds. These were uncached package tests, not real PostgreSQL integration.
- `git diff --check 7bd9523abff5494806d0971fecb6ecae6ec67cce HEAD`: exit 0.

Executor logs were independently read, not accepted from their summary alone. The baseline log fails at the precise viewer create-farm control assertion (expected 0, received 1), rather than an environmental failure. Existing demo admin-pages log records five passing scenarios; its source was inspected and it was not represented as live server/database validation. All six supplied member/viewer/support ponds and device-config images were opened and inspected; they show retained data and read-only controls consistent with the tests. They do not establish full visual acceptance.

## Direct programming and remove-ai-slops pass

Loaded `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/programming/SKILL.md`, its TypeScript reference README, and `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md`. Applied their review criteria directly without cleanup edits.

- Deletion/existence ladder: permission predicates are needed and reused across five views, rather than a speculative single-use abstraction. Existing auth/session and API seams are reused. No new parser, normalization layer, provider, fallback privilege or redundant extraction exists.
- Tests: no deletion-only source checks, tests merely pinning a requested removal, tautologies, implementation-generated expected values, prompt/prose snapshots or source-string assertions were introduced. Absence assertions here observe denied user controls together with retained reads and zero HTTP writes, which is the requested authorization behavior. The two unit cases derive expected role sets from the independently inspected policy. Wire fixtures isolate the frontend but do not purport to validate backend authorization; backend policy/middleware and uncached tests independently support that matrix. The viewer baseline failure demonstrates a real regression detector.
- Production quality: no new any, type assertion escape, ignore, non-null assertion, new broad catch, logging, thrown literal, mutable export, side effect in permission predicates, missing awaited HTTP write, new dependency or layer violation. Nullable/string role inputs are the existing session boundary contract and deny by exact membership; there is no role normalization that could accidentally widen access. Added negative checks are authorization guard clauses. Repeated handler checks complement hidden controls and protect alternate submission paths.
- All ten smell categories considered: obvious comments, redundant defense, complexity, needless abstraction, boundary violations, dead code, duplication, performance, behavioral coverage and module size. No criterion-violating finding. Pure LOC: tenantPermissions 8; Ponds 76; Devices 141; Rules 51; Alarms 41; Products 217; new E2E 118; domain tests 103; new config 15. Products remains in the warning band; this narrowly scoped authorization edit does not justify an unrelated extraction. Test narratives group related resource navigation, and the fixtures use a fixed test port with strictPort/reuseExistingServer=false; parallel runs must remain coordinated. These are notes, not failure of the stated outcome.
- Typecheck and relevant runtime checks passed. No separate Web lint/static security scanner script is configured in package.json; these gates are N/A rather than silently claimed passed. No exhaustive tagged variant was added to production; the wire fixture routing uses path conditionals, not domain variant parsing.
- After forming my independent recommendation, inspected only the code-review report's skill coverage section at `docs/evidence/tenant-permission-ui/2026-10-11/code-review.md:46-55`. It explicitly records both loaded skills, TypeScript references, independent server role expectations, deletion-only/tautological/prompt-prose/implementation-mirroring test coverage, no speculative extraction/parsing/normalization, handler guards, and LOC warnings. This confirms the required corresponding perspective check without consuming that reviewer's verdict.

## Checked artifact paths

Root repository `/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink`:

- docs/README.md; docs/CONTRIBUTING.md; relevant rows in docs/PLAN.md, docs/EXTENSIONS.md:17-34 and docs/ACCEPTANCE.md:24,57-58,75-76.
- internal/authorization/policy.go; internal/authorization/policy_test.go; internal/adminapi/auth.go; internal/adminapi/server.go; internal/adminapi/server_test.go:621-673.
- docs/evidence/tenant-permission-ui/2026-10-11/README.md (brief, QA matrix, environment and self-review/notepad section).
- docs/evidence/tenant-permission-ui/2026-10-11/code-review.md:46-55 (skill coverage section only, inspected after independent recommendation).
- docs/evidence/tenant-permission-ui/2026-10-11/baseline-regression.log; unit.log; build.log; permissions.log; admin-pages.log; backend.log.
- docs/evidence/tenant-permission-ui/2026-10-11/browser/member-ponds.png; member-device-config.png; viewer-ponds.png; viewer-device-config.png; support-ponds.png; support-device-config.png.

Frozen Web worktree `/tmp/iolink-permission-review-20261011`:

- All changed files: DESIGN.md; src/domain/tenantPermissions.ts; src/views/PondsView.vue; DevicesView.vue; RulesView.vue; ProductsView.vue; AlarmsView.vue; tests/domain.test.ts; e2e/tenant-permissions.spec.ts; playwright.config.ts; playwright.permissions.config.ts.
- Additional flow context: src/stores/auth.ts; src/router/index.ts; src/api/admin.ts; src/api/platform.ts; e2e/admin-pages.spec.ts; package.json; tsconfig.json.
- Independent browser run artifacts: test-results/tenant-permissions/ (six freshly generated role screenshots and Playwright last-run metadata).

## Exact evidence gaps and limitations

- The code-review report was initially absent; before finalizing it became available and its skill coverage section was inspected as recorded above. No missing report-coverage gap remains. No other reviewer's verdict was consumed.
- No separate manual QA matrix/notepad was supplied; the README contains the scoped matrix and self-review/notepad content, as confirmed by the parent. Browser behavior was independently rerun. User-owned visual acceptance remains outside this gate.
- Live PostgreSQL/Timescale, real backend HTTP-to-database browser integration, field account roles, deployment, image publication and automatic upgrade were not verified or claimed. HTTP fixtures prove frontend behavior only. This UI maintenance patch does not newly accept the whole R37.a backend stage.
- Owner/admin device restore/move/disable and bulk alarm HTTP success were preserved by code inspection and policy/helper verification; this fresh E2E fixture does not run every such live mutation. No stated criterion failure was found.
- This is one independent approval. The separate second independent approval for the same source SHA is still required before the parent records release/phase completion.
- `omo-agent-toolkit ulw-loop status --json` returned ULW_LOOP_PLAN_MISSING. The mandated fallback report location is therefore `.omo/evidence/tenant-permission-ui-gate-review.md`.

Final recommendation: APPROVE for the exact Web source SHA above. No blockers with a violatedCriterion and proving evidencePointer were found.
