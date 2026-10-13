# Todo 6 independent code review

codeQualityStatus: WATCH
recommendation: APPROVE
reportPath: .omo/evidence/todo6-independent-code-review.md
reviewDate: 2026-09-27
reviewer: /root/todo6_code_review (independent; no production edits)
reviewTree: shared working tree
treeManifest: ef5bf5cff614339c255d2846a7631af042a45417bce16186f2a02c3cdd2ab6e2
notepadPath: N/A; no separate notepad was supplied
blockers: []

## Goal and success criteria

Todo 6 delivers the R28-R29 notification path: alarm and durable outbox intent in one transaction; external delivery after commit; current owner and explicit subscription revalidation; bounded workers; lease and restart recovery; retry/backoff, permanent, disabled, and unknown states; safe WeChat HTTP/token/template handling; and truthful software versus external evidence. The stage gate requires positive and negative tests, contract checks, real PostgreSQL/Timescale evidence where available, and two independent reviews of one final tree. Real WeChat delivery remains `external_blocked` without credentials, consent, and a device receipt.

## Reviewed scope and diff

I read the authoritative documentation in `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/ACCEPTANCE.md` R28-R29, the Todo 6 plan entry, and the architecture invariants. I reviewed the final diff and source for:

- `internal/core/alarm.go`, `internal/core/pipeline.go`, and removal of the old best-effort `internal/core/notify.go`;
- `internal/migrate/sql/006_notification_delivery.sql`;
- `internal/notifications/worker.go` and `worker_integration_test.go`;
- `internal/persistence/notifications.go`;
- `internal/wechat/sender.go` and `sender_test.go`;
- `cmd/iolinkd/main.go`, `Makefile`, and supporting config wiring;
- `docs/evidence/rebuild/notifications.txt` and `.omo/evidence/todo6-raw-qa.log`.

The final candidate manifest is the SHA-256 over 106 sorted regular paths from `git ls-files -co --exclude-standard -z`, excluding `.omo/` and `docs/evidence/`; each record is `relative-path + NUL + sha256(file bytes) + LF`. Review artifact paths are excluded to avoid self-reference. The resulting digest is `ef5bf5cff614339c255d2846a7631af042a45417bce16186f2a02c3cdd2ab6e2`.

## Findings

### CRITICAL

None.

### HIGH

None. The previously reproduced defects are fixed in this candidate:

- token transport/decode and temporary token-provider outcomes now carry `ErrRetryable`, while a send transport/decode failure remains `ErrUnknown`;
- token expiry causes exactly one refresh and one retry;
- message, value, and time template fields are configurable, and duplicate keys fail closed before network access;
- `users.wechat_subscribed` is explicit, defaults false, is joined with active membership and current owner resolution, and has a negative consent test;
- current owner resolution remains after claim, and sends occur after the claim transaction commits.

The retained overlay probes independently exercise these fixes and pass: `.omo/evidence/todo6-code-review-probes-third.txt` using `.omo/evidence/todo6-code-review-probes/overlay.json`.

### MEDIUM

- `internal/notifications/worker_integration_test.go:162-200` checks that the first retry is scheduled in the future and verifies six terminal attempts, then makes later attempts immediately due. It does not assert every 1/2/4/8/16-minute value or the no-call-before-due boundary. The worker formula is simple and the DSN-backed run is recorded, so this is a coverage watch item rather than a stage blocker.

### LOW

- `internal/wechat/sender.go:203-207` returns a plain retryable HTTP error rather than wrapping `notifications.ErrRetryable`; the worker's default branch still applies the bounded retry state, and the current behavior is covered by the HTTP failure tests. Preserving the typed sentinel would make adapter callers less dependent on the worker fallback.
- `cmd/iolinkd/main.go:200-209` waits for the notification worker before closing HTTP and MQTT. Full admission-stop ordering is part of the broader M5/R29 operations gate; Todo 6's worker is context-cancelled and bounded, and the gate receipt records this as outside the current blocker set.

## Skill-perspective check

This check ran. I explicitly loaded `omo:remove-ai-slops`, `omo:programming`, and the Go language references for error handling, concurrency, testing, and module size. No deletion-only or tautological tests, prompt/prose pins, expected values derived from the output under test, broad catches, provider SDK leaks, speculative abstraction, or oversized new module was found. New production files are below the 250 pure-LOC ceiling (worker 166, persistence 73, sender 185); formatting is clean. The remaining medium/low items above are maintainability and test-depth observations, not correctness blockers.

## Verification evidence

- `GOCACHE=/tmp/iolink-gocache go test -overlay=.omo/evidence/todo6-code-review-probes/overlay.json ./internal/wechat ./internal/notifications ./cmd/iolinkd -race -shuffle=on -count=1 -v`: exit 0 for all sender probes, WeChat tests, deadline handling, and command tests; notification DB cases explicitly skip when this reviewer shell has no DSN. Captured output: `.omo/evidence/todo6-code-review-probes-third.txt`.
- `git diff --check`: exit 0.
- `gofmt -l internal/notifications internal/wechat internal/persistence/notifications.go cmd/iolinkd/main.go`: no output.
- The executor's dedicated container and DSN-backed runs are recorded in `.omo/evidence/todo6-raw-qa.log`, including captured verbose output for owner transfer, consent disablement, duplicate claiming, retry, unknown/deadline, and expired lease recovery. The evidence explicitly labels no-DSN unit-only runs and real WeChat delivery `external_blocked`.
- The full gate rerun is recorded in `.omo/evidence/todo6-independent-gate-review.md` with `recommendation: APPROVE`, `blockers: []`, and the same digest `ef5bf5cff614339c255d2846a7631af042a45417bce16186f2a02c3cdd2ab6e2`.
- `make verify` reached existing app/admin tests but the reviewer sandbox rejects their IPv6 `httptest` listener (`listen tcp6 [::1]:0: socket: operation not permitted`); the raw artifact records this as partial rather than pass. This environment limit does not invalidate the focused and DSN-backed Todo 6 evidence.
- Real WeChat credentials, consent, and device receipt evidence were unavailable and remain `external_blocked`.

## Blockers

[]

Recommendation: **APPROVE** Todo 6 code quality for the final candidate digest above. The medium and low observations should remain visible in the ledger, but they do not block this stage.
