# M7a GB catalog persistence increment

This increment adds the provider-neutral GB catalog refresh coordinator and PostgreSQL persistence boundary. It remains below the runtime boundary: no SIP/UDP transport, automatic tenant/device creation, ZLM, or composition-root video enablement is included.

## Implemented behavior

- bounded catalog pages with device/SN binding, `SumNum` validation, duplicate idempotency, conflicting duplicate rejection, and deterministic snapshot ordering;
- ten-second refresh deadlines, stale marking that preserves the prior directory, cancellation-safe retry, and completed-refresh expected-count conflict rejection;
- migration 014 adds registration timestamp/peer uniqueness protection and catalog page counters/timestamps/state constraints;
- PostgreSQL refresh locking and same-SN idempotency, atomic channel replacement, missing-channel `present=false` handling, transactional stale state, and database-clock deadline enforcement;
- real isolated PostgreSQL/Timescale integration, race, shuffle, migration, vet, full verification, contract, documentation, and architecture checks.

## Review

The fixed implementation snapshot is `6bee0aea1c485611cfeaef3050f237820f58bc89`. Independent code and gate reviews are archived as [final-code-review.md](final-code-review.md) and [final-gate-review.md](final-gate-review.md); both are APPROVE with no blockers. Real GB/SIP devices, UDP/RTP runtime, ZLM, browser playback, and WeChat playback remain `external_blocked` or later increments.
