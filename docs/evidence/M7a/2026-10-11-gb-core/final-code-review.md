# M7a GB core code-quality review

- `codeQualityStatus`: WATCH
- `recommendation`: APPROVE
- `snapshotSHA`: `e691d944666ef6913a42b6c5ba4388fd69a3128f`
- `reviewScope`: provider-neutral `internal/gb` cumulative GB implementation from `c67f7a3..e691d944`; HTTP/media changes in the same history were excluded
- `blockers`: `[]`
- `reportPath`: `.omo/evidence/m7a-gb-code-review-e691d94.md`
- `ulwStatus`: `ULW_LOOP_PLAN_MISSING`; no current attempt directory or notepad was available, so the required fallback path was used

## Goal and success criteria

Review the final GB snapshot for consumed-nonce expiry and bounded state, strict XML single-root/trailing handling, Digest/replay/peer/registration/keepalive authorization, registration conflict serialization, SIP/XML size limits, and bounded catalog `SumNum`/page semantics. The result must preserve fail-closed behavior, meaningful regression tests, and truthful evidence. Real GB UDP, NAT, sipgo transport, catalog aggregation, RTP/PS, INVITE/ACK/BYE, restart, and camera acceptance remain outside this provider-neutral increment.

## Input and diff inspected

The full cumulative diff was inspected with `git diff c67f7a3..e691d944 -- internal/gb docs/evidence/M7a/2026-10-11-gb-core`. The scoped product files are:

- `internal/gb/contracts.go`
- `internal/gb/digest.go`
- `internal/gb/message_xml.go`
- `internal/gb/nonce.go`
- `internal/gb/gb_test.go`

The checked-in scope receipts were also read as untrusted evidence:
`docs/evidence/M7a/2026-10-11-gb-core/README.md`, `full-test.log`, `targeted-race.log`, `vet.log`, and `format.log`. The working tree had an unrelated uncommitted update to `targeted-race.log` plus untracked evidence files; those were not used to establish this result. Fresh commands below were run against the exact snapshot SHA.

## Skill perspective check

This check ran before judging test relevance or maintainability. I loaded and consulted `omo:remove-ai-slops` and `omo:programming` (Go guidance). The overfit/slop pass found no remaining tautological MD5 assertion after the final cleanup, no deletion-only or requested-removal-only test, no untyped production escape hatch, broad catch, provider SDK leakage, or needless parsing added for this goal. The tests exercise the Digest, registration, status, XML, nonce, and concurrency seams. The remaining test-evidence and module-size concerns are recorded below as medium follow-ups.

## Findings by severity

### CRITICAL

None.

### HIGH

None. The prior expiry-first Keepalive path, registration lookup fail-open, unregister peer-conflict bypass, and same-peer registration race are closed in this snapshot and covered by fresh tests. Realm is checked against the registered device in both Challenge and HandleRegister.

### MEDIUM

1. **GB test module is far over the programming/remove-ai-slops size ceiling.** `internal/gb/gb_test.go:1-430` is approximately 453 pure non-comment lines (480 total), combining Digest/registration fixtures, status/Keepalive cases, nonce-store tests, XML parser tests, and a serialized-store fake. This is a maintainability and review-risk issue rather than a current behavior failure. Split by behavior seam before the next substantial GB increment (for example registration/digest, nonce, XML, and status tests).

2. **Two fail-closed registration tests do not assert absence of a write.** `internal/gb/gb_test.go:226-277` checks the returned `ErrRegistrationConflict` for a conflicting peer and a peer-store failure, but does not assert `len(store.applied) == 0`. A future regression that applies the registration and then returns the error would still pass these tests, despite the repository boundary rule that errors must not leave partial state. The current `applyRegistration` returns before `ApplyRegistration` for these paths; this is an evidence-strength gap, not a finding against the current implementation.

3. **Transaction retransmission cache eviction is arbitrary once 1024 live entries exist.** `internal/gb/digest.go:158-175` deletes the first map key when the cache is full, without preferring expired entries. Under more than 1024 valid transaction IDs inside the 60-second replay window, an otherwise legal retransmission can miss its cached response and apply registration again. The bounded cache is a reasonable memory guard for this provider-neutral increment, but the capacity/eviction tradeoff is undocumented and should be resolved with expiry-first eviction or an explicit fail-closed admission policy before high-volume SIP deployment.

