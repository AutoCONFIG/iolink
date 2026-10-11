# M7a GB PostgreSQL adapter worker evidence

Scope: `internal/gbpg` and migration `015_video_gb_transport.sql`.

The adapter implements the `gb.DeviceStore` port with PostgreSQL transactions:
credential decryption is bound to tenant/device/version, realm and CIDR policy
are required, registration lookup uses address/port/transport together, and
keepalive or unregister updates require the live identity. Migration 015 adds
realm and CIDR policy storage and replaces the 014 peer index with a
transport-aware unique index.

Observed checks with disposable PostgreSQL/TimescaleDB on loopback:

```text
IOLINK_TEST_PG_DSN=<disposable DSN> go test -race -shuffle=on -count=1 ./internal/gbpg ./internal/migrate  PASS
IOLINK_TEST_PG_DSN=<disposable DSN> go vet ./internal/gbpg ./internal/migrate  PASS
go test ./...  PASS
gofmt -l internal/gbpg/*.go  (no output)
```

Tests cover encrypted credential lookup, disabled/unknown devices, empty
allowlist, address/port/transport binding, keepalive, unregister, expiry,
same-address UDP/TCP registrations, and concurrent peer uniqueness. No SIP/UDP
socket, RTP/PS, ZLM, composition wiring, or real device claim is included.
