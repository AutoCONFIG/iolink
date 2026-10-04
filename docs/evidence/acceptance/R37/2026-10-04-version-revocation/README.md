# M6b permission-version correction and final verification

Requirements: R36.a/R37.a only. Verified executable source: `ef6510ffa5e43fd9796352ae324e09dfc3820c68`; web gitlink: `e84092326d9979e066c611e2be8eb6b9736e09f3`. A subsequent evidence-only candidate commit contains this receipt; its independent review must name that exact candidate. M6b remains pending until two fresh independent approvals.

## Environment and reproduction

2026-10-04, Asia/Shanghai. Dedicated Docker container `iolink-todo9-pg`, PostgreSQL 16.15 and TimescaleDB 2.30.0, local port 55439. Tests create/drop isolated databases. `IOLINK_TEST_PG_DSN` was supplied to every database invocation; credentials are redacted. `GOFLAGS=-buildvcs=false` was used because Go VCS stamping is unavailable in the isolated checkout. Exact source, environment, commands, duration and exit codes are in [verification.json](verification.json).

The previous `c56cb22` candidate was rejected: [independent report](review-c56cb22-reject.md), [reviewer probes](reviewer-probe-red.log). Permission-version increments after authentication still committed telemetry and singular/batch alarm writes; tenant-only contexts used a legacy alarm write path. Its full PASS claims lacked artifacts for the cited source and are not reused as final verification.

Owner regressions failed before the correction: [telemetry and partial-context red](red.log), [alarm version red](red-alarm.log). The same cases pass after the correction: [green](green.log). Telemetry now returns 403; both alarm routes return 404; all rejected writes preserve the complete telemetry/sensor/shadow/alarm/outbox snapshot. Five incomplete context variants reject both open and already-confirmed singular/batch alarm operations with `ErrForbidden`.

The full suite also caught an old idempotency fixture that supplied only a tenant ID: [failure](fixture-failure.log). The fixture now uses its existing valid support identity, retaining the idempotency assertion. Fake executor checks call the production `AuthorizeTenantWrite` seam and reject both role and permission-version revocation before the side effect: [fixture regression](fixture-green.log). No real future job execution is claimed.

## Source-bound final checks

Executed runner: `node docs/evidence/acceptance/R37/2026-10-04-m6b-revocation/verify.mjs`. Every row below exited 0 on `ef6510f`; the two verbose database runs contain zero skipped tests.

| Check | Result and raw output |
|---|---|
| `make verify` | build, vet, full Go race+shuffle suite with real DB: [PASS](logs/go-verify.log) |
| `make docs-tools` | [PASS](logs/docs-tools.log) |
| `make verify-contracts` | 53 operations, 245 fixtures: [PASS](logs/contracts.log) |
| architecture manifests | 55 requirements: [PASS](logs/architecture.log) |
| M6b and adjacent real HTTP/DB tests | product/model/device, telemetry, singular/batch alarm version revocation, isolation, migration, production mux: [PASS](logs/m6b-focused.log) |
| farm/member revocation and fake executor | [PASS](logs/m6b-revocation.log) |
| frontend unit tests | 21 tests: [PASS](logs/web-unit.log) |
| frontend typecheck / production build | [typecheck PASS](logs/web-typecheck.log), [build PASS](logs/web-build.log) |
| browser E2E | 16 scenarios, including role management and support-expiry timezone/precision: [PASS](logs/web-e2e.log) |
| `git diff --check` | [PASS](logs/diff-check.log) |

Browser scenarios exercise the demo UI adapter; actual backend routes and persistence are verified separately by the real HTTP/Timescale tests. The timezone/precision browser artifacts remain at the unchanged web snapshot under [D16](../../../rebuild/M6b-D16/README.md). Hardware, real WeChat, public TLS and capacity acceptance remain `external_blocked`; R36.b/R37.b future resources follow their own stages.
