# Independent M6c final acceptance review

recommendation: APPROVE
confidence: high for the scoped software source; bounded for retained Docker delivery evidence
blockers: []
reviewedSource: c7be94a0d239b10571e01d44a6879bfc49e6da96
reviewedWeb: 0aa7771acf0ef6322c9ac1e4c9839616646bb675
reviewedWorktree: /tmp/iolink-m6c-review-acceptance-c7be94a
reviewDate: 2026-10-05 (Asia/Shanghai)
reviewer: independent acceptance lane, no implementation edits or subagents

This approval is R38/R39.a and R40 **software** acceptance of the assigned snapshot. It is not approval of complete external R40 delivery, R39.b, or M0–M8 completion. A second independent approval of the same final snapshot is still required before recording the stage complete. A later test-only correction is outside this verdict.

## Original intent and desired outcome

originalIntent: Complete M6c after M6b: offline signed License import/status, safe instance/device quota and feature decisions, recoverable first setup, and one offline application image containing the backend and admin web UI.

desiredOutcome: An operator supplies their own accounts and a deployment-bound License, sees accurate authorization and quota status, resumes a previously incomplete installation on its original data volume, can retain existing basic monitoring when authorization expires, and receives a checksum-verified amd64 package with version/dependency metadata. Invalid uploads, missing verification capability, quota excess, unauthorized actions and clock rollback fail closed without replacing valid authorization or leaving partial business writes.

userOutcomeReview: The corrected source and retained artifacts support these scoped outcomes. The embedded real UI shows permanent License `offline-fixture`, the documented deployment ID and usage 1/3. The offline installation stops before serving when no License is supplied, then resumes on the same deployment/volume with one initialization, one tenant creation and one License import. The retained running image remains healthy and has five persisted simulator telemetry rows. The UI screenshot matrix covers status, permissions, loading, retry, rejection, session expiry and quota lifecycle at 375/768/1440 widths. External issuer/customer-host delivery remains explicitly blocked.

## Criteria and direct review

| Criterion | Independent conclusion and evidence |
|---|---|
| R38 / EXTENSIONS signed state table | Inspected `internal/license/{license,parsing,signature}.go` and `internal/core/license_{runtime,import,clock,audit}.go`. RSA-PSS/SHA256 verification uses raw payload bytes and 32-byte salt before payload parsing. Signed fields, configured key ID, deployment binding, validity and persisted clock latch drive decisions. Missing/invalid/unavailable cannot admit devices or optional features. Successful import and audit commit together; rejected import retains the old signed certificate and audits only a digest/reason. Current real DB tests and signer/parser race tests passed. |
| R38 private-key/runtime boundary | `cmd/license-sign` alone loads a protected private PEM and signs original bytes. Dockerfile copies only `iolinkd` and `mqtt-sim`. Tracked production source has no private PEM literal. Read-only running-container checks confirm simulator executable and `/app/license-sign` absent. Public-key config requires paired key file/key ID; invalid RSA bounds fail startup. |
| R39.a quota and lifecycle | Register and restore acquire `license_state FOR UPDATE` before tenant authorization/resource locks; quota counts all `disabled_at IS NULL` devices in the same write transaction. Restore checks the disabled device, license and quota, resets session/shadow state and commits atomically; repeated enabled restore is a no-op. Real DB concurrent registration permits exactly two of four when limit is two, with two quota rejections. Current restore/shadow, clock latch, one-connection and feature guard tests passed. Disabled devices are excluded from quota; basic ingestion/query/alarm paths do not gain a License gate that would cut off existing monitoring. |
| R39.a signed feature guard | Real DB fixture matrix exercises valid, permanent, overage, expired, not_before, clock_error, missing, invalid signature, instance mismatch, verification unavailable and monitoring-only through the shared production guard for video/openapi/automation/reports. Valid/permanent/overage allow signed features; all other cases produce zero fake future effects. R39.b actual future HTTP/MQTT/worker entry points and real gateway/subdevice behavior remain deferred to their specified phases. |
| Common N / safe restore errors | Inspected production auth, actor/version reload, transaction boundaries and exact mappings. Corrected `internal/adminapi/handlers.go:862` returns HTTP 500 `{error:internal_error}` for unexpected restore failures; diagnostics are not returned. Existing HTTP tests cover exact 204/404/403/409/503 mappings. Current adminapi race/shuffle suite passed. The newly added internal-error test is duplicated in the register table rather than covering restore; this is a coverage NOTE, not evidence that the inspected restore mapping violates the contract. |
| R40 software stop/resume and integrated image | Read stop/resume/status/audit logs, Docker build log, image identity, manifest and real embedded UI evidence. Independently queried the retained container: health `ok`, readiness `ready`, same deployment `923f4a64-2baf-438a-9317-cf401f019989`, sensor_data count 5, and setup.initialized/setup.tenant_created/license.imported each 1. Runtime image ID matches the retained manifest. Internal network/pull-never evidence is explicit and does not claim a clean offline Docker daemon/host. |
| R40 checksum and inventory | Installer checks SHA256SUMS and path restrictions before docker load/Compose; tampered VERSION log shows checksum refusal without subsequent load/start. Inspected install/build scripts, manifest, SHA256SUMS, SPDX, dependency notices and web lockfile. Package inventories enumerate Go/npm and identify exact images; unavailable notice sources and unscanned OS/database packages are disclosed rather than certified. |
| AGENTS architecture/provider/diagnostics | Pure license package uses the standard library. PG transactions are added at the existing core persistence seam; no provider SDK is introduced into pure License rules and no plugin/service abstraction is added. Inspecting error/audit paths found no raw License, key, password, JWT or telemetry payload in new diagnostics. Architecture manifests independently passed for 55 requirements and architecture tests passed 30/30. |

