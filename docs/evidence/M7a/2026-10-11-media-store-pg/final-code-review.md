# M7a PostgreSQL media Store code review — exact SHA `55c7b42`

**codeQualityStatus:** WATCH
**recommendation:** APPROVE
**reportPath:** `.omo/evidence/m7a-mediapg-code-review-55c7b42.md`
**blockers:** `[]`
**reviewed SHA:** `55c7b42d4c3eec77a122fc4d29295a0b4973ab4c`
**review mode:** independent, read-only, detached worktree `/tmp/iolink-review-55c7b42`

## Scope

This exact snapshot is the prior approved media Store stop/retry fence at
`051e895e0c3a5bf21e5b91f10716a7c09b2b38b2` plus the committed playback regression:

- `internal/playback/service_test.go`: `TestProcessMediaJobRecordsRetryableStop`,
  which drives a real `ProcessMediaJob` through a provider `ErrRetryable` Stop and
  verifies one provider call, a retryable persisted result, and the propagated error.

The detached worktree was clean and pinned to the full SHA above. No current-root
untracked files or later media/PG edits were included. Production paths in
`internal/mediapg` and `internal/playback` were re-read to confirm the regression
matches the Store retry fence and does not change provider/runtime scope.

`omo-agent-toolkit ulw-loop status --json` reports `ULW_LOOP_PLAN_MISSING`; the required
fallback report path is therefore used. I consulted `docs/README.md`, M7a design/API/DDL,
`docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`, media contracts, migrations, and the
`internal/mediapg`/`internal/playback` tests.

## Verification

The disposable Timescale/PostgreSQL 16 instance was used through a redacted loopback
`IOLINK_TEST_PG_DSN`; its credentials are not recorded.

| Check | Result | Evidence |
|---|---|---|
| `gofmt -d internal/mediapg internal/playback` | PASS (no output) | `.omo/evidence/m7a-mediapg-code-review-55c7b42-artifacts/gofmt.txt` |
| `git diff --check HEAD^ HEAD` | PASS | `.omo/evidence/m7a-mediapg-code-review-55c7b42-artifacts/diff-check.txt` |
| `go vet ./internal/mediapg ./internal/playback` | PASS | `.omo/evidence/m7a-mediapg-code-review-55c7b42-artifacts/vet-scoped.txt` |
| `go vet ./...` | PASS | `.omo/evidence/m7a-mediapg-code-review-55c7b42-artifacts/vet-full.txt` |
| `IOLINK_TEST_PG_DSN=<redacted> go test -race -shuffle=on -count=1 -v ./internal/mediapg ./internal/playback` | PASS; all PG Store/CAS/authorization/expiry/segment/shared-session/retry tests and the new `TestProcessMediaJobRecordsRetryableStop` passed | `.omo/evidence/m7a-mediapg-code-review-55c7b42-artifacts/test-race-scoped-pg.txt` |
| `IOLINK_TEST_PG_DSN=<redacted> go test -race -shuffle=on -count=1 ./...` | PASS, exit 0; all packages ended `ok` or `[no test files]`, including `mediapg` and `playback` | `.omo/evidence/m7a-mediapg-code-review-55c7b42-artifacts/test-full-race-pg.txt` |

Every `internal/mediapg` source/test file remains below 250 nonblank/non-comment LOC;
the test addition does not introduce an oversized module.

## Historical blocker check

- **Lease/source/session CAS:** `ClaimJob`, `RevalidateJob`, and `FinishJob` bind the
  running owner and DB-clock lease; finish verifies job payload, camera source, stream,
  and stream sessions before state mutation. Stale-source and malformed-payload tests
  pass.
- **Current authorization and license:** worker/media/segment paths recheck current
  tenant/user token, membership/permission versions, farm access, camera/source, session
  state, and License/clock-error conditions before provider work. Revocation and
  fail-closed PG regressions pass.
- **DB-clock expiry:** lease, media access, mapping, and active-sibling decisions use
  `clock_timestamp()`, including strict expiry at the database boundary.
- **Idempotency and shared streams:** duplicate session/start intents are reused safely;
  stream/session transactions serialize stop/create and only the last live session queues
  Stop. Stopped streams restart through a fresh start intent.
- **Retry bound and retry fence:** attempts are capped at six; retryable Stop persists
  stream `stopping`, leaves creates fenced, and `ClaimJob` reclaims due `retryable`
  rows. The PG `TestStopRetryKeepsStoppingAndRejectsCreate` and the new playback test
  jointly cover result classification plus persisted retry behavior.
- **Cleanup semantics:** live same-stream replacements set `StopSuperseded` and suppress
  only obsolete provider Stop; stale source, disabled camera, and revoked membership
  still call provider Stop. Existing playback regressions pass.

## Findings

### CRITICAL

None.

### HIGH

None. No historical blocker remains at this exact SHA; the added test is behavior-focused
and does not weaken any authorization, CAS, expiry, ownership, or retry boundary.

### MEDIUM

1. **Pre-existing lock-order inversion (residual, non-blocking):**
   `StopOwnedPlayback` locks a session before the stream (`internal/mediapg/store.go:187-200`),
   while `FinishJob` locks the stream before all sessions (`internal/mediapg/job_finish.go:55-75`).
   A simultaneous user stop and worker finish can therefore be resolved by PostgreSQL as a
   deadlock; rollback is fail-closed and retryable at the caller, and no deadlock appeared
   in the focused/full race runs. This predates `55c7b42` (which only adds a test) and is
   not a release blocker for this snapshot.

### LOW

1. Existing rollback defers use `context.Background()` in unchanged library paths; this
   is cancellation hygiene, not a regression in this test-only commit.
2. Existing malformed lease IDs collapse parse errors to `ErrLeaseLost`; behavior is
   fail-closed but diagnostics are less precise. This is unchanged.

## Required skill checks

- **`remove-ai-slops`: consulted and applied.** The new test observes the provider boundary
  and persisted job result through `ProcessMediaJob`; it is not tautological,
  deletion-only, prose-pinning, or implementation-constant mirroring. No production
  extraction, parser, normalization, or provider SDK slop was added.
- **`programming`: consulted and applied.** The test uses the existing narrow fake seams
  and asserts behavior that fails when retry classification or provider invocation is
  broken. It introduces no untyped escape hatch, needless abstraction, or brittle
  implementation detail. Full race/vet verification is green.

## Decision

**APPROVE** exact SHA `55c7b42d4c3eec77a122fc4d29295a0b4973ab4c`; there are no
CRITICAL/HIGH findings and no blockers. The lock-order item is a pre-existing residual
risk only. Real ZLM/RTSP/GB, camera, browser, WeChat, and production-License acceptance
remain `external_blocked` by the documented M7a scope and are not claimed here.
