# M6c final gate review — 8ad550552d56f061222cd1ff53226000003c703e

recommendation: APPROVE
reviewer_role: independent final quality gate
reviewed_source: 8ad550552d56f061222cd1ff53226000003c703e
prior_approved_source: c7be94a0d239b10571e01d44a6879bfc49e6da96
worktree: /tmp/iolink-m6c-review-quality-8ad5505
blockers: []

## originalIntent

Deliver the M6c software scope for R38, R39.a, and R40: signed License parsing/import/status and durable rollback-clock protection; atomic license-first device admission/restore with quota and feature guards; safe API error categories and rejection auditing; and resumable checksum-verified offline x86_64 Docker delivery with backend and admin web in one runtime. External issuer/customer-host30min/ARM64/WeChat/hardware/R39.b acceptance remains external_blocked.

## desiredOutcome

The final software snapshot must preserve valid License state on invalid import, serialize admission and restore against current authorization, expose safe stable errors, and deliver the tested offline runtime chain. The source change relative to the already approved c7be94a source is limited to moving the redundant `RegisterDevice` internal-error row into `RestoreDevice` and retaining its passing regression log.

## userOutcomeReview

APPROVE for the exact source SHA above. The delta has no production behavior change: it removes one duplicate table row from `TestRegisterDeviceMapsLicenseErrors`, adds the missing operational internal-error case to `TestRestoreDeviceMapsLicenseErrors`, and records the passing package run. The restore handler maps unknown operational failures to HTTP 500 with the stable `internal_error` category; the focused independent race run passed all restore cases, including the newly retained internal case. Source equivalence against c7be94a is exact outside the two intended files.

R38/R39.a/R40 software evidence remains consistent with the authority and prior independent review: full Go verify/race/shuffle, real Timescale integration, signed schema/admission/restore tests, OpenAPI contract checks, web tests/build, 11-scenario browser matrix with 39 screenshots, integrated image, offline stop/resume/tamper/simulator evidence, and architecture-manifest checks. The prior c7be report directly covered programming and remove-ai-slops criteria; I rechecked the current delta and full evidence for deletion-only tests, tautological or output-derived assertions, unnecessary normalization/extraction, swallowed errors, and scope drift. None violates a stated criterion.

Only later receipt/status bookkeeping may record this software approval. It does not convert external_blocked issuer, clean customer host 30-minute, ARM64, WeChat, hardware, or R39.b items into passed evidence.

## checkedArtifactPaths

- `AGENTS.md`, `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/PLAN.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`, `docs/design-m6c.md`
- `docs/evidence/M6c/2026-10-05/README.md`
- `docs/evidence/M6c/2026-10-05/m6c-verify-ec89218.log`
- `docs/evidence/M6c/2026-10-05/m6c-integration-ec89218.log`
- `docs/evidence/M6c/2026-10-05/m6c-focused-new-schema.log`
- `docs/evidence/M6c/2026-10-05/m6c-contracts.log`
- `docs/evidence/M6c/2026-10-05/m6c-browser.log`
- `docs/evidence/M6c/2026-10-05/m6c-docker-final.log`
- `docs/evidence/M6c/2026-10-05/m6c-offline-stop.log`, `m6c-offline-resume.log`, `m6c-offline-audit.txt`, `offline-followup.log`, `offline-tamper.log`, `offline-manifest.json`, `offline-SHA256SUMS`, `m6c-image-identity.txt`
- 39 retained browser images audited through `/tmp/m6c-c7be94a-sheet-{375,768,1440}.png`; integrated `docs/evidence/M6c/2026-10-05/m6c-live-ui.png` inspected separately
- `.omo/evidence/m6c-license-delivery.md`, `.omo/evidence/m6c-manual-qa-final.md`
- prior independent report `/tmp/m6c-quality-c7be94a.md`; prior programming/overfit reports `/tmp/m6c-quality-af20fcc.md`, `/tmp/m6c-acceptance-af20fcc.md`
- current delta: `internal/adminapi/server_test.go`, `docs/evidence/M6c/2026-10-05/adminapi-restore-regression.log`
- independent run: `/tmp/m6c-quality-final-8ad5505-restore.log`

## verification

