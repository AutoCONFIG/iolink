# M7a GB PostgreSQL adapter code review

- **Reviewed exact SHA:** `ec4366caf71683e2e486c7dc61e3a9746fb79bca`
- **Scope:** `internal/gbpg/store.go`, `internal/gbpg/store_test.go`, `internal/migrate/sql/015_video_gb_transport.sql`
- **codeQualityStatus:** **CLEAR**
- **recommendation:** **APPROVE**
- **blockers:** `[]`
- **Report path:** `.omo/evidence/m7a-gbpg-code-review-ec4366c.md`
- **Review mode:** independent, read-only; no product files were changed.

## Review basis

I reviewed the exact commit diff against its parent, the GB domain port, migration 013/014 precedents, the M7a design/API/acceptance material, and the referenced gate receipts. I also checked the working tree remained at the exact SHA; unrelated untracked `.tmp/` and evidence files were excluded from scope.

The implementation stays behind the narrow `gb.DeviceStore` port. It:

- rejects nil dependencies, malformed device IDs, disabled devices, empty/invalid realms, empty/malformed CIDR policies, bad credential ciphertext, invalid peer identities, and invalid registration windows;
- decrypts credentials with the GB device tenant/device/version binding and never persists the cleartext password;
- compares the complete peer identity (address, port, and UDP/TCP transport);
- uses PostgreSQL `clock_timestamp()` for lookup/keepalive expiry fences;
- locks the target device row during registration, performs replacement/unregister changes in one transaction, and maps the partial unique peer index conflict to `gb.ErrPeerConflict`;
- uses parameterized SQL and rollback defers on all transaction paths;
- atomically adds realm/CIDR columns and replaces the old peer uniqueness index in migration 015.

The migration’s printable ASCII realm and CIDR-count constraints agree with the adapter’s fail-closed parsing. Legacy empty defaults are rejected by `LookupDevice`/registration until configured, which is safe for this boundary.

## Findings

### CRITICAL

None.

### HIGH

None. No credential disclosure, cross-peer identity acceptance, authorization bypass, transaction partial-commit path, or duplicate active identity race was found.

### MEDIUM

None.

### LOW

1. Registration timestamps are accepted from the caller and written as parameters (`internal/gbpg/store.go:106-146`), while expiry predicates use the database clock. The current GB application service supplies its injected trusted clock and validates the registration window before calling this adapter, so this is not a release blocker. A future direct caller should preserve that trust boundary or move timestamp derivation into the store.
2. The test suite does not assert every rollback-preservation permutation (for example, inspecting the prior row after a duplicate conflict or wrong-peer unregister). The transaction/unique-index implementation is straightforward and the concurrent conflict integration case passed; this is a coverage opportunity, not a correctness blocker.

## Test and evidence review

The exact-SHA evidence I verified or inspected includes:

- Real isolated PostgreSQL focused integration tests with race detector and shuffle:
  `IOLINK_TEST_PG_DSN='<provided disposable DSN>' go test -race -shuffle=on -count=1 -v ./internal/gbpg` — **PASS**, all four GB adapter cases.
- The same isolated PostgreSQL environment, relevant packages (`gb`, `gbpg`, `gbcatalog`, `gbcatalogpg`, `migrate`) with `-race -shuffle=on` — **PASS**.
- Repository-wide `go test -race -shuffle=on -count=1 ./...` at this SHA — **PASS**.
- `gofmt` cleanliness and `git diff --check` for the scoped files — **PASS**.
- Inspected gate receipts for `go vet ./internal/gbpg ./internal/migrate`, `make verify` (including repository vet/race), `make docs-tools`, `make verify-contracts` (96 operations / 568 fixtures), and architecture manifests (55 requirements) — all recorded **PASS** in `.omo/evidence/m7a-gbpg-gate-review-ec4366c-artifacts/`.
- The focused tests cover encrypted credential binding and secrecy, empty policy rejection, realm/CIDR admission, expiry and disabled-state rejection, transport replacement, keepalive, Expires=0 unregister, same address/port across different transports, and concurrent same-transport conflict.

No live SIP/UDP/TCP runtime, physical GB camera, RTP/PS/ZLM provider, or playback surface was available. Those remain external/integration scope and are not claimed by this adapter review.

## Required skill perspectives

- **`omo:remove-ai-slops`** was consulted. The new tests exercise observable database behavior and failure paths; they are not deletion-only, tautological, or implementation-constant tests. No unnecessary production parsing/extraction or speculative abstraction was added.
- **`omo:programming`** was consulted. The Go code uses typed sentinel errors, parameterized SQL, explicit transaction boundaries, rollback defers, bounded validation, and no untyped escape hatch. No brittle prompt-style or implementation-mirroring test pattern was found.

## Decision

**APPROVE exact SHA `ec4366caf71683e2e486c7dc61e3a9746fb79bca`.** No CRITICAL or HIGH findings remain and there are no blockers for this persistence increment.
