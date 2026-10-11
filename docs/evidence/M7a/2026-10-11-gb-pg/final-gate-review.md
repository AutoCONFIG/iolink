# M7a GB PostgreSQL registration persistence gate review

- reviewed snapshot: `ec4366caf71683e2e486c7dc61e3a9746fb79bca`
- reviewer: independent read-only gate reviewer (`/root/m7a_gb_gate_review_final2`)
- recommendation: **APPROVE**
- blockers: `[]`
- reviewed: 2026-10-11 (Asia/Shanghai)

## originalIntent

Review the exact PostgreSQL persistence increment for GB28181 registration identity:
`internal/gbpg` and migration `015_video_gb_transport.sql`. The adapter must decrypt
credential ciphertext with the domain GB binding, validate realm and allowed CIDR
policy, persist and look up exact address/port/transport identity, enforce expiry and
enabled state, support keepalive and Expires=0 unregister transitions, and make
registration replacement/conflict behavior transactional and race-safe.

## desiredOutcome

The provider adapter must implement the existing narrow `gb.DeviceStore` port without
provider leakage into the domain. Database constraints and transaction boundaries must
fail closed for malformed credentials, realm/policy values, disabled or unauthorized
peers, expired registrations, transport mismatches, and duplicate active peer identity.
Migration 015 must apply atomically after 014 and preserve the documented video runtime
boundary; real SIP/UDP devices and media composition remain outside this increment.

## userOutcomeReview

The exact snapshot delivers the requested persistence seam and real isolated PostgreSQL
evidence. `LookupDevice` requires an enabled 20-digit GB device, a nonempty printable
realm, a nonempty parseable CIDR policy, and successfully opens the encrypted credential
with the GB device AAD binding. `ApplyRegistration` validates identity/time fields,
locks the target row, enforces the configured CIDR, clears/replaces prior registration
inside one transaction, and relies on the active `(peer_address, peer_port, transport)`
unique index for cross-device conflict safety. `LookupPeer` and `MarkKeepalive` use the
same exact identity and database clock expiry fence; Expires=0 clears the registered
identity transactionally. No password or DSN secret is recorded in this report.

## Exact-SHA scope

A clean detached worktree was reset to the full SHA above. The commit changes only:

- `internal/gbpg/store.go`
- `internal/gbpg/store_test.go`
- `internal/migrate/sql/015_video_gb_transport.sql`

No product code was edited during review.

## Verification evidence

Receipts are retained under `.omo/evidence/m7a-gbpg-gate-review-ec4366c-artifacts/`.
The focused command used the requested isolated disposable PostgreSQL instance (the
secret-bearing DSN is intentionally omitted here):

- `IOLINK_TEST_PG_DSN='<disposable isolated DSN>' go test -race -shuffle=on -count=1 -v ./internal/gbpg ./internal/migrate` — **PASS**.
  `internal/gbpg` passed all four integration cases: encrypted credential and empty
  policy rejection; transport-bound registration, keepalive, and unregister; disabled,
  expired, and out-of-policy rejection; same address/port across transports plus
  concurrent unique conflict. `internal/migrate` passed embedded migration,
  credential persistence, concurrent-up, rollback/checksum, legacy, and schema
  constraint suites. Receipt: `gbpg-focused-ec4366c.log`.
- The same disposable-PostgreSQL environment ran `make verify` — **PASS**: build,
  repository-wide vet, and repository-wide race/shuffle tests, including `gbpg` and
  `migrate`. Receipt: `gbpg-make-verify-ec4366c.log`.
- `go vet ./internal/gbpg ./internal/migrate` — **PASS**; `gofmt -l` clean;
  `git diff --check HEAD^ HEAD` — **PASS**. Receipt: `gbpg-vet-ec4366c.log`.
- `make docs-tools` — **PASS**; `make verify-contracts` — **PASS** (`96` target
  operations and `568` synthetic fixtures). Receipts: `gbpg-docs-tools-ec4366c.log`
  and `gbpg-verify-contracts-ec4366c.log`.
- `python3 scripts/check_architecture_manifests.py --all` — **PASS** (`55`
  requirements). Receipt: `gbpg-architecture-ec4366c.log`.

## Direct skill perspective pass

- **remove-ai-slops:** the adapter tests assert observable PostgreSQL behavior across
  credential secrecy, realm/CIDR admission, expiry, transport identity, unregister,
  and concurrent conflict. They are not tautological, deletion-only, or
  implementation-constant tests. The production adapter is a required narrow port
  implementation; no speculative provider abstraction or useless extraction was added.
- **programming:** SQL uses parameters for all data, transactions have rollback
  defers, row locking and a unique partial index provide race safety, DB clock checks
  expiry, typed sentinel errors are returned, and invalid cipher/policy/identity data
  fails closed. Race, vet, format, full verification, and migration tests pass.

## Evidence gaps and external scope

No live SIP socket, UDP/TCP transport runtime, RTP/PS/ZLM provider, browser/WeChat
playback, or physical GB camera was available. Those R43 runtime cases remain
`external_blocked` in `docs/IMPLEMENTED.md`; this adapter gate does not claim them
complete. No criterion-linked gap blocks this persistence increment.
