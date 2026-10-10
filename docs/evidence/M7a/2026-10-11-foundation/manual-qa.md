# M7a foundation manual QA receipt

Date: 2026-10-11
Snapshot under review: `30f345cfad1d541eff2cc4a1fcb1605b42f04221`

This increment has no browser surface. Manual QA used the real HTTP routes and an isolated PostgreSQL 16.15 / TimescaleDB 2.30.2 container. The disposable database was created and dropped by the test harness; no other containers were touched.

| Scenario | Result | Evidence |
|---|---|---|
| Valid RTSP and GB value objects | PASS | `domain-crypto-tests.log`, `domain-crypto.log` |
| URI/userinfo/query/fragment/control/traversal/authority rejection | PASS | `domain-crypto-tests.log`, `domain-crypto-report.md` |
| Playback TTL and exact expiry boundary | PASS | `domain-crypto-tests.log` |
| HKDF/AES-GCM round trip, independent wire decode, AAD and tamper rejection | PASS | `domain-crypto-tests.log`, `cipher-boundary-green.log` |
| Authenticated 4097-byte plaintext rejected by the adapter | PASS | `cipher-boundary-green.log`, `cipher-boundary-without-guard.log`, `cipher-boundary-overlay.json` |
| Migration 013 constraints and rollback | PASS | `storage-lifecycle.log`, `pg-http.log` |
| Ciphertext persistence and source/history foreign keys | PASS | `storage-lifecycle.log`, `pg-http.log` |
| Active and disabled camera history blocks pond deletion with HTTP 409 | PASS | `storage-lifecycle.log`, `pg-http.log` |
| Empty pond deletion, foreign resource hiding, and viewer rejection | PASS | `storage-lifecycle.log`, `pg-http.log` |
| Existing paginated device endpoint regression | PASS after test fixture correction | `pagination-before.log`, `pagination-after.log`, `verify-after-boundary-test.log` |
| Repository verification, docs tools, OpenAPI contracts and 22 contract tests | PASS | `verify-after-boundary-test.log`, `docs-tools.log`, `contracts.log`, `contract-tests.log` |

The focused and full checks completed without skips. Real RTSP/GB hardware, provider runtime, browser playback, WeChat playback and public network acceptance remain `external_blocked` by the approved M7a scope; this increment intentionally keeps video capability disabled.

