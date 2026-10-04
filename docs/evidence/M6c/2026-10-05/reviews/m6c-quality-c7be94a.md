# M6c independent quality gate — c7be94a

recommendation: APPROVE
confidence: high (0.90)
reviewed_source: c7be94a0d239b10571e01d44a6879bfc49e6da96
reviewed_web: 0aa7771acf0ef6322c9ac1e4c9839616646bb675
comparison_base: 069a1724ffab937b1e0215b623ea825d653805d0
worktree: /tmp/iolink-m6c-review-quality-c7be94a
blockers: []

## originalIntent

Deliver the M6c software chain for R38, R39.a and R40: original-byte signed License authorization, durable rollback-clock protection, atomic device admission/restore and quota enforcement, safe API/CLI errors and rejection audits, a shared optional-feature guard, and resumable offline x86_64 Docker delivery. Backend and admin web belong to one runtime image; web-mini remains separate. Real issuer, customer clean-host 30-minute installation, ARM64, WeChat/hardware and future R39.b entry points remain external_blocked.

## desiredOutcome

Users can inspect/import a signed License without an invalid attempt replacing the current certificate, retain existing basic monitoring when authorization expires, add/restore devices only under valid signed quota and current resource authorization, and initialize/resume a checksum-verified offline package with self-chosen accounts and a stable deployment ID.

## userOutcomeReview

The corrected source satisfies the stated M6c software outcome. There is no reproduced violation of a stated software success criterion. This recommendation applies only to the exact source/web SHAs above and does not declare external acceptance or the overall R55 milestone complete.

The previous rejected paths are closed directly in production source: RegisterDevice opens a transaction, locks license_state, then authorizes the tenant and checks admission (internal/core/admin_store.go:613-624). RestoreDevice similarly locks license_state before tenant authorization and the device row (:694-726). The default restore operational failure is now HTTP 500/internal_error (internal/adminapi/handlers.go:862). A temporary Go test overlay injected an unknown store error and reproduced that response without editing the locked worktree; /tmp/m6c-c7be94a-restore-probe.log records PASS.

## criterion review

- **R38 signed state and parsing:** internal/license verifies RSA-PSS SHA256/salt-32 over original payload bytes, then parses strict UTF-8 JSON. Required fields, duplicate/unknown keys, trailing objects, UTC time, null features, feature enumeration, decimal int64 bounds, RSA 2048..8192 and payload/envelope sizes have adversarial tests. 011 schema stores payload/signature/digest, with no independently mutable authorization-field cache. Runtime status/admission reverify the signed bytes/key ID/digest/deployment rather than trusting presentation fields. Invalid imports roll back; accepted state and import audit commit atomically. Raw rejection digest and audit failure propagation are tested.
- **R38 durable clock:** observation commits in a separate short transaction, latches rollback greater than five minutes, and survives business rollback. Admission/import recheck clock under the License transaction; clock errors after waiting are observed again after releasing the transaction. Reconciliation requires reaching the old high-water time and writes its audit. Direct real-DB tests passed for latch, lock-wait recheck and a one-connection pool.
- **R39.a quota/restore/features:** License-first row locking serializes increasing device operations and signed quota counting across tenants. Existing monitoring paths have no newly introduced License cutoff. Disable releases the active-device count; restore rechecks admission and clears last_seen/shadow while incrementing session_version. Valid/permanent/overage feature states allow only signed features; denied states prevent the fake future executor effects. Actual future optional entry points remain R39.b. Real Timescale tests passed for concurrent registration and restore/feature states.
- **R40 software delivery:** Dockerfile builds and embeds admin web, includes mqtt-sim, and excludes the signer. Installer validates checksums and traversal-safe manifest paths before Docker load, checks actual image IDs, uses pull_policy never and an internal network, and stops before serving without License. Retained stop/resume logs and status/audit files show the same deployment ID, one setup initialization, successful authorized resume and readiness. Retained followup records five simulator telemetry rows and a failed external probe. Metadata includes versions, image IDs, SPDX dependency inventory and notices with explicit legal-scan limits. Customer-host timing and ARM64 remain external_blocked.

## independently reproduced verification

- go test ./internal/license ./cmd/license-sign ./internal/adminapi -count=1: PASS. This initial run used the main checkout while its tracked source HEAD was c7be94a; only unrelated CI edits/build artifacts were present. The dedicated locked worktree was subsequently used for the source, database and overlay review.
- IOLINK_TEST_PG_DSN=<isolated Timescale DSN> go test -race -shuffle=on ./internal/core ./internal/platform ./internal/migrate -count=1 -v: PASS, retained at /tmp/m6c-quality-db-c7be94a.log. These integration tests were executed against the task-specific Timescale/PostgreSQL container; no skipped database result was counted as passing.
- Temporary read-only overlay: go test -overlay=/tmp/m6c-c7be94a-overlay.json ./internal/adminapi -run '^TestRestoreDeviceMapsLicenseErrors/internal-probe$' -count=1 -v: PASS, /tmp/m6c-c7be94a-restore-probe.log.
- python3 scripts/check_architecture_manifests.py --all: PASS; python3 -m pytest scripts/test_check_architecture_manifests.py -q: 30 PASS. The phase architecture gate remains intentionally pending until final dual-review registration.
- All 39 retained browser PNGs inspected via three contact sheets, one for each 375/768/1440 width. Permission/loading/errors/rejection/status/disabled/quota states are visible with no page-width overflow. Integrated m6c-live-ui.png inspected separately and shows permanent offline-fixture License and 1/3 usage.
- Existing isolated runtime independently inspected: actual image sha256:60f87d30159cca4a601a0dce7b4e20af170a0f4c44a20c91f2ad03507bc10b2a, revision 333fc427a4cd9c49715193176993c1e1827dedc6; internal network=true; mqtt-sim executable; signer absent; readiness returns ready.

