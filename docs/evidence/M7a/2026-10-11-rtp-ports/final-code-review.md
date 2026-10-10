# M7a RTP port allocator code-quality review

- codeQualityStatus: WATCH
- recommendation: APPROVE
- snapshotSHA: cffec89c2e2f25347711faee1c8b835d129d3f86
- requestedIncrementSHA: 411353aea98ebedd06192d711f4fe5e62e2a2d47 (superseded by the lock-held context recheck in cffec89)
- reviewScope: only internal/media/rtp cumulative implementation and tests; unrelated HTTP/GB/media packages excluded
- blockers: []
- reportPath: .omo/evidence/m7a-rtp-code-review-411353a.md
- ulwStatus: ULW_LOOP_PLAN_MISSING; no current attempt directory or notepad was available, so the required fallback report path was used

## Goal and success criteria

Review the bounded RTP/RTCP pair allocator for the frozen M7a design contract: allocate exactly twenty even RTP ports 30000..30038, reserve adjacent RTCP RTP+1 atomically, remain unique under concurrent allocation/release, reject invalid or duplicate releases, and fail closed for nil/canceled contexts without consuming capacity. Check scope control, production/test maintainability, and evidence truthfulness. The allocator is provider-neutral; UDP socket binding, ZLM integration, persistent stream ownership, and camera acceptance remain later/external work.

## Inputs and artifacts inspected

The full cumulative diff for this increment was inspected with:
git diff fd1a918..cffec89 -- internal/media/rtp

Changed files:
- internal/media/rtp/ports.go (92 lines)
- internal/media/rtp/ports_test.go (147 lines)

The design and deployment contracts were read from docs/design/M7a-video.md:171-174 and docs/deploy/M7a-video.md:10-19. Existing media seam evidence in docs/evidence/M7a/2026-10-11-media-seams/ was treated as untrusted background; it does not claim an RTP runtime. No RTP-specific receipt or notepad was present. The working tree had unrelated untracked evidence files; they were not used to establish this result.

## Skill perspective check

Before judging test relevance or maintainability, I loaded and consulted omo:remove-ai-slops and omo:programming (Go guidance). The remove-ai-slops pass found no deletion-only tests, tautological assertions, broad catch/escape hatch, dead production helper, provider SDK leakage, or unnecessary parsing/data extraction. The production file is small and single-purpose (bounded allocator). The tests exercise the public allocation, validity, release, context, and concurrency seams. Two evidence-quality follow-ups are recorded below.

## Findings by severity

### CRITICAL

None.

### HIGH

None. The final snapshot checks context both before and after taking the allocator mutex (ports.go:43-66 and 68-92), so a canceled context that waited behind another operation cannot reserve or release a pair. The fixed constants and indexed array yield twenty disjoint pairs, and all state transitions are mutex-protected.

### MEDIUM

1. Tests derive the frozen port contract from implementation constants. ports_test.go:20 and 128-144 build expected values from MinRTPPort, MaxRTPPort, and RTCPOffset. If those constants drifted away from the M7a contract (30000..30038 even RTP with RTCP through 30039), these tests would continue to pass. Keep the implementation constants, but add contract-level literals or a dedicated assertion against the frozen range. This is an evidence gap, not a current implementation failure.

2. The lock-contention cancellation fix is not directly regression-tested. ports.go:53-56 and 81-85 correctly re-check ctx.Err() after acquiring the mutex, but ports_test.go:64-87 only covers an already-canceled context. A deterministic test that holds the allocator lock while canceling a waiter would protect the exact regression fixed by cffec89. Current code behavior and the fresh race run are good; this is a test-strength follow-up.

3. Release is identified only by the pair value, so a stale release can free a reused pair. After Release(P), another caller can Allocate() and receive the same P; a delayed second Release(P) then clears the new owner allocation (ports.go:68-90). The current test checks an immediate duplicate release before reuse (ports_test.go:53-61) but not this lifecycle boundary. The provider layer should serialize ownership or the allocator should return an opaque generation/token if concurrent teardown paths can issue stale releases. This is a medium integration/design risk for the later open/close flow, not a blocker for this isolated provider-neutral allocator increment.

### LOW

1. Allocator is intentionally in-memory and does not probe or bind UDP sockets. Multiple allocator instances or an external process can still contend for the same OS ports; the later provider/composition layer must use one shared allocator and treat bind failure as a failed session.

2. Exported symbols have no Go doc comments. This is not a correctness issue and go vet is clean, but package documentation should be added before this becomes a public integration seam.

## Criterion review

- Port range: allocation yields exactly RTP 30000, 30002, ..., 30038; each RTCP value is RTP+1 (ending at 30039), matching the design and firewall range.
- Atomic reservation and concurrency: one mutex protects the fixed twenty-slot array; concurrent callers cannot duplicate a slot and excess callers receive ErrExhausted.
- Context fail-closed: nil contexts return ErrInvalidContext; canceled contexts return the underlying cancellation/deadline error before mutation; the final snapshot also checks after mutex acquisition.
- Release boundary: malformed parity/range/adjacency returns ErrInvalidPair; unallocated pairs return ErrNotAllocated; a successful release makes the slot reusable. The stale-release ownership caveat is recorded above.
- Scope: no provider SDK, UDP listener, ZLM API, persistence, or external device claim was added.

## Fresh verification at exact snapshot

Environment: Go 1.26.8, linux/amd64; git rev-parse HEAD = cffec89c2e2f25347711faee1c8b835d129d3f86.

- go test -race -shuffle=on -count=1 ./internal/media/rtp: PASS
- go vet ./internal/media/rtp: PASS
- test -z "$(gofmt -d internal/media/rtp/*.go)": PASS
- git diff --check cffec89^ cffec89 -- internal/media/rtp: PASS
- go test -race -shuffle=on -count=1 ./...: PASS
- go vet ./...: PASS
- make verify (build, vet, race/shuffle suite): PASS

## External and deferred evidence

No real UDP bind, ZLM openRtpServer/close lifecycle, persistent stream/SSRC ownership, NAT/firewall, GB camera, RTP/PS media, or restart reconciliation is implemented or evidenced by this snapshot. Those checks remain external_blocked or later-scope work; the fresh tests prove only the in-memory allocator and its fake-free concurrency behavior.

## Final recommendation

APPROVE the provider-neutral RTP allocator at exact SHA cffec89c2e2f25347711faee1c8b835d129d3f86, with blockers=[]. Carry the three medium test/ownership follow-ups and the external-blocked runtime evidence into the next M7a increment.

