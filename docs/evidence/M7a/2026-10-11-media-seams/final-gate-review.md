# M7a media/ZLM final gate review

recommendation: APPROVE
snapshotSHA: c315742561b4f2547e45baff1b43db620d3a5dce
reviewer: independent read-only final gate reviewer
blockers: []

## originalIntent

At the fixed M7a media/ZLM snapshot, verify the provider-neutral media seams and
the fail-closed ZLMediaKit build boundary while video remains disabled. The
named checks are: nil token issuer rejection; token-only media authorization for
OpenAPI `security: []`; inactive and source-stale start-job skips; absolute HTTPS
manifest URLs; pinned ZLM identity, required patch hashes, clean checkout,
patch apply and reverse checks, and rejection of an unapproved manifest before
submodule fetch; disabled video composition; source-vs-docs-only identity; and
the documented API/contract behavior.

## desiredOutcome

A reviewable exact fixed snapshot whose software guards pass and whose evidence
does not claim an unbuilt ZLM/RTSP/GB or external playback path. The increment
must remain video-disabled and must record unavailable provider/runtime and
real-camera/WeChat checks as `external_blocked`.

## userOutcomeReview

**PASS.** `internal/playback/service.go` checks `Tokens == nil` before issuance;
the regression test confirms `ErrUnavailable` and no session insertion. The
media gateway does not call `requireActor`; with `context.Background()` it
verifies the dedicated token, audience, claims, token hash, session state and
expiry, matching the two video OpenAPI operations with `security: []`. The job
worker persists `JobSkipped` and makes no provider call when a revalidated start
lease is inactive or source-stale. Enabled playback rejects empty, relative,
HTTP, and hostless bases and emits only an absolute HTTPS manifest URL.

`deploy/zlm-secure-build/build.sh` checks the pinned source SHA, manifest
approval, base-image digest and every required patch SHA before any submodule
update/fetch. It rejects dirty tracked/index/untracked source state, performs
`git apply --check`, applies each patch, and reverse-checks each applied patch.
The synthetic probe passes the approved apply/reverse path and rejects unapplied,
dirty and unapproved inputs before fetch/build. The checked manifest is
intentionally `external_blocked` with null patch/base-image digests, so no ZLM
build or runtime is claimed.

No composition-root call constructs or enables `playback.Service`, a provider,
ZLM, RTSP, GB worker, media HTTP route or playback UI. `docs/IMPLEMENTED.md`
continues to mark M7a in progress and the implementation/evidence README
explicitly limits this increment to seams. Product code stops at implementation
source `8f011599e94f886077f7947bbbe5150774b97ff`; commits through the reviewed
snapshot `c315742561b4f2547e45baff1b43db620d3a5dce` are documentation/evidence
packaging only. The refreshed `source-sha256sums.txt` matches all five listed
artifacts, including `build.sh`.

## verification

| Check | Result | Evidence |
|---|---|---|
| `go test -race -shuffle=on -count=1 ./internal/media ./internal/playback` | PASS | fresh run; scoped receipt `docs/evidence/M7a/2026-10-11-media-seams/targeted-race.log` |
| `go vet ./internal/media ./internal/playback`; `git diff --check` | PASS | fresh run; `docs/evidence/M7a/2026-10-11-media-seams/diff-check.log` |
| `make verify` | PASS; build, vet, full race/shuffle suite; `internal/media/zlm` and `internal/playback` pass | fresh run; `docs/evidence/M7a/2026-10-11-media-seams/full-verify.log` |
| `make docs-tools` | PASS | fresh run |
| `make verify-contracts` | PASS; 96 operations, 568 fixtures, including video 22 operations | fresh run; `docs/evidence/M7a/2026-10-11-zlm-security/manifest-gate.log` and contract command output |
| `.venv/contracts/bin/python scripts/test_video_contracts.py` | PASS | fresh run |
| `python3 scripts/check_architecture_manifests.py` | PASS; 55 requirements | fresh run; `docs/evidence/M7a/2026-10-11-media-seams/architecture.log` |
| `sh -n deploy/zlm-secure-build/build.sh` | PASS | fresh run; `docs/evidence/M7a/2026-10-11-zlm-security/recipe-syntax.log` |
| `sh deploy/zlm-secure-build/build-probe.sh` | PASS; approved apply/reverse plus dirty/unapplied/unapproved rejection | fresh run; `docs/evidence/M7a/2026-10-11-media-seams/media-gateway-zlm-review.log` |
| `sha256sum -c docs/evidence/M7a/2026-10-11-zlm-security/source-sha256sums.txt` | PASS; all five files | fresh run |