## direct remove-ai-slops and programming pass

Consulted both SKILL.md files. Directly inspected production diff/tests, boundary parsing, errors, transactions, locking, logging and lifecycle code. No deletion-only tests, tests merely asserting a requested removal, tautological output-derived expectations or useless normalization were found that violate the stated goal. Boundary parsing/duplicate-key walking and exact raw digest handling are justified by the strict signed-input contract. Database failure injection, real transactions/concurrent admissions, signed fixtures and fake feature effects distinguish observable behavior; fake future execution is expressly allowed by R39.a.

Notes, not blockers:

- ec89218 adds a duplicate unknown-error case to TestRegisterDeviceMapsLicenseErrors instead of adding it to the restore table. The restore production behavior is correct and my independent overlay reproduces it; the retained restore regression coverage should be improved. Pointer: internal/adminapi/server_test.go:561-563 and :592-611.
- beginWriteTransaction is a pass-through wrapper and checkDeviceAdmission reacquires the already-held License lock. These add minor maintenance/query overhead, without changing the required lock order or safety. Pointer: internal/core/tenant_write_auth.go:28; internal/core/license_runtime.go:106.
- Touched established modules exceed the skill's 250 pure-LOC preference: cmd/iolinkd/main.go 366, internal/adminapi/handlers.go 1127, server.go 338, core/admin_store.go 1101; several existing test/checker files are also oversized. These are maintenance notes; no M6c success criterion mandates their split.
- Tablet License details wrap into a narrow column at 768px, but remain visible and usable. No criterion requires a different layout.
- Parser maximum-payload test refers to the production limit; independent signer tests use explicit 45KiB boundaries, reducing false confidence.

Prior /tmp/m6c-quality-af20fcc.md and /tmp/m6c-acceptance-af20fcc.md explicitly cover programming/remove-ai-slops, including deletion-only/tautological test checks. Their coverage was consulted only after directly checking artifacts and does not replace this pass. Their incomplete explicit mention of every overfit category is covered by this report's direct pass.

## checkedArtifactPaths

Paths below are relative to the locked worktree unless absolute:

- AGENTS.md; docs/README.md; docs/PLAN.md; docs/PLAN-DETAILS.md; docs/EXTENSIONS.md; docs/ACCEPTANCE.md; docs/IMPLEMENTED.md; docs/CONTRIBUTING.md; docs/design-m6c.md; docs/api/license-openapi.yaml; docs/api/admin-openapi.yaml; docs/DEPLOY.md.
- internal/license/{license.go,parsing.go,signature.go,raw_input.go,license_test.go}; internal/core/{admin_store.go,tenant_write_auth.go,license_runtime.go,license_clock.go,license_import.go,license_audit.go,m6c_license_test.go,m6c_guard_test.go,m6c_transaction_test.go}; internal/persistence/tenant_authorization.go; internal/adminapi/{handlers.go,license.go,server_test.go,m6c_errors_test.go}; internal/migrate/sql/011_license.sql; internal/platform/{setup.go,setup_test.go}; cmd/license-sign/{main.go,main_test.go}; cmd/iolinkd/license_cli_test.go; Dockerfile; scripts/{install-offline.sh,build-offline-bundle.sh,offline-smoke.sh}; deploy/docker-compose.offline.yaml.
- web/src/views/SystemView.vue; web/e2e/m6c.spec.ts; web submodule history/diff; .omo/evidence/m6c-license-delivery.md; .omo/evidence/m6c-manual-qa-final.md.
- docs/evidence/M6c/2026-10-05/README.md; m6c-integration-ec89218.log; m6c-focused-new-schema.log; m6c-verify-ec89218.log; m6c-browser.log; m6c-live-ui.log/png; m6c-offline-stop.log; m6c-offline-resume.log; m6c-offline-status-before.json; m6c-offline-status-after.json; m6c-offline-audit.txt; offline-followup.log; offline-tamper.log; offline-manifest.json; offline-SHA256SUMS; m6c-image-identity.txt; architecture-gate.log; all 39 browser/**/*.png.
- /tmp/m6c-quality-db-c7be94a.log; /tmp/m6c-c7be94a-restore-probe.log; /tmp/m6c-c7be94a-overlay.json; /tmp/m6c-c7be94a-server_test.go; /tmp/m6c-c7be94a-sheet-{375,768,1440}.png; /tmp/m6c-quality-af20fcc.md; /tmp/m6c-acceptance-af20fcc.md.

## exactEvidenceGaps

- Customer clean offline x86_64 30-minute installation, issuer-supplied production License/renewal, ARM64, WeChat and physical hardware are external_blocked; future real optional executors remain R39.b.
- Retained offline image/install/browser live evidence uses runtime 333fc427, not c7be94a's corrected runtime code. The final License-first source and safe restore error are reproduced through direct code review/real-DB tests/overlay, but this pass does not claim the retained image exercises those changes. Fresh final release-image evidence would improve provenance.
- The locked tree intentionally lacks generated internal/web/dist and web node_modules. A full cmd/iolinkd/image/web test rebuild was not reproduced there. Retained complete Go, image and web logs were consulted; the independently run core/platform/migrate tests do not need the generated assets.
- Older .omo manual QA files retain older SHA/count references; docs/evidence/M6c/2026-10-05/README.md and corrected ec89218 logs are the current evidence index.
- No ulw-loop plan exists: omo-agent-toolkit ulw-loop status --json returned ULW_LOOP_PLAN_MISSING. This report therefore uses the fallback evidence artifact path in addition to the requested /tmp delivery.
- Phase registration remains pending; this review alone is not two independent approvals and does not authorize declaring full external completion.
