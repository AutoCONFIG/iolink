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

Implementation follow-up commit: `abccf0a1392ff31786ce1137588a731df4572eff`.
It maps device registration License, quota, and verification-capability failures to the
stable HTTP contract (`license_required`, `device_quota_exceeded`, and
`license_unavailable`). The follow-up focused test and the full verification suite passed.

## External and deployment limits

The checks use local images and a dedicated test-only database. They do not claim a clean
customer host installation, external License issuer, ARM64 support, real WeChat credentials,
or physical hardware acceptance. Those remain `external_blocked` until supplied evidence exists.

## Final offline delivery candidate

Functional implementation SHA: `2f2aab627f39c9474a35e2ef39e06d9115b810d1`

| Scenario | Exact command / action | Observed result | Artifact |
|---|---|---|---|
| Full repository verification | `TMPDIR="$PWD/.tmp" make verify` | PASS: build, vet, race, shuffled tests | command output in task run |
| Contract verification | `make verify-contracts` | PASS: 56 operations, 276 fixtures | command output in task run |
| Real Timescale integration | `IOLINK_TEST_PG_DSN=postgres://iolink_test:...@127.0.0.1:33778/iolink_test?sslmode=disable go test -race -shuffle=on -count=1 ./internal/migrate ./internal/core ./internal/notifications ./internal/persistence ./internal/platform ./internal/wechat` | PASS: all six packages | dedicated TimescaleDB container `iolink-m6c-pg-20261004` |
| Runtime image | `docker build -t local/iolinkd:m6c-test-v2 .` | PASS: frontend build, embedded assets, `iolinkd` and `/app/mqtt-sim` binaries | local image manifest |
| Offline bundle | `IOLINKD_IMAGE=local/iolinkd:m6c-test-v2 IOLINK_DB_IMAGE=timescale/timescaledb:latest-pg16 sh scripts/build-offline-bundle.sh .tmp/iolink-offline-v3` | PASS: image manifests, IDs, migrations, frontend, SBOM, license/dependency lists, optional configs and checksums | `.tmp/iolink-offline-v3/metadata/manifest.json`, `.tmp/iolink-offline-v3/SHA256SUMS` |
| License-gated offline install | `IOLINK_SETUP_INPUT=.tmp/setup-input.json sh .tmp/iolink-offline-v3/install.sh` on a fresh compose volume without issuer input | PASS: checksum verification, docker load, migrate, setup init/status, then exit 3 before serving with explicit License requirement | `/tmp/iolink-offline-v3-install.log` |
| Simulated report | seeded an isolated smoke device, then ran `/app/mqtt-sim` for 8 seconds; queried `sensor_data` | PASS: 8 telemetry rows persisted | `/tmp/iolink-mqtt-sim-v2.log` |
| Runtime simulator availability | `docker compose ... exec -T iolinkd /app/mqtt-sim --help` | PASS, exit 0 | `/tmp/iolink-mqtt-sim-help.txt` |
| M6c lifecycle regression | `IOLINK_TEST_PG_DSN=postgres://iolink_test:<redacted>@127.0.0.1:33778/iolink_test?sslmode=disable go test -race -shuffle=on -count=1 ./internal/migrate ./internal/core ./internal/notifications ./internal/persistence ./internal/platform ./internal/wechat` | PASS: all six packages; restore clears disabled state and shadow after quota/license admission | TimescaleDB container `iolink-m6c-pg-20261004` |
| Web submodule | `npm test --prefix web` and `npm run build --prefix web` | PASS: 21 tests; production build embeds license and restore controls | web commit `be8a6810fce45c68b40e68a29c7fbafdffa0805e` |
| Final repository gate | `TMPDIR="$PWD/.tmp" make verify`; `make docs-tools`; `make verify-contracts` | PASS: full Go race/shuffle suite, docs checks, 56 operations / 276 fixtures | command output in task run |
| Runtime image assets | `docker build -t local/iolinkd:m6c-final .` | PASS: frontend build, embedded assets, `/app/iolinkd`, `/app/mqtt-sim` | local image `local/iolinkd:m6c-final` |

The successful install used a fresh named volume after the earlier test volume was removed;
the service was stopped and the test volume removed after the run. No external issuer,
ARM64 host, physical device, or WeChat credential was available, so those remain
`external_blocked`.
