# M7a foundation final gate review

- **Recommendation:** APPROVE
- **Snapshot SHA:** `9a1a1f44af06b1b46a66624bbb0d0c73ed67dfed`
- **Reviewed scope:** first M7a foundation slice only: internal/domain video values, internal/videocredential cipher, migration 013, retained-history deletion behavior, and their tests/evidence.
- **Confidence:** High for this bounded slice; no confidence claim is made for the deferred M7a provider/media/network/UI work.

## Original intent

Deliver the first video foundation increment while keeping video capability disabled: typed RTSP/GB28181 source and playback values; a narrow credential-cipher port with independent HKDF-SHA256/AES-256-GCM, random nonce, and tenant/entity/version/purpose binding; migration 013 with the approved camera, GB, stream, session, segment, composite-FK, TTL/hash, RTP/RTCP, HLS-path, and source-exclusivity constraints; and deletion behavior that preserves video history and returns HTTP 409 when a pond with camera history is deleted.

## Desired outcome

The foundation code must compile, pass focused and repository verification, enforce the domain and cryptographic boundaries, persist only ciphertext under the approved schema, reject invalid/cross-tenant storage relationships, roll back migration failures atomically, and retain active or disabled camera history when resource deletion is attempted. Evidence must be tied to the same frozen snapshot and must not claim the full R43/M7a external acceptance.

## User outcome review

The inspected implementation and evidence satisfy the bounded foundation outcome.

- Domain values reject unsafe RTSP syntax, userinfo/query/fragment/control/traversal forms, invalid GB identities, invalid playback windows, terminal/unknown states, and unsupported codecs. Valid encoded paths and H264/AAC/no-audio cases are covered.
- The cipher derives the dedicated HKDF label, uses AES-256-GCM with random nonce envelopes, binds purpose plus fixed-width tenant/entity/version AAD, checks context cancellation, fails closed for invalid bindings/tampering/truncation/incorrect keys and AAD, and rejects an independently authenticated 4097-byte plaintext envelope. The archived boundary overlay proves the length guard is behaviorally necessary.
- Migration 013 enforces the approved composite tenant/farm/pond/channel/session relationships, source exclusivity, credential envelope minimum, positive versions, legal states, five-minute expiry, token hash size, RTP port pairing constraints, and dated HLS segment path. The real PostgreSQL tests exercise invalid SQLSTATE cases, ciphertext persistence, and atomic rollback.
- Existing pond deletion maps the video foreign-key violation to domain conflict and HTTP 409. The HTTP matrix covers active and disabled cameras, farm and pond history, empty pond success, cross-tenant/missing resources, member/viewer rejection, unauthenticated access, and preservation of rows on rejected deletes.
- `docs/IMPLEMENTED.md` still records M7a as in progress, and the manual QA receipt explicitly marks real RTSP/GB, browser, WeChat, provider, and public-network acceptance as `external_blocked`; no full R43 completion is implied.

## Findings by severity

### CRITICAL

None.

### HIGH

None. No criterion-backed production correctness, tenant-isolation, transaction, cryptographic-integrity, or accidental-video-enable blocker was found.

### MEDIUM

None.

### LOW / notes

- `golangci-lint` and `nilaway` are unavailable in the recorded environment; this is a tooling gap, not a stated foundation success criterion, and available vet/race/real-PostgreSQL checks pass.
- External camera, GB network, browser, WeChat, provider, TLS/ACL/NAT, and media-worker evidence is absent by design and remains outside this increment’s acceptance.

## Blockers

```json
[]
```

No stated success criterion is violated by the reviewed foundation slice.

## Skill-perspective and overfit/slop pass

I loaded and applied:

- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/programming/SKILL.md`

My direct pass over the changed production and test code found no deletion-only or removal-only tests, tautological expected values, implementation-constant-only tests, unnecessary parsing/normalization, broad untyped escapes, needless abstraction, dead code, or missing regression coverage for the shipped behavior. The independent AES/HKDF decoder and authenticated oversized-envelope fixture verify wire behavior and fail under removal of the production guard. The existing touched store/handler modules are legacy large files; this increment adds only the narrow SQLSTATE-to-conflict mapping. The independent code-review report explicitly records the same skill perspective and overfit/slop coverage.

## Checked artifact paths

### Authoritative scope and design

- `AGENTS.md`
- `docs/README.md`
- `docs/PLAN.md`
- `docs/EXTENSIONS.md`
- `docs/ACCEPTANCE.md`
- `docs/CONTRIBUTING.md`
- `docs/IMPLEMENTED.md`
- `docs/design/M7a-video.md`
- `docs/design/video-proposal.sql`
- `docs/deploy/M7a-video.md`
- `docs/api/video-openapi.yaml`

### Implementation and source identity

- `internal/domain/video_source.go`
- `internal/domain/video_playback.go`
- `internal/domain/video_credentials.go`
- `internal/domain/video_test.go`
- `internal/videocredential/cipher.go`
- `internal/videocredential/cipher_boundary_test.go`
- `internal/videocredential/cipher_format_test.go`
- `internal/videocredential/cipher_test.go`
- `internal/migrate/sql/013_video_foundation.sql`
- `internal/migrate/video_foundation_test.go`
- `internal/core/video_history_test.go`
- `internal/core/m6b_http_test.go`
- `internal/core/admin_store.go`
- `internal/adminapi/handlers.go`
- `docs/evidence/M7a/2026-10-11-foundation/final-source-hashes.log`

### Executor/manual QA and independent code review

- `docs/evidence/M7a/2026-10-11-foundation/README.md`
- `docs/evidence/M7a/2026-10-11-foundation/manual-qa.md`
- `docs/evidence/M7a/2026-10-11-foundation/domain-crypto.log`
- `docs/evidence/M7a/2026-10-11-foundation/domain-crypto-report.md`
- `docs/evidence/M7a/2026-10-11-foundation/cipher-boundary-green.log`
- `docs/evidence/M7a/2026-10-11-foundation/cipher-boundary-without-guard.log`
- `docs/evidence/M7a/2026-10-11-foundation/cipher-boundary-overlay.json`
- `docs/evidence/M7a/2026-10-11-foundation/storage-lifecycle.log`
- `docs/evidence/M7a/2026-10-11-foundation/pg-http.log`
- `docs/evidence/M7a/2026-10-11-foundation/verify-after-boundary-test.log`
- `.omo/evidence/m7a-foundation-final-code-review-20261011.md`
- `.omo/evidence/m7a-foundation-final-code-review-20261011-artifacts/domain-crypto-review.log`
- `.omo/evidence/m7a-foundation-final-code-review-20261011-artifacts/pg-http-review.log`
- `.omo/evidence/m7a-foundation-final-code-review-20261011-artifacts/make-verify-review.log`
- `.omo/evidence/m7a-foundation-final-code-review-20261011-artifacts/contracts-review.log`
- `.omo/evidence/m7a-foundation-final-code-review-20261011-artifacts/contract-pytest-review.log`
- `.omo/evidence/m7a-foundation-final-code-review-20261011-artifacts/final-source-hashes-review.log`

### Fresh independent gate checks

- `.omo/evidence/m7a-foundation-final-gate-review-20261011-artifacts/domain-crypto-gate.log`
- `.omo/evidence/m7a-foundation-final-gate-review-20261011-artifacts/vet-gate.log`
- `.omo/evidence/m7a-foundation-final-gate-review-20261011-artifacts/pg-http-gate.log`

## Exact evidence gaps and scope limits

- No current ulw-loop plan exists (`omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`), so no attempt-dir/notepad artifact was available; this report is written at the requested dated root evidence path.
- The full M7a/R43 acceptance artifacts are intentionally not present: camera HTTP API, SSRF/DNS pinning runtime, secured ZLM build, media worker, GB runtime, playback gateway/token/revocation, browser/WeChat UI, and real external camera/GB/TLS/NAT evidence remain deferred or `external_blocked`.
- These gaps do not block this foundation recommendation because they are explicitly outside the first-slice success criteria and are retained in `docs/IMPLEMENTED.md` as incomplete.

## Final verdict

**APPROVE** the foundation increment at `9a1a1f44af06b1b46a66624bbb0d0c73ed67dfed`. This is the required second independent gate review and is limited to the inspected foundation slice.

