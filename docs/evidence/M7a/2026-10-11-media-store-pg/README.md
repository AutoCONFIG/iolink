# M7a PostgreSQL media Store increment

This increment adds the PostgreSQL implementation of the narrow media Store
and segment mapping ports. It persists playback sessions, canonical streams,
idempotent start and stop jobs, lease/CAS transitions, current authorization,
DB-clock expiry checks, and bounded retry handling.

## Implemented behavior

- tenant, member, farm, camera, source, license, token-version and stream
  checks are revalidated at the database boundary;
- start requests are idempotent and shared streams remain consistent across
  concurrent sessions;
- stop transitions fence new creates in `stopping`, preserve that fence across
  retryable provider failures, and permit a fresh start only after terminal
  cleanup;
- stale, disabled, revoked and superseded stop jobs distinguish provider
  cleanup from a live replacement, with worker leases and source/session CAS;
- segment mappings require a live authorized session and current scope;
- all writes use explicit PostgreSQL transactions and fail closed on malformed
  or unavailable state.

## Verification

The exact implementation snapshot is
`55c7b42d4c3eec77a122fc4d29295a0b4973ab4c`.

- disposable PostgreSQL/TimescaleDB 16.15/2.30.2 integration tests passed
  with `-race -shuffle=on`, including authorization, idempotency, expiry,
  source CAS, segment mapping, stop/start fencing and retry reclaim;
- playback service regression coverage verifies retryable provider stops are
  persisted as retryable jobs;
- `go vet`, `gofmt`, `make verify`, `make docs-tools`, `make verify-contracts`
  (96 operations / 568 fixtures), and architecture manifest checks passed;
- the exact-snapshot independent code and gate reviews are archived as
  [final-code-review.md](final-code-review.md) and
  [final-gate-review.md](final-gate-review.md); both approved the product
  snapshot, which was published as `v0.0.26`.

The media provider composition, ZLM/RTSP/GB network runtime, browser playback,
WeChat playback, and physical camera/GB evidence remain `external_blocked` or
later M7a increments; video remains disabled.
