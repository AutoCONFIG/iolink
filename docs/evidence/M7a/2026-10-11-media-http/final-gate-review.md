# M7a HTTP/HLS media gate review

recommendation: APPROVE
snapshotSHA: 5765e23aac2f3c54f15f27a7fc1ef0eccbe88a90
reviewer: independent read-only final gate reviewer
blockers: []
notepadPath: none (`omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`)
reportPath: `.omo/evidence/m7a-media-http-gate-review.md`

## originalIntent

At fixed snapshot `5765e23`, review the provider-neutral HTTP/HLS media gateway seam from the user brief. Verify rejection of URL RawPath, fragments, query and encoded paths; malformed segment UUID returns 400; enabled playback accepts only a strict absolute HTTPS manifest base; manifest/segment output caps, atomic segment mapping, and bounded deadlines are enforced; generic store failures map to 503; unsupported HLS tags and unsafe paths are rejected; token-only media endpoints honor OpenAPI `security: []`; response headers/statuses are safe; video composition remains disabled; and evidence identifies unavailable external media paths explicitly.

## desiredOutcome

A reviewable exact snapshot whose software behavior passes the named HTTP/HLS checks, whose media token is the only credential on the two `security: []` endpoints, and whose production composition does not enable a provider, playback service, or media route. Real ZLMediaKit, RTSP/GB devices, browser/WeChat playback, and external network acceptance remain `external_blocked`.

## userOutcomeReview

**PASS.** `internal/mediahttp/handler.go` rejects non-empty `RawQuery`, `RawPath`, and `Fragment`, requires the exact `/media/v1/` prefix, validates the token and UUID path components, and returns 400 for malformed segment UUIDs before calling the application. `internal/playback/service.go` rejects non-HTTPS, hostless, userinfo-bearing, query-bearing, fragment-bearing, and opaque manifest bases before session insertion. Manifest and segment reads apply default 3 second and 10 second contexts, enforce 256 KiB and 8 MiB limits before returning data, rewrite only accepted date/hour TS paths, generate segment IDs in the application, and call one batch mapping operation. Generic authorization/mapping store failures normalize to `ErrUnavailable`, which the handler returns as 503; pending returns 425 with `Retry-After: 1`; bad media returns 502; successful responses set `Cache-Control: no-store` and `Referrer-Policy: no-referrer` with the declared media content types. Unknown HLS tags, URI-bearing tags, traversal, encoded/query/fragment paths, and unknown mappings are rejected.

`AuthorizeMedia` intentionally does not require an actor context. It verifies the dedicated token, audience, claims, expiry, token hash, session identity, source version, and live session state, matching the two video OpenAPI operations whose `security` is `[]`. The composition root in `cmd/iolinkd/main.go` does not construct `playback.Service`, `mediahttp.New`, a media provider, ZLM, RTSP/GB worker, or media route; `docs/IMPLEMENTED.md` and the media evidence keep video in progress and external runtime checks blocked.

## successCriteria

| criterion | result | evidence pointer |
|---|---|---|
| HTTP URL RawPath/Fragment/query/encoded rejection and malformed segment UUID = 400 | PASS | `internal/mediahttp/handler.go:25-66`; `internal/mediahttp/handler_test.go:163-184`; fresh targeted race run |
| Strict HTTPS manifest base with no userinfo/query/fragment/opaque/hostless forms | PASS | `internal/playback/service.go:96-98,501-504`; `internal/playback/service_test.go:193-211` |
| Manifest 256 KiB and segment 8 MiB caps | PASS | `internal/playback/service.go:245-248,279-282,319-322`; `internal/mediahttp/handler_test.go:223-237,239-258` |
| Atomic batch mapping and no residual rows on failure | PASS for the provider-neutral seam contract | `internal/media/contracts.go:204-207`; `internal/playback/service.go:311-335`; `internal/mediahttp/handler_test.go:199-213`; no provider adapter is enabled in this increment |
| Manifest/segment deadlines | PASS | `internal/playback/service.go:235-248,262-282`; `internal/mediahttp/handler_test.go:215-220` observes a bounded mapping context; code inspection confirms default 3s/10s values, and the fresh targeted race run passes |
| Generic authorization/mapping store error = 503 | PASS | `internal/playback/service.go:208-210,268-271,386-394`; `internal/mediahttp/handler_test.go:154-161,260-270` |
| Unknown/unsafe HLS tags and paths rejected | PASS | `internal/playback/service.go:289-310,339-364`; `internal/mediahttp/handler_test.go:186-213` |
| Token-only media authorization for OpenAPI `security: []` | PASS | `docs/api/video-openapi.yaml:483-556`; `internal/playback/service.go:193-221`; `internal/playback/service_test.go:301-341` |
| Headers/status contract | PASS | `internal/mediahttp/handler.go:19-23,38-65,81-111`; `internal/mediahttp/handler_test.go:135-152,239-258`; `docs/api/video-openapi.yaml:490-555` |
| Composition remains disabled and evidence is explicit | PASS | `cmd/iolinkd/main.go:301-361`; `docs/IMPLEMENTED.md:40`; `docs/evidence/M7a/2026-10-11-media-http/README.md:1-34` |

