# M7a media job lease/CAS increment

This increment adds the provider-neutral in-memory media job lease seam. It
keeps claim, reclaim, revalidation and finish operations compare-and-swap
safe under concurrency, with source/session identity fences and terminal
handling for skipped and unknown results. The store remains outside the
composition root; PostgreSQL persistence and provider workers are later
increments.

## Implemented behavior

- deterministic bounded claims with cryptographically random lease tokens;
- canonical injected store clock for lease expiry and stale-worker fencing;
- retry scheduling with a maximum attempt bound;
- source, camera, stream and session identity revalidation;
- cancellation, malformed input, nil receiver and worker bound fail-closed
  handling;
- terminal states do not replay skipped, failed, unknown, disabled, ready or
  stopped work;
- validation, state and concurrency tests split into cohesive files below the
  250 pure non-comment LOC project limit.

## Verification

The exact implementation snapshot is `582700bc710e1fb09aa1a185d6f4c5a83ea81fc6`.

- `go test -race -shuffle=on -count=1 ./internal/mediajobs` PASS;
- `go test -race -shuffle=on -count=1 ./internal/playback ./internal/mediahttp` PASS;
- scoped `go vet`, gofmt and diff checks PASS;
- `IOLINK_TEST_PG_DSN=<disposable loopback DSN> make verify` PASS;
- `make docs-tools`, `make verify-contracts` (96 operations / 568 fixtures),
  and `python3 scripts/check_architecture_manifests.py --all` PASS.

The two independent exact-snapshot reviews are archived as
[final-code-review.md](final-code-review.md) and
[final-gate-review.md](final-gate-review.md). Both are APPROVE with no
blockers. Reviewers recorded only follow-up medium/low hardening notes; they
did not block this provider-neutral increment.

Real PostgreSQL persistence for this media seam, ZLM/RTSP/GB runtime, browser
playback, WeChat playback and production worker composition remain later or
`external_blocked`; video remains disabled.
