# M7a GB28181 signaling core increment

This scoped increment adds the provider-neutral `internal/gb` signaling core. It does not open a UDP socket, create tenants or devices, invoke sipgo, or claim a real-camera acceptance result.

## Implemented behavior

- REGISTER challenge and MD5 Digest verification, with optional `qop=auth`, exact method/URI/realm binding, constant-time response comparison, nonce expiry and one-time use.
- Registered-device lookup and configured peer-prefix enforcement. A peer already bound to another device is rejected. Transaction IDs replay the same successful response for a bounded period without applying registration twice.
- Expires `0` unregisters; valid registration TTL is 60 through 3600 seconds. Registration status is online only while the expiry and three-keepalive window are both valid.
- MESSAGE XML for Keepalive and Catalog uses a 64 KiB input bound, 16-level token depth bound, a 1000-item catalog bound, exact `SumNum`/`DeviceList Num` consistency, expected device binding, and DTD/entity/CDATA rejection.
- Ports are limited to `DeviceStore`, `NonceStore`, `Clock`, and `Transport`; no provider SDK or network runtime is included.

## Verification

The scoped test covers successful no-qop and qop Digest flows, RFC Digest interoperability, unknown devices, peer ACLs, invalid fields, nonce expiry/replay, Expires bounds, peer conflicts, transaction retransmission, registration status transitions, registered-peer Keepalive handling, XML positive parsing, device mismatch, DTD/entity/oversize/depth/catalog-count rejection, race testing, and vet.

The scoped test and vet receipts are stored beside this README. The real GB UDP device, NAT, SIP transport, catalog pagination, RTP/PS, INVITE/BYE, and restart acceptance remain external or later increments.

Status: implementation complete; independent code and gate review required before release.
