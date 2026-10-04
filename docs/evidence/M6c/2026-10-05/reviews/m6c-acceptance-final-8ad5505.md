# Independent final M6c acceptance gate

recommendation: APPROVE
confidence: high (0.91) for the scoped software source; bounded for retained image provenance
blockers: []
reviewedSource: 8ad550552d56f061222cd1ff53226000003c703e
reviewedWeb: 0aa7771acf0ef6322c9ac1e4c9839616646bb675
reviewedWorktree: /tmp/iolink-m6c-review-acceptance-8ad5505
reviewDate: 2026-10-05 (Asia/Shanghai)
reviewer: fresh independent acceptance leaf; no implementation edits or subagents

This is approval of R38/R39.a and R40 software scope at the exact source/web SHAs above. It preserves actual issuer/customer installation, ARM64, WeChat/hardware and future R39.b gaps. It does not certify complete external R40 delivery or R55/M0–M8 completion.

## originalIntent

Complete the M6c software chain after M6b: offline original-byte signed License import/status, persistent clock protection, atomic device admission and restoration with instance quota, a shared feature guard, resumable local setup, and a checksum-verified amd64 package containing one backend plus admin-web runtime. Preserve authentication/resource authorization and safe diagnostics.

## desiredOutcome

An operator establishes self-chosen platform and tenant credentials, obtains a stable deployment ID, imports an instance-bound License, sees authorization and quota status, and resumes a stopped first installation on its existing volume. Invalid input must retain existing authorization. Unavailable capability, bad signature, clock error, quota excess and unauthorized actions fail closed. Existing basic monitoring continues under expired authorization; new/restored devices and optional features obey signed authorization.

## userOutcomeReview

The exact final source satisfies the assigned software outcome. The moved regression now exercises the production restore HTTP route with an unexpected store error and observes HTTP 500/internal_error. Its injected diagnostic text is absent from the response. This directly closes the prior coverage note rather than merely removing a duplicate assertion.

The independently inspected retained offline runtime is healthy, is isolated by an internal Docker network, uses the documented image ID/source label, contains mqtt-sim and excludes license-sign. The database has the original deployment ID, five simulator telemetry rows and one audit each for setup.initialized, setup.tenant_created and license.imported. Stop/resume logs show first installation refusing to serve without License and later continuing on the same volume. Real integrated UI evidence shows permanent offline-fixture License, matching deployment ID and usage 1/3. The 39-image wire-fixture matrix shows authorization states, permissions, loading, retry, upload rejection and quota lifecycle across 375/768/1440 widths.

The retained runtime image predates the corrected License-first locking and safe restore error. Those specific branches are established by corrected production source, source-equivalent ec89218 full verification/real-database logs, independent c7be94a database evidence, and this exact final restore regression. The older image is not presented as exercising corrected branches or as a rebuilt final release.

## snapshot and retained-evidence equivalence

- Locked HEAD is exactly 8ad550552d56f061222cd1ff53226000003c703e; source and web were clean before and after review.
- `git diff c7be94a HEAD` contains exactly two files: internal/adminapi/server_test.go moves one internal-error row from the register table into the restore table; docs/evidence/M6c/2026-10-05/adminapi-restore-regression.log adds a full-package PASS receipt. No production file or preexisting evidence/PNG differs.
- `git diff ec89218 HEAD --name-status` contains the corrected test plus evidence/index additions, no production changes. Thus the retained ec89218 full verification and real-DB results apply to identical final production code.
- Compared 333fc42 with final source: affected runtime files are internal/core/admin_store.go (License lock before tenant authorization for register/restore), internal/core/tenant_write_auth.go (transaction forwarding helper), and internal/adminapi/handlers.go (restore internal_error). Offline CLI/setup/signature, Docker/Compose, installer/build scripts, migration and UI production code are unchanged by those corrections. Architecture checker/test edits are not runtime delivery changes.
- Web 0f3869c0c813bcb906b05eb52c9983e4b148db4f versus 0aa7771acf0ef6322c9ac1e4c9839616646bb675 differs only by Playwright default testIgnore for m6c.spec.ts; product source is identical.
- Evidence pointers: /tmp/m6c-acceptance-final-8ad5505-equivalence.log and /tmp/m6c-acceptance-final-8ad5505-runtime-delta.patch; prior full acceptance /tmp/m6c-acceptance-c7be94a.md and code review /tmp/m6c-quality-c7be94a.md were read, then their cited current artifacts were inspected.

