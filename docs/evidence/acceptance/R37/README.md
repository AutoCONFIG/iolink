# R37 M6b RBAC and revocation evidence

Date: 2026-10-01 (latest verification; earlier transcripts retained)

Environment: dedicated Docker container `iolink-todo9-pg`; PostgreSQL/Timescale DSN used verbatim below.

## Executed checks and captured output

Raw command transcript: [`commands.txt`](commands.txt).
Latest raw outputs: [full Go verification](2026-10-01-go.txt), [real HTTP and database role scenarios](2026-10-01-http.txt), and [contract checks](2026-10-01-contracts.txt).
Review corrections and expired verdicts are recorded in [the review history](review-history.md).

```text
$ IOLINK_TEST_PG_DSN=postgres://iolink:iolink-test-only@127.0.0.1:55439/iolink?sslmode=disable go test -race -shuffle=on ./internal/core ./internal/access ./internal/migrate ./internal/persistence ./internal/adminapi ./internal/appapi -count=1
PASS (owner/admin/member/viewer/support expiry, transaction audit, cross-tenant alarm and revocation assertions)

$ make verify-contracts
PASS (53 target operations, 244 synthetic request/response fixtures)

$ python3 scripts/check_architecture_manifests.py --all
PASS (55 requirements, owners, providers and references)

$ cd web && npm run build && npm run e2e; npm test
PASS (production build; 7 Playwright scenarios; 9 unit tests)
```

## Role and operation matrix

| Role | Read same-tenant data | Confirm alarm | Mutate resources/members | Support expiry |
|---|---:|---:|---:|---:|
| owner | yes | yes | yes | n/a |
| admin | yes | yes | yes | n/a |
| member/operator | assigned farms | yes | no | n/a |
| viewer | assigned farms | no (403) | no (403) | n/a |
| support | explicitly assigned farms | yes while active | no member changes | required future `expires_at` |

Tenant admins use the organization/member page at `/tenants`; platform admins without tenant membership are restricted to tenant lifecycle routes and cannot enter tenant business-resource routes. Membership and lifecycle updates increment permission versions, write audit events in the same transaction, and fail closed on database errors.

The 2026-09-30 review correction adds an in-memory Casbin role/resource/action policy at the admin HTTP boundary. Core farm-member listing independently checks that policy, the caller identity, and a live owner/admin membership before returning rows. A real PostgreSQL test rejects support access to both assigned and unassigned farm membership lists, missing actor identity, and a forged role. Policy tests reject viewer confirmation, support membership reads, platform business reads, and unknown resources.

The 2026-10-01 correction classifies batch confirmation as the alarm-confirm action. Member/support batches use live tenant and farm assignments in the same transaction; any hidden alarm rejects the whole batch without changing visible alarms. Deterministic alarm row locking preserves duplicate/concurrent idempotency. Real HTTP tests cover member/viewer/support reads, hidden-device 404, membership-list 403, cross-farm atomic rejection, successful confirmation, and repeat/concurrent confirmation. Direct core batch calls with restricted actor context use the same farm authorization.

Production mux regression verifies `/api/v1` and all three existing `/api/v2` routes reach the application authentication middleware. The V2 handler's scoped database behavior is covered by the existing telemetry integration tests.

Farm creation preserves a tenant member role; farm ownership is represented by the farm membership and does not grant tenant-wide administration.

Future Key, video, command, job and report role checks remain assigned to their M6d/M7/M8 acceptance rows.

## Run metadata

- Requirement/version: R37.a, M6b RBAC and revocation, base `14bec065559a187571f9741dd2eaf812c8f48a41` plus the uncommitted final working tree under review.
- Dependencies: Go 1.26.8; PostgreSQL/Timescale image `timescale/timescaledb@sha256:75d58b53f3337a6babd3a1cce5e503f2300bdede62dd7d743651deb1a95b6d76`; Node v24.21.0; npm 11.19.0; Playwright 1.63.0.
- Test data: owner/admin/member/viewer/support roles, future and missing support expiry, inactive tenant/membership, cross-tenant alarm, farm reassignment, and failed authorization paths.
- Browser artifacts: tenant organization/member page screenshots emitted under `.omo/evidence/` by `web/e2e/admin-pages.spec.ts`; build and seven Playwright scenarios passed.
- Review state: software gate passed on 2026-10-01. Two independent read-only reviewers approved the same final worktree snapshot; future R37.b resources and external inputs remain deferred by the acceptance matrix.
