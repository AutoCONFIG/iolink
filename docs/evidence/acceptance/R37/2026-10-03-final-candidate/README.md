# M6b final candidate verification receipt

This receipt records the assembled M6b candidate before independent review. It is
not a gate approval by itself.

- Source candidate: `fd1f2ab7dd9852a158b26a280c49c6fc284f426b`
- Web submodule: `e84092326d9979e066c611e2be8eb6b9736e09f3`
- Date: 2026-10-03 Asia/Shanghai
- Database: dedicated Docker container `iolink-todo9-pg`, PostgreSQL 16.15,
  TimescaleDB 2.30.0; DSN supplied through `IOLINK_TEST_PG_DSN` and redacted
- Raw local artifacts: `.omo/evidence/m6b-final/` in the candidate worktree

## Verification

| Surface | Command | Result |
|---|---|---|
| Go build, vet, race and shuffle suite | `make verify` | exit 0 |
| M6b real database regression | `go test -race -shuffle=on -count=1 -v ./internal/core ./internal/migrate ./internal/appapi ./internal/adminapi -run 'Test(M6b|AppResource|AppTelemetry|AppAlarm|TelemetrySubmission|TelemetryHTTP|LegacyTenant|Platform|ProductionMux)'` | exit 0, no skipped tests |
| Contract fixtures | `make verify-contracts` | 53 operations, 245 fixtures, exit 0 |
| Architecture manifest | `.venv/contracts/bin/python scripts/check_architecture_manifests.py --all` | 55 requirements, exit 0 |
| Frontend unit tests | `npm test --prefix web` | 21 tests passed |
| Frontend typecheck/build | `npm run typecheck --prefix web; npm run build --prefix web` | exit 0 |
| Browser E2E | `npm run e2e --prefix web -- --config /tmp/m6b-final-playwright.config.mjs` | 16 tests passed |
| Git hygiene | `git diff --check` | exit 0 |

The browser suite covers Asia/Shanghai, UTC and America/New_York displays,
unchanged saves with whole and fractional seconds, edited UTC conversion and
reload. The focused Go suite covers owner/admin tenant-wide scope, assigned
lower-role scope, platform-admin exclusion, stale-token rejection, cross-tenant
404s, live role checks, singular and batch alarm confirmation, and atomic
no-partial-write behavior.

The M6b gate remains `pending` until two independent reviewers explicitly
approve this exact source snapshot.
