# Existing Key / Open / device-restore HTTP regression

Date: 2026-10-14. Scope: R02.c real response contracts, R36.b/R37.b resource ownership and membership revocation, R41–R42 existing API Key/Open behavior, and the existing R38–R39 device-restore operation. This is a bounded existing-function regression, not License delivery acceptance or a new feature milestone. Independent review and release are owned by the root agent.

## Environment and invocation

- Dedicated root-owned Timescale/PostgreSQL endpoint: `127.0.0.1:33127`, database/user `iolink`; disposable test databases are created and dropped by `testdb`.
- `IOLINK_TEST_PG_DSN` was populated for every real-PG run. The disposable password is omitted here; no deployment credentials are used.
- `IOLINK_CONTRACT_PYTHON=$PWD/.venv/contracts/bin/python`, `GIN_MODE=release`.
- Go/Python versions: [go-version.log](go-version.log), [python-version.log](python-version.log).
- Tested source hashes: [source-sha256.log](source-sha256.log). Root subsequently freezes the shared release candidate; no staging or release was performed by this worker.

Final commands (the populated test DSN is an environment variable):

```sh
IOLINK_CONTRACT_PYTHON="$PWD/.venv/contracts/bin/python" GIN_MODE=release \
  go test -race -shuffle=on -count=1 \
  -run '^Test(KeyOpenContract|M6d(Issue|Concurrent|BodyDigest|Restart|Throttle|PublishedVector|ConfiguredRate|ResourceHTTP|DeviceResourceHTTP|SignedHTTP|Administration|ResourceValidation|Rotation|TimestampWindow))' \
  -v ./internal/core
GIN_MODE=release go test -race -shuffle=on -count=1 ./internal/adminapi ./internal/openapi
make verify-contracts
```

The first command requires the populated dedicated `IOLINK_TEST_PG_DSN`. Actual HTTP requests use TCP `httptest.Server`, current authorization/core services, migrations, real PostgreSQL, and existing local License/vault prerequisites. Bodies and secrets stay in memory or validator stdin; test errors contain only method, route template, status and schema issue keywords. No browser, real License signing service, external WeChat, or hardware checks were run.

## Physical coverage and binary observables

Eleven unique physical operations: five Key management operations, five signed Open reads, and one restore operation. Restore is checked against both admin and license contracts, giving twelve spec-operation checks without double-counting the HTTP operation.

| Scenario / test | Observable asserted | Artifact |
|---|---|---|
| `ManagementAndFiveSignedReads` | Current schemas accept real GET/POST keys, rotate, revoke, audit and five Open read responses; list/audit omit secret; resource sets filter actual rows; rotated/revoked credentials stop authenticating | `final-real-pg.log` |
| `InvalidIssuanceHasNoKeyOrAuditWrites` | Twelve invalid/foreign body cases return declared 400/404 and preserve key/audit/device digest | `final-real-pg.log` |
| `ForeignManagementAndMembershipRevocation` | Foreign rotate/revoke return 404 with unchanged state; deactivated membership invalidates the existing token immediately with 401 | `final-real-pg.log` |
| `ScopeOwnershipReplayAndRate` | Hidden/foreign resources return 404; nonce replay returns 401 without another reservation; exhausted token bucket returns 429 and preserves consumed-nonce behavior | `final-real-pg.log` |
| `SignedInputBoundsAndNonceState` | Zero/negative/noninteger pond IDs return 400; malformed encoded signed queries return 401 without consuming nonce | `final-real-pg.log` |
| `DatabaseFailuresAreInternalAtomicAndDiagnosed` | Thirteen real relation faults yield 500, declared response shapes, unchanged key/audit/device state after rollback, and matching request-ID ERROR records with `postgres_42P01`; no test credentials in dump | `final-real-pg.log` |
| `MissingVaultFailsUnavailableWithoutWrites` | Missing vault root gives 503 on issuance and all five signed Open surfaces; no partial business writes; every request has constant safe capability diagnostic without invented error | `final-real-pg.log` |
| `MissingKeyManagementPortRejectsWithoutWrites` | All five management surfaces with absent real management dependency return 503, unchanged state, safe diagnostics | `final-real-pg.log` |
| `RestoreOnePhysicalOperationAndIdempotence` | First and repeated restore return actual 204 accepted by both specs; device disabled state is cleared | `final-real-pg.log` |
| `RestoreRejectsForeignMissingAndReadOnly` | Foreign/missing restore returns actual 404, fresh viewer token returns actual 403; both specs accept responses and state is unchanged | `final-real-pg.log` |
| `RestoreLateStorageFailureRollsBackAndDiagnoses` | Renamed shadow relation causes actual 500 after the device UPDATE; disabled/session/key/audit digest remains unchanged and correlated safe SQL diagnostic exists; both specs validate the actual status/header/body | `final-real-pg.log` |
| Existing non-browser M6d regressions | Published signature vector, body digest, restart nonce/rate persistence, timestamp limits, concurrent nonce/rate, role/resource guards and configuration remain passing | `final-real-pg.log` |
| All admin/Open package tests | Exit 0, race-enabled, shuffled; admin 23.714s / Open 1.150s | `final-http-unit.log` |
| Static contract checks | Exit 0, 96 target operations / 621 synthetic fixtures; these do not count as additional live handler coverage | `final-contracts.log` |