## criterion review

| Criterion | Direct conclusion and artifact support |
|---|---|
| R38 signed states and invalid-import retention | License signature verifies original payload bytes with RSA-PSS/SHA256 salt 32 before strict payload parsing. Required fields, duplicate/unknown keys, time/feature/int64 grammar and input/key-size boundaries reject. Runtime re-verifies persisted payload/signature/digest/key ID and binding. Import/audit is atomic; invalid rejection rolls back and audits digest/reason. Inspected internal/license/*, internal/core/license_{runtime,import,audit}.go and 011 schema, signer/parser tests, current full verification and real-DB records. |
| R38 clock and private-key boundary | Independent short clock transaction preserves high-water/latch across business failures; lock-wait checks repeat under the write transaction. Reconciliation refuses before old high-water and records audit. Runtime Dockerfile excludes signer; existing container independently confirmed absent. Source/key parsing and tests plus /tmp/m6c-quality-db-c7be94a.log support the clock/admission/error paths. |
| R39.a quota/restore | Register/restore lock license_state before tenant/resource locks; count and business write share the transaction. Disabled devices are excluded; restored device clears last_seen/shadow and advances session version. Inspection of the corrected source and real-DB tests supports concurrent quota, restoration and rollback boundaries. Final HTTP mapping regression passed independently. |
| R39.a feature/base behavior | Shared guard consumes reverified signed state; all four features and eleven state classes have real-DB fixture/fake-future-effect coverage. Existing basic ingestion/query/alarm paths gained no authorization cutoff. Fake effects are within the explicit R39.a scope and do not establish R39.b actual future executors. |
| R40 software first setup and offline delivery | Same-volume stop/resume and one setup/import observed from logs and running DB; health/readiness and image/signer isolation reproduced. Dockerfile builds/embeds web into the single iolinkd runtime. Installer validates SHA256SUMS and traversal-safe paths before image load, compares loaded image IDs and uses pull-never/internal-network Compose. VERSION tamper evidence fails before load/start. |
| R40 metadata/UI | Manifest identifies architecture/images/source; SHA256SUMS covers package artifacts; SPDX/notice inventory states its legal-scan limits. Browser source/log/PNG matrix covers required software UI flows; real UI evidence is separately identified. Narrow tablet details wrap heavily, a maintenance/usability NOTE with no demonstrated failed scoped criterion. |
| AGENTS authority/architecture/security/evidence | Read docs/README, PLAN/attachments, ACCEPTANCE, CONTRIBUTING, IMPLEMENTED/design/contracts. Pure License rules have standard-library dependencies; provider transactions remain in the documented existing adapter seam. No added provider plugin/service abstraction or claim of future provider support found. New diagnostics use constant/safe error responses and digest/reason audits. Architecture manifests independently pass; phase acceptance remains candidate pending dual approvals. |

## independently reproduced commands

1. `go test -race -shuffle=on ./internal/adminapi -run '^TestRestoreDeviceMapsLicenseErrors$' -count=1 -v`: exit 0; success/not_found/forbidden/required/quota/clock/conflict/unavailable/internal all passed. Exact log: /tmp/m6c-acceptance-final-8ad5505-restore.log. Internal failure produced status 500 and expected internal_error through the real HTTP handler.
2. `python3 scripts/check_architecture_manifests.py --all`: exit 0, manifests/owners/providers/references and AGENTS scope passed for 55 requirements.
3. `git diff --check c7be94a..HEAD`: exit 0.
4. Read-only Docker image/network/binary/health/readiness and PostgreSQL queries reproduced the retained runtime identity/outcome. Exact log: /tmp/m6c-acceptance-final-8ad5505-runtime.log. Actual image is sha256:60f87d30159cca4a601a0dce7b4e20af170a0f4c44a20c91f2ad03507bc10b2a, OCI revision 333fc427a4cd9c49715193176993c1e1827dedc6; network internal=true; deployment 923f4a64-2baf-438a-9317-cf401f019989; five telemetry rows; one audit per setup/import step.

No skipped integration result was counted as passing. Full source/database/web/image suites were not rerun in this delta review; their exact retained artifacts and source equivalence were checked. The locked checkout lacks generated embedded dist/node_modules, so no full locked-tree command/image rebuild is claimed here. Prior independent c7be94a full/focused DB evidence remains valid for unchanged production, and this final test-only delta has its own independent regression.

## programming and remove-ai-slops direct pass

Consulted /home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.13/skills/programming/SKILL.md, its Go/TypeScript references, and remove-ai-slops/SKILL.md. Applied the criteria directly to the final delta, production parsing/import/admission/restore/setup/installer code and introduced test classes.

- Excessive/useless tests: final delta eliminates a duplicate register case while adding the missing observable restore failure case. The replacement fails if production returns restore failed or leaks the diagnostic. This is meaningful route regression coverage, not a deletion-only test or a test merely verifying requested removal.
- Tautological/implementation-mirroring tests: HTTP cases use explicit stable error/status expectations independent of handler output; signature tests construct independent signed inputs; DB tests assert rows/errors after real transactions/concurrency. Wire fixtures drive production UI at the HTTP boundary. Future-effect counters are an expressly permitted R39.a fake, not claimed R39.b proof.
- Unnecessary extraction/parsing/normalization: beginWriteTransaction is a pass-through helper (NOTE). Reacquiring the already held License row lock is redundant query overhead (NOTE). Strict JSON duplicate/grammar handling is required by the signed-input contract. Preserving original bytes and re-verifying persisted signed state is necessary, not normalization into a second authorization cache.
- Programming: typed sentinels/stable error mapping, resource ownership, current actor/version reload, explicit transactions, rollback behavior, signer separation, context handling and safe logs were inspected. No criterion-linked scope drift or false confidence was found. Established oversized handlers/SQL/test modules and helper duplication are maintenance NOTES; this read-only scoped review does not require unrequested architecture refactoring.
- Prior code review /tmp/m6c-quality-c7be94a.md explicitly covers both skills, excessive/useless/deletion-only/removal-only/tautological/implementation-mirroring test criteria and unnecessary normalization/extraction, plus its evidence gaps. Its duplicate-register note is corrected by this final delta. Report coverage supplements, and does not replace, this direct pass.

## checkedArtifactPaths

Paths relative to the locked worktree unless absolute:

- AGENTS.md; docs/{README,PLAN,PLAN-DETAILS,EXTENSIONS,ACCEPTANCE,CONTRIBUTING,IMPLEMENTED,DEPLOY,design-m6c}.md; docs/api/license-openapi.yaml; docs/architecture/{acceptance-map,provider-matrix,invariants}.yaml.
- internal/license/{license,signature,parsing,raw_input,license_test}.go; internal/core/{admin_store,tenant_write_auth,license_runtime,license_clock,license_import,license_audit,m6c_license_test,m6c_guard_test,m6c_transaction_test}.go; internal/adminapi/{license,handlers,server_test,m6c_errors_test}.go; internal/platform/setup.go; internal/migrate/sql/011_license.sql; cmd/license-sign/{main,main_test}.go.
- Dockerfile; .dockerignore; deploy/docker-compose.offline.yaml; scripts/{install-offline,build-offline-bundle,offline-smoke}.sh; web/src/views/SystemView.vue; web/e2e/m6c.spec.ts and source-submodule equivalence diff.
- Notepad/executor: .omo/evidence/m6c-license-delivery.md; manual QA: .omo/evidence/m6c-manual-qa-final.md; docs/evidence/acceptance/R{38,39,40}/README.md; docs/evidence/M6c/2026-10-05/README.md. Historical .omo SHA/count prose was not mistaken for current evidence.
- Current retained logs: docs/evidence/M6c/2026-10-05/{adminapi-restore-regression,m6c-verify-ec89218,m6c-integration-ec89218,m6c-focused-new-schema,m6c-contracts,m6c-browser,m6c-web-tests,m6c-web-build,m6c-web-existing-e2e,m6c-docker-final,architecture-manifests-ec89218,architecture-tests-ec89218,architecture-gate,m6c-live-ui,m6c-offline-stop,m6c-offline-resume,offline-followup,offline-tamper}.log.
- Delivery records: same directory m6c-image-identity.txt; offline-manifest.json; offline-SHA256SUMS; sbom.spdx.json; dependency-notices.txt; m6c-offline-containers.json; m6c-offline-status-{before,after}.json; m6c-offline-audit.txt; m6c-live-ui.png.
- All 39 browser PNGs are byte-identical to the prior snapshot. Reopened all three existing labeled contact sheets /tmp/m6c-c7be94a-sheet-{375,768,1440}.png and the real m6c-live-ui.png; inspected visual states, rather than approving by count.
- Prior independent reports /tmp/m6c-acceptance-c7be94a.md and /tmp/m6c-quality-c7be94a.md; /tmp/m6c-quality-db-c7be94a.log with explicit individual M6c test/state PASS records; this review's restore/runtime/equivalence log and runtime-delta patch listed above.

## exactEvidenceGaps

1. Actual issuer/customer License import/renewal and a clean disconnected customer x86_64 host completing installation in 30 minutes remain external_blocked under R38/R40. Local isolated Docker evidence is software evidence only.
2. ARM64, real WeChat/hardware, actual gateway/subdevice integration and R39.b future actual optional entry points remain unaccepted/deferred as documented.
3. Retained image revision 333fc427 predates License-first/restore error corrections. No final 8ad5505 image/build receipt exists. Current corrected production is source-equivalent to verified ec89218; corrected paths have real DB and final HTTP evidence. Rebuild/bind the final distributable before calling this older image the corrected release. This is a provenance limitation within the expressly assigned software/delta review, not a claim of final image delivery.
4. SPDX inventories Go/npm dependencies and identifies images; OS/database package licenses and some source notices are not fully scanned. No complete legal compliance approval is made.
5. The M6c architecture gate's retained software_status=not_run rejection is expected while reviews await registration. Phase state must only change after two independent approvals on this exact source snapshot.
6. `omo-agent-toolkit ulw-loop status --json` returned ULW_LOOP_PLAN_MISSING, so no currentAttemptDir/goalId is available. Requested deliverable is /tmp/m6c-acceptance-final-8ad5505.md; a fallback copy is .omo/evidence/m6c-acceptance-final-8ad5505-gate-review.md in the primary workspace. No file in the locked checkout was edited.

## bookkeeping authorization

This explicit approval may be recorded later in receipts/status bookkeeping as the independent acceptance approval of source 8ad550552d56f061222cd1ff53226000003c703e and web 0aa7771acf0ef6322c9ac1e4c9839616646bb675. When the second independent same-snapshot approval exists, M6c software status may be recorded passed while retaining all external_blocked/R39.b gaps and older-image provenance. Documentation/receipt-only bookkeeping does not invalidate this software approval if it does not change production/test source, scope, or evidence claims.

## final same-snapshot quality cross-check

After completing the independent acceptance pass, inspected /tmp/m6c-quality-final-8ad5505.md. It explicitly approves the same source 8ad550552d56f061222cd1ff53226000003c703e, includes its independent nine-case restore race run, and enumerates programming/remove-ai-slops plus excessive/useless, deletion-only/removal-verification, tautological/implementation-mirroring tests and unnecessary extraction/parsing/normalization criteria. Its cited restore artifact /tmp/m6c-quality-final-8ad5505-restore.log is present and its conclusions align with this independently reproduced result. The two final independent software approvals now target the same snapshot. Later receipt/status-only bookkeeping may register the software result while preserving the listed gaps and image provenance.
