# GB transport identity worker evidence

## Scope

This worker increment changes only `internal/gb/*.go`. A registration now carries a validated `PeerIdentity` containing both the `netip.AddrPort` and the exact `TransportProtocol`. The only accepted protocols are lowercase `udp` and `tcp`. `DeviceStore.LookupPeer`, `ApplyRegistration`, and `MarkKeepalive` all preserve that binding; Digest nonces and transaction replay keys also include it. No PostgreSQL adapter, SIP socket, provider runtime, composition wiring, or external camera claim was added.

## Positive and negative scenarios

- `TestPeerIdentityRejectsUnknownTransport` accepts UDP and TCP and rejects empty, uppercase, TLS, NUL-containing, port-zero, unspecified-address, and other invalid identities.
- `TestRegisterDigestBindsNonceToTransport` challenges on UDP, changes the source to TCP, and verifies `ErrNonceInvalid` with zero registration writes.
- `TestHandleKeepaliveRequiresRegisteredTransport` registers on UDP, sends the same keepalive over TCP, and verifies `ErrUnknownDevice` with no store mutation.
- Existing Digest no-qop/qop, replay, transaction retransmission, peer ACL, registration conflict, Expires, keepalive expiry, XML bounds, and nonce expiry/replay scenarios remain green.
- Unknown registration status now resolves offline, preserving fail-closed behavior.

## Verification

Observed on the working tree before handoff:

```text
$ gofmt -w internal/gb/*.go
$ go test -race -shuffle=on -count=1 ./internal/gb
ok   git.hyhy.fun/rsplab/iolink/internal/gb  1.441s
$ go vet ./internal/gb
(no output; exit 0)
$ go test ./...
ok   git.hyhy.fun/rsplab/iolink/internal/gb  0.060s
... all repository packages passed ...
```

The focused race output and full repository output were observed directly in the worker session. The parent integration should rerun the focused commands after any shared-tree merge and archive the final exact-snapshot receipts.

## Follow-up boundary

The next PostgreSQL GB adapter must persist `PeerIdentity.Transport` alongside address and port, query by the complete identity, and reject malformed transport values before writing. Its integration tests must exercise same-address UDP/TCP registrations and keepalive lookups against real PostgreSQL. This worker intentionally does not modify migrations or the adapter.
