# M4 gate review

recommendation: APPROVE
candidate: 2bf4205d613da8611c9b24f05e2340026b551144
review_date: 2026-10-08 Asia/Shanghai
reviewer: independent final gate reviewer

## originalIntent

Deliver the M4 mini-program software demonstration boundary: six navigable pages, typed API and session boundaries, polling/history semantics, alarm loading and confirmation, and a subscription authorization demonstration. The user explicitly allows a lightweight mini-program demonstration because a dedicated owner will complete the production mini-program. Real WeChat login, device execution, subscription prompts, HTTPS, credentials, and qualification must remain explicitly external-blocked.

## desiredOutcome

The candidate must provide an executable and typechecked software surface with positive and error behavior evidenced by tests and browser QA. R25-R27 may be software-passed while their real WeChat/device portions remain `external_blocked`. Completion still requires a second independent reviewer to approve this same snapshot before the project status is changed.

## userOutcomeReview

The candidate meets the requested lightweight demonstration outcome. The Vite surface renders the login page and all six required navigation entries. The alarm page visibly handles an unauthorized/no-backend state without fabricating data, exposes refresh and confirmation controls, and keeps the alarm center available after simulated agreed, denied, and cancelled subscription decisions. The API layer has typed latest/history/alarm/session payloads, status-specific errors, malformed JSON handling, and 204 handling. No real WeChat success is claimed.

## checkedArtifacts

- `docs/README.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`
- `docs/IMPLEMENTED.md` (status bookkeeping remains pending until the two-review gate is complete)
- `docs/architecture/acceptance-map.yaml` (R25-R27 software passed / external blocked records and evidence paths)
- `docs/evidence/M4/2026-10-08/README.md`
- `docs/evidence/acceptance/R25/README.md`
- `docs/evidence/acceptance/R26/README.md`
- `docs/evidence/acceptance/R27/README.md`
- `web-mini/App.vue`, `web-mini/src/pages.ts`, `web-mini/src/page-contract.ts`, `web-mini/src/app.ts`
- `web-mini/src/api.ts`, `web-mini/src/types.ts`, `web-mini/src/domain/{alarms,history,login,polling,session,subscription}.ts`
- `web-mini/src/pages/*.vue`, `web-mini/pages.json`, `web-mini/manifest.json`
- `web-mini/tests/domain.test.ts`
- `scripts/run_external_case.py`, `scripts/check_architecture_manifests.py`

## reproducedVerification

All commands below were run against the candidate checkout, sequentially where installation was involved:

```text
npm ci --prefix web-mini                         PASS
npm test --prefix web-mini                      PASS (1 file, 15 tests)
npm run typecheck --prefix web-mini             PASS
npm run build --prefix web-mini                 PASS (Vite production build)
python3 scripts/run_external_case.py --case R25-R27 --provider wechat --mode software
                                                   PASS; software=passed, external=external_blocked
python3 scripts/check_architecture_manifests.py --all
                                                   PASS (55 requirements)
make verify-contracts                            PASS (67 operations, 334 fixtures)
git diff <parent> <candidate> --check           PASS
```

The first parallel installation/read attempt raced `npm ci` and produced transient missing-module errors; the required commands were rerun sequentially after installation and passed. This is an execution note, not a candidate failure.

Manual browser QA against the local Vite surface (`http://127.0.0.1:5173/`) reproduced:

1. Initial login page with all six navigation buttons.
2. Navigation to home, ponds, realtime, history, and alarms, each showing its page heading.
3. Alarm page unauthorized/error and empty states without fabricated alarm rows.
4. Simulated subscription agreed, denied, and cancelled states, each retaining the alarm-center message.

## externalClassification

R25, R26, and R27 remain `external_blocked` for real `wx.login`/code2session, six-page WeChat-device execution, and the native subscription prompt. The repository has no AppID/Secret, HTTPS deployment, qualification, or test device evidence. The software runner and acceptance records correctly distinguish software pass from external block; no mock is treated as real WeChat acceptance.

## removeAiSlopsPass

I read and applied `omo:remove-ai-slops`. The changed production files contain no deletion-only or tautological tests, debug logging, dead test scaffolding, oversized module, or unnecessary abstraction that violates an M4 criterion. The tests exercise observable API/domain outcomes and error classes rather than merely asserting the new file text. The compact Vue templates are consistent with the explicitly approved demonstration scope.

## programmingPass

I read and applied `omo:programming`. Typecheck is green and no `any`, suppression directive, or untyped `latest` return remains in the candidate diff. The API boundary still uses a generic transport cast after JSON parsing and the subscription adapter maps unknown provider values to cancelled; these are maintainability notes for the future production mini-program, not failures of the stated lightweight M4 software boundary. No programming finding creates a criterion-level blocker here.

## blockers

[]

## exactEvidenceGaps

1. A second independent reviewer must approve this same candidate before TODO 8/M4 is marked complete.
2. Real WeChat credentials, HTTPS, qualification, and a device are still required for external R25-R27; they are intentionally not blockers to this software-only gate.
3. After the two approvals, update `docs/IMPLEMENTED.md` bookkeeping to replace the current TODO 8 pending entry and retain the R25-R27 external-blocked wording. This is release bookkeeping, not a code failure in this candidate.

