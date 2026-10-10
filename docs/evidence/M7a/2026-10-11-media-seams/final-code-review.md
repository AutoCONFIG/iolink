# M7a media/ZLM code-quality review (fixed snapshot)

- `codeQualityStatus`: WATCH
- `recommendation`: APPROVE
- `snapshotSHA`: `c315742561b4f2547e45baff1b43db620d3a5dce`
- `reviewWorktree`: `/tmp/iolink-m7a-media-zlm-code-review-final6` (detached, clean, exact SHA)
- `blockers`: `[]`
- `notepadPath`: none; `omo-agent-toolkit ulw-loop status --json` reported `ULW_LOOP_PLAN_MISSING`, so the required fallback path is used.

## Scope and diff

The cumulative implementation diff was reviewed with `git diff 3b92853..c315742` (33 added files, 1,832 lines). Product scope is `internal/media/contracts.go`, `internal/media/zlm/{gate.go,gate_test.go}`, `internal/playback/{service.go,service_test.go}`, and `deploy/zlm-secure-build/{README.md,build-manifest.json,build.sh,build-probe.sh,patches/README.md}`. The final product implementation is commit `8f011599e94f886077f7947bbbe5150774b97ff`; `1897f13`, `6e35b44`, `248b2b2`, and `c315742` only change evidence documentation or the source-hash receipt. This matches the README/final-verification distinction and was independently checked with `git diff --name-status 8f01159..c315742`.

## Skill perspective check

The `programming` skill and its Go reference, plus `remove-ai-slops`, were loaded before judging tests and maintainability. The direct overfit/slop pass found no deletion-only tests, removal-only tests, prose or prompt pins, tautological output-derived assertions, untyped escape hatches, broad exception handling, provider SDK imports, dead provider helpers, needless production extraction/normalization, or speculative abstraction. Tests assert observable state, error, provider-call, URL, token, lease, and gate behavior. The module-size rule does identify the non-blocking item below.

## Findings by severity

### CRITICAL

None.

### HIGH

None. The named fail-closed and exact-snapshot criteria are met.

### MEDIUM

1. **HTTPS base validation accepts URL query/fragment/userinfo and can produce a malformed grant URL.** `internal/playback/service.go:315-322` trims the base string and appends path segments after `url.Parse` has accepted any absolute `https` URL with a host. For example, `https://media.example.test/media/v1?x=1` becomes a URL whose token/session suffix is part of the query rather than the gateway path; userinfo would also be carried into every grant. The current tests cover empty, relative, HTTP, and hostless bases, but not these forms. This is a configuration correctness and token-placement risk, not a blocker for the current disabled-video seam because no composition root enables it.

2. **Oversized application module.** `internal/playback/service.go` is 344 pure non-comment LOC (the skill ceiling is 250). It combines grant creation, owner reads/stops, token authorization, worker processing, normalization, URL validation, and retry mapping. The code remains readable and tests pass, so this is a maintainability follow-up rather than a gate blocker.

3. **ZLM identity short-circuit test does not observe probe ordering.** `internal/media/zlm/gate_test.go:51-65` is named `TestCheckRejectsUnapprovedIdentityBeforeProbe`, but `fakeRuntime` has no probe-call counter. A regression that probed capabilities before comparing identity would still return `ErrUnapprovedBuild` and pass. Production control flow at `internal/media/zlm/gate.go:65-78` is correctly ordered; the test should be strengthened in a later change to lock the security/cost boundary.

### LOW

1. `docs/evidence/M7a/2026-10-11-zlm-security/README.md:46-48` says the “implementation tree and evidence files” are hashed, while `source-sha256sums.txt` lists five ZLM gate/recipe artifacts only. The listed hashes are correct; narrow the prose or extend the manifest when the evidence package is next revised.

## Independent verification

All commands were run from the clean detached worktree at the fixed SHA.

- `go test -race -shuffle=on -count=1 ./internal/media ./internal/playback ./internal/media/zlm`: PASS.
- `go vet ./internal/media ./internal/playback ./internal/media/zlm`: PASS.
- `sh -n deploy/zlm-secure-build/build.sh deploy/zlm-secure-build/build-probe.sh`: PASS.
- `sh deploy/zlm-secure-build/build-probe.sh`: PASS. The synthetic approved patch was applied and reverse-checked; dirty, unapplied, and unapproved inputs were rejected before fetch/build.
- `sha256sum -c docs/evidence/M7a/2026-10-11-zlm-security/source-sha256sums.txt`: PASS for all five entries, including the refreshed `build.sh` hash.
- `make docs-tools && make verify-contracts`: PASS (video contract included; 96 operations and 568 fixtures).
- `make web-verify`: PASS after initializing the pinned `web` submodule; frontend typecheck/build and mini tests/build passed.
- `make verify`: PASS after frontend embedding; Go build, vet, and full `-race -shuffle=on` suite passed, including `internal/media/zlm` and `internal/playback`.
- `git diff --check 3b92853..c315742`: PASS.

The real ZLMediaKit build remains correctly `external_blocked`: the manifest is `external_blocked`, required security patch files and hashes are absent, base/final image digests are absent, and the host lacks CMake. No RTSP/GB provider, PostgreSQL media adapter, HTTP media route, browser/WeChat playback, camera, or live ZLM success is claimed.

## Named gate conclusions

- **Build hashes:** current hashes match `build.sh`, manifest, gate sources, and patch README.
- **Token issuer:** `RequestPlayback` checks `Tokens == nil` before issuing and the regression test asserts `ErrUnavailable` with no session insertion.
- **Token-only media:** `AuthorizeMedia` intentionally omits actor-context validation for the two OpenAPI `security: []` paths while checking token verification, audience/claims, token hash, live session state, and expiry.
- **Revoked/stale jobs:** revalidated inactive or source-stale `JobStart` leases are persisted as `JobSkipped` without provider calls.
- **HTTPS manifest:** enabled playback rejects empty/relative/HTTP/hostless bases before persistence and emits an absolute HTTPS URL for valid configuration.
- **ZLM gate:** the recipe checks approved manifest/source identity and required digests, clean checkout, `git apply --check`, apply, reverse-check, and refuses an unapproved manifest before submodule update/fetch.
- **Evidence identity:** implementation source and docs-only packaging commits are explicitly separated and match the fixed SHA history.
