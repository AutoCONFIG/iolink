# M7a camera management increment

This increment adds tenant-scoped camera management while video capability remains disabled. It includes the application service and PostgreSQL adapter, five `/user/v1/cameras` operations, and read-only `/api/v1/cameras` list/get routes for the mini program. It does not enable RTSP/GB ingest, playback, ZLM, media workers, or browser UI.

Frozen implementation and evidence snapshot: `aa0aba1463ee60502092d9770bb8f80656d1f2cb` (`camera management` plus the platform-admin isolation fix and final receipts).

## QA receipt

| Area | Result | Evidence |
|---|---|---|
| Camera application and PostgreSQL adapter, real isolated PostgreSQL, race/shuffle | PASS | `targeted-race.log` |
| HTTP/composition routes, auth, strict JSON, role/tenant scope and mini read paths | PASS | `targeted-race.log`, `http-audit-fix.md` |
| Platform ADMIN with an active support grant cannot read mini camera list/get | PASS; both return 403 before `CameraReader`, row count unchanged | `http-audit-fix.md` |
| Full repository build, vet and race/shuffle with isolated PostgreSQL | PASS; no skips | `full-verify.log` |
| OpenAPI and synthetic contracts | PASS; 96 operations and 568 fixtures | `contracts.log`, `contract-tests.log` |
| Architecture manifest and provider boundaries | PASS | `architecture.log` |
| Package vet and whitespace checks | PASS | `vet.log`, `diff-check.log` |

The initial independent HTTP audit found and rejected one authority leak. Commit `3f00236` adds a narrow user-authority port, fail-closed behavior when that port is unavailable, and real PostgreSQL ADMIN+support regressions. The pre-fix finding is retained in `http-audit-before-fix.md`.

External RTSP/GB hardware, secured ZLM runtime, public TLS/ACL/NAT, browser playback and WeChat device acceptance remain `external_blocked`. This increment does not claim R43 or complete TODO 11a.

## Dual review

The same frozen snapshot received independent APPROVE results from the code reviewer and gate reviewer. Receipts: [code review](final-code-review.md) and [gate review](final-gate-review.md). The previous F1 rejection is retained in [the pre-fix audit](http-audit-before-fix.md); the live-authority guard and real PostgreSQL regression are in [the fix audit](http-audit-fix.md).

