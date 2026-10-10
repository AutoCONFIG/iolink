# M7a GB signaling core final gate review

recommendation: APPROVE
snapshotSHA: e691d944666ef6913a42b6c5ba4388fd69a3128f
reviewer: independent read-only final gate reviewer
blockers: []

## originalIntent

Review the fixed GB28181 signaling-core snapshot that supersedes the requested
`2ad4156` nonce-bound snapshot. The scoped increment must provide provider-neutral
REGISTER Digest/qop and replay handling, bounded nonce state and expiry cleanup,
peer allowlist/conflict checks, Expires and keepalive/offline transitions,
bounded XML/catalog-page parsing with XML-smuggling rejection, and narrow ports.
It must keep UDP/RTP, sipgo, real devices, media runtime, and video composition
disabled and honestly external-blocked.

## desiredOutcome

A reviewable pure GB core whose positive and hostile boundary cases pass, whose
memory replay state cannot grow without bound, and whose evidence does not claim
the later SIP/UDP/RTP/catalog-worker or real-camera acceptance work.

## userOutcomeReview

PASS. `internal/gb` remains provider-neutral: it imports only the standard
library, has `DeviceStore`, `NonceStore`, `Clock`, and `Transport` ports, and is
not assembled by `cmd/iolinkd` or any runtime/network provider. The fixed source
serializes peer conflict check plus apply, fails closed on lookup errors, rejects
expired registrations before Keepalive, and enforces the 64 KiB SIP boundary.

The nonce store now bounds `entries + used` to 4096 and records consumed nonce
expiry times; `Consume` removes expired pending and consumed entries before replay
lookup, while `Issue` evicts the oldest expiry when at capacity. Tests cover
consumption, replay, expiry, and the 4097-entry bound. Digest tests cover the RFC
vector, no-qop and `qop=auth`, algorithm/field rejection, nonce replay and
transaction retransmission. Peer CIDR checks, unknown device/peer rejection,
peer conflicts (including `Expires: 0`), registration TTL bounds, keepalive
recovery, explicit offline, missed keepalive threshold, and expired keepalive
rejection are covered. REGISTER challenge and handling bind the request realm
to the configured device realm, with a wrong-realm regression. XML tests cover
64 KiB input, depth 16, DTD/entity/CDATA,
single-root/trailing content, device mismatch, catalog count and `DeviceList Num`
versus bounded `SumNum` page semantics.

## blockers

[]

## checked artifact paths

- `AGENTS.md`
- `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`
- `docs/design/M7a-video.md`, `docs/deploy/M7a-video.md`, `docs/evidence/M7a/2026-10-11-gb-core/README.md`
- `internal/gb/contracts.go`, `internal/gb/digest.go`, `internal/gb/message_xml.go`, `internal/gb/nonce.go`, `internal/gb/gb_test.go`
- composition and provider search across `cmd/iolinkd` and `internal`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/programming/SKILL.md`

## verification

- `git rev-parse HEAD` — `e691d944666ef6913a42b6c5ba4388fd69a3128f`.
- `go test -race -shuffle=on -count=1 ./internal/gb` — PASS.
- `go vet ./internal/gb` — PASS.
- `make verify` — PASS: build, vet, full race/shuffle suite; `internal/gb` PASS.
- `make verify-contracts` — PASS: 6 schemas, 96 operations, 568 synthetic fixtures.
- `make docs-tools` — PASS.
- `git diff --check 2ad4156..HEAD` — PASS.

## code-review and skill-perspective coverage

No committed GB-specific code-review report, manual user-surface QA matrix, or
ulw notepad was present in the supplied evidence at review time. Per gate policy
I loaded both required skills and performed the direct pass over the complete
GB diff, production package, and tests. The added tests assert observable
behavior and the concurrent conflict regression; they are not deletion-only,
tautological, requested-removal-only, or implementation-constant tests. No
unnecessary production parsing/normalization, provider SDK dependency, broad
exception handling, dead provider helper, or speculative abstraction was found.
`gb_test.go` is a large test module (over the 250 pure-LOC guideline), recorded
as a maintainability note; it does not violate a named stage criterion.

## exact evidence gaps and external_blocked scope

- No SIP socket, sipgo adapter, UDP/RTP/PS/SSRC, INVITE/ACK/BYE, catalog
  aggregation/timeout worker, restart reconciliation, ZLM/media worker, HTTP
  video route, browser/WeChat playback, or real GB camera/device evidence is
  present. These are later increments or external dependencies and remain
  `external_blocked`; this review grants no R43 completion.
- The parser is a bounded message/page seam; it does not prove transactional
  multi-page catalog replacement or missing-channel session revocation, which
  the design explicitly defers to the catalog/application increment.
- There is no assembled user-facing GB surface, so manual visual QA is not
  applicable to this disabled core increment.

These gaps are explicitly outside this scoped core gate and are not criterion
failures. Final recommendation: **APPROVE** with video and real-device paths
remaining disabled/external-blocked.
