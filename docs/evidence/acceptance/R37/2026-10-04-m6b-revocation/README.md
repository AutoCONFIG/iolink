# M6b live authorization and fake executor verification

Requirement scope: R36.a/R37.a, including live role and membership revocation,
tenant disable, platform authority change, and fake executor context invalidation.
Final source commit: `7ea4790cadb319ff734b80e044c4326a19ee8757`.
Web submodule: `c54237e9d1a424bc9cb0b3f700014e2e966cf2cc`.

The run used the dedicated Docker PostgreSQL/Timescale instance with its DSN
provided only through the process environment. The receipt contains no password
or DSN. Environment versions are recorded in `logs/verification.json`.

Run from this repository root:

```sh
node docs/evidence/acceptance/R37/2026-10-04-m6b-revocation/verify.mjs
```

The harness runs the frontend unit/typecheck/build/E2E checks, documentation and
architecture checks, full Go verification, contracts, the M6b focused suite, and
the new revocation/fake-executor suite. It writes complete command output and
exit codes to `logs/` and refuses a focused run containing skips.

| Scenario | Result | Evidence |
|---|---|---|
| Full Go build, vet, tests with real isolated DB | PASS | `logs/go-verify.log` |
| M6b tenant/RBAC focused scenarios | PASS, no skipped focused checks | `logs/m6b-focused.log` |
| Farm update after committed role, membership, tenant, or authority revocation | PASS: forbidden and unchanged row in all four cases | `logs/m6b-revocation.log` |
| Tenant member update after the same four revocations | PASS: forbidden and unchanged membership in all four cases | `logs/m6b-revocation.log` |
| Fake executor scope propagation | PASS: tenant, actor and role reach deferred callback | `internal/core/fake_executor_m6b_test.go`, `logs/m6b-revocation.log` |
| Fake executor invalidation | PASS: committed role revocation prevents callback side effect | `internal/core/fake_executor_m6b_test.go`, `logs/m6b-revocation.log` |
| Frontend unit/typecheck/build and browser scenarios | PASS | `logs/web-unit.log`, `logs/web-typecheck.log`, `logs/web-build.log`, `logs/web-e2e.log` |
| Contract and architecture checks | PASS: 53 operations / 244 fixtures and 55 requirements | `logs/contracts.log`, `logs/architecture.log` |
| Formatting and diff check | PASS | `logs/diff-check.log` |

The shared mutation boundary is `internal/core/tenant_write_auth.go`.
Transaction-backed mutations validate the live membership, tenant state, user
authority, claimed role and policy immediately before the write. Existing pool
mutation methods use the same live check before their write path. Calls without
an M6b tenant context retain the pre-existing M0-M5 behavior.

This evidence does not claim future Key/video/command/job/report resources or
external hardware, WeChat, public TLS, or capacity acceptance.
