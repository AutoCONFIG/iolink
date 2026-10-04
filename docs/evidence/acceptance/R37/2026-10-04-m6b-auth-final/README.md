# M6b live write authorization final candidate

Historical candidate only: independent review rejected `c56cb22b00712d1cd8b2624af60e7f06636fbf24` for permission-version bypasses, partial alarm context authorization, and missing source-bound full verification artifacts. The PASS claims below are retained as historical prose and do not certify that candidate. The replacement receipt is [version revocation](../2026-10-04-version-revocation/README.md).

This receipt covers the M6b R36.a/R37.a authorization-boundary correction at source `e4608328e31915a7fb3f4d5c78cb2fcf8c0fdf9b`. The checked web submodule is `e84092326d9979e066c611e2be8eb6b9736e09f3`.

## Environment

- Date: 2026-10-04 Asia/Shanghai
- Database: dedicated Docker container `iolink-todo9-pg`, real PostgreSQL 16 with TimescaleDB 2.30.0
- DSN: isolated test DSN supplied through `IOLINK_TEST_PG_DSN`; credentials are not recorded

## Verification

- `make verify`: PASS, including build, vet, race, shuffle, and full Go test suite with the real database
- `make docs-tools`: PASS
- `make verify-contracts`: PASS, 53 operations and 245 fixtures
- architecture manifests: PASS, 55 requirements
- focused revocation run: PASS, race+shuffle real HTTP/Timescale coverage for farm/member writes, product/model create and publish, device assignment, platform authority, membership, tenant, role, and permission-version changes
- fake executor run: PASS, tenant/actor/role propagation and committed role revocation before side effect

The focused command and complete output are in `logs/revocation.log`. Every rejected mutation returned a 4xx response and the product, model, device, shadow, audit, farm, and membership snapshots remained unchanged.
