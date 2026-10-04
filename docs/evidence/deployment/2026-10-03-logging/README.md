# Persistent internal diagnostics

User-requested additional deployment milestone, 2026-10-03; related boundaries R04/R29.
Behavior source: `e1b78e12d9dea3c1f60412fce1bcdaa0adb33b0e`.
Published frontend gitlink: `c54237e9d1a424bc9cb0b3f700014e2e966cf2cc`.
The frozen snapshot received two independent approvals: gate review
`.omo/evidence/logging-gate-review.md` and code review `code-review.md`.

Environment: Docker 29.8.2, Compose 5.5.1, PostgreSQL 16.15, TimescaleDB 2.30.2.
Exact image digest is in `logs/database-image.log`. All database tests used a fresh
isolated Compose database; `IOLINK_TEST_PG_DSN` was set, not skipped.
Secrets were generated in memory and excluded from recorded output.

Run from the repository root with Node and Docker available:

```sh
node docs/evidence/deployment/2026-10-03-logging/verify.mjs
```

The harness reserves loopback ports 18091/11891/55451, builds current source,
uses the locally pulled database image with `--pull never`, and removes its own
random Compose project and volume. It also clears the test checkout's `deploy/logs`;
run in an isolated checkout, never against a server or an existing log archive.
Complete commands, expected/actual exits and timings are in `logs/verification.json`.
Cleanup ran successfully after that metadata was written; its logs are included separately.
Raw Docker progress output retains its trailing padding spaces. The source
`git diff --check` passed before evidence import; checking the imported raw logs
reports those spaces. They are preserved as execution output, not source formatting.

| Scenario | Observed result | Evidence |
|---|---|---|
| Build, migrate, stdin admin bootstrap, startup | All exit 0; readiness 200, login 200 | `logs/docker-build.log`, `logs/migrate.log`, `logs/admin-init.log`, `logs/verification.json` |
| Actual HTTP request correlation and denied request | Returned server request ID matches JSON row; denied request 401; caller ID ignored | `logs/persistent-json.log`, `logs/verification.json` |
| Actual authenticated MQTT3 QoS1 report | Temperature 26 committed once; internal debug event recorded | `logs/telemetry-count.log`, `logs/persistent-json.log` |
| Secret redaction | Generated passwords/root key/MQTT secret and supplied JWT/path/query/body identifiers absent | Harness assertions, `logs/verification.json`, `logs/logging-focused.log` |
| Restart persistence | Original request ID retained after restart and readiness restored | `logs/restart-json.log`, `logs/restart-wait.log` |
| Invalid level and unwritable directory | Exit 1, safe specific message, no raw path/secret | `logs/invalid-level.log`, `logs/invalid-path.log` |
| Rotation, concurrency, failed/short writes and closed sink | Race/shuffle tests pass; health fails on write failure; closed sink cannot reopen | `logs/logging-focused.log` |
| HTTP success/denied/failure/panic/unmatched paths | Status, log level, route template and safe request ID verified | `logs/logging-focused.log` |
| Safe startup errors and hidden runtime errors | Race/shuffle CLI tests pass | `logs/cli-focused.log` |
| Complete Go build/vet/tests against real database | Exit 0 | `logs/go-verify.log` |
| Existing frontend tests/typecheck/build | 9 tests, typecheck and build pass | `logs/web-tests.log`, `logs/web-typecheck.log`, `logs/web-build.log` |
| Contracts and architecture manifests | 53 operations / 244 fixtures; 55 requirements validated | `logs/contracts.log`, `logs/manifests.log` |

No UI behavior was changed. File output is JSON with bounded rotation, protected
permissions and safe error classification. CLI operations use stderr only.
Real WeChat, physical devices, public TLS, existing-volume Timescale upgrade and
capacity acceptance are not covered by this milestone (`external_blocked` where applicable).
R36/R37 M6b remains pending independently.
