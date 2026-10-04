# M6c final manual QA

Reviewed implementation tree: `1df1d6b6e301271b7ea1fa60857310795a89edc6`.

| # | Scenario | Command | Expected | Observed | Verdict |
|---|---|---|---|---|---|
| 1 | License admin and fail-closed admission | `go test ./internal/adminapi ./internal/core -count=1` | API and device admission tests pass | Both packages passed; missing verification capability returns `license.ErrUnavailable` | PASS |
| 1a | Device admission HTTP error contract | `go test ./internal/adminapi -run 'TestRegisterDeviceMapsLicenseErrors' -count=1` | License, quota and capability errors map to stable HTTP responses | 403 `license_required`, 403 `device_quota_exceeded`, 503 `license_unavailable` all passed | PASS |
| 1b | Device restore HTTP error contract | `go test ./internal/adminapi -run 'TestRestoreDeviceMapsLicenseErrors' -count=1` | Success is 204; missing device is 404; license/quota/capability errors retain stable codes | Success, 404 `not_found`, 403 `license_required`, 403 `device_quota_exceeded`, and 503 `license_unavailable` all passed | PASS |
| 2 | Full Go regression | `TMPDIR=$PWD/.tmp make verify` | Build, vet, race and shuffled tests pass | All packages passed | PASS |
| 3 | Real database path | `IOLINK_TEST_PG_DSN=<isolated Timescale DSN> go test -race -shuffle=on -count=1 ./internal/migrate ./internal/core ./internal/notifications ./internal/persistence ./internal/platform ./internal/wechat` | Migration, quota, authorization, restore, notification, persistence, platform and WeChat adapter tests pass | All six packages passed on the isolated TimescaleDB 2.30.2/PostgreSQL 16.15 container; restore clears disabled state and shadow | PASS |
| 4 | API contract surface | `make verify-contracts` | OpenAPI fixtures remain valid | 56 operations and 276 fixtures passed | PASS |
| 5 | Deployment configuration | `IOLINK_PG_PASSWORD=test-password IOLINK_SECRET_KEY=<32-byte-test-key> docker compose -f deploy/docker-compose.yaml config`; offline config with `IOLINK_DB_IMAGE` and `IOLINKD_IMAGE`; `sh -n scripts/build-offline-bundle.sh scripts/install-offline.sh` | Both Compose files resolve and scripts parse | Production and offline Compose resolved; shell syntax passed; offline network remains internal and uses `pull_policy: never` | PASS |
| 6 | Integrated frontend boundary | `test -f internal/web/dist/index.html; rg '^//go:embed all:dist' internal/web/embed.go; rg 'web-mini' docs/DEPLOY.md .gitmodules` | `web/` is embedded in iolinkd; web-mini is a separate build input | Embedded `internal/web/dist` exists and Dockerfile builds `web/dist`; docs keep `web-mini` separate | PASS |
| 7 | Runtime image contents | `docker build -t local/iolinkd:m6c-final .`; container checks for `/app/iolinkd`, `/app/mqtt-sim` and `mqtt-sim --help` | One integrated runtime image contains backend, embedded web build and simulator | Docker build passed; frontend build ran in image and both binaries are executable | PASS |

External issuer credentials, real WeChat login/subscription, physical devices, ARM64, and clean-customer-host installation remain `external_blocked`.
