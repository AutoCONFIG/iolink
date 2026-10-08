# R42 M6d signed request acceptance

Tested source: `657807e195bcfc80b940706c58ad166a2de7a56f`.
Web: `66c52624f6d97c9dd7fe29b05b64b3db39c4a72d`.

Commands, environment, behavior matrix, red/green history, and screenshots:
[M6d 2026-10-07](../../M6d/2026-10-07/README.md).

Dual review is pending. Earlier reports apply only to their recorded snapshots;
this is not completed acceptance.

Coverage: published vector through production verifier, RFC3986/path rules,
body-only tampering without nonce consumption, timestamp bounds, concurrent
durable replay, burst10/refill, HTTP 429+Retry-After, body overflow, and fresh
Service reading persisted nonce/rate state. Reconstruction is not a process-crash
recovery drill.