### LOW

1. **Unknown registration status values are treated as online.** `internal/gb/contracts.go:74-78` returns `RegistrationOnline` for any status other than the explicit `offline` value when the time fields are fresh. The planned persistence schema constrains status to `online`/`offline`, so this is not a current path with the trusted adapter, but the core helper would be safer if it required `RegistrationOnline` explicitly and otherwise failed closed.

2. **The checked-in GB receipt logs are not a full exact-SHA audit trail.** `full-test.log` and `targeted-race.log` are short package summaries from the earlier GB fixes and do not encode the final SHA or all final test names. This review compensates with fresh exact-SHA runs recorded below; future GB receipts should include the commit SHA and the complete command line.

## Criterion review

- **Nonce:** `MemoryNonceStore` uses a cryptographically random 24-byte nonce, expires pending entries, records consumed nonces through their expiry, cleans both maps on Consume, and bounds `entries + used` to `MaxMemoryNonces` under one mutex. Eviction removes state and cannot reissue a consumed random value.
- **Digest and replay:** MD5 is the stated GB2016 compatibility baseline; method, request URI, realm, nonce, qop, cnonce, and nc are validated; response comparison is constant-time after length checking; nonce identity is bound to device, realm, URI, and source IP; replay returns `ErrNonceReplay`. Transaction responses are cached for 60 seconds and registration application is serialized per Service.
- **Peer and registration:** device IDs are exactly 20 decimal digits; configured peer CIDRs are enforced; request realm is bound to the registered realm; unknown device and peer-store errors fail closed; a peer already bound to another device is rejected for both registration and Expires=0; the lookup/apply check is serialized in the service.
- **Keepalive:** only an online, unexpired registration with a fresh keepalive window can reach `MarkKeepalive`; expired/offline registrations are rejected before the store mutation; source AddrPort and message DeviceID must match the registered peer/device.
- **XML boundary:** input is capped at 64 KiB, token depth at 16, and catalog items at 1000; DTD/entity/CDATA/directive content is rejected; exactly one root is required; non-whitespace trailing text and additional roots are rejected; expected device IDs, decimal SN, and bounded catalog counts are validated.
- **Catalog pages:** `SumNum` is bounded and represents the total; `DeviceList Num` must equal the current page item count, cannot exceed `SumNum`, and a nonzero total cannot carry an empty page. A one-item page with `SumNum=2` is accepted, while zero-count and over-count cases are rejected.
- **Scope:** no GB provider SDK, UDP socket, durable catalog aggregation, RTP/PS, INVITE/BYE, or external device claim was added. The composition seam remains provider-neutral.

## Fresh verification at the exact SHA

- `git rev-parse HEAD`: `e691d944666ef6913a42b6c5ba4388fd69a3128f`.
- `go test -race -shuffle=on -count=1 ./internal/gb`: **PASS**.
- `go vet ./internal/gb`: **PASS**.
- `go test -race -shuffle=on -count=1 ./...`: **PASS**.
- `go vet ./...`: **PASS**.
- `test -z "$(gofmt -d internal/gb/*.go)"`: **PASS**.
- `git diff --check c67f7a3..e691d944 -- internal/gb docs/evidence/M7a/2026-10-11-gb-core`: **PASS**.
- `make verify-contracts`: **PASS** (96 operations and 568 synthetic fixtures).
- `make docs-tools`: **PASS**.

## External and deferred evidence

The repository has no running GB UDP transport, sipgo adapter, NAT/ACL environment, simulated device process, catalog-page collector, RTP/PS media path, INVITE/BYE lifecycle, restart reconciliation, or real camera in this snapshot. Those acceptance rows are `external_blocked` or later-scope work and are not represented as software passes here. The fresh test run proves the provider-neutral core and its fakes only.

## Final recommendation

**APPROVE** the GB provider-neutral core at `e691d944666ef6913a42b6c5ba4388fd69a3128f`, with `blockers=[]`. Carry the three medium follow-ups and external-blocked runtime evidence into the next GB increment.
