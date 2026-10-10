# M7a FOUNDATION independent implementation code review — 2026-10-11

## Verdict

**APPROVE the M7a foundation increment at the reviewed snapshot.** This approval is limited to the first foundation slice (typed source/playback values, credential cipher, migration 013, and retained-history deletion behavior). It does not approve or complete R43, the M7a phase, or any provider/media/UI work.

```json
{
  "fullSHA": "30f345cfad1d541eff2cc4a1fcb1605b42f04221",
  "sourceTree": "30f345cfad1d541eff2cc4a1fcb1605b42f04221",
  "codeQualityStatus": "CLEAR",
  "recommendation": "APPROVE",
  "reportPath": ".omo/evidence/m7a-foundation-final-code-review-20261011.md",
  "blockers": []
}
```

Reviewer: `/root/m7a_foundation_final_code_review`. I did not participate in the implementation, did not modify tracked files, and did not delegate this review. The locked review worktree was `/tmp/iolink-m7a-foundation-final-code-20261011`; its tracked status was clean at the start and end of review.

## Scope and authority

- Reviewed diff: `2fc2b6ec584cfc97f7cfe46c8f3a1051446a43cf..30f345cfad1d541eff2cc4a1fcb1605b42f04221`.
- Goal: video-default-off R43 foundation only: typed RTSP/GB values, a 300-second playback limit and H264/AAC compatibility, a narrow credential cipher port, independent HKDF-SHA256/AES-256-GCM with random nonce and purpose/tenant/entity/version AAD, approved 013 storage constraints (including tenant/farm/source FKs, TTL/hash, RTP/RTCP pairs, HLS path shape and retained history), and HTTP 409 when existing pond history is protected by the video FK.
- Authority read in order: `AGENTS.md`, `docs/README.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, `docs/CONTRIBUTING.md`, `docs/IMPLEMENTED.md`, approved `docs/design/M7a-video.md`, `docs/design/video-proposal.sql`, `docs/api/video-openapi.yaml`, `docs/deploy/M7a-video.md`, and the M7a foundation evidence README and logs.
- `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`; no current attempt directory or notepad was available. The report therefore uses the requested dated root evidence path and cites the checked-in foundation evidence plus fresh review artifacts.
- Changed implementation/test files are the 14 files listed by the final source-hash manifest: `internal/domain/video_credentials.go`, `video_playback.go`, `video_source.go`, `video_test.go`; `internal/videocredential/cipher.go`, `cipher_boundary_test.go`, `cipher_format_test.go`, `cipher_test.go`; `internal/migrate/sql/013_video_foundation.sql`, `video_foundation_test.go`; `internal/core/video_history_test.go`, `m6b_http_test.go`, `admin_store.go`; and `internal/adminapi/handlers.go`. The remaining diff is documentation/evidence and the design-only pagination fixture correction.

## Findings by severity

### CRITICAL

None.

### HIGH

None. No production correctness, tenant-isolation, transaction, cryptographic integrity, or accidental runtime enablement defect was found in the reviewed foundation scope.

### MEDIUM

None remaining.

The previous independent review identified a verification blocker in `TestVideoCredentialRejectsOversizedEnvelope`: its all-zero input was unauthenticated, so removing the Open length guard would still have produced the same authentication error. The final snapshot fixes this. `internal/videocredential/cipher_boundary_test.go:76-108` now independently derives the key, creates a valid AES-GCM envelope for a 4097-byte plaintext, proves the reference decoder accepts it at lines 96-99, and then requires the adapter to reject the authenticated 4125-byte envelope. The archived overlay run shows the normal test passes and the same test fails when only the production upper-bound guard is removed. This is meaningful negative coverage, not a test that merely mirrors a constant.

### LOW

- `git diff --check` reports trailing spaces in the archived design gate review Markdown, where they are intentional Markdown hard-break spaces. This is evidence formatting only and is unrelated to production code.
- The first full `make verify` preflight in the locked checkout could not find ignored frontend build assets. I copied the existing root `web/dist` and `internal/web/dist` from the same recorded web SHA `f62f66347f6bd7d7e5dbd3ec3a6e7a83fbf13350` for test setup only. The final `make verify` passed; no UI build or video UI claim is made.

Neither low item blocks approval.

## Skill-perspective check

**Ran.** I loaded and applied both required skills:

- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/programming/SKILL.md`, including the Go index and relevant type-pattern, data-modeling, error-handling, and testing references.

The overfit/slop pass covered the changed production code and tests. It found no deletion-only test, requested-removal-only test, prose/prompt assertion, tautological expected value, implementation-constant-only test, broad untyped domain escape hatch, needless production parsing/normalization, speculative provider abstraction, or unnecessary extraction. The independent AES/HKDF interoperability and authenticated oversized-envelope fixtures assert wire behavior that an implementation change must preserve; they do not simply certify source constants. Constructors parse untrusted URI/GB/time/binding input once, private fields protect domain invariants, and the zero-value cipher/binding checks are required Go boundary checks. New files remain below the 250 pure-LOC ceiling; the existing touched handlers/store are legacy large modules and received only the narrow conflict mapping needed by this increment.

## Verified implementation

### Domain and credential cipher