## verification

- `go test -race -shuffle=on -count=1 ./internal/mediahttp ./internal/playback` — PASS.
- `go vet ./internal/mediahttp ./internal/playback` — PASS.
- `go test ./...` — PASS.
- `make verify` — PASS: build, vet, and full `-race -shuffle=on` package suite, including `internal/mediahttp` and `internal/playback`.
- `make docs-tools` — PASS.
- `make verify-contracts` — PASS: 96 operations and 568 synthetic fixtures, including 22 video operations.
- `git diff HEAD^ HEAD --check` — PASS.

## changedFilesAndDiff

The HTTP/HLS production/test diff at `5765e23^..5765e23` is:

- `internal/mediahttp/handler.go`
- `internal/mediahttp/handler_test.go`
- `internal/playback/service.go`
- `internal/playback/service_test.go`

The cumulative snapshot also contains the provider-neutral media contract and adjacent GB core work listed by `git diff --name-status 7574972..5765e23`; the full package and contract checks above passed for that cumulative tree.

## checkedArtifactPaths

- `AGENTS.md`
- `docs/README.md`, `docs/PLAN.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, `docs/CONTRIBUTING.md`, `docs/IMPLEMENTED.md`
- `docs/design/M7a-video.md`, `docs/api/video-openapi.yaml`
- `docs/evidence/M7a/2026-10-11-media-http/README.md`
- `docs/evidence/M7a/2026-10-11-media-seams/README.md`, `final-code-review.md`, `final-gate-review.md`
- `internal/media/contracts.go`, `internal/mediahttp/handler.go`, `internal/mediahttp/handler_test.go`, `internal/playback/service.go`, `internal/playback/service_test.go`
- `cmd/iolinkd/main.go`

## codeReviewAndSlopPass

The `omo:programming` and `omo:remove-ai-slops` instructions were loaded. The direct pass found no criterion-blocking deletion-only tests, removal-only tests, prompt/prose pins, tautological output-derived assertions, broad error swallowing, untyped production escape hatches, dead provider helpers, needless production normalization, or speculative provider abstraction. Tests observe status, headers, mapping calls/state, bounded contexts, token claims/hash, and rejected media structures. The prior media-seams code review and gate report are for `c315742`, not this exact HTTP snapshot; no dedicated exact-snapshot code-review report or browser manual-QA matrix is present. The direct source/test review and fresh command runs cover the named software criteria, so those are evidence gaps only.

## exactEvidenceGapsAndExternalBlocked

- There is no `currentAttemptDir` because no ulw-loop plan exists; this report uses the mandated fallback path.
- No real media provider adapter, ZLMediaKit build/runtime, RTSP/GB device, browser, WeChat device, or external network evidence is available. `docs/evidence/M7a/2026-10-11-media-http/README.md` marks those checks `external_blocked`; the composition intentionally remains disabled.
- The batch mapping atomicity is a port contract with a fake seam test; no SQL/provider adapter exists in this increment to validate a real transaction. This does not block the provider-neutral disabled seam criterion.

No criterion-linked evidence gap remains. **APPROVE.**
