# Manual QA: platform and user API split

QA snapshot: root `2e7205d` before this evidence commit; web submodule `953335e`.

| # | Scenario | Command or action | Expected | Observed | Verdict |
|---|---|---|---|---|---|
| 1 | Ordinary user business API | `go test -race -shuffle=on ./internal/adminapi ./cmd/iolinkd -count=1` | User route, tenant context and role checks pass | Go route/auth regression passed | PASS |
| 2 | Platform controls remain available | Existing admin route tests in the same command | `/admin/v1` login and platform handlers continue to work | Focused suite passed | PASS |
| 3 | Platform account on user API | `TestPlatformAdminCannotUseUserBusinessRoutes` | `/user/v1/login` rejects platform account | HTTP 403; no user token issued | PASS |
| 4 | User without tenant | `TestUserRoutesRequireTenantContext` | Business data is denied without tenant context | HTTP 403 | PASS |
| 5 | Real database onboarding and isolation | `IOLINK_TEST_PG_DSN=<redacted disposable Timescale DSN> go test -race -shuffle=on ./internal/core -run 'TestPlatformConsoleOnboardingAndIsolation\|TestCLISetupCreatesOnlyPlatformAdmin' -count=1` | No implicit tenant; assigned user sees only owned tenant resources | Both tests passed against isolated PG16/Timescale | PASS |
| 6 | Unknown user API does not fall through to SPA | `TestUnknownAPIDoesNotServeSPA` | JSON 404 | `/user/v1missing` returned JSON 404 | PASS |
| 7 | Frontend request split | `npm test -- --run`, `npm run typecheck`, `npm run build` in `web/` | Business requests use `/user/v1`; platform requests use `/admin/v1` | 58 tests, typecheck and build passed | PASS |
| 8 | Contract regression | `make verify-contracts` | Existing target contracts remain valid | 74 operations / 366 fixtures passed | PASS |

The browser visual acceptance, real WeChat credentials, hardware and public TLS remain external/user-owned checks and are not claimed here.
