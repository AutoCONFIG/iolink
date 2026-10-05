# M6d Open API evidence

This directory records the M6d software gate for R41 and R42. The release
snapshot must include the source and web revisions named below.

## Covered behavior

- tenant-scoped API key issue, list, rotation and immediate revocation;
- one-time 32-byte secret response with AES-GCM instance-key encryption at rest;
- RFC3986 query/path canonicalization and the fixed vector in
  `docs/api/open-openapi.yaml`;
- HMAC-SHA256 verification, timestamp window, atomic nonce replay protection,
  body tamper rejection and per-key token-bucket rate limiting;
- tenant, scope and resource-set enforcement on `/open/v1/ponds`,
  `/open/v1/devices` and `/open/v1/alarms`;
- administrator UI states for issue, list, rotate, revoke and one-time secret display.

## Verification commands

```text
go test ./internal/core ./internal/openapi ./internal/adminapi ./cmd/iolinkd -count=1
go test -race -shuffle=on ./internal/core ./internal/openapi ./internal/adminapi ./internal/migrate ./internal/persistence ./internal/authorization -count=1
IOLINK_TEST_PG_DSN="$IOLINK_TEST_PG_DSN" go test ./internal/core -run TestM6d -count=1
npm test --prefix web -- --run
npm run build --prefix web
make verify-contracts
make docs-tools
python3 scripts/check_architecture_manifests.py --all
docker build -t iolink:m6d-local .
```

The real Timescale test used the isolated `iolink-m6c-pg-20261004` container
on host port `33778`; its password is intentionally omitted from this record.

## External status

Issuer, customer-host, ARM64, WeChat and physical-device evidence are not
needed for this software-only stage. Any later external provider checks remain
`external_blocked` until their credentials or hardware are available.