## Independent commands and observed results

Commands executed in the locked worktree against its exact HEAD; source and submodule were clean before and after. The dedicated PG fixture was TimescaleDB 2.30.2/PostgreSQL 16.15 at 127.0.0.1:33778, with a test-only CREATEDB account; credentials are omitted here.

1. `IOLINK_TEST_PG_DSN=<isolated> TMPDIR=/tmp/m6c-go-tmp go test -race -shuffle=on ./internal/migrate ./internal/core ./internal/notifications ./internal/persistence ./internal/platform ./internal/wechat -count=1`: exit 0, all six packages passed (core 107.191s).
2. `go test -race -shuffle=on ./internal/adminapi ./internal/license ./cmd/license-sign -count=1`: exit 0, all three packages passed.
3. `IOLINK_TEST_PG_DSN=<isolated> TMPDIR=/tmp/m6c-go-tmp go test -v -race -shuffle=on ./internal/core ./internal/platform -run 'Test(RegisterDevice_|RestoreDevice_|License|DeviceAdmission_|ReconcileLicenseClock_|Setup)' -count=1`: exit 0. Ten focused core tests, all eleven signed feature/state subcases, and platform password policy tests passed; **zero skips** in this verbose M6c run.
4. `python3 scripts/check_architecture_manifests.py --all`: exit 0, 55 requirements/owners/providers/references and AGENTS scope passed.
5. `python3 -m pytest scripts/test_check_architecture_manifests.py -q`: exit 0, 30 passed.
6. Existing root contract virtualenv Python invoked on this worktree's `scripts/check_contracts.py`: exit 0, 57 operations / 284 synthetic request-response fixtures. This is static contract evidence, not a claim that every live route was driven.
7. `git diff --check 069a172..HEAD -- cmd internal deploy scripts docs/api docs/design-m6c.md docs/m6c-schema.sql`: exit 0.
8. Read-only running-container binary/health/readiness and SQL queries: runtime image ID/source label, same deployment ID, five telemetry rows and one audit per setup/import step reproduced the retained delivery outcome.

An initial broad command including `cmd/iolinkd` failed at compile setup because this locked clean checkout lacks generated `internal/web/dist` (and a first TMPDIR argument referred to a nonexistent directory). Those attempts are **not passed checks**. The package was not independently rerun after creating/building generated files because the assignment prohibits worktree edits. The Makefile requires the frontend embed prerequisite; retained `m6c-verify-ec89218.log`, `m6c-integration-ec89218.log` and Docker build evidence include the command package. The successful independent six-package DB run and focused M6c run cover the corrected production seams. No skipped integration is counted as passing.

## Browser and artifact inspection

All 39 PNGs under `docs/evidence/M6c/2026-10-05/browser/` were opened as a labeled contact sheet (`/tmp/m6c-browser-contact.jpg`): six License states, forbidden, loading, load error, pending/rejected upload, disabled device and quota rejection, each at 375/768/1440. The image matrix matches its eleven-scenario Playwright source/log; no page overflow/cropped controls requiring a criterion blocker was visible. `m6c-live-ui.png` was opened separately at readable resolution and compared with the retained real UI log and database/container identity. Production-preview wire fixtures are correctly described as fixtures; the real screenshot is the separately identified integrated backend+web image.

Checked authoritative paths: `AGENTS.md`, `docs/{README,PLAN,EXTENSIONS,ACCEPTANCE,IMPLEMENTED,CONTRIBUTING,DEPLOY,design-m6c}.md`, `docs/api/license-openapi.yaml`, `docs/architecture/{acceptance-map,provider-matrix,invariants}.yaml`.

