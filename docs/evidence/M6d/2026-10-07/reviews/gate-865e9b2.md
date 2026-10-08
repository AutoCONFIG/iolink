# M6d Gate Review — source 865e9b2 / web 3c1d860

- recommendation: **APPROVE**
- confidence: HIGH
- review date: 2026-10-07 (Asia/Shanghai)
- reviewed source: `865e9b28f531c1402bcf96a53c8663ad0e2dab89`
- reviewed web: `3c1d860e2cca0b6ed8207142f5506784f5fef452`
- baselines: source `v0.0.9` / `9f46b6e549d36a6eac8c197acfbe64d2a91585a4`; web `0aa7771acf0ef6322c9ac1e4c9839616646bb675`

## originalIntent

Deliver M6d R41/R42: tenant and resource scoped API Keys with one-time 32-byte secrets encrypted for later HMAC verification; immediate rotation/revocation; License and role enforcement; safe audit and production admin UI; canonical signed `/open/v1` requests with the published vector, inclusive ±300-second window, durable atomic nonce replay protection, and shared deployment-configurable token bucket defaults of 60/minute and burst 10.

## desiredOutcome

A tenant owner can safely issue, view once, rotate, revoke, and audit a Key in the production management surface. A correctly signed authorized open request succeeds, while malformed signatures, body tampering, stale/future timestamps, replay, scope/resource/tenant violations, License denial, and throttling fail closed with the specified status behavior. Secrets and signed payloads do not appear in logs or audit.

## userOutcomeReview

**APPROVE.** The immutable product tree implements the requested behavior. The prior d82aa82 H1/H2/H3/M3 blockers are repaired in the product snapshot: nonblank malformed device entries are rejected before an issuance request; server resource parsing rejects null arrays and invalid item values before persistence; timestamp comparison uses unsigned absolute difference and passes correctly signed extremes; startup configuration wires rate and burst through composition and Docker/deployment docs. I reproduced the focused real Timescale/race/shuffle tests and the contract/architecture checks against this detached tree. The production browser evidence covers issue, one-time display disappearance after reload, rotation, revocation, safe audit, malformed resource requests, scope selection, and 375/768/1440 widths. The screenshots show the masked secret region and audit actions; narrow tables use internal scrolling as documented.

## goalBreakdown

- [ACHIEVED] R41 tenant-scoped issue/list/rotate/revoke/audit endpoints and admin UI — `internal/adminapi/api_keys.go`, `internal/core/api_keys.go`, `web/src/views/SystemView.vue`, `web/src/views/APIKeyAuditView.vue`; live browser log and screenshots.
- [ACHIEVED] Secret is 32 random bytes, shown once, and AES-GCM encrypted for HMAC verification — `internal/core/api_keys.go`, `internal/core/api_key_crypto.go`, migration `internal/migrate/sql/012_open_api.sql`; `TestM6dIssueAndAuthenticateOpenKey`, production vector test.
- [ACHIEVED] Immediate rotation/revocation and atomic audit/state behavior — `internal/core/api_keys.go`; `TestM6dIssueAndAuthenticateOpenKey`, `TestM6dRotation_rejectsChangedResourceOwnershipWithoutWrites`, live browser.
- [ACHIEVED] Scope and farm/pond/device resource authorization, cross-tenant 404, License and current membership/role checks — `internal/core/api_key_resources.go`, `internal/core/open_resources.go`, authorization path; `m6d_resource_http_test.go`, stale membership and feature denial tests.
- [ACHIEVED] Safe diagnostics/audit and injected logger — `internal/openapi/server.go`, `internal/operations/request_logging.go`, `internal/core/api_key_admin.go`; live audit response/screenshot and logging-safe implementation.
- [ACHIEVED] Canonical query/path/HMAC and published vector — `internal/core/open_auth.go`, `docs/api/open-openapi.yaml`; `TestM6dPublishedVector_authenticatesWithProductionVerifier`, canonical tests.
- [ACHIEVED] Inclusive ±300-second window with overflow-safe extremes — `internal/core/open_auth.go:101-107`; `TestM6dTimestampWindow_signedBoundsAndExtremes` covers ±300, ±301, zero, negative, min/max int64, and overflow delta with correctly recomputed signatures.
- [ACHIEVED] Durable atomic nonce and shared rate limit, defaults 60/minute burst10, deployment overrides, 429 Retry-After — `internal/core/open_auth.go`, `internal/core/open_rate.go`, `internal/platform/config.go`, composition in `cmd/iolinkd/main.go`; concurrent/restart/configured-rate tests and deployment docs.
- [ACHIEVED] Contract/docs/status evidence for R41/R42 — `docs/api/admin-openapi.yaml`, `docs/api/open-openapi.yaml`, `docs/evidence/M6d/2026-10-07/README.md`, acceptance files, gate manifest.

## priorBlockerRepairCheck

