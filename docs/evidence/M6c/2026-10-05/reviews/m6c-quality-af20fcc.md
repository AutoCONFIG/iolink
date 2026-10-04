# M6c quality gate review — af20fcca8296a32438bc564079077fdcbf854af7

recommendation: REJECT
confidence: high
reviewed_source: af20fcca8296a32438bc564079077fdcbf854af7
reviewed_web: 0aa7771acf0ef6322c9ac1e4c9839616646bb675
comparison_base: 069a1724ffab937b1e0215b623ea825d653805d0
worktree: /tmp/iolink-m6c-review-quality-20261005

## originalIntent

Deliver the M6c software chain for R38, R39.a and R40: signed-original License authorization with durable rollback-clock protection; atomic, License-gated device quota/admission and restore; safe HTTP/CLI errors and rejection audits; the shared optional-feature guard; and a resumable offline x86_64 Docker delivery. R39.b, real issuer/customer host/ARM64/WeChat/hardware remain external_blocked.

## desiredOutcome

A user can safely import and inspect a signed License, retain basic monitoring while authorization is invalid/expired, add/restore devices only under the signed quota and lock order, and install/resume the bundled integrated runtime offline with the documented evidence.

## userOutcomeReview

The snapshot is substantially implemented. I reproduced focused Go tests, the real Timescale package run, and architecture tests. License parser/signature tests, API error mapping tests, the durable clock tests, overage feature guard tests, and offline evidence are present. The browser evidence is visually coherent at 375/768/1440px and the live integrated screenshot shows the signed License view. However, two stated acceptance contracts remain violated in production code, so this snapshot cannot pass the final gate.

## blockers

- id: R39.a-lock-order
  violatedCriterion: R39.a / docs/design-m6c.md “all operations that increase device count lock the License row first, then device/tenant rows”; R39.a concurrent admission safety.
  observation: `RegisterDevice` and `RestoreDevice` call `beginTenantWrite` before `checkDeviceAdmission`/`lockLicenseState`. `beginTenantWrite` calls `AuthorizeTenantWrite`, which acquires tenant and membership `FOR SHARE` locks before the License row. Restore then locks the device row and License row. This is the inverse of the required License-first order and leaves the deadlock/race class unproven despite green concurrency tests.
  evidencePointer: `internal/core/admin_store.go:605-618` and `:684-717`; `internal/core/tenant_write_auth.go:14-35`; `internal/persistence/tenant_authorization.go:28-49`; `internal/core/license_runtime.go:142-149`.

- id: R38-http-error-enum
  violatedCriterion: R38 HTTP contract / docs/api/license-openapi.yaml error enum (all errors use stable safe codes; operational failure is `internal_error`).
  observation: The restore handler's unknown operational-error branch returns `{"error":"restore failed"}`. That value is absent from the License/admin contract enum, which requires `internal_error` for an unclassified server failure. It is a stable-looking phrase but not the contracted code and is inconsistent with the register/import handlers.
  evidencePointer: `internal/adminapi/handlers.go:833-864` (default branch at `:862`); `docs/api/license-openapi.yaml:91-93,113-131`.

## checkedArtifactPaths

- `/tmp/iolink-m6c-review-quality-20261005` (immutable source snapshot)
- `/tmp/iolink-m6c-review-quality-20261005/web` (web submodule at 0aa7771acf0ef6322c9ac1e4c9839616646bb675)
- `/tmp/iolink-m6c-review-quality-20261005/docs/design-m6c.md`
- `/tmp/iolink-m6c-review-quality-20261005/docs/EXTENSIONS.md`
- `/tmp/iolink-m6c-review-quality-20261005/docs/ACCEPTANCE.md`
- `/tmp/iolink-m6c-review-quality-20261005/docs/api/license-openapi.yaml`
- `/tmp/iolink-m6c-review-quality-20261005/internal/license/*.go`
- `/tmp/iolink-m6c-review-quality-20261005/internal/core/license_clock.go`
- `/tmp/iolink-m6c-review-quality-20261005/internal/core/license_import.go`
- `/tmp/iolink-m6c-review-quality-20261005/internal/core/license_runtime.go`
- `/tmp/iolink-m6c-review-quality-20261005/internal/core/admin_store.go`
- `/tmp/iolink-m6c-review-quality-20261005/internal/adminapi/license.go`
- `/tmp/iolink-m6c-review-quality-20261005/internal/adminapi/handlers.go`
- `/tmp/iolink-m6c-review-quality-20261005/scripts/install-offline.sh`
- `/tmp/iolink-m6c-review-quality-20261005/deploy/docker-compose.offline.yaml`
- `/tmp/iolink-m6c-review-quality-20261005/docs/evidence/M6c/2026-10-05/README.md`
- `/tmp/iolink-m6c-review-quality-20261005/docs/evidence/M6c/2026-10-05/m6c-integration-final.log`
- `/tmp/iolink-m6c-review-quality-20261005/docs/evidence/M6c/2026-10-05/m6c-focused-new-schema.log`
- `/tmp/iolink-m6c-review-quality-20261005/docs/evidence/M6c/2026-10-05/m6c-offline-resume.log`
- `/tmp/iolink-m6c-review-quality-20261005/docs/evidence/M6c/2026-10-05/m6c-offline-audit.txt`
- `/tmp/iolink-m6c-review-quality-20261005/docs/evidence/M6c/2026-10-05/offline-followup.log`
- `/tmp/iolink-m6c-review-quality-20261005/docs/evidence/M6c/2026-10-05/m6c-live-ui.png`
- all 39 browser PNGs under `/tmp/iolink-m6c-review-quality-20261005/docs/evidence/M6c/2026-10-05/browser/`

## verification

- `go test ./internal/license ./cmd/license-sign ./internal/adminapi -count=1` — PASS.
- Real DB: `go test -race -shuffle=on ./cmd/iolinkd ./internal/migrate ./internal/core ./internal/platform -count=1 -v` using isolated Timescale/PostgreSQL — PASS; output retained at `/tmp/m6c-quality-db-af20fcc.log` and corroborated by `m6c-integration-final.log`.
- `python3 -m pytest scripts/test_check_architecture_manifests.py -q` — PASS (30 tests). The architecture gate log intentionally remains `R38/R39/R40 software_status=not_run` pending the dual review; this is not treated as proof of completion.
- Visual inspection via image tools covered generated sheets for all 39 browser images at 375/768/1440 and `m6c-live-ui.png`; no material layout overflow was observed.

## remove-ai-slops and programming pass

Direct pass over the changed Go/TypeScript and tests found no deletion-only tests, prompt-prose assertions, broad swallowed catches, or unnecessary production extraction that creates a stated-criterion failure. Tests use real PostgreSQL for transaction behavior and a fake executor only for the explicitly future R39.a guard. The maximum-payload parser unit test references the implementation constant, but the signer boundary test independently constructs 45KiB and 45KiB+1 inputs; this is recorded as a test-quality note, not a blocker.

## exactEvidenceGaps

- No retained test currently asserts the required License-first lock order; the source ordering itself proves the violation above.
- The architecture gate is intentionally pending (`architecture-gate.log`); final phase status must remain pending until two independent reviewers approve the corrected snapshot.
- External issuer, clean customer host 30-minute run, ARM64, WeChat and physical hardware are correctly `external_blocked` and are not blockers for this software-only review.
