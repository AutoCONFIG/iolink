# M7a RTSP pinning and GB transport identity final gate review

- reviewed snapshot: `8519fd5dc31a6e77788ace6f66802c9bfc0272d1`
- reviewer: independent read-only gate reviewer (`/root/m7a_gb_gate_review_final2`)
- recommendation: **APPROVE**
- blockers: `[]`
- reviewed: 2026-10-11 (Asia/Shanghai)

## originalIntent

Review the exact follow-up snapshot for the M7a RTSP admission/pinning and GB28181
transport identity increment. The follow-up specifically closes the peer identity
boundary by rejecting zero-port and unspecified addresses while preserving the earlier
RTSP SSRF/path/port checks and GB digest, nonce, replay, expiry, XML, registration,
and transport binding behavior.

## desiredOutcome

The release snapshot must accept only a concrete, allowed GB source endpoint with UDP or
TCP identity and must fail closed for invalid identity values. RTSP must remain a
provider-neutral, numeric, allowlisted endpoint seam. This scoped increment must not
claim real camera, SIP/UDP/TCP, ZLMediaKit, browser, or WeChat playback acceptance.

## userOutcomeReview

The exact snapshot adds the expected validation to `PeerIdentity.Valid`: a peer now
requires a valid address, nonzero source port, non-unspecified address, and exactly
`udp` or `tcp` transport. Regression coverage includes `192.0.2.10:0`, `0.0.0.0:5060`,
and `[::]:5060`. The prior RTSP admission and GB behavior remains unchanged and passed
again at this SHA. The user receives an approved protocol-boundary increment with
runtime external acceptance correctly remaining `external_blocked`.

## Exact-SHA scope

A clean detached worktree was reset to the full SHA above. The delta from the previous
reviewed product SHA `8e379182f4872b4850148e1f204f6c7c03140c16` is limited to:

- `internal/gb/contracts.go` — reject port 0 and unspecified addresses.
- `internal/gb/gb_test.go` — regression cases for those invalid peers.

No code in the root shared worktree was used as evidence; the review checkout was
immutable and separate.

## Verification evidence

Receipts are in `.omo/evidence/m7a-rtsp-gb-gate-review-8519fd5-artifacts/`:

- `go test -race -shuffle=on -count=1 -v ./internal/media/rtsp ./internal/gb` — **PASS**;
  all RTSP and GB tests, including the new invalid-peer cases, passed.
- `go vet ./internal/media/rtsp ./internal/gb` — **PASS**; `gofmt -l` clean;
  `git diff --check HEAD^ HEAD` — **PASS**.
- `make verify` — **PASS**: build, repository-wide vet, and shuffled repository-wide
  race tests.
- `make docs-tools` — **PASS**; `make verify-contracts` — **PASS** (`96` target
  operations and `568` synthetic fixtures).
- `python3 scripts/check_architecture_manifests.py --all` — **PASS** (`55` requirements).

No PostgreSQL schema or adapter changed in this increment, so a disposable PostgreSQL
run is not applicable. No real RTSP camera, GB SIP/UDP/TCP network, ZLMediaKit,
browser, or WeChat device was available; those external R43 cases remain explicitly
`external_blocked` and are not represented as passed.

## Direct skill perspective pass

- **remove-ai-slops:** the two-line production validation and its three-case regression
  are directly tied to the peer identity boundary. They are neither tautological,
  deletion-only, nor implementation-mirroring tests, and no unnecessary extraction or
  compatibility layer was added.
- **programming:** validation occurs at the constructor and operation boundary, uses
  typed sentinel errors, keeps the immutable peer value comparable, and introduces no
  mutable state or concurrency surface. Focused race, full race, vet, formatting, and
  contract checks pass. MD5 remains the documented GB2016 compatibility baseline.

## Evidence gaps

No criterion-linked gap blocks this increment. Real-device/network/browser/WeChat
acceptance is outside this boundary snapshot and remains `external_blocked` in
`docs/IMPLEMENTED.md`.
