# M4 final gate review — frozen API demonstration

recommendation: APPROVE
blockers: []
candidate: 2b66812fd0d6e64609c1bc3fccbccccf71a4f99d
candidateTree: 1bd83aabd5ea4ef617cbf5975dea270de67d92ca
review_date: 2026-10-08 Asia/Shanghai
reviewer: independent final gate agent; no participation in candidate edits
codeQualityStatus: WATCH

## originalIntent

The current review assignment explicitly limits the mini-program to an API demonstration: ApiDemo, response schemas, real local HTTP fixture, alarm page, 39 tests, contract/architecture/build checks, and R25-R27 classified as external_blocked. The expected user result is a runnable small software demonstration that can call the documented APIs and show success or failure, while a dedicated production mini-program implementation and real WeChat/device acceptance remain outside this software approval.

The authority map in docs/README.md, docs/PLAN.md M4, docs/ACCEPTANCE.md R25-R27 and docs/CONTRIBUTING.md was consulted. The full R25-R27 requirements remain in force for eventual external acceptance; this approval does not mean those requirements or the full M4 production phase are complete.

## desiredOutcome

For this frozen software candidate:

- M4.demo.pages: six navigable page entries with the minimal API demonstration entrance.
- M4.demo.http: login JSON headers/body, typed runtime response decoding, safe HTTP/JSON/network errors, and 204 confirmation through the actual request builder and a real local HTTP fixture.
- M4.demo.alarms: loading, list, empty and error presentation; successful confirmation and failed confirmation preserving state; agree/deny/cancel demonstration retaining access.
- M4.demo.verification: the supplied 39-test suite, typecheck, production build, architecture manifest checks and API contract checks pass against the frozen source.
- M4.demo.external-classification: R25-R27 real code2session, native subscription prompt and device acceptance remain external_blocked without fake success claims.
- M4.demo.review: two independent approvals must refer to the same source snapshot before software completion bookkeeping.

These identifiers describe the explicit current assignment, not new requirements imposed by this review.

## userOutcomeReview

APPROVE the limited API demonstration. The app mounts all six actual pages, provides login and resource request forms, masks the returned login token in output, presents history JSON containing unit/time/points, and exposes the real-time 30-second polling option. The mounted component test proves polling, background pause, foreground refresh and unmount cleanup. Alarm confirmation is only committed in local display after API success, and failure leaves it unconfirmed. The no-backend browser run showed 请求失败 without the successful empty-state message; simulated agreed, denied and cancelled results each left the alarm page and navigation accessible.

Runtime Zod decoding replaces the previous successful-JSON cast. The ephemeral local Node HTTP fixture verifies the production request builder's login content type/body, pond decoding, null latest, 204 confirmation and encoded device path. It is a real local HTTP exchange, not a real backend or WeChat acceptance test. Full device execution, old-field stale labels in a production UI, true code2session/user persistence, resource-transfer authorization and native subscription remain governed by R25-R27 and are not claimed here.

## checkedArtifactPaths