- H1 malformed device grant: repaired by `web/src/domain/api-key-scope.ts:21-25` and production live loop for `[,] , ,`/trailing input in `web/e2e/m6d-live.spec.ts:34-43`; focused web tests pass.
- H2 timestamp subtraction overflow: repaired by unsigned absolute difference in `internal/core/open_auth.go:101-107`; focused real DB test passes the old reproducer class.
- H3 invalid/null resource persistence: repaired by `internal/adminapi/api_key_resource_input.go` custom null/array boundary and `validateAPIKeyInput` item checks; real browser issues 400 for zero/negative/empty/null resources and focused tests show no writes.
- M3 missing deployment rate configuration: repaired by `internal/platform/config.go`, `cmd/iolinkd/main.go`, Compose/.env/docs; defaults and 30/minute burst2 initialization/refill/Retry-After/rotation tests pass.

## verification

| Scenario | Command/evidence | Result |
|---|---|---|
| Focused real DB + race + shuffle (independent run) | `IOLINK_TEST_PG_DSN=<isolated> GIN_MODE=release go test -race -shuffle=on -count=1 ./internal/core ./internal/adminapi ./internal/openapi ./internal/platform -run 'TestM6d|TestCanonicalOpen|TestConsumeOpenRate|TestConfiguredOpenRate|TestOpenAPIRateConfig|TestAPIKeyResourceInput'` | PASS; core 8.967s, adminapi 1.028s, openapi no selected tests, platform 1.013s. DSN was not printed. |
| Full Go verification | `docs/evidence/M6d/2026-10-07/logs/final-2-go-verify.log` | PASS: build, vet, race, shuffle, isolated Timescale, live browser test; core 94.435s. |
| Web unit/type/build | `docs/evidence/M6d/2026-10-07/logs/review-fixes-final-web.log`; independent `npm test --prefix web -- --run` + `npm run build --prefix web` | PASS: 45 tests, typecheck, production build. |
| Contract and architecture | `make verify-contracts` / `python3 scripts/check_architecture_manifests.py --all` | PASS: 67 operations / 334 synthetic fixtures; 55 architecture requirements. |
| Production browser | `docs/evidence/M6d/2026-10-07/logs/review-fixes-browser-green.log`; `docs/evidence/M6d/2026-10-07/browser/` | PASS: real tenant owner flow, malformed/foreign/null resource rejection, one-time secret, rotation, revoke, audit; screenshots at 375/768/1440 plus audit. |
| Docker | `docs/evidence/M6d/2026-10-07/logs/final-06d2486-contracts-web-docker.log` | PASS: production frontend/embed and image build. |

## programmingAndSlopPass

Loaded and applied `omo:programming` plus Go/TypeScript references and `omo:remove-ai-slops`. The current diff has no deletion-only, prose-pinning, tautological production assertion, broad exception/catch, unsafe type escape, or unneeded provider abstraction that violates a stated criterion. Tests exercise production handlers and real DB/browser paths; the old d82aa82 weak tests are supplemented by correctly signed boundary/extreme and body-only cases. One-line Vue handlers and bundle-size warnings are maintainability notes only and do not fail R41/R42.

## checkedArtifactPaths

- `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/PLAN.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`
- `docs/evidence/M6d/2026-10-07/README.md`
- `docs/evidence/M6d/2026-10-07/logs/final-2-go-verify.log`
- `docs/evidence/M6d/2026-10-07/logs/review-fixes-final-focused.log`
- `docs/evidence/M6d/2026-10-07/logs/review-fixes-final-web.log`
- `docs/evidence/M6d/2026-10-07/logs/review-fixes-browser-green.log`
- `docs/evidence/M6d/2026-10-07/logs/final-06d2486-contracts-web-docker.log`
- `docs/evidence/M6d/2026-10-07/browser/m6d-live-real-tenant-owner-a6b29-evokes-and-reads-safe-audit/key-issued-375.png`
- `docs/evidence/M6d/2026-10-07/browser/m6d-live-real-tenant-owner-a6b29-evokes-and-reads-safe-audit/key-issued-768.png`
- `docs/evidence/M6d/2026-10-07/browser/m6d-live-real-tenant-owner-a6b29-evokes-and-reads-safe-audit/key-issued-1440.png`
- `docs/evidence/M6d/2026-10-07/browser/m6d-live-real-tenant-owner-a6b29-evokes-and-reads-safe-audit/audit.png`
- prior report `docs/evidence/M6d/2026-10-07/reviews/code-d82aa82.md` and repair log `docs/evidence/M6d/2026-10-07/logs/review-fixes-final-focused.log`
- production source paths under `internal/core/`, `internal/adminapi/`, `internal/openapi/`, `internal/platform/`, `internal/migrate/sql/012_open_api.sql`, `cmd/iolinkd/main.go`, and nested `web/src/`, `web/e2e/m6d-live.spec.ts`

## exactEvidenceGaps

- No R41/R42 product evidence gap found for this snapshot.
- `docs/evidence/M6d/2026-10-07/README.md` and `docs/IMPLEMENTED.md` intentionally remain pending until both independent reviewers approve; this is release bookkeeping, not a product failure.
- Synthetic contract fixtures remain synthetic and are not counted as handler proof; real handler/DB/browser evidence above covers the required outcomes.
- External issuer credentials, WeChat, hardware, and unrelated later-stage acceptance remain `external_blocked` by scope and do not block R41/R42.

## blockers

[]



