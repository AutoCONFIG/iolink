# M6d Open API evidence

This directory records the M6d software gate for R41 and R42. The release
snapshot must include the source and web revisions named below.

## Covered behavior

- tenant-scoped API key issue, list, rotation and immediate revocation;
- `openapi` License feature fail-closed on key administration and signed requests;
- one-time 32-byte secret response with AES-GCM instance-key encryption at rest;
- RFC3986 query/path canonicalization and the fixed vector in
  `docs/api/open-openapi.yaml`;
- HMAC-SHA256 verification, timestamp window, atomic nonce replay protection,
  body tamper rejection and per-key token-bucket rate limiting;
- tenant, scope and resource-set enforcement on `/open/v1/ponds`,
  `/open/v1/devices` and `/open/v1/alarms`;
- administrator UI states for issue, list, rotate, revoke and one-time secret display.
- real HTTP handler response and post-restart key authentication;
- concurrent nonce winner/replay loser and deterministic burst/refill rate-limit checks.

## Verification commands

```text
go test ./internal/core ./internal/openapi ./internal/adminapi ./cmd/iolinkd -count=1
IOLINK_TEST_PG_DSN="$IOLINK_TEST_PG_DSN" go test ./internal/core -run 'TestM6d' -count=1
IOLINK_TEST_PG_DSN="$IOLINK_TEST_PG_DSN" go test -race -shuffle=on ./internal/core ./internal/openapi ./internal/adminapi ./internal/migrate ./internal/persistence ./internal/authorization -count=1
make verify
npm test --prefix web -- --run
npm run build --prefix web
make verify-contracts
make docs-tools
python3 scripts/check_architecture_manifests.py --all
npm test --prefix web -- --run
npm run typecheck --prefix web
npm run build --prefix web
npm run e2e --prefix web -- --reporter=line
docker build -t iolink:m6d-local .
```

The real Timescale tests used the isolated `iolink-todo9-pg` container
(`timescale/timescaledb:latest-pg16`) on host port `55439`; its password is
intentionally omitted from this record. The focused M6d run completed in the
disposable database created by `internal/testdb`.

The focused M6d tests and the full race/shuffle suite passed on 2026-10-06.
`make verify`, `make verify-contracts`, `make docs-tools`, the architecture
manifest check, the web unit/type/build checks, and `docker build -t
iolink:m6d-local .` also passed. The stage remains pending until two
independent reviewers approve the same release snapshot and the acceptance
manifest is updated.

The existing admin browser suite completed 16/16 scenarios. It exercises the
system page in the demo browser surface; the signed Open API HTTP path is
covered by the real Timescale test above.

## External status

Issuer, customer-host, ARM64, WeChat and physical-device evidence are not
needed for this software-only stage. Any later external provider checks remain
`external_blocked` until their credentials or hardware are available.
