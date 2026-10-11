# M7a PostgreSQL media Store gate review

- reviewed snapshot: `55c7b42d4c3eec77a122fc4d29295a0b4973ab4c`
- reviewer: independent read-only gate reviewer (`/root/m7a_gb_gate_review_final2`)
- scope: exact snapshot's retryable media-stop regression test plus the complete existing mediapg/playback seam
- recommendation: **APPROVE**
- blockers: `[]`
- reviewed: 2026-10-11 (Asia/Shanghai)

## Original intent

The PostgreSQL media Store and playback worker must preserve transactional
session/stream/job behavior, current authorization and DB-clock expiry, source
and lease CAS, bounded retries, and shared stop/start fencing. A provider stop
retry must be recorded as retryable without replaying or losing the cleanup
intent; stale/disabled/revoked cleanup still calls Stop, while a live replacement
suppresses only the obsolete stop. The snapshot also needed a playback-service
regression proving provider retry is reported and persisted.

## Desired outcome

The exact adapter/application seam passes its real PostgreSQL and worker tests
without implying that ZLM, RTSP/GB networking, browser, or WeChat runtime is
composed or enabled.

## User outcome review

The exact SHA adds the requested `TestProcessMediaJobRecordsRetryableStop`.
Against disposable PostgreSQL, it passes together with the stop retry fence,
stop/start fence, cleanup distinction, shared-session, expiry, source CAS,
authorization, segment, idempotency, attempts, and license regressions. The full
repository race suite and all contract/architecture checks are green.

## Reproduced checks at exact SHA

All checks ran in clean detached worktree
`/tmp/iolink-mediapg-gate-55c7b42`, pinned to the exact SHA above. `web/dist` was
linked from the existing ignored build artifact solely for the embed build; no
tracked source was changed by that setup.

| Check | Result | Evidence |
|---|---|---|
| `IOLINK_TEST_PG_DSN=<disposable loopback DSN> go test -race -shuffle=on -count=1 -v ./internal/mediapg ./internal/playback` | **PASS**; 39 top-level/subtest entries passed, including `TestProcessMediaJobRecordsRetryableStop` and all PG stop/retry/fence regressions | `.omo/evidence/m7a-mediapg-gate-review-55c7b42-artifacts/m7a-mediapg-55c7b42-focused.log` |
| `IOLINK_TEST_PG_DSN=<disposable loopback DSN> make verify` | **PASS**; build, full vet, and full race/shuffle suite completed with all packages green, including mediapg, playback, migrate, and PG-backed packages | `.omo/evidence/m7a-mediapg-gate-review-55c7b42-artifacts/m7a-mediapg-55c7b42-verify.log` |
| `go vet ./internal/mediapg ./internal/playback` | **PASS** | `.omo/evidence/m7a-mediapg-gate-review-55c7b42-artifacts/m7a-mediapg-55c7b42-vet.log` |
| `gofmt -l internal/mediapg internal/playback` | **PASS**; no files listed | `.omo/evidence/m7a-mediapg-gate-review-55c7b42-artifacts/m7a-mediapg-55c7b42-gofmt.log` |
| `git diff --check HEAD^ HEAD` | **PASS** | `.omo/evidence/m7a-mediapg-gate-review-55c7b42-artifacts/m7a-mediapg-55c7b42-diff.log` |
| `make docs-tools` | **PASS**; pinned validators installed | `.omo/evidence/m7a-mediapg-gate-review-55c7b42-artifacts/m7a-mediapg-55c7b42-docs-tools.log` |
| `make verify-contracts` | **PASS**; 96 operations and 568 fixtures | `.omo/evidence/m7a-mediapg-gate-review-55c7b42-artifacts/m7a-mediapg-55c7b42-contracts.log` |
| `python3 scripts/check_architecture_manifests.py --all` | **PASS**; 55 requirements/owners/providers/references | `.omo/evidence/m7a-mediapg-gate-review-55c7b42-artifacts/m7a-mediapg-55c7b42-architecture.log` |

## Direct skill perspective pass

- **remove-ai-slops:** the 16-line test is a meaningful provider-error to
  retryable-job regression. It asserts both the provider call and persisted
  result, is neither tautological nor deletion-only, and adds no unnecessary
  production extraction or abstraction.
- **programming:** the test uses the existing fake store/provider seam, keeps
  error/status assertions explicit, and introduces no production change. Exact
  SHA formatting, vet, race, and full verification pass.

## Checked artifact paths

- `docs/README.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`
- `docs/ACCEPTANCE.md` (R43)
- `docs/design/M7a-video.md` (CAS, retry bounds, stop/new-session race)
- `docs/IMPLEMENTED.md`
- `internal/media/contracts.go`
- `internal/mediapg/{helpers.go,jobs.go,job_finish.go,store.go,media.go,validation.go,media_segment_test.go,stop_authorization_test.go}`
- `internal/playback/{service.go,service_test.go}`
- `.omo/evidence/m7a-mediapg-gate-review-55c7b42-artifacts/`

## Evidence gaps and external status

1. **`external_blocked`:** no real ZLM/RTSP/GB network, camera, browser, or
   WeChat playback is composed or claimed by this test-only snapshot; those
   remain outside this adapter gate.
2. Disposable PostgreSQL at the isolated loopback DSN was exercised. No
   production database or physical device was claimed.
3. No criterion-linked gaps remain for this exact test increment; the added test
   is covered by the focused and full race runs above.