Checked production/test diff from M6b baseline `069a172` through reviewed HEAD, particularly `internal/license/*`, `internal/core/license_*.go`, `internal/core/admin_store.go`, `internal/core/tenant_write_auth.go`, `internal/core/m6c_*_test.go`, `internal/adminapi/{license,handlers,server,m6c_errors_test,server_test}.go`, `internal/platform/{config,setup}.go`, `cmd/{iolinkd,license-sign,offline-sbom}`, `internal/migrate/sql/011_license.sql`, `Dockerfile`, `.dockerignore`, offline Compose/build/install/smoke/uninstall scripts, and web License adapter/schema/SystemView/DevicesView/e2e/test changes.

Checked executor/manual QA/notepad paths: `.omo/evidence/m6c-license-delivery.md`, `.omo/evidence/m6c-manual-qa-final.md`, `docs/evidence/m6c-license-delivery.md`, `docs/evidence/acceptance/R{38,39,40}/README.md`. Older .omo prose is historical and does not substitute for the final source-bound evidence.

Checked final evidence directory `docs/evidence/M6c/2026-10-05/`: README, every retained Go/contracts/architecture/web/browser log, Docker build/image identity, stop/resume/status/audit/container records, live UI log/PNG, offline followup/tamper logs, manifest, SHA256SUMS, SPDX, dependency notices, web lockfile and all 39 browser PNGs.

## Programming and remove-ai-slops direct pass

Consulted `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.13/skills/{programming,remove-ai-slops}/SKILL.md` and Go/TypeScript programming references. Performed the required direct diff/test/production pass rather than accepting executor success prose.

- Excessive/useless tests: the second identical register `internal_error` row is redundant (NOTE). No deletion-only test or test merely certifying a requested removal found.
- Tautological/implementation-mirroring tests: parser size/signature tests use independent signed input or explicit boundary values; DB tests observe committed rows/errors; wire UI tests drive production UI through HTTP fixtures. Future-effect counter is appropriate to the explicitly scoped R39.a fake executor, not proof of R39.b real effects.
- Unnecessary extraction/parsing/normalization: `beginWriteTransaction` is a forwarding-only extraction (NOTE). Strict signed grammar parsing is demanded by the M6c contract; it preserves raw bytes for signatures and does not normalize credentials into authorization. Revalidation of persisted signed state is necessary because the database is not a separate authorization cache.
- Programming/security: typed sentinels and stable HTTP errors are used at the introduced boundaries; transaction/rollback error handling, clock latching, actor/version checks, key separation and fail-closed guards were inspected. No criterion-linked maintenance/security regression found. Existing manual SQL/style does not justify rejecting an expressly scoped adapter-seam change.
- Current separate code-review artifact is unavailable in this locked snapshot. The evidence directory was inspected before deciding; this independent direct programming/slop pass provides coverage, so report absence is a NOTE under the assigned gate rules rather than an invented blocker.

## Exact evidence gaps / limitations (not software blockers)

1. Actual issuer-issued/renewed customer License and clean disconnected customer x86_64 host within 30 minutes: `external_blocked` under R38/R40. No claim of completed external delivery.
2. ARM64, real WeChat, physical devices, actual gateway/subdevices and R39.b future HTTP/MQTT/worker entry points: not passed; deferred/external scope is accurately retained.
3. Retained offline image is source `333fc427a4cd9c49715193176993c1e1827dedc6`, image `sha256:60f87d30159cca4a601a0dce7b4e20af170a0f4c44a20c91f2ad03507bc10b2a`. It predates the License-first/error-mapping correction. Its installation/packaging smoke cannot certify those corrected runtime branches; current source tests and direct inspection establish them. Rebuild/rebind a final distributable before describing this older image as the corrected release.
4. Final command-package checks were inspected in retained logs, not independently rerun in the locked checkout because generated embedded assets are absent. No false passing claim for the attempted direct command.
5. M6c architecture gate currently rejects `software_status=not_run` pending two reviews. `--all` and its tests pass; documentation correctly leaves phase candidate status pending. Recording completion must update evidence/status only after required independent same-snapshot approvals.
6. SPDX/notice inventory does not scan every OS/database package and has disclosed notice-source gaps; no complete legal compliance finding is made.
7. No ulw-loop plan exists (`ULW_LOOP_PLAN_MISSING`). The assigned report path is `/tmp/m6c-acceptance-c7be94a.md`; no production or evidence file in the locked worktree was edited.

Final recommendation: APPROVE for the assigned c7be94a M6c software acceptance scope. No blocker can be tied to a proved failed success criterion in that scope.
