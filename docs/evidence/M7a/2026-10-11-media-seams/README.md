# M7a media session and playback seams

This increment adds provider-neutral application seams while the video capability remains disabled by default. It does not add a provider, SQL adapter, HTTP route, ZLM build, RTSP client, GB runtime, or playback UI.

## Scope

- `internal/media/contracts.go` defines session, source-version, token, media authorization, job, lease/CAS, license, and provider ports.
- `internal/playback/service.go` implements `RequestPlayback`, `ReadPlayback`, `StopPlayback`, `AuthorizeMedia`, and `ProcessMediaJob` orchestration.
- `internal/playback/service_test.go` exercises the required lifecycle and failure contracts with fakes only.

The service rejects playback-management requests when `Enabled` is false, when provider/license capability is absent, or when actor context is missing. The media manifest/segment authorization seam intentionally accepts only the dedicated token context because its contract has `security: []`; it still verifies audience, claims, session live state, and the SHA-256 token hash. `CreatePlayback` is required to perform the atomic session plus start-job write and to recheck source version. `FinishJob` is required to CAS lease token, source version, and session validity. The store owns live membership and owner checks.

## Evidence

| Scenario | Invocation | Observable result | Artifact |
|---|---|---|---|
| Playback TTL and source-version job identity | `go test -race -shuffle=on -count=1 -v ./internal/media ./internal/playback` | `TestRequestPlaybackPersistsFiveMinuteSessionAndVersionedIdempotencyKey` passes; grant is exactly 300 seconds and start key is `start:9:7` | [targeted-race.log](targeted-race.log) |
| Source-version CAS conflict | same command | `TestRequestPlaybackReturnsSourceVersionConflictWithoutGrant` passes; a stale source cannot return a grant | [targeted-race.log](targeted-race.log) |
| Owner-only read/stop and exact expiry boundary | same command | cross-user read/stop return not-found and `now == expires_at` becomes expired | [targeted-race.log](targeted-race.log) |
| License/provider fail-closed | same command | disabled runtime, missing license, and rejected license produce errors before session insertion | [targeted-race.log](targeted-race.log) |
| Missing token issuer fail-closed | `go test -race -shuffle=on -count=1 ./internal/media ./internal/playback && go vet ./internal/media ./internal/playback` | `TestRequestPlaybackFailsClosedWhenTokenIssuerIsMissing` returns `ErrUnavailable` and confirms no session insertion; commit `4bf9a27` | [token-issuer-guard.log](token-issuer-guard.log) |
| Token gateway authorization | same command | audience validation, token hash handoff, and pending state are exercised; pending returns `ErrPending` | [targeted-race.log](targeted-race.log) |
| Job lease/CAS/unknown semantics | same command | provider unknown is persisted as `unknown` with the lease token; CAS lease loss returns `ErrLeaseLost` without replay | [targeted-race.log](targeted-race.log) |
| Media gateway without business actor | `go test -race -shuffle=on -count=1 ./internal/playback ./internal/media` | `AuthorizeMedia` accepts `context.Background()` for the token-only `security: []` media path while invalid token and revoked-session cases remain rejected | [media-gateway-zlm-review.log](media-gateway-zlm-review.log) |
| ZLMediaKit patch application gate | `sh deploy/zlm-secure-build/build-probe.sh` | required patch is applied and reverse-checked; unapplied, dirty, and unapproved inputs are rejected before fetch/build | [media-gateway-zlm-review.log](media-gateway-zlm-review.log) |
| Revoked job and manifest URL boundaries | `go test -race -shuffle=on -count=1 ./internal/playback ./internal/media` | inactive/source-stale start jobs remain `skipped` without provider calls; enabled playback rejects non-HTTPS manifest bases before persistence | [playback-job-url-guards.log](playback-job-url-guards.log) |
| Static checks | `go vet ./internal/media ./internal/playback` | exit status 0 | [targeted-race.log](targeted-race.log) |
| Full Go package regression | `go test ./...` | exit status 0; all packages pass, including `internal/media/zlm` and `internal/playback` | [full-test.log](full-test.log) |
| Repository verification | `make verify` | exit status 0; build, vet, race/shuffle package checks pass | [full-verify.log](full-verify.log) |

## Final candidate

The current implementation candidate is commit `1897f1362cec02ae472216114dd7dd16954c1ee2` (`docs: record playback boundary guards`). It includes the nil token issuer guard, token-only media gateway boundary, fail-closed ZLM patch application gate, revoked-job skip behavior, and HTTPS manifest validation. The current-snapshot receipt is [final-verification.txt](final-verification.txt); focused historical receipts remain in [token-issuer-guard.log](token-issuer-guard.log), [media-gateway-zlm-review.log](media-gateway-zlm-review.log), and [playback-job-url-guards.log](playback-job-url-guards.log). Independent code and gate approval is still required for this exact snapshot.

## Limits

No production composition root enables this service. Real PostgreSQL transaction code, media token cryptography, provider safety construction, RTSP/GB ingest, and external camera/WeChat acceptance remain for later M7a increments.