- `git rev-parse HEAD`: 8ad550552d56f061222cd1ff53226000003c703e
- `git diff --name-status c7be94a0d239b10571e01d44a6879bfc49e6da96 HEAD`: exactly the two intended files above; no source drift.
- `git diff --check c7be94a0d239b10571e01d44a6879bfc49e6da96 HEAD`: PASS.
- `go test -race ./internal/adminapi -run '^TestRestoreDeviceMapsLicenseErrors$' -count=1 -v`: PASS; all nine restore cases pass, including `internal`, with no race failures.
- retained evidence checked: Go full verify and real DB/race logs PASS; docs/contracts 57 operations/284 fixtures PASS; browser 11 scenarios PASS; architecture gate intentionally remains blocked pending dual-review registration.

## direct skill-perspective and overfit pass

Consulted `programming/SKILL.md`, its Go reference, and `remove-ai-slops/SKILL.md`. The review applies their code-quality criteria without editing production or adding speculative refactors. Direct current inspection covered `internal/license/{license,parsing,signature,raw_input}.go`, `internal/license/license_test.go`, `internal/core/{license_runtime,license_clock,license_import,license_audit,admin_store}.go`, `internal/core/{m6c_transaction,m6c_guard}_test.go`, `internal/adminapi/{license,handlers,server_test}.go`, 011 migration, offline scripts/Compose/Dockerfile, and web SystemView/e2e sources.

- Excessive/useless tests: the final delta removes an exact duplicate register row and adds a previously unretained restore operational-error scenario. The new row exercises an authenticated HTTP endpoint with an injected unknown store error and independently expects 500/internal_error. This guards observable safe failure behavior.
- Deletion-only/removal-verification tests: none added. Existing tests cover signed state, transaction failures, clock latch, live database concurrency, and optional-feature effects; they do not merely pin removal of a field or code fragment.
- Tautological/implementation-mirroring tests: restore response expectations come from the external error contract, not the returned output. Signed state fixtures change signature/state/quota meaningfully. Existing maximum payload tests use explicit 45KiB limits. Fake future execution remains the expressly permitted R39.a boundary; it is not claimed as R39.b acceptance.
- Production extraction/parsing/normalization: strict parsing and duplicate JSON detection implement the signed-input contract; raw input hashing preserves rejection digests. No new production abstraction or normalization exists in this delta. Existing pass-through transaction wrapper and duplicate license-row lock are minor maintenance/query-overhead notes.
- Programming: typed sentinel errors are branched with errors.Is; operational errors are collapsed at the HTTP boundary; License state is locked before tenant/device row locks; mutation and auditing use transaction boundaries; clock observation survives business rollback. Runtime private-key exclusion is visible in Dockerfile, offline evidence and signer boundary. No newly swallowed failure or scope drift was found.
- Existing oversized established modules, some multi-step fixture tests, tablet wrapping, bundle warnings and non-strict project tooling remain NOTES. No stated software success criterion requires a module split, a different layout, or a new lint stack.

Prior code-review coverage is explicit in `/tmp/m6c-quality-c7be94a.md` under `direct remove-ai-slops and programming pass`, and in the af20fcc reports. They mention deletion-only/tautological tests, meaningful signed/database fixtures, production extraction and parsing justification. Their prose does not enumerate every overfit category in the same vocabulary; the direct pass above closes that coverage wording gap without substituting report claims for artifact inspection.

## exactEvidenceGaps

- External issuer-supplied production License/renewal, clean customer-host 30-minute installation, ARM64 build/acceptance, real WeChat, physical hardware, and future R39.b real optional entry points remain `external_blocked` as explicitly required by the scope.
- Retained offline/image/browser evidence uses runtime/build SHAs named in the M6c evidence README (333fc427/ec89218 lineage), while this final source snapshot is the c7be94a lineage plus the two test/evidence-only delta files. The final production source is equivalent to c7be94a and its independently retained real-DB run was inspected. The retained image does not exercise the later License-first/restore-error runtime correction; source review, the retained corrected real-DB/full race logs and independent restore HTTP run provide the software evidence for that correction. No new runtime change exists in the final delta. A new final image would improve release provenance; no scoped software criterion explicitly requires it.
- `omo-agent-toolkit ulw-loop status --json` reports `ULW_LOOP_PLAN_MISSING`; no notepad path was supplied or found under `.omo`. This is an evidence-process gap only and is not tied to an R38/R39.a/R40 success criterion.
- The architecture gate log intentionally says software_status=not_run until two independent final approvals are registered; this review is one approval and must be paired with the other reviewer on the same SHA.
- Existing maintenance notes (oversized established Go modules and retained UI layout/warnings) are not criterion failures.


## report placement

No ulw-loop plan exists. Fallback report: `.omo/evidence/m6c-gate-review.md`; requested delivery: `/tmp/m6c-quality-final-8ad5505.md`. Source files were not edited.
