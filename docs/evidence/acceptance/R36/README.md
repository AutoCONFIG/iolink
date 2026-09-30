# R36 M6b tenant isolation evidence

Date: 2026-10-01 (latest verification; earlier transcripts retained)

Environment: dedicated Docker container `iolink-todo9-pg`; PostgreSQL/Timescale DSN used verbatim below. No production data or external service credentials were used.

## Executed checks and captured output

Raw command transcript: [`commands.txt`](commands.txt).

```text
$ IOLINK_TEST_PG_DSN=postgres://iolink:iolink-test-only@127.0.0.1:55439/iolink?sslmode=disable go test -race -shuffle=on ./internal/core ./internal/access ./internal/migrate ./internal/persistence ./internal/adminapi ./internal/appapi -count=1
PASS (all six packages; Docker PostgreSQL/Timescale)

$ make verify
PASS (go build, go vet, go test -race -shuffle=on ./...)

$ make docs-tools
PASS (documentation tooling environment)

$ make verify-contracts
PASS (53 target operations, 244 synthetic request/response fixtures)

$ python3 scripts/check_architecture_manifests.py --all
PASS (55 requirements, owners, providers and references)

$ cd web && npm run build && npm run e2e; npm test
PASS (production build; 7 Playwright scenarios; 9 unit tests)
```

## Covered boundaries

- Active membership selects one tenant; a second tenant's farms, ponds, devices and telemetry are excluded.
- Farm assignments use active `farm_memberships`; owners remain visible and assigned members are visible only in the same tenant.
- Missing tenant claims are rejected for users with active memberships; membership version changes invalidate old tokens.
- Users with only inactive or expired memberships receive no unscoped fallback token and are rejected at login/authentication.
- Inactive tenants reject device authentication, status changes, telemetry ingestion and generic V2 submission.
- Cross-tenant confirmed alarms return not-found and do not report a successful confirmation.
- Legacy farms, sensor rows, shadows and audit rows with NULL tenant ownership are backfilled to the default tenant during migration.
- `TestLegacyTenantReconciliationAndRollback` verifies the complete migration chain, forced migration failure rollback, reconciliation rows, and generated legacy owner membership.
- `TestM6bAdminHTTPAssignedFarmReadsAndBatchConfirmation` uses actual admin login, HTTP handlers and TimescaleDB with two farms in one tenant. Member/viewer/support receive only the assigned farm, pond, device, alarm and statistics; hidden device and product assignment reads return 404. The executed test output is in [the HTTP transcript](../R37/2026-10-01-http.txt).

External hardware, public MQTT exposure and production migration are not claimed by this software evidence.

## Run metadata

- Requirement/version: R36.a, M6b tenant isolation, exact release source commit `1c9f1eaf95053f9409cfd784f5481cdddb7f1c12`; web submodule `c54237e9d1a424bc9cb0b3f700014e2e966cf2cc`.
- Dependencies: Go 1.26.8; PostgreSQL/Timescale image `timescale/timescaledb@sha256:75d58b53f3337a6babd3a1cce5e503f2300bdede62dd7d743651deb1a95b6d76`; Node v24.21.0; npm 11.19.0; Playwright 1.63.0.
- Test data: isolated migrations create default tenant; M6b fixture uses tenants 601/602, users 601/603/604/605, farms 601/602, farm assignment 603→601, revoked user 605, and cross-tenant alarm/device rows.
- Browser artifacts: `.omo/evidence/todo10-m6a-products-created.png`, `.omo/evidence/todo10-m6a-products-model-draft.png`, `.omo/evidence/todo10-m6a-products-published-assigned.png` and the tenant-page screenshots emitted by `web/e2e/admin-pages.spec.ts`.
- Review state: fixed on source commit `1c9f1eaf95053f9409cfd784f5481cdddb7f1c12` after the platform-admin legacy-owner bug. Earlier approvals are stale/rejected for this exact snapshot; fresh independent double review is pending. Do not mark the R36 software gate passed yet. Future R36.b resources and external inputs remain deferred by the acceptance matrix.
