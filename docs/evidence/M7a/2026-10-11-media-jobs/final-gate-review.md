# M7a media jobs lease/CAS gate review

- reviewed snapshot: `582700bc710e1fb09aa1a185d6f4c5a83ea81fc6`
- reviewer: independent read-only gate reviewer (`/root/m7a_gb_gate_review_final2`)
- scope: exact `internal/mediajobs` split commit only; later `internal/mediapg` untracked files were excluded
- reviewed: 2026-10-11 (Asia/Shanghai)
- recommendation: **APPROVE**
- blockers: `[]`

## Original intent

Add the provider-neutral M7a media job lease/CAS seam. Jobs must be claimable by
one worker, reclaimable after lease expiry, revalidated against current
authorization/source version, and finishable only by the current lease. Retryable
results schedule a later attempt; unknown and skipped results become terminal
without replay; malformed inputs, expired leases, stale source state, and
canceled contexts fail closed.

## Desired outcome

`internal/mediajobs.MemoryStore` provides a deterministic in-memory seam for the
application worker while keeping provider SDKs and persistence adapters out of
the domain package. Lease ownership remains exclusive under concurrency and
stale workers cannot cause duplicate or unauthorized media side effects. The
582700b refactor keeps each production/test module below 250 pure non-comment
lines without changing the observable lease contract.

## User outcome review

The exact snapshot satisfies the requested seam:

- enqueue rejects malformed jobs and duplicate IDs;
- claim serializes under a mutex, orders eligible jobs deterministically,
  increments attempts, assigns a cryptographically random token, and reclaims
  expired leases up to `MaxAttempts`;
- revalidation checks token/owner/job identity, lease expiry, and current
  source/camera/stream authorization;
- finish checks context, lease identity, both caller and store clocks, source
  fences, and result status before clearing the lease;
- retryable results require `NextAt` and are claimable only at that time;
- unknown, failed, disabled, ready, stopped, and skipped outcomes become
  non-running terminal states (skipped is canceled and is not replayed);
- the concurrent-claim regression proves exactly one worker receives a lease;
- production code imports only the internal media contracts and standard
  library, with no pgx, SIP, ZLM, RTSP, HTTP, MQTT, or runtime-provider SDK.

## Reproduced checks at exact SHA

Checks ran in clean detached worktree
`/tmp/iolink-mediajobs-gate-582700b`, pinned to the exact SHA above. The
worktree was clean and contained no later `mediapg` files or dirty test edits.
The ignored frontend `web/dist` artifact was linked from the existing build only
to let the repository-level verifier run; no tracked source was changed.

| Check | Result | Evidence |
|---|---|---|
| `go test -race -shuffle=on -count=1 -v ./internal/mediajobs` | **PASS, exit 0** | Split lease/CAS, source-fence, expiry, retry, skipped-terminal, nil-receiver, and concurrency tests pass. `/tmp/m7a-mediajobs-582700b-race.log` |
| `go vet ./internal/mediajobs` | **PASS, exit 0** | `/tmp/m7a-mediajobs-582700b-vet.log` |
| `gofmt -l internal/mediajobs` and `git diff --check HEAD^ HEAD` | **PASS** | No files reported and no whitespace errors. `/tmp/m7a-mediajobs-582700b-gofmt.log`, `/tmp/m7a-mediajobs-582700b-diff.log` |
| `IOLINK_TEST_PG_DSN=<disposable loopback DSN> make verify` | **PASS, exit 0** | Full build/vet/race suite passed, including `internal/mediajobs`; PG-backed packages (`camerapg`, `gbcatalogpg`, `migrate`, and others) passed against the disposable loopback database. Password is omitted from this report. `/tmp/m7a-mediajobs-582700b-verify2.log` |
| `make docs-tools` | **PASS** | Pinned documentation validators installed successfully. `/tmp/m7a-mediajobs-582700b-docs-tools.log` |
| `make verify-contracts` | **PASS, exit 0** | 96 target operations and 568 synthetic request/response fixtures passed. `/tmp/m7a-mediajobs-582700b-contracts.log` |
| `python3 scripts/check_architecture_manifests.py --all` | **PASS, exit 0** | 55 requirements, owners, providers, references, and AGENTS scopes passed. `/tmp/m7a-mediajobs-582700b-architecture.log` |

## Direct skill perspective pass

- **remove-ai-slops:** reviewed the exact diff, production code, and split tests
  for redundant validation, dead/comment-only code, needless extraction,
  duplicated logic, wrong-layer imports, tautological tests, and tests that
  merely mirror implementation. The tests exercise observable lease behavior
  and adversarial expiry/source/concurrency paths. No criterion-violating slop
  was found. Pure non-comment LOC is 203 (`store.go`), 59 (`validation.go`),
  208 (`store_test.go`), 23 (`validation_test.go`), 33
  (`concurrency_test.go`), and 36 (`test_helpers_test.go`).
- **programming:** context cancellation is checked before mutation, all shared
  job state is mutex-protected, lease tokens use `crypto/rand`, source fences
  and dual-clock expiry checks are explicit, and race/vet/full verification
  pass. No maintenance, false-confidence, or scope-drift issue tied to a
  stated success criterion was found.

## Checked artifact paths

- `docs/README.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`
- `docs/design/M7a-video.md` (worker lease/CAS semantics)
- `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`
- `docs/evidence/M7a/2026-10-11-media-seams/README.md`
- `internal/media/contracts.go`
- `internal/mediajobs/store.go`
- `internal/mediajobs/validation.go`
- `internal/mediajobs/store_test.go`
- `internal/mediajobs/validation_test.go`
- `internal/mediajobs/concurrency_test.go`
- `internal/mediajobs/test_helpers_test.go`
- `Makefile`
- `scripts/check_architecture_manifests.py`

## Evidence gaps and external status

1. **`external_blocked`:** no real ZLM/RTSP/GB provider runtime, camera,
   browser, or WeChat playback was exercised. Those remain disabled or
   unimplemented for this provider-neutral memory-store increment and are
   outside this exact `internal/mediajobs` scope.
2. The checked store is an in-memory seam; PostgreSQL persistence and production
   worker integration are later increments. The repository-level disposable
   PG verify passed, but it does not turn those later media-provider features
   into completed acceptance evidence.

No blocker is recorded for exact `582700bc710e1fb09aa1a185d6f4c5a83ea81fc6`.
