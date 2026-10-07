# R42 M6d signed request acceptance

Tested source: `21c3704197f89a91387084ed092af8e79c757eb2`.
Web: `3c1d860e2cca0b6ed8207142f5506784f5fef452`.

Commands, environment, behavior matrix, red/green history, and screenshots:
[M6d 2026-10-07](../../M6d/2026-10-07/README.md).

Dual review is pending. Earlier reports apply only to their recorded snapshots;
this is not completed acceptance.

Coverage: published vector through production verifier, RFC3986/path rules,
body-only tampering without nonce consumption, timestamp bounds, concurrent
durable replay, burst10/refill, HTTP 429+Retry-After, body overflow, and fresh
Service reading persisted nonce/rate state. Reconstruction is not a process-crash
recovery drill.
