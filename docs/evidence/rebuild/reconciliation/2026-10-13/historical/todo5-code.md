# Todo 5 independent code review

codeQualityStatus: WATCH
recommendation: APPROVE
reportPath: .omo/evidence/todo5-independent-code-review.md
reviewDate: 2026-09-27
reviewer: independent code-quality reviewer
reviewTree: shared working tree; final tree manifest `c4a1c98e459d5d135bdf0121efd0e5b022ad9ee5944a0149d20ebd9f8a5fc43e`
notepadPath: N/A (no separate notepad was provided)

## Goal and success criteria

Todo 5 implements R14-R22: app/admin authentication and token separation, owner assignment and audit, operation-boundary authorization, farm/pond/device/rule/alarm lifecycle, history/latest/stats aggregation, documented 400/401/404/409 behavior, and atomic confirmation/move/transfer behavior. The acceptance gate requires positive and negative paths, contract checks, real PostgreSQL/Timescale evidence where available, race coverage, truthful external-blocked classification, and two independent approvals over one worktree.

## Reviewed scope and diff

I reviewed the Todo 5 changes in the shared tree and the full relevant diff with `git diff`, including:

- `internal/appapi/handlers.go`, `internal/appapi/server.go`
- `internal/adminapi/handlers.go`, `internal/adminapi/server.go`
- `internal/core/admin_store.go`, `internal/core/repos.go`, `internal/core/service.go`
- `internal/appapi/server_test.go`, `internal/appapi/history_test.go`
- `internal/adminapi/server_test.go`
- `internal/core/m2_integration_test.go` and related core/history tests
- supporting domain, platform, migration, access, and documentation changes present in the same shared worktree

The review also checked `git diff --check`, formatting, compile/vet diagnostics, architecture manifests, and the contract fixtures. The complete raw QA summary and the independent gate review are retained at `.omo/evidence/todo5-raw-qa.log` and `.omo/evidence/todo5-independent-gate-review.md`.

## Findings

### CRITICAL

None.

### HIGH

None. The reviewed implementation now validates app/admin signing-key separation, expiry and token versions; carries owner predicates into detail/history/alarm operations; records owner-transfer audit data transactionally; maps missing resources to the documented status codes; and keeps batch/move/transfer state changes transactional. No cross-user access, partial-commit, or fail-open defect that blocks R14-R22 was found in the current tree.

### MEDIUM

- `internal/adminapi/handlers.go:233-250,276-301` builds pond `device_count` and `latest` from `ListDevices(..., 200, ...)`. A pond with more than 200 devices is therefore not fully represented and its newest device may be omitted. Current R22 tests cover the no-fixed-alarm-cap and deterministic tie cases, but do not construct a >200-device pond. This is a bounded scalability/coverage risk, not a current acceptance blocker.
- `internal/adminapi/handlers.go:276-294` treats every provider error from `Telemetry.Latest` as “no latest” and returns `nil`; the app equivalent is `internal/appapi/handlers.go:68-83`. This keeps the API fail-closed for data exposure, but it cannot distinguish an empty shadow from a database/provider failure and can hide operational faults. The main history/stats paths propagate provider errors, and the current tests do not require an error-specific latest response.
- The changed production modules remain oversized under the loaded `remove-ai-slops` ceiling: `internal/adminapi/handlers.go` (~730 pure LOC), `internal/core/admin_store.go` (~518), `internal/appapi/handlers.go` (~381), and `internal/core/repos.go` (~298). They are existing responsibility clusters expanded for Todo 5 rather than deletion-only or speculative abstractions. Splitting by handler/repository responsibility would improve maintainability, but it is outside this phase’s acceptance blocker set.

### LOW

- `farm_id` existence and actor-aware owner transfer are selected through optional consumer-side interfaces in `internal/adminapi/handlers.go:116-124,199-216`. The composition root supplies the concrete core implementation, so the production path retains the checks; a third-party `AdminStore` implementation that omits the optional capability would lose those extra checks. This is a seam-design limitation rather than a defect in the shipped provider.

## Skill-perspective checks

I explicitly consulted `omo:remove-ai-slops` and `omo:programming` before judging maintainability and test relevance. The overfit/slop pass found no deletion-only tests, tests that merely verify a requested removal, tautological self-comparisons, prompt/prose pins, or tests whose expected values are derived from the output under test. The added tests exercise observable HTTP/DB behavior (ownership, token expiry/revocation, history snapshots, alarm filtering and atomicity). No untyped escape hatch, broad exception equivalent, provider SDK leak, or boundary parser that belongs in production code was introduced in the reviewed Go paths. The oversized-module notes above are the only material maintainability debt found under the slop guidance.

## Verification evidence

Local checks run against the current shared tree:

- `git diff --check`: PASS.
- `gofmt -l` on the changed Go paths: no output, PASS.
- `GOCACHE=/tmp/iolink-todo5-review-gocache go vet ./...`: PASS.
- `GOCACHE=/tmp/iolink-todo5-review-gocache go test ./... -run '^$' -count=1`: PASS (all packages compile).
- `python3 scripts/check_architecture_manifests.py --all`: PASS; 55 requirements/manifests.
- `.venv/contracts/bin/python scripts/check_contracts.py`: PASS; 35 operations and 159 fixtures.

Recorded provider-backed evidence in `.omo/evidence/todo5-raw-qa.log` and `docs/evidence/rebuild/ownership-api.txt` reports PASS for:

- app/admin HTTP package suites;
- dedicated `timescale/timescaledb:latest-pg16` container `iolink-todo5-pg` with the redacted `IOLINK_TEST_PG_DSN`;
- owner transfer/audit, token revocation, snapshot history, alarm filtering, confirmation atomicity, lifecycle/move, restart/authentication, and real MQTT integration;
- `go test -race -shuffle=on ./internal/access ./internal/ingestion ./internal/persistence ./internal/core`;
- `make verify`, architecture checks, contract checks, and `git diff --check`.

The current review shell cannot independently re-run the full HTTP suite because the sandbox rejects the `httptest` IPv6 listener (`listen tcp6 [::1]:0: operation not permitted`), and Docker socket access is denied (`permission denied /var/run/docker.sock`). Those are environment limitations; they are not reported as local passes. We therefore treat the dedicated-container output as recorded evidence, not as a claim of local reproduction. WeChat credentials/device evidence remain `external_blocked` as required.

## Blockers

[]

The two independent reviews are bound to the same final tree manifest above. Recommendation: **APPROVE** Todo 5 code quality, with `WATCH` reserved for the bounded residual risks listed under MEDIUM.
