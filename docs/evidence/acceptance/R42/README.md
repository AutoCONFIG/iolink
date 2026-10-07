# R42 M6d signed request acceptance

Tested source: `b4b93359976c0529c8f52f50b02a968e3f99750e`.
Web: `207dcfa751545e11a63a3a013da7e60552fecd75`.

Commands, environment, behavior matrix, red/green history, and screenshots:
[M6d 2026-10-07](../../M6d/2026-10-07/README.md).

Dual review is pending. Earlier reports apply only to their recorded snapshots;
this is not completed acceptance.

Coverage: published vector through production verifier, RFC3986/path rules,
body-only tampering without nonce consumption, timestamp bounds, concurrent
durable replay, burst10/refill, HTTP 429+Retry-After, body overflow, and fresh
Service reading persisted nonce/rate state. Reconstruction is not a process-crash
recovery drill.
