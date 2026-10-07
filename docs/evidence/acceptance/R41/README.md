# R41 M6d API Key acceptance

Candidate source: `51898df` (web submodule `248f100`).

Environment: Go 1.26, PostgreSQL 16 + TimescaleDB isolated container
`iolink-todo9-pg` on `127.0.0.1:55439`; each test creates and drops a disposable
database through `internal/testdb`.

Executed checks:

```text
IOLINK_TEST_PG_DSN=postgres://... go test ./internal/core -run 'TestM6d' -count=1 PASS
go test ./internal/core ./internal/openapi ./internal/adminapi ./cmd/iolinkd -count=1 PASS
npm test --prefix web -- --run PASS (28 tests)
npm run typecheck --prefix web PASS
npm run build --prefix web PASS
npm run e2e --prefix web -- --reporter=line PASS (16 scenarios)
```

Covered behavior includes one-time encrypted secret display, tenant and License
feature authorization, resource scope submission, rotation, immediate revoke,
audit log listing, and post-restart persistence. No secret or signature material
is returned by the audit endpoint.
