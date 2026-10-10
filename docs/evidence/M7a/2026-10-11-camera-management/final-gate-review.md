# M7a camera-management final gate review

- recommendation: **APPROVE**
- snapshot: `aa0aba1463ee60502092d9770bb8f80656d1f2cb`
- reviewed worktree: `/tmp/iolink-m7a-camera-final-gate-20261011`
- reviewer: independent final gate review (read-only)
- confidence: **high** for the frozen software scope; **medium** for real PostgreSQL reproduction in this environment because the receipt's disposable DSN credentials were intentionally omitted and the local attempt could not authenticate

## originalIntent

Ship the M7a camera-management increment: tenant/farm-scoped camera configuration and reads, with strict HTTP boundaries, durable transactional audit/stop-job behavior, and video capability still disabled. The F1 correction must prevent a platform `ADMIN` who has an active support tenant/farm grant from reading either mini-program camera route.

## desiredOutcome

At the exact frozen snapshot, `/api/v1/cameras` and `/api/v1/cameras/{id}` return constant redacted 403 responses for platform `ADMIN` before `CameraReader`; ordinary tenant users retain live tenant/farm scope; configuration fails closed when License/provider capability is absent; mutations commit camera state, audit, session revocation, and stop jobs atomically; responses contain no source URI or credentials; RTSP/GB/media playback remains disabled.

## userOutcomeReview

**PASS.** The F1 guard is mounted before the mini camera handlers. `appapi.authRequired` performs a live `UserAuthority` lookup and marks the request; `cameraContextRequired` returns 503 when the authority port is unavailable and 403 for `ADMIN`, before list/get can call `CameraReader` (`internal/appapi/server.go:316-324`, `internal/appapi/cameras.go:16-52`). The real-PG fixture creates an `ADMIN` with an active support tenant membership and farm grant and exercises both list and detail; the recorded response is exactly `{"code":"forbidden","message":"forbidden"}` and the camera row count is unchanged (`cmd/iolinkd/camera_http_auth_test.go:52-76`, `docs/evidence/M7a/2026-10-11-camera-management/http-audit-fix.md:5-22`, `http-f1-final-verify.txt:4-11`).

Live tenant/farm authorization is enforced inside the PostgreSQL transaction: tenant, actor, active membership, role, authority, permission version, and farm grant are checked; list filters by tenant and live farm scope; get locks and rechecks the farm (`internal/camerapg/authorization.go:11-78`, `internal/camerapg/read.go:18-53`). The receipt records owner/admin/member/viewer/support mini and non-mini scope cases and live revocation (`http-audit-fix.md:24-50`).

Create/replace perform target authorization before License clock observation, then require a valid video License and complete provider/cipher capability; missing key, cipher, runtime, invalid/expired/wrong-deployment License, and clock failures fail closed without committed camera state (`internal/camera/service.go:68-123`, `internal/camera/license.go:27-48`, `internal/camerapg/configuration_test.go:98-136`, `cmd/iolinkd/camera_http_auth_test.go:103-123`). Disable remains available for safety and persists logical disable plus stop intent even when License/provider dependencies are unavailable (`internal/camerapg/invalidation_test.go:36-61`).

Replace/disable use one PostgreSQL transaction for camera mutation, session revocation, idempotent `video.stop` jobs, and audit; trigger failures roll back the camera and all intents (`internal/camerapg/write.go:29-70`, `internal/camerapg/invalidation.go:9-35`, `internal/camerapg/invalidation_test.go:11-126`). HTTP parsing rejects unknown/duplicate fields, undeclared tenant/farm fields, malformed URLs, oversized bodies, non-JSON media, and invalid pagination before application calls; DTOs contain no URI or credential fields and errors are constant code/message objects (`internal/adminapi/camera_input.go:17-115`, `internal/adminapi/camera_boundary_test.go:65-158`, `cmd/iolinkd/camera_http_fixture_test.go:94-122`).

The composition root wires `Availability` as nil and does not instantiate RTSP/GB workers, ZLM, playback, or media jobs; camera configuration therefore returns unavailable while metadata reads and logical disable remain available (`cmd/iolinkd/main.go:249-267`, `cmd/iolinkd/camera.go:11-20`, `docs/evidence/M7a/2026-10-11-camera-management/README.md:1-5`). Deferred external RTSP/GB/ZLM/GB/UI/WeChat work is explicitly marked `external_blocked` in the evidence README.

