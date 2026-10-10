# M7a RTP/RTCP port-pair allocator increment

This increment adds a provider-neutral in-memory allocator for the approved RTP/RTCP range. It reserves the 20 even RTP ports `30000..30038` and the adjacent RTCP port for each pair. It is not wired into UDP, ZLM, GB signaling, or the composition root; video remains disabled.

## Implemented behavior

- deterministic lowest-free allocation across 20 pairs;
- atomic release and reuse, invalid and duplicate release errors;
- concurrent allocation uniqueness and bounded exhaustion;
- cancellation and nil-context fail-closed behavior, including a context recheck while holding the allocator lock;
- explicit RTP-even/RTCP-plus-one validation.

## Verification and review

The fixed snapshot is `cffec89c2e2f25347711faee1c8b835d129d3f86`. Scoped race/vet/gofmt/diff checks, full `make verify`, contract checks, and docs tools passed. Independent code and gate reviews are archived as [final-code-review.md](final-code-review.md) and [final-gate-review.md](final-gate-review.md); both are APPROVE with no blockers.

OS UDP binding, SIP/GB runtime, ZLM, real cameras, browser playback, and WeChat playback remain deferred or `external_blocked`.