- docs/README.md; docs/CONTRIBUTING.md; docs/PLAN.md; docs/PLAN-DETAILS.md; docs/EXTENSIONS.md; docs/ACCEPTANCE.md; docs/IMPLEMENTED.md; docs/FRONTEND-HANDOVER.md.
- docs/api/openapi.yaml (login, latest, history, alarm and model response contracts); docs/architecture/acceptance-map.yaml (R25-R27 software/external status and evidence paths).
- docs/evidence/M4/2026-10-08/README.md; docs/evidence/acceptance/R25/README.md; docs/evidence/acceptance/R26/README.md; docs/evidence/acceptance/R27/README.md.
- docs/evidence/M4/2026-10-08/reviews/code-review-final.md: APPROVE on this exact commit. Its findings and skill coverage were rechecked directly; prose approval was not treated as proof.
- docs/evidence/M4/2026-10-08/reviews/code-review.md and gate-review.md: historical reports on 2bf4205; excluded as approvals for this snapshot.
- .omo/plans/modular-maintainability-refactor.md (Todo 8 context); .omo/boulder.json; .omo/ulw-execute/ledger.jsonl; .omo/evidence/todo8-independent-gate-review.md (historical context only).
- Complete M4 change span: git diff d9844c3 2b66812; final increment: git diff 8c6eb8b 2b66812; generated asset/stat scope examined.
- web-mini/App.vue; src/ApiDemo.vue; src/api.ts; src/response-schemas.ts; src/types.ts; src/pages.ts; src/page-contract.ts; src/app.ts; src/pages/*.vue; src/domain/{alarms,history,login,polling,session,subscription}.ts.
- web-mini/tests/api.test.ts; tests/pages.test.ts; tests/domain.test.ts; package.json; package-lock.json; vite.config.ts; tsconfig.json; DESIGN.md; dist/index.html and generated assets.
- scripts/run_external_case.py; scripts/check_architecture_manifests.py; scripts/check_contracts.py.
- Skills consulted: /home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.19/skills/remove-ai-slops/SKILL.md; programming/SKILL.md; programming/references/typescript/README.md; browser/SKILL.md.

No notepad path was supplied; available Boulder/ledger/plan context was inspected. No current ulw-loop goal exists: omo-agent-toolkit ulw-loop status --json returned ULW_LOOP_PLAN_MISSING. This report is therefore also saved at the mandated fallback .omo/evidence/m4-gate-review.md.

## reproducedVerification

Environment: Linux, existing locked Node/npm dependencies, Vitest 5.0.2, Vite 8.3.1, happy-dom; same shared HEAD confirmed before and after the build. Existing unrelated .github/workflows/ci.yml and .tmp/ were neither modified nor used as candidate evidence. The build left no web-mini worktree diff.

| Command | Actual result |
|---|---|
| npm test --prefix web-mini | PASS: 3 files; 39 passed (39); duration 3.09s |
| npm run typecheck --prefix web-mini | PASS: vue-tsc --noEmit, exit 0 |
| npm run build --prefix web-mini | PASS: 104 modules; dist/index.html, index-Uit-Fdm0.css, index-BVkCSKA8.js; build 358ms |
| python3 scripts/run_external_case.py --case R25-R27 --provider wechat --mode software | PASS: software=passed, integration=not_required, external=external_blocked, exit_code=0 |
| python3 scripts/check_architecture_manifests.py --all | PASS: 55 requirements, owners, providers, references and AGENTS.md scope |
| make verify-contracts | PASS: admin 44 + license 3 + open 5 + app 15 = 67 operations, 334 synthetic fixtures; live handler verification is R02.c |
| npm audit --prefix web-mini --omit=optional | PASS: found 0 vulnerabilities |
| git diff 8c6eb8b 2b66812 --check | FAIL: generated index-BVkCSKA8.js lines 11, 12, 20 contain trailing whitespace |
| git diff d9844c3 2b66812 --check -- web-mini/src web-mini/tests web-mini/App.vue web-mini/vite.config.ts web-mini/package.json docs/evidence | PASS: handwritten candidate scope |
| git diff --name-only HEAD -- web-mini | Empty; reproduced build matches frozen generated assets |

The full diff whitespace failure is a NOTE, since a whitespace-clean generated artifact is not a stated success criterion. It corrects the executor README's unqualified PASS claim and does not invalidate the reproduced functional gates.

## directManualQAMatrix

Vite served http://127.0.0.1:5186/ using npm run dev --prefix web-mini -- --host 127.0.0.1 --port 5186. The Codex in-app browser was driven directly; page observations were read after every action.

| Scenario | Observed outcome | Result |
|---|---|---|
| Initial login and empty code submit | Six navigation buttons; 微信登录凭据不能为空 | PASS |
| Home navigation | 首页 heading, Bearer Token input and 调用接口 | PASS |
| Pond navigation | 池塘 heading, Bearer Token input and 调用接口 | PASS |
| Realtime navigation | Device model detail form plus API form/device input and 每30秒刷新（后台暂停） checkbox | PASS |
| History navigation | Device input, five metrics, today/7d/30d selector and API button | PASS |
| Alarm navigation with no usable backend | 报警中心, refresh, 请求失败, no 当前没有报警 message, no fabricated rows | PASS |
| Simulate agree | 已同意订阅; alarm center/navigation still available | PASS |
| Simulate deny | 已拒绝订阅; alarm center/navigation still available | PASS |
| Simulate cancel | 已取消订阅; alarm center/navigation still available | PASS |
| Local HTTP/component positive/error paths | HTTP fixture plus mounted alarm login/history/polling tests passed | PASS (software fixture; external service not exercised) |

A 1280x720 screenshot was viewed in my browser session; no horizontal overflow was visible at that viewport. The executor subsequently supplied browser/login.jpg and browser/alarms.jpg; I opened both images and confirmed the login/form/navigation and alarm failure/cancelled demonstration surfaces agree with my direct run. browser/actions.json contains all ten actual page/action observations. This is not a claim of exhaustive responsive/visual acceptance.

## directRemoveAiSlopsPass

I directly read the production code, the final/full candidate diff and every test in the three files.

- No deletion-only tests or tests merely pinning a requested removal were found. JSON headers, status mapping, malformed payload rejection, pending count and rendered confirmation outcomes test observable behavior.
- No new tautological test deriving its expected value from the output under test was found. The fixture is independent of the schemas. No prose/prompt snapshot test was added.
- Some coverage claims are stronger than the tests: the existing page-contract test checks nonempty metadata strings, not rendered loading/offline states; the six-page test titled “exercises pond and alarm transport” only navigates and proves page mounting. These are false-confidence/maintenance NOTES, not missing criteria, because mounted alarm interaction, actual HTTP boundary and direct browser navigation evidence separately establish the assigned outcome.
- Existing controller lifecycle calls in domain.test.ts have no lifecycle assertions; actual polling behavior is separately asserted in pages.test.ts. The generic malformed-success test uses one gross invalid shape per endpoint and does not exhaust schema field combinations. Neither observation is treated as full external or contract proof.
- Production JSON parsing and Zod decoding are necessary untrusted-response boundaries. There is no unnecessary extraction/parser/normalization framework, speculative provider abstraction or new debug logging. decoded is reused across the API methods, not a single-caller extraction.
- Measured pure lines: api.ts 90; ApiDemo.vue 83; response-schemas.ts 38; AlarmsPage.vue 84; api.test.ts 72; pages.test.ts 102. No touched module exceeds 250 pure LOC.

## directProgrammingPass

Runtime values are parsed at the API boundary; MiniApiError safely exposes client messages/status; the response shape now matches the inspected history/latest/alarm contracts. JSON content type and a default 10-second AbortSignal are present. No new any, ts-ignore or ts-expect-error was found. Current program behavior and the requested build/typecheck pass.

Maintenance NOTES confirmed independently: broad transport/JSON catches without narrowing (api.ts); duplicated schema/interface definitions with mutable fields (types.ts/response-schemas.ts); non-exhaustive page dispatch (ApiDemo.vue); test-only header assertion (api.test.ts). tsconfig lacks the skill's additional strict flags, and the Node HTTP fixture ignores AbortSignal, so its success does not prove timeout behavior. These are bounded current maintenance risks; no stated demonstration criterion requires an additional timeout test, new HTTP library, configuration migration or architecture change, so they do not block.

## codeReviewSkillCoverage

code-review-final.md explicitly names both skills, rejects deletion-only/removal/prose/tautological test patterns, reviews boundary parsing and abstractions, and records broad catches, schema duplication, non-exhaustive dispatch and the header assertion. Its overfit/slop coverage is directionally correct but misses the weak pre-existing metadata/lifecycle assertions and overstates the six-page transport test. This direct pass supplies those observations; report coverage was not substituted for independent inspection.

## blockers

[]

No specific criterion failure was demonstrated in the authorized API demonstration scope.

## exactEvidenceGaps

1. No AppID/Secret, HTTPS deployment/domain, qualification, real code2session/user persistence, native subscribe prompt/receipt or device evidence. No real-device frontend-to-backend resource-transfer chain evidence. R25-R27 remain external_blocked; this review approves no external acceptance.
2. The initial executor README gave only screenshot session references. That gap is now closed by inspected browser/actions.json, browser/login.jpg and browser/alarms.jpg; direct browser behavior also agrees. Responsive/mobile/device visual acceptance remains outside this software demonstration.
3. No separate notepad input was supplied. No current ulw-loop plan/attempt directory exists; the required fallback report is used.
4. The README's unqualified git diff --check PASS cannot be reproduced for the commit's generated JS. See reproducedVerification for exact file/lines; it is a NOTE, not a functional criterion failure.
5. docs/IMPLEMENTED.md still contains the older pending TODO 8 and R25-R27 implementation wording. Reconcile software-demonstration status after combining this approval with code-review-final.md on the same commit; retain external_blocked and do not mark full production M4 complete.
6. Prior gate-review.md approves 2bf4205 and cannot count as an approval of this candidate. The independent final code review and this review both reference 2b66812; future source changes need a fresh snapshot review.

## Supplement — final executor artifacts inspected

The parent supplied these artifacts while this review was completing, without source edits. I read the command/results log, parsed the browser action JSON, inspected the backend run/result rows for failures/skips, read the referenced backend assertion code, and opened both saved screenshots:

- docs/evidence/M4/2026-10-08/logs/frontend-contracts.log: snapshot 2b66812; 39 tests, typecheck/build/audit, software runner, 55 architecture requirements and 67/334 contracts all exit 0. Its final git diff --check is a working-tree check and does not contradict this review's commit-diff whitespace NOTE.
- docs/evidence/M4/2026-10-08/logs/live-backend.log: 477 lines, explicit real_timescaledb=2.30.0 test observations, both core and appapi packages PASS, no SKIP rows. Revocation returns 401 with identical before/after state; role scopes return expected visible resource counts; latest/history/model responses and confirmation role matrix are shown. The assertion sites in internal/core/app_resource_scope_http_test.go and m2_integration_test.go and internal/appapi/server_test.go match the described checks.
- docs/evidence/M4/2026-10-08/browser/actions.json: ten observations for initial login, empty code, five resource pages and agree/deny/cancel. These agree with my independently driven browser sequence.
- docs/evidence/M4/2026-10-08/browser/login.jpg and alarms.jpg: opened and visually inspected; empty login form/six navigation entries and failed alarm load/cancelled demo are visible.

Exact backend command, supplied by its executor after inspecting the log (credentials intentionally omitted):

```text
GIN_MODE=release IOLINK_TEST_PG_DSN=<dedicated iolink-todo9-pg:55439; secret read in memory> go test -race -shuffle=on -count=1 -v ./internal/core ./internal/appapi -run 'Test(M2OwnershipLifecycleAndTokenRevocation|HistoryAndAlarmFiltersUseSnapshotOwnership|AppResourceHTTP|LoginAndListPonds|WaterLatest|AuthRequired|TenantRoleConfirmationMatrix)'
exit 0
```

The supplied backend receipt supports actual backend SQL/ownership/transfer/history/confirmation/revocation behavior. I inspected the source assertions and artifact evidence; I did not rerun this PostgreSQL command myself. My reproduced client checks remain the independent verification for this API demonstration. Neither backend receipt nor browser simulation is real WeChat/device acceptance. Recommendation remains APPROVE with blockers=[] on the same frozen candidate.