## blockers

[]

## checkedArtifacts

- `AGENTS.md`
- `docs/README.md`
- `docs/PLAN.md`
- `docs/EXTENSIONS.md`
- `docs/ACCEPTANCE.md`
- `docs/design/M7a-video.md`
- `docs/api/video-openapi.yaml`
- `docs/IMPLEMENTED.md`
- `docs/evidence/M7a/2026-10-11-camera-management/README.md`
- `docs/evidence/M7a/2026-10-11-camera-management/full-verify.log`
- `docs/evidence/M7a/2026-10-11-camera-management/targeted-race.log`
- `docs/evidence/M7a/2026-10-11-camera-management/http-audit-before-fix.md`
- `docs/evidence/M7a/2026-10-11-camera-management/http-audit-fix.md`
- `docs/evidence/M7a/2026-10-11-camera-management/http-f1-final-verify.txt`
- `docs/evidence/M7a/2026-10-11-camera-management/contracts.log`
- `docs/evidence/M7a/2026-10-11-camera-management/contract-tests.log`
- `internal/appapi/server.go`, `internal/appapi/cameras.go`, `internal/appapi/wechat.go`
- `internal/adminapi/cameras.go`, `internal/adminapi/camera_input.go`, `internal/adminapi/auth.go`
- `internal/camera/service.go`, `internal/camera/license.go`, `internal/camera/ports.go`
- `internal/camerapg/authorization.go`, `read.go`, `prepare.go`, `write.go`, `invalidation.go`, `store.go`
- `cmd/iolinkd/main.go`, `cmd/iolinkd/camera.go`
- camera HTTP and PostgreSQL tests under `cmd/iolinkd/*camera*test.go` and `internal/camerapg/*_test.go`

## verification

- `go test ./internal/camera ./internal/adminapi ./internal/appapi` — PASS.
- `go test -race -shuffle=on -count=1 ./internal/adminapi ./internal/appapi ./internal/camera` — PASS.
- `/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.venv/contracts/bin/python scripts/check_contracts.py` — PASS (96 operations, 568 fixtures).
- `go test ./cmd/iolinkd -run '^$' -count=1` — PASS after providing the existing embedded web dist needed by the repository submodule boundary.
- Direct real-PG F1 rerun was attempted with the redacted receipt DSN but failed authentication; it is therefore not claimed as a fresh pass. The committed receipt records the real isolated-PG run and exact 403 bodies.
- Evidence `full-verify.log` records build, vet, and full `go test -race -shuffle=on ./... -count=1` as green with no skipped integration packages; `targeted-race.log` records focused race coverage.

## remove-ai-slops and programming pass

Direct diff/source/test pass found no criterion-blocking slop: no broad error swallowing, secret-bearing production logs, dead camera helpers, deletion-only or tautological regressions, implementation-mirroring tests, or unnecessary provider abstraction. Boundary parsing is typed and strict; transaction and provider errors are narrowed to safe sentinels. Existing oversized modules (`cmd/iolinkd/main.go`, `internal/appapi/server.go`) predate this increment's small edits and are outside this gate's stated acceptance criteria. No camera-management-specific code-review report was present in the frozen evidence directory; direct source/diff/test review above covers the criterion checks, so this is an evidence gap only, not a blocker.

## exact evidence gaps and scope limits

- The camera-management README identifies the implementation commit as `3f00236`; snapshot `aa0aba1463ee60502092d9770bb8f80656d1f2cb` contains the same implementation plus evidence receipts. `git diff 3f00236 aa0aba1463ee60502092d9770bb8f80656d1f2cb -- <production/test paths>` is empty, so the reviewed behavior is unchanged at the requested snapshot, but the receipts do not embed the full snapshot SHA.
- Fresh real-PG reproduction was blocked by unavailable credentials; no external hardware, secured ZLM, RTSP/GB network, browser playback, UI, TLS/NAT/ACL, or WeChat-device evidence was attempted. Those items are explicitly out of scope or `external_blocked` for this increment.
