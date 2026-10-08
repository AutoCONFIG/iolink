# User API split verification

Candidate scope: platform administration remains on `/admin/v1`; ordinary business users use `/user/v1`; the existing `/admin/v1` business paths remain available for compatibility.

## Snapshot

- Root implementation commits: `c38f881` and `1606bb9`.
- Web submodule: `953335e` (`feat: route business console through user api`), pushed to `github.com:AutoCONFIG/iolink-webui` `main`.
- The `.tmp/` directory is pre-existing and is not part of this evidence.

## Checks

| Surface | Command | Result |
|---|---|---|
| Go route and authorization regression | `go test -race -shuffle=on ./internal/adminapi ./cmd/iolinkd -count=1` | PASS |
| Go full unit suite | `go test ./...` | PASS |
| Real PostgreSQL/Timescale console flow | `IOLINK_TEST_PG_DSN=<redacted disposable Timescale DSN> go test -race -shuffle=on ./internal/core -run 'TestPlatformConsoleOnboardingAndIsolation\|TestCLISetupCreatesOnlyPlatformAdmin' -count=1` | PASS |
| Web behavior | `npm test -- --run` in `web/` | 58 tests PASS |
| Web types and build | `npm run typecheck && npm run build` in `web/` | PASS |
| Static API contracts | `make verify-contracts` | 74 operations / 366 fixtures PASS |
| Unknown API fallback | `TestUnknownAPIDoesNotServeSPA` | `/user/v1missing` returns JSON 404 |

## Covered behavior

- `/user/v1/farms` and `/user/v1/tenants` work for a tenant user with an active membership.
- `/user/v1/license`, `/user/v1/platform/stats`, `/user/v1/platform/users`, and `/user/v1/tenants/{id}/status` are not mounted.
- A user without tenant context receives the tenant-context failure on `/user/v1/stats`.
- A platform administrator cannot use `/user/v1` business routes.
- Platform administrator routes remain available under `/admin/v1`.
- The web client uses `/user/v1` for business data and `/admin/v1` for platform controls; registration uses `/user/v1/register` while login keeps the compatible `/admin/v1/login` entry.

External browser visual acceptance remains with the user. No external WeChat, hardware, or public TLS result is claimed by this software verification.
