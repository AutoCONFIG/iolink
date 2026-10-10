# M7a camera-management final code review

- **Decision:** APPROVE
- **codeQualityStatus:** CLEAR
- **recommendation:** APPROVE
- **Frozen snapshot:** `aa0aba1463ee60502092d9770bb8f80656d1f2cb`
- **Branch:** `codex/m7a-camera-management-20261011`
- **Review worktree:** `/tmp/iolink-m7a-camera-final-code-20261011` (locked, detached at the frozen snapshot)
- **Implementation range reviewed:** `50e72e5..aa0aba1463ee60502092d9770bb8f80656d1f2cb`; the final commit `aa0aba1` is the documented F1 verification receipt on top of the implementation/F1 fix.
- **blockers:** `[]`

## Review basis and skill checks

I independently inspected the authoritative project map (`docs/README.md`), project guidance (`AGENTS.md`, `docs/CONTRIBUTING.md`), M7a design/deployment/acceptance material (`docs/EXTENSIONS.md`, `docs/design/M7a-video.md`, `docs/deploy/M7a-video.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`), the pre-fix audit, the F1 fix evidence, and the frozen-snapshot diff. The prior audit was treated as untrusted until its finding and the fix were checked in source and on real PostgreSQL.

The required `omo:programming` and `omo:remove-ai-slops` skills were loaded. The Go programming review covered typed ports/errors, transaction boundaries, provider direction, concurrency, and tests. No untyped escape hatch, brittle implementation-only assertion, needless production parsing outside the HTTP boundary, or error-handling violation that affects this increment was found. The remove-ai-slops pass covered changed production code and tests: there are no deletion-only tests, tautological tests, tests that only restate a requested removal, or production extraction/normalization that is unrelated to the camera boundary. Exact crypto-format and redaction assertions are contract/security checks, not overfit tests.

## Findings by severity

### CRITICAL

None.

### HIGH

None.

### MEDIUM

None.

### LOW

None.

## Requirement and regression review

- **Tenant/farm ownership and live authorization:** `internal/camerapg/authorization.go:11-60,63-92` derives tenant/user/role from request context, re-reads live tenant, user authority, membership and permission version, and applies farm ownership/grants. `read.go` and `prepare.go` lock and re-check the tenant/farm/pond relationship; list filtering occurs before cursor/limit. Cross-tenant, hidden-farm and revoked-grant reads return the safe not-found/forbidden categories.
- **Platform ADMIN separation on both surfaces:** admin camera routes are mounted only under the user surface (`internal/adminapi/cameras.go:14-21`) and the admin auth path live-loads authority (`internal/adminapi/auth.go:121-170`). The mini surface has read-only camera routes and its guard requires a successful live authority lookup, rejects platform ADMIN, and rejects missing tenant context (`internal/appapi/cameras.go:20-56`; `internal/appapi/server.go:316-324`). The pre-fix audit (`docs/evidence/M7a/2026-10-11-camera-management/http-audit-before-fix.md`) identified the support-context ADMIN leak; the F1 source change is present and the real-PG regression test now gets constant 403 for both list and detail while the camera row count is unchanged (`cmd/iolinkd/camera_http_auth_test.go:52-72`, `docs/evidence/M7a/2026-10-11-camera-management/http-f1-final-verify.txt`).
- **License/provider fail-closed behavior:** `internal/camera/service.go:68-126` performs target authorization before license-clock observation, re-checks target and license state in the mutation transaction, requires the configured license verifier/cipher/runtime capability, and maps verifier/clock/provider failures to safe forbidden/unavailable errors. `cmd/iolinkd/camera.go:12-20` deliberately leaves media availability unassembled, so configuration/replace fails closed while reads and logical disable remain available. The real-PG tests cover missing key, expired/denied clock ordering, and runtime-disabled behavior.
- **Transaction, audit and job atomicity:** `internal/camerapg/store.go` owns begin/commit/rollback and maps provider errors to safe categories. `write.go:11-69` updates the camera, invalidates streams/sessions, inserts the audit event, and commits through one transaction. `invalidation.go:9-35` creates idempotent `video.stop` intents and performs no external provider call. Concurrency, rollback-on-audit/cipher failure, repeated disable, and source-version invalidation tests are included and pass.
- **Strict JSON and redaction:** `internal/adminapi/camera_input.go:23-115` enforces object shape, duplicate/unknown-field rejection, UTF-8, content type and a 16 KiB limit before application calls. Camera DTO/configuration secret fields redact `String`, `GoString` and JSON serialization. Both HTTP surfaces use constant `{code,message}` bodies (`internal/adminapi/cameras.go:23-61`, `internal/appapi/cameras.go:26-72`) and do not expose URI, credentials, SQL/provider text or request data.
- **Provider boundaries and no accidental media enablement:** camera/application packages use narrow store/license/cipher/availability ports; PostgreSQL behavior is in `internal/camerapg`; composition is in `cmd/iolinkd`. No RTSP/GB/ZLM/media worker is assembled in this increment. M7a docs consistently state that media playback is out of scope and video remains disabled.
- **Concurrency, real-PG and contract evidence:** archived evidence reports targeted race/shuffle, full verification, architecture/provider checks, and 96-operation/568-fixture contract checks passing. I reran focused real-PG race/shuffle camera, camerapg, adminapi and appapi tests and the cmd/iolinkd HTTP scenarios against the isolated PostgreSQL instance at `127.0.0.1:32904`; all passed. The cmd scenarios covered endpoint separation, role/farm/tenant scopes, pagination-before-cursor, missing license key fail-closed behavior, resource hiding before license-clock denial, and the platform-ADMIN mini regression. I also reran `go vet` for the scoped packages and the non-DB unit tests; they passed.

## Evidence inspected

- `docs/evidence/M7a/2026-10-11-camera-management/README.md`
- `docs/evidence/M7a/2026-10-11-camera-management/http-audit-before-fix.md`
- `docs/evidence/M7a/2026-10-11-camera-management/http-audit-fix.md`
- `docs/evidence/M7a/2026-10-11-camera-management/http-f1-final-verify.txt`
- `docs/evidence/M7a/2026-10-11-camera-management/targeted-race.log`
- `docs/evidence/M7a/2026-10-11-camera-management/full-verify.log`
- `docs/evidence/M7a/2026-10-11-camera-management/contracts.log`
- `docs/evidence/M7a/2026-10-11-camera-management/architecture.log`

## Scope limits and confidence

**Confidence: high for the reviewed M7a camera-management scope.** This is a code/contract/database review of the frozen snapshot with independent focused real-PG execution. It does not claim external hardware, GB28181/RTSP provider, ZLM/media worker, browser, WeChat or external deployment acceptance; those are intentionally disabled or marked `external_blocked` in the M7a evidence and implementation ledger. The full repository verification logs were inspected rather than rerun in this locked review worktree. The generated ignored web assets were restored only to run the focused `cmd/iolinkd` checks; no tracked files were modified.
