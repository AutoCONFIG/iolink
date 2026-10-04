# M6c License and offline delivery verification

Date: 2026-10-04 (Asia/Shanghai)

## Source changes

The candidate adds strict signed payload and envelope parsing, offline RSA-PSS signing,
clock high-water status reporting, rejection audit records, platform-admin License HTTP
integration, quota concurrency coverage, offline Compose, and checksum-verified image
bundling. The runtime image embeds `web/dist`; `web-mini` remains an external mini-program
build input and is not a server runtime.

## Verification matrix

| Scenario | Command | Result | Evidence |
|---|---|---|---|
| Go build, vet, race and shuffled unit suite | `make verify` | PASS | Command exited 0; all packages passed |
| OpenAPI and fixture contracts | `make verify-contracts` | PASS | 56 operations and 276 fixtures |
| Real Timescale migration/core integration | `IOLINK_TEST_PG_DSN=<isolated DSN> make integration` | PASS | Dedicated TimescaleDB 2.30.2/PostgreSQL 16 container; migration, authorization, quota, notifications, persistence and provider tests passed |
| Concurrent device quota | `go test ./internal/core -run TestRegisterDevice_enforcesLicenseQuotaInOneTransaction` with isolated DSN | PASS | Four concurrent registrations produced two successes and two `ErrQuotaExceeded`; invalid import retained prior License and wrote rejection audit |
| Clock error status | `go test ./internal/core -run TestLicenseStatus_reportsPersistedClockError` with isolated DSN | PASS | Persisted `license_clock.clock_error` returned `state=clock_error` |
| Signer boundaries | `go test ./cmd/license-sign ./internal/license -count=1` | PASS | 45 KiB accepted, 45 KiB+1 rejected, insecure key permissions rejected, signed output reverified; 2048/4096/8192-bit parser coverage |
| Admin License HTTP boundary | `go test ./internal/adminapi -count=1` | PASS | Platform GET/POST, tenant 403, duplicate JSON 400, clock error 409 |
| Compose syntax and offline policy | `docker compose ... config`; `sh -n scripts/*.sh` | PASS | Production and offline files resolve; offline services use `pull_policy: never` and an `internal: true` network |
| Offline bundle and tamper check | `scripts/build-offline-bundle.sh <temp>` then modify `VERSION` and run `install.sh` | PASS | Local image manifests, IDs, tarballs, admin frontend copy, docs, and SHA256SUMS generated; tampering rejected before image load |

## Fail-closed correction

Final candidate commit: `3c2a504383b599e1985d7b7678daf49ac9608986`.

| Scenario | Command | Result | Evidence |
|---|---|---|---|
| Missing runtime verification capability | `go test ./internal/core -run TestRegisterDevice_rejectsWhenLicenseVerificationIsUnavailable -count=1` | PASS | Device registration returns `license.ErrUnavailable` and does not write a device |
| Malformed authorized License upload audit | `go test ./internal/adminapi -run TestLicenseImportMapsClockErrorAndRejectsDuplicateJSON -count=1` | PASS | Duplicate-key request returns 400 and records the exact raw request for rejection auditing |
| Full Go verification after correction | `TMPDIR=$PWD/.tmp make verify` | PASS | Build, vet, race, shuffled tests all passed |
| Real TimescaleDB integration after correction | `IOLINK_TEST_PG_DSN=<isolated DSN> go test ./internal/migrate ./internal/core ./internal/notifications ./internal/persistence ./internal/wechat -count=1 -v` | PASS | All five packages passed against TimescaleDB 2.30.2/PostgreSQL 16.15 |
| Contract verification after correction | `make verify-contracts` | PASS | 56 target operations and 276 fixtures passed |

## External and deployment limits

The checks use local images and a dedicated test-only database. They do not claim a clean
customer host installation, external License issuer, ARM64 support, real WeChat credentials,
or physical hardware acceptance. Those remain `external_blocked` until supplied evidence exists.
