# M7a RTSP pinning and GB transport identity code review — exact SHA `8519fd5`

**codeQualityStatus:** WATCH
**recommendation:** APPROVE
**reportPath:** `.omo/evidence/m7a-rtsp-gb-code-review-8519fd5.md`
**blockers:** `[]`
**reviewed SHA:** `8519fd5dc31a6e77788ace6f66802c9bfc0272d1`

## Scope and identity

The exact reviewed commit is `8519fd5dc31a6e77788ace6f66802c9bfc0272d1`, whose
parent is `8e379182f4872b4850148e1f204f6c7c03140c16`. It adds the final GB peer
identity validation correction (rejecting zero ports and unspecified addresses), plus
regression cases, on top of the RTSP admission/DNS pinning and GB transport-identity
increment.

The cumulative scoped diff from the prior release base `0039563ff35ff0491bdfb2d4137e73de5368550a`
contains only:

- `internal/media/rtsp/admission.go`
- `internal/media/rtsp/admission_test.go`
- `internal/gb/contracts.go`
- `internal/gb/digest.go`
- `internal/gb/nonce.go`
- `internal/gb/gb_test.go`

Untracked workspace entries are unrelated evidence/temp files and were not used as
review input. No provider SDK, network listener, PostgreSQL adapter, SIP runtime,
composition wiring, credential logging, or external-camera claim was added.

## Skill perspective and slop pass

The required `remove-ai-slops` and `programming` skills were loaded and consulted
before judging the Go changes and tests. The overfit/slop pass found no deletion-only
test, test that merely pins a requested removal, tautological expected value, prompt or
prose assertion, implementation-only constant mirror, broad untyped escape hatch, or
unnecessary production parsing/normalization. The tests exercise observable admission,
DNS answer filtering, cancellation, race safety, transport-bound nonce/registration/
keepalive behavior, replay, and invalid peer boundaries. Provider-neutral boundaries
remain narrow and are not speculative runtime abstractions.

## Findings

### CRITICAL

None.

### HIGH

None. RTSP admission fails closed on missing boundaries, invalid syntax, disallowed
ports, resolver failure, cancellation, unsafe/mixed DNS answers, and disallowed addresses.
It verifies every DNS answer before returning one deterministic numeric endpoint and
never emits source credentials or hostname. GB registration, nonce, transaction replay,
peer lookup, and keepalive state retain the exact `AddrPort` plus lowercase `udp`/`tcp`
transport identity; zero-port and unspecified addresses are rejected before use. The
current changes do not widen tenant/device authorization or activate an unreviewed
provider.

### MEDIUM

1. `internal/gb/gb_test.go` is 513 pure non-comment LOC (and `internal/gb/contracts.go`
   is 240), exceeding the programming/remove-ai-slops architectural size ceiling for
   the test module and entering the warning band for the contract module. The test file
   combines Digest/registration, status/keepalive, nonce, XML, and concurrency fakes.
   This is a maintainability/review-risk follow-up rather than a correctness blocker in
   this focused increment; split by behavior seam before another substantial GB change.

2. The transaction replay cache in `internal/gb/digest.go:143-177` still evicts an
   arbitrary map key at 1024 entries instead of preferring expired entries. This logic
   predates this increment; under sustained high-volume SIP retransmits it can lose a
   still-live cached response and repeat registration work. It is bounded and does not
   weaken the new transport binding, so it remains a documented follow-up rather than a
   release blocker for this provider-neutral seam.

### LOW

1. `internal/media/rtsp/admission.go:174-187` hard-codes the known cloud metadata
   addresses required by the design. Any future platform-specific metadata endpoint
   must be added deliberately; the explicit CIDR allowlist and special-address checks
   still fail closed for the current supported set.

2. The package is not wired into runtime composition, and no real RTSP/ZLM or GB
   UDP/SIP device evidence exists in this snapshot. This is correctly deferred and
   remains `external_blocked`; this report does not claim R43 or video enablement.

## Contract and boundary review

- `rtsp.New` requires a resolver, non-empty valid CIDR allowlist, and non-empty nonzero
  port allowlist; `Pin` uses `network="ip"`, checks context before/after resolution,
  rejects userinfo/query/fragment/control/backslash/traversal, and requires all A/AAAA
  answers to be valid, allowed, and non-special. `Endpoint.URI()` emits only the
  pinned numeric address, selected port, and validated escaped path.
- `PeerIdentity.Valid` requires a valid nonzero-port, non-unspecified `netip.AddrPort`
  and exact `TransportUDP` or `TransportTCP`. DeviceStore, NonceStore records, message
  handlers, registration conflict checks, and transaction keys preserve the identity.
  Unknown registration states fail closed through `Registration.StatusAt`.
- Existing API/OpenAPI contracts are unchanged by this internal seam. `make
  verify-contracts` passed all 96 target operations and 568 fixtures; no live handler
  verification or external media claim is implied.

## Verification at exact SHA

| Check | Result |
|---|---|
| `git rev-parse HEAD` / parent identity | PASS; exact SHA and parent verified |
| `go test -race -shuffle=on -count=1 ./internal/gb ./internal/media/rtsp` | PASS |
| `go vet ./internal/gb ./internal/media/rtsp` | PASS |
| `go test -race -shuffle=on -count=1 ./...` | PASS |
| `go vet ./...` | PASS |
| `gofmt` check on all six scoped files | PASS |
| `git diff --check 0039563..8519fd5 -- internal/gb internal/media/rtsp` | PASS |
| `make verify-contracts` | PASS; 96 operations / 568 fixtures |
| `make docs-tools` | PASS (dependency/tool setup completed with exit 0) |

## Decision

**APPROVE** exact SHA `8519fd5dc31a6e77788ace6f66802c9bfc0272d1`; blockers are empty.
The `WATCH` status records only the inherited test-module size and bounded transaction
cache follow-ups. The verdict is limited to the RTSP admission/GB transport-identity
provider-neutral seams; RTSP provider hardening, ZLM, SIP runtime, real cameras,
HLS/browser/WeChat playback, and production License acceptance remain deferred or
`external_blocked`.