- `RTSPVideoSource` accepts only syntax-valid `rtsp` authorities and rejects userinfo, query/fragment, malformed ports, raw or bracketed non-IP authorities, control characters, backslashes, encoded traversal and nested escapes. Valid hostname, IPv4, bracketed IPv6 and encoded path cases round trip. `String`, `GoString`, and JSON serialization do not expose the URI.
- `GBVideoSource` enforces a positive device identifier and exactly twenty ASCII decimal channel digits.
- `VideoPlaybackWindow` rejects zero/negative/over-300-second windows and applies the exact `now >= expires_at` boundary. Unknown and terminal states are inactive. `CompatibleVideoTracks` accepts H264 with AAC or no audio and rejects implicit H265/G711/missing-video support.
- `Cipher` derives a dedicated 32-byte HKDF-SHA256 key under `iolink-camera-credential-v1`, uses AES-256-GCM with a random nonce envelope, binds purpose plus fixed-width tenant/entity/version AAD, checks context cancellation, and fails closed for unavailable instances, invalid bindings, wrong key/AAD/tag, tampering, truncation, and over-limit ciphertext. Plaintext and escaped-Unicode maximum fixtures round trip, while the independently authenticated 4097-byte envelope is rejected.

### SQL and deletion lifecycle

- Migration 013 matches the approved proposal after the two design-only header comments. Composite foreign keys bind camera to tenant/farm/pond, GB channels/devices to tenant, streams/sessions to camera/tenant/source version, and segments to sessions. Constraints enforce source exclusivity, positive versions, five-minute sessions, token hash length, valid session states, even RTP ports in `30000..30038`, and the dated HLS TS path. The design’s retained-history behavior is preserved by logical camera disablement and non-cascading camera/history references.
- `DeletePond` maps the video FK `23503` to `domain.ErrConflict`, and the user handler returns HTTP 409. Existing M7a HTTP tests exercise active and disabled cameras, farms, foreign/missing resources, role rejection, unauthenticated access, and the no-history 204 path; rejected deletes leave camera, resource, and session counts intact.

## Fresh verification

All fresh outputs are under `.omo/evidence/m7a-foundation-final-code-review-20261011-artifacts/`.

| Check | Result | Artifact |
|---|---|---|
| `go test -race -shuffle=on -count=1 -v ./internal/domain ./internal/videocredential` | PASS; all domain, URI, codec, crypto, Unicode, cancellation, tamper, truncation, and size cases passed | `domain-crypto-review.log` |
| `go vet ./internal/domain ./internal/videocredential ./internal/migrate ./internal/core ./internal/adminapi` | PASS; no diagnostics | `vet-review.log` |
| `GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated disposable DSN> go test -race -shuffle=on -count=1 -v ./internal/migrate ./internal/core -run 'TestM7aVideo'` | PASS against PostgreSQL 16.15/TimescaleDB 2.30.2; migration constraints, real ciphertext persistence, rollback, FK/TTL/hash/RTP/HLS checks and UserRoutes deletion matrix passed with no skips | `pg-http-review.log` |
| `GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated disposable DSN> make verify` | PASS; build, vet, race/shuffle tests across all Go packages; core ran against the isolated database | `make-verify-review.log` |
| `make docs-tools` and `make verify-contracts` | PASS; 96 operations and 568 synthetic fixtures across all OpenAPI files | `docs-tools-review.log`, `contracts-review.log` |
| Root web contract pytest (`scripts/test_video_contracts.py`, `scripts/test_m6a_contracts.py`) | PASS; 22 tests | `contract-pytest-review.log` |
| Final source identity | PASS; all 14 SHA256 entries match the frozen files | `final-source-hashes-review.log` |
| Overlay removal of only the Open upper-bound check | PASS as a negative verification: the normal test passes and the guard-removed overlay fails with `oversized authenticated ciphertext accepted` | checked-in `docs/evidence/M7a/2026-10-11-foundation/cipher-boundary-green.log`, `cipher-boundary-without-guard.log`, `cipher-boundary-overlay.json` |

The dedicated worktree remained clean after these checks; copied frontend assets are ignored setup files only.

## Scope and limits

The snapshot correctly leaves video unavailable by default and `docs/IMPLEMENTED.md` still records M7a as in progress. There is no camera HTTP API, provider adapter, SSRF allowlist/DNS pinning runtime, ZLM security build, SIP/GB runtime, media worker, playback-token gateway, network listener, job processing, or video UI in this increment. Those are deferred by the approved design and must be reviewed separately. Real RTSP/GB hardware, public TLS/ACL/NAT, browser playback, WeChat playback, and external license/provider acceptance remain `external_blocked`; no such acceptance is claimed.

`golangci-lint` and `nilaway` were unavailable in the review environment and were not claimed as passed. The repository’s available `go vet`, race/shuffle, real PostgreSQL, HTTP, OpenAPI, and pytest checks passed. Confidence in this limited foundation verdict is **high** for the inspected scope; confidence does not extend to the deferred provider/media/network/UI increment.

## Final status

**CLEAR / APPROVE.** No CRITICAL or HIGH finding remains, the prior MEDIUM verification blocker is corrected and independently discriminated, the implementation scope matches the foundation goal, and the required final artifact is present at `.omo/evidence/m7a-foundation-final-code-review-20261011.md`.
