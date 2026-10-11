# M7a RTSP and GB transport boundary increment

This increment hardens two provider-neutral admission boundaries while video
remains disabled. It does not open sockets, invoke ZLMediaKit or sipgo, wire a
runtime provider, or claim real camera/GB acceptance.

## Implemented behavior

- `internal/media/rtsp` requires non-empty CIDR and port allowlists, parses only
  credential-free RTSP authorities, resolves every A/AAAA answer on each call,
  rejects unsafe or mixed addresses, and returns a numeric pinned endpoint;
- the RTSP endpoint preserves only the escaped path and numeric destination,
  with no hostname, userinfo, query, fragment, or provider operation;
- `internal/gb` binds registration, nonce, transaction, keepalive, and store
  contracts to a validated UDP/TCP `PeerIdentity` containing address and port;
- GB peer identities reject unknown transports, zero ports, and unspecified
  addresses; cross-transport nonce and keepalive reuse fails closed.

## Verification and review

The exact implementation snapshot is
`8519fd5dc31a6e77788ace6f66802c9bfc0272d1`.

- focused `go test -race -shuffle=on -count=1 ./internal/gb ./internal/media/...`
  and full `go test ./...` passed;
- scoped and full `go vet`, formatting/diff checks, `make verify`,
  `make docs-tools`, `make verify-contracts` (96 operations / 568 fixtures),
  and architecture checks passed;
- independent exact-snapshot code and gate reviews are archived as
  [final-code-review.md](final-code-review.md) and
  [final-gate-review.md](final-gate-review.md); both are APPROVE with no
  blockers.

The next GB PostgreSQL adapter and RTSP/ZLM provider still require separate
increments. Real RTSP/GB devices, ZLM, browser, WeChat, and external network
acceptance remain `external_blocked`.
