# M7a RTP port-pair allocator final gate review

recommendation: APPROVE
snapshotSHA: cffec89c2e2f25347711faee1c8b835d129d3f86
requestedSnapshot: 411353a (superseded by cffec89)
reviewer: independent read-only final gate reviewer
blockers: []

## originalIntent

Review the provider-neutral RTP allocation increment originally requested at
`411353a`, using the fixed successor `cffec89`. The allocator must provide 20
disjoint UDP RTP/RTCP pairs matching the frozen M7a boundary: even RTP ports
30000 through 30038, RTCP at RTP+1 through 30039, deterministic exhaustion,
safe release/reuse, invalid-pair rejection, concurrent uniqueness, and context
cancellation without capacity mutation.

## desiredOutcome

A small, race-safe allocator seam that later GB/ZLM workers can use without
overlapping RTP/RTCP sockets, while the repository continues to make no claim
that UDP sockets, RTP/PS media, ZLM, SIP, or real-camera playback are running.

## userOutcomeReview

PASS. `internal/media/rtp/ports.go` defines exactly 20 slots from 30000..30038,
requires even RTP and RTCP=RTP+1, reserves each slot under a mutex, reports
`ErrExhausted` after all 20, rejects malformed pairs, rejects double release,
and reuses released slots deterministically. Allocation and release check the
context before and after acquiring the mutex; canceled operations do not alter
capacity. The final `cffec89` lock-held recheck closes the cancellation race
that existed in the requested predecessor.

The concurrent test proves 64 callers yield exactly 20 unique successes and
the rest exhaustion errors. Boundary and invalid-pair tests cover both ends of
the range, odd RTP values, mismatched RTCP, out-of-range values, release/reuse,
nil/canceled contexts, and no-capacity mutation on cancellation.

## blockers

[]

## checked artifact paths

- `AGENTS.md`
- `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`
- `docs/design/M7a-video.md`, `docs/deploy/M7a-video.md`, `docs/evidence/M7a/2026-10-11-design/README.md`
- `internal/media/rtp/ports.go`, `internal/media/rtp/ports_test.go`
- `docs/evidence/M7a/2026-10-11-gb-core/final-code-review.md` as adjacent-scope evidence only
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/programming/SKILL.md`

## verification

- `git rev-parse HEAD` — `cffec89c2e2f25347711faee1c8b835d129d3f86`.
- `go test -race -shuffle=on -count=1 ./internal/media/rtp` — PASS.
- `go vet ./internal/media/rtp` — PASS.
- `test -z "$(gofmt -d internal/media/rtp/*.go)"` — PASS.
- `make verify` — PASS (build, vet, full race/shuffle suite; RTP package PASS).
- `make verify-contracts` — PASS (6 schemas, 96 operations, 568 fixtures).
- `make docs-tools` — PASS.
- `git diff --check 411353a^ HEAD` — PASS.

## code-review and skill-perspective coverage

I loaded and applied the `remove-ai-slops` and `programming` criteria directly
to the complete two-file RTP diff, production code, and tests. The tests assert
observable allocations, errors, bounds, release state, concurrency, and context
behavior; they are not deletion-only, tautological, or implementation-constant
tests. The production seam has no provider SDK import, broad exception path,
dead helper, needless parser/normalizer, or speculative abstraction. The
allocator is a focused single-responsibility module and the test module is
appropriate for the stated boundary matrix.

No separate RTP-specific code-review report or manual user-surface QA matrix
was present. The direct pass and fresh exact-SHA runs provide coverage. There is
no assembled RTP user surface in this increment, so browser/manual visual QA is
not applicable.

## exact evidence gaps and external_blocked scope

- This increment does not bind operating-system UDP sockets or integrate ZLM,
  sipgo, RTP/PS, INVITE/ACK/BYE, GB signaling, a durable allocator/DB lease, or
  restart reconciliation. Those belong to later provider/runtime increments.
- No real GB camera, NAT/ACL, ZLM runtime, browser, WeChat device, or external
  media evidence exists. Those remain `external_blocked`; this review grants no
  R43 completion.
- The in-memory allocator protects process-local concurrent callers only. A
  future multi-instance or persisted worker must add a shared lease/transaction
  boundary before production media use.

These are explicit scope limits and do not violate the allocator increment's
success criteria. Final recommendation: **APPROVE** with runtime media work
remaining deferred or `external_blocked`.