## checked artifact paths

- `AGENTS.md`, `docs/README.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, `docs/CONTRIBUTING.md`, `docs/IMPLEMENTED.md`
- `docs/design/M7a-video.md`, `docs/deploy/M7a-video.md`, `docs/api/video-openapi.yaml`
- `internal/media/contracts.go`, `internal/media/zlm/gate.go`, `internal/media/zlm/gate_test.go`
- `internal/playback/service.go`, `internal/playback/service_test.go`
- `cmd/iolinkd/camera.go`, composition/search results under `cmd/iolinkd` and `internal`
- `deploy/zlm-secure-build/README.md`, `build-manifest.json`, `build.sh`, `build-probe.sh`, `patches/README.md`
- `docs/evidence/M7a/2026-10-11-media-seams/README.md`, `targeted-race.log`, `full-test.log`, `full-verify.log`, `media-gateway-zlm-review.log`, `token-issuer-guard.log`, `playback-job-url-guards.log`, `final-verification.txt`
- `docs/evidence/M7a/2026-10-11-zlm-security/README.md`, `build-recipe.log`, `environment.log`, `manifest-gate.log`, `gate-race.log`, `gate-vet.log`, `gate-format.log`, `recipe-syntax.log`, `source-sha256sums.txt`
- `.omo/evidence/m7a-media-zlm-gate-review-20261011.md` (prior nil-token rejection and fix history), `.omo/evidence/m7a-media-zlm-gate-review.md` (prior exact-SHA documentation rejection), and `.omo/evidence/m7a-media-zlm-gate-review-20261011-token-nil-probe.txt`

## code-review and skill-perspective coverage

No dedicated code-review report for exact snapshot `c315742` exists under the
scoped M7a media evidence directory. Per gate policy, I read the available
evidence and ran the direct coverage pass. The prior gate reports were treated
as untrusted history; the nil-token finding is confirmed fixed in source and
tests. The required `remove-ai-slops` and `programming` skills were loaded.

The direct overfit/slop pass found no deletion-only, requested-removal-only,
tautological, implementation-mirroring, prose-only, broad exception, dead
provider helper, needless production normalization, or speculative abstraction.
Tests assert observable TTL, token, lease/CAS, state, provider-call, URL and
ZLM gate behavior. `internal/playback/service.go` is 344 pure non-comment LOC,
above the skill's 250-line maintainability threshold; this is a residual
maintainability note and does not violate a stated M7a acceptance criterion.

No separate user-facing media manual-QA matrix exists because no media surface
is assembled in this increment. The checked-in seam QA tables and fresh command
runs cover the software scope; browser/WeChat/camera/ZLM runtime QA remains
unclaimed and `external_blocked`.

## exact evidence gaps, external_blocked, and residual risks

- `build-manifest.json` remains `external_blocked`: required security patch
  files and hashes, base-image/final-image digests, CMake, hardened ZLM binary,
  and live capability probe are absent. No RTSP redirect/origin/log-redaction
  runtime, GB28181 runtime, PostgreSQL media adapter, media HTTP route, browser
  playback, camera, or WeChat evidence exists yet.
- The media tests use fakes, so they prove application orchestration contracts
  but not a future SQL store's transaction/CAS implementation or real provider
  side-effect ordering. Those are deferred to later M7a increments.
- `internal/playback/service.go` exceeds the programming skill's 250 pure-LOC
  guideline and may merit later decomposition; it is outside this gate's named
  success criteria.
- `omo-agent-toolkit ulw-loop status --json` reports no plan, so there is no
  `currentAttemptDir` or notepad path. This report uses the required fallback
  path `.omo/evidence/m7a-zlm-security-gate-review.md`.

These are explicit scope limits or maintainability notes, not blockers for the
disabled media/ZLM seam gate. No criterion-linked evidence gap remains.