Open currently declares no numeric pagination query parameters. This work tests the actual signature query boundary; it does not invent undocumented limit/page behavior. Device relation faults may occur during feature/authentication usage lookup before the resource query, so their evidence establishes safe surface classification rather than an assumed internal phase.

## Proven defects and smallest changes

- `real-before.log`: duplicate/case-variant issuance keys could be accepted with writes; boundary error values disagreed with current contract; invalid pond IDs returned 404. Strict exact-key JSON parsing now rejects malformed/unknown/duplicate/trailing input before issuance, boundary responses use the current enum, and invalid IDs return 400.
- `auth-sql-before.log` / `database-before.log`: three independently probed SQLSTATE 42P01 faults in Open authentication were incorrectly returned as 401; resource/management SQL failures lacked correlated provider diagnostics. Root introduced typed authentication/replay errors while preserving genuine SQL errors; the HTTP adapters now distinguish known auth errors from actual 500 provider failures and emit safe injected diagnostics.
- `vault-before.log`: absent vault capability was incorrectly classified as issuance 500/Open 401. Root introduced a capability sentinel; adapters now return safe 503 without partial writes or manufactured errors.
- `all-surface-status-before-schema.log`: actual safe 500/503 behavior was verified on each applicable surface before adding the missing response declarations. Open 500/503 schemas constrain the public error enum.
- `restore-actual-status-before-schema.log`: both validators inspected actual 500 response data and rejected absent response declarations. Admin/license restore now declare the existing provider-failure response; no restore business behavior was added.

## Historical experiment corrections

- `positive-before.log` is a fixture setup error (multiple parameterized SQL statements), not a product finding; the fixture was corrected before real coverage.
- `restore-before.log` renamed `audit_events` on the mistaken assumption restore writes an audit event. The actual 204 was correct for the existing implementation; no audit behavior was added. The fault was replaced with the real late `device_shadows` deletion dependency.
- `restore-shadow-before-schema.log` predates the correction that the secondary license validator must use the captured actual response status. The authoritative repeat is `restore-actual-status-before-schema.log`; the final test validates both specs using actual status, Content-Type and bytes.
- Earlier `*-before` and intermediate `*-after` files are RED/reproduction evidence, not release passes. Final success is determined only by the final commands and their complete outputs.

## Status

Source changes are frozen. The final real-PG command exited 0 (`PASS`, core 83.409s), including all eleven new test functions and the selected existing non-browser M6d regressions, with no failed or skipped cases. Package and static-contract commands also exited 0. Root is responsible for the full shared-candidate gate and two independent reviews before recording milestone completion.
