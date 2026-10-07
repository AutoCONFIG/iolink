# R42 M6d signed request acceptance

Candidate source: `51898df` (web submodule `248f100`).

Executed checks:

```text
IOLINK_TEST_PG_DSN=postgres://... go test ./internal/core -run 'TestM6d' -count=1 PASS
IOLINK_TEST_PG_DSN=postgres://... go test -race -shuffle=on ./internal/core ./internal/openapi ./internal/adminapi ./internal/migrate ./internal/persistence ./internal/authorization -count=1 PASS
go test ./internal/openapi -count=1 PASS
make verify-contracts PASS (61 operations, 300 synthetic fixtures)
```

The focused integration cases cover the published HMAC vector, RFC3986
canonicalization, body tampering, stale and future timestamps, atomic concurrent
nonce replay, burst-10 and 60/minute refill behavior, real 429 `Retry-After`,
and successful signed HTTP access. The database-backed nonce and rate state is
reloaded by a fresh service instance to verify restart persistence.
