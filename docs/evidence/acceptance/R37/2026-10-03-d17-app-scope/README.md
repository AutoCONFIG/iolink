# D17 app tenant manager scope regression

Requirement: R36.a/R37.a and the tenant administrator scope in `docs/EXTENSIONS.md`. Status: implemented and locally verified; fresh independent review of the assembled release snapshot is pending. This evidence does not mark the M6b gate passed.

Source commit: `62acecee62160d48efcd48f0e161a927ad583ef6`, based on candidate `ce23acdeb9d450b48649f0279e1f6c3b02afc3f2`. The web gitlink remains `483cb2cecc4a368fdf6fc3406fa682207b514aea`; this fix changes only backend source and tests. Recorded on 2026-10-03 UTC. Environment: Linux amd64, Go 1.26.8, PostgreSQL 16.15, TimescaleDB 2.30.0. The dedicated test DSN is redacted; every database test creates and drops its own isolated database through `internal/testdb`.

The app pond, device, model, history, and alarm operations now share an active database membership predicate. Tenant-wide access requires current tenant and actor context, a matching live owner/admin role, and USER authority. Farm owners or assigned member/viewer/support users retain farm scope. Platform ADMIN identities cannot inherit ordinary memberships; explicit expiring support remains farm limited. Alarm confirmation also rechecks a write-capable live role at the core boundary. Telemetry retains its transactional policy check and atomic telemetry/projection/shadow/alarm/outbox writes.

## Recorded scenarios and binary observables

The final invocation was `IOLINK_TEST_PG_DSN=<dedicated redacted DSN> GOFLAGS=-buildvcs=false go test -race -shuffle=on -count=1 -v ./...`, exit 0. Raw output, HTTP status/body observations, database snapshots and Timescale version receipts are in [go-race-shuffle.log](go-race-shuffle.log). The run contains no skipped tests.

| Scenario | Test invocation selector within the full suite | Required and observed binary result | Artifact |
|---|---|---|---|
| USER owner/admin without farm membership reads the unassigned same-tenant model | `TestAppResourceScopeWhenTenantManagerHasNoFarmAssignment` | Both roles return the model with readable fields, no error | `go-race-shuffle.log` |
| USER owner/admin without farm membership writes telemetry | `TestAppTelemetryWriteWhenTenantManagerHasNoFarmAssignment` | Accepted property; exactly one telemetry, water projection and shadow row | `go-race-shuffle.log` |
| Real app login and HTTP role/resource matrix | `TestAppResourceHTTPWhenRoleDeterminesFarmScope` | Managers list two farms' ponds/devices/alarms; member/viewer/support list one; all allowed read routes return 200; unassigned lower-role and cross-tenant routes return 404; history contains the stored value 25; statistics counts agree with scope | `go-race-shuffle.log` |
| Real app POST after farm assignment revocation | `TestAppTelemetryHTTPWhenManagerFarmAssignmentIsRevoked` | Owner/admin return 202 and each affected data store contains one row; member/viewer/support return 404 and complete persisted snapshots remain identical | `go-race-shuffle.log` |
| Hidden same-tenant alarm confirmation | `TestAppResourceHTTPWhenRoleDeterminesFarmScope` | Owner/admin return 204; member/support return 404; viewer returns 403; denied cases have identical database snapshots | `go-race-shuffle.log` |
| Core viewer or revoked admin alarm write | `TestAppAlarmConfirmDeniedWhenLiveRoleCannotConfirm` | Typed not-found error and identical persisted snapshots | `go-race-shuffle.log` |
| Forged actor, role downgrade, revoked membership, missing actor, platform ADMIN | `TestAppResourceScopeWhenManagerContextIsForged` | Core model read returns typed not-found; no tenant-wide read is granted | `go-race-shuffle.log` |
| Old JWT after role/membership/authority change | `TestAppResourceHTTPWhenManagerTokenLosesAuthority` | Read and telemetry POST return 401; persisted snapshots remain identical | `go-race-shuffle.log` |
| Full Go regression including admin batch confirmation, MQTT and owner transfer | `./... -race -shuffle=on -count=1` | Exit 0, no skipped tests | `go-race-shuffle.log` |

Baseline invocation: `IOLINK_TEST_PG_DSN=<dedicated redacted DSN> go test -race -shuffle=on -count=1 -v ./internal/core -run 'TestApp(ResourceScopeWhenTenantManagerHasNoFarmAssignment|TelemetryWriteWhenTenantManagerHasNoFarmAssignment)$'`. [baseline-red.log](baseline-red.log) records exit 1 on ce23 source with the new requirement tests: both roles returned not-found for both model reads and writes, and no writes persisted. These tests were not changed to pass after the production fix.

## Existing test contradictions corrected

The prior telemetry HTTP test treated the same-tenant unassigned farm as hidden even for tenant owner/admin. It now retains hidden-farm denial for member/viewer/support and cross-tenant denial for every role; the dedicated new HTTP test adds the corresponding owner/admin positive case. The prior admin farm-membership revocation denial contradicted tenant-wide manager access; it is replaced by explicit positive owner/admin and negative limited-role HTTP scenarios. Core revoked membership or mismatched actor now fails at the resource scope with not-found instead of reaching the later write-policy forbidden check; the tests retain exact typed rejection and byte-identical state assertions.

The M2 owner-transfer test previously used tenant role owner to represent a farm owner. Its fixture now uses tenant role member and explicitly selects that tenant after login, preserving its intended before-transfer access and after-transfer 404 assertions. The extra selection is necessary because login also ensures default organization membership. The actual transfer and revoked-token assertions remain intact.

## Other checks and review notes

- `make verify-contracts DOCS_PYTHON=<existing repository contract virtualenv>/bin/python`: exit 0; 53 operations and 245 synthetic fixtures, [contracts.log](contracts.log).
- `<existing repository contract virtualenv>/bin/python scripts/check_architecture_manifests.py`: exit 0; 55 requirements, owners, providers and references, [architecture.log](architecture.log).
- `GOFLAGS=-buildvcs=false go vet ./...` and `git diff --check`: exit 0 with empty stdout/stderr, captured as command result receipts in [verification-results.json](verification-results.json).

Self-review: the new source helper owns app farm-resource scope; inputs remain the existing typed context/IDs and SQL values, with only fixed role literals used in SQL construction. The new helpers each have three parameters and multiple callers, and add no generic interfaces, logging, dependencies or untyped escape hatches. New files measure 41/118/180/37 pure lines. Existing `repos.go` and the M2 integration test remain larger than the skill recommendation; this scoped authorization fix preserves their established layout. Regression tests distinguish the ce23 bug and drive actual HTTP and database outcomes. External WeChat credentials, devices, public MQTT and production migration remain `external_blocked`; the HTTP tests use the existing credential exchanger fixture and do not claim those external acceptances.
