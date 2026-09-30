# R37 platform-admin owner backfill regression (2026-10-01)

Scope: M6b R37.a / EXTENSIONS platform-admin boundary. Exact release source snapshot: `1c9f1eaf95053f9409cfd784f5481cdddb7f1c12`; web submodule: `c54237e9d1a424bc9cb0b3f700014e2e966cf2cc`.

The pre-fix regression is captured in `red-migration-http.log` and `red-core.log`: a legacy `ADMIN` farm owner was backfilled as an ordinary tenant/farm owner, admin login selected that tenant, and `GET /admin/v1/farms` returned 200. Stored ordinary ADMIN memberships and owner assignment also bypassed the boundary.

The green run uses the dedicated test-only PostgreSQL/Timescale DSN (credentials redacted here), real embedded migrations, core service, admin HTTP server, and `-race -shuffle=on`. It verifies:

- USER farm owners receive owner memberships and retain reads.
- ADMIN farm ownership remains in place for data preservation, is recorded in reconciliation, and creates no tenant/farm membership.
- ADMIN login has no tenant business context; tenant resource reads return 403.
- Legacy ordinary ADMIN memberships fail closed for default tenant, role/version, list-tenants, old JWT, and fresh login.
- Explicit time-limited support plus farm assignment works; expiry and revocation reject requests.
- ADMIN cannot become farm owner through scoped or unscoped `SetFarmOwnerByActor`; `EnsureUser` does not create an ordinary membership for an ADMIN identity.

The migration/HTTP and core red/green logs are committed in this directory: `red-migration-http.log`, `red-core.log`, `green-regressions.log`, `full-relevant-race.log`, `verify-contracts.log`, `docs-tools.log`, and `architecture.log`. Their source evidence was captured under `.omo/evidence/m6b-platform-owner-fix/`. Hardware, external WeChat, and production migration acceptance remain `external_blocked`.
