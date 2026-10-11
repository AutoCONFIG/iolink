# M7a GB28181 PostgreSQL registration persistence

## Scope

This increment is the PostgreSQL adapter for the existing `gb.DeviceStore` port and migration `015_video_gb_transport.sql`. It persists the GB registration identity with address, port, and transport, decrypts device credentials through the domain GB binding, validates realm and allowed peer CIDRs, and applies expiry, enabled-state, keepalive, unregister, replacement, and conflict rules transactionally.

It does not enable video, open SIP or media sockets, assemble ZLM/RTP/PS providers, or claim a live camera, browser, WeChat, or physical GB device acceptance.

## Exact snapshot and review

- Product snapshot: `ec4366caf71683e2e486c7dc61e3a9746fb79bca`.
- Migration: `internal/migrate/sql/015_video_gb_transport.sql`.
- Adapter and tests: `internal/gbpg/store.go`, `internal/gbpg/store_test.go`.
- Independent code review: `final-code-review.md` (exact snapshot, APPROVE).
- Independent gate review: `final-gate-review.md` (exact snapshot, APPROVE, blockers `[]`).

## Verification

All commands used an isolated disposable PostgreSQL 16/TimescaleDB container. The DSN and password are intentionally omitted from this evidence.

- Focused real-PostgreSQL `go test -race -shuffle=on -count=1 -v ./internal/gbpg ./internal/migrate`: PASS.
- Full `make verify` with the same real PostgreSQL environment: PASS.
- `go vet ./internal/gbpg ./internal/migrate`: PASS.
- `gofmt -l` and `git diff --check`: PASS.
- `make docs-tools`: PASS.
- `make verify-contracts`: PASS, 96 target operations and 568 synthetic fixtures.
- `python3 scripts/check_architecture_manifests.py --all`: PASS, 55 requirements.

Receipts are in this directory. The focused suite covers encrypted credential and empty-policy rejection, transport-bound registration and keepalive, Expires=0 unregister, disabled/expired/out-of-policy rejection, same address and port across UDP/TCP, and concurrent uniqueness conflict. Migration tests cover 015 application, rollback, checksums, constraints, and credential persistence.

## External boundary

Live SIP/UDP/TCP runtime, RTP/PS/ZLM composition, physical GB cameras, browser playback, WeChat playback, and production network evidence remain `external_blocked` and are not counted as complete by this increment.
