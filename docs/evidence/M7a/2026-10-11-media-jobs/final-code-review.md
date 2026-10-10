# M7a mediajobs code review — exact SHA 582700b

- `codeQualityStatus`: **WATCH**
- `recommendation`: **APPROVE**
- `reportPath`: `.omo/evidence/m7a-media-jobs-code-review-582700b.md`
- `blockers`: `[]`
- reviewed SHA: `582700bc710e1fb09aa1a185d6f4c5a83ea81fc6`
- parent chain: `72369f5` (initial store) → `9ebdaab` (clock/max-attempt/worker/nil/source hardening) → `86c9bf5` (clock callback lock-order fix) → `582700b` (validation/test/concurrency split)
- reviewer: independent read-only review; no source or tracked artifact edits
- scope: all six files in `internal/mediajobs` at this SHA: `store.go`, `validation.go`, `store_test.go`, `validation_test.go`, `test_helpers_test.go`, `concurrency_test.go`. Later untracked `internal/mediapg` and dirty parent-worktree files are excluded.
- date: 2026-10-11 (Asia/Shanghai)

## Goal and acceptance lens

The snapshot provides a provider-neutral in-memory media-job lease/CAS seam and splits the validation/test/concurrency responsibilities without changing the public store interface. The review checked canonical store-clock expiry, stale lease rejection, one-worker claim ownership, retry scheduling and maximum attempts, terminal statuses (`unknown`, `skipped`, `failed`, `disabled`, `ready`, `stopped`), context fail-closed behavior, input bounds, source/session identity fencing, and the refactor's scope/maintainability. Design references were `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/design/M7a-video.md` (worker lease/CAS and retry rules), `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, and `internal/media/contracts.go`.

## Skill perspective check

I explicitly loaded and applied:

- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/programming/SKILL.md`

The remove-ai-slops pass found no deletion-only tests, tests that merely prove a requested removal, tautological output assertions, broad exception/error swallowing, provider/SQL boundary leakage, needless abstraction, or unnecessary production parsing/normalization. The split files are cohesive and all are below the 250 pure non-comment LOC ceiling; moving shared validation helpers and concurrency fixtures is a maintainability improvement rather than a token-count-only dump. The programming pass found typed sentinel errors, checked errors, standard `context.Context` placement, `crypto/rand` lease tokens, and race-safe state mutation. The findings below are residual contract/test-coverage items, not AI-slop or unsafe untyped escape hatches.

## Findings

### CRITICAL

None.

### HIGH

None. No correctness blocker was found in the final snapshot: `ClaimJob` derives eligibility and `leasedUntil` from the injected store clock (`store.go:98-150`), while `RevalidateJob` and `FinishJob` use the same clock source (`store.go:152-202`). Lease identity is checked under the mutex (`validation.go:31-37`), stale finish/revalidate operations are rejected, and terminal statuses clear the lease before they become non-claimable. The refactor only relocated helpers and tests; it did not alter those transitions.

### MEDIUM

1. **Cancellation is only checked before the injectable clock and mutex wait.** All public methods check `contextError` before work, but `Enqueue`/`ClaimJob` call `s.clock()` before locking and each mutating method can wait on `s.mu` after the check (`store.go:61-96`, `98-150`, `152-202`, `204-218`). If a caller cancels while a clock callback or mutex wait is in progress, the operation can still mutate the store; `ProcessMediaJob` can then observe a claimed lease with a canceled context and leave it to expire. Add a second cancellation check at the last safe point before mutation (and a regression with a blocking clock/lock) if the store contract requires atomic fail-closed cancellation. The current pre-canceled regression (`store_test.go:134-138`) passes, so this is a residual race window rather than a release blocker for the normal nonblocking clock.

2. **A max-attempt retry result can remain labeled `retryable` forever.** `validResult` accepts `JobRetryable` whenever `NextAt` is nonzero (`validation.go:39-46`), and `FinishJob` stores `resultState(retryable)` even when `lease.Attempts == MaxAttempts` (`store.go:188-202`, `validation.go:48-61`). `ClaimJob` then skips attempts at or above `MaxAttempts` (`store.go:123-131`), leaving a snapshot that says `retryable` but can never be claimed again. `playback.jobResult` normally converts its sixth attempt to `failed`, so this is not reached by the current application path; the store boundary should still reject or convert a retryable result at the bound and test the stored terminal status. `TestMemoryStoreBoundsWorkerAndMaxAttemptRetry` currently asserts only “not replayed” (`store_test.go:188-216`).

3. **Job-kind validation is weaker than the media contract.** `Enqueue` rejects only an empty kind (`store.go:65`), so an arbitrary nonempty kind can be scheduled and claimed. The worker later falls through its default branch and burns retries before failing (`internal/playback/service.go:415-446`). The in-memory seam should accept only `media.JobStart` and `media.JobStop`, with a negative test, to keep malformed jobs fail-closed and avoid retry churn. This is an input-boundary hardening item; no provider side effect is reached for the current default branch.

### LOW

1. `workerID` is length-checked after `strings.TrimSpace` but the untrimmed value is stored (`store.go:102-149`). Values with surrounding whitespace are accepted and can diverge from a persistence adapter that treats the owner as a bounded identifier. Reject or canonicalize whitespace at the boundary if this seam is used beyond tests.

2. `MemoryStore` is constructor-backed but its exported zero value has a nil clock/map; a zero-value non-nil store can panic in `Enqueue`/`ClaimJob`/`RevalidateJob`/`FinishJob`. Nil receiver calls now fail closed and are regression-tested (`validation_test.go:11-26`); either document constructor-only use or initialize/guard the zero value in a later hardening pass.

3. `Enqueue` and `Snapshot` copy `JobAuthorization` by value but retain its `License.Payload`/`Signature` slice aliases. A caller mutating those slices concurrently can race with a later authorization read. The store is an in-memory test seam and no current path mutates these slices, so this remains low risk.

## Verified behavior

- **Canonical lease clock:** caller clock skew cannot extend or prematurely reclaim a lease; claim, revalidate, and finish use the injected store clock. `FinishJob` additionally requires the caller timestamp not to be past the stored lease.
- **CAS and ownership:** random 128-bit lease token, job kind/camera/source/stream/session identity fence, mutex-serialized claim, deterministic available-at/ID ordering, and stale duplicate finish rejection are covered by `TestMemoryStoreLeaseCASAndUnknownIsTerminal` and the 32-worker test in `concurrency_test.go`.
- **Retries and terminal outcomes:** retryable results require `NextAt` and are scheduled; unknown/failed/disabled/ready/stopped/skipped clear lease state and are not claimable. The max-attempt non-replay boundary is covered; the terminal-label issue above remains a medium follow-up.
- **Context and bounds:** pre-canceled contexts, zero lease, nil receiver, maximum worker length, invalid initial attempts, and duplicate IDs are handled fail closed. The cancellation-during-wait window is not currently pinned.
- **Refactor scope:** the public `JobStore`, `MemoryStore` state machine, and behavior tests are unchanged in meaning; `validation.go` owns sentinels/lease/result helpers, `test_helpers_test.go` owns deterministic fixtures, and `concurrency_test.go` isolates the concurrency scenario. All six mediajobs files are below 250 pure LOC.
- **Ownership boundary:** this provider-neutral store has no actor/tenant parameter; job authorization and live ownership are supplied by the application/provider store. Within this scoped seam, no cross-job token or identity acceptance was found.

## Commands and evidence

All commands below ran against a clean detached worktree at exact SHA `582700bc710e1fb09aa1a185d6f4c5a83ea81fc6`. Logs were copied to `.omo/evidence/m7a-media-jobs-code-review-582700b-artifacts/`.

| Command | Result | Evidence |
|---|---|---|
| `go test -race -shuffle=on -count=1 ./internal/mediajobs` | PASS | `scoped-race-shuffle.log` |
| `go test -race -shuffle=on -count=3 ./internal/mediajobs` | PASS | `scoped-race-shuffle-count3.log` |
| `go test -race -shuffle=on -count=1 -v ./internal/mediajobs` | PASS; all 8 tests | `scoped-race-shuffle-v.log` |
| `go vet ./internal/mediajobs` | PASS | `scoped-vet.log` |
| `gofmt -d internal/mediajobs/*.go` | PASS; no output | `gofmt.log` |
| `git diff --check 72369f5 582700b` | PASS; no output | `diff-check.log` |
| `go vet ./...` | PASS after copying existing ignored `internal/web/dist` into the detached worktree | `full-vet-with-internal-dist.log` |
| `go test ./...` | PASS after the same ignored frontend artifact setup | `full-test-with-internal-dist.log` |
| `go test -race -shuffle=on -count=1 ./...` | PASS after the same ignored frontend artifact setup | `full-race-shuffle-with-internal-dist.log` |
| `go vet ./...` / `go test ./...` with no generated frontend | FAIL only at `internal/web/embed.go:8:12: pattern all:dist: no matching files found` during setup; not caused by mediajobs | `full-vet.log`, `full-test.log` |
| `omo-agent-toolkit ulw-loop status --json` | `ULW_LOOP_PLAN_MISSING`; required fallback report path used | command output retained in review transcript |

The temporary frontend artifact was ignored/untracked and no source or tracked file in the reviewed snapshot was changed. No real PostgreSQL/Timescale, ZLM/RTSP/GB device, browser, WeChat, or production worker run is claimed; those remain `external_blocked` and outside this provider-neutral memory-store increment.

## Decision

**APPROVE** the exact `582700b` mediajobs snapshot. There are no CRITICAL or HIGH blockers. Track the MEDIUM items above before expanding the store or relying on it as a production authorization boundary; the validation/test/concurrency split itself is scoped, cohesive, and verified.
