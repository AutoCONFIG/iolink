# Todo 5 independent gate review

recommendation: APPROVE
reviewDate: 2026-09-27
reviewer: independent gate reviewer
reviewTree: shared working tree; final tree manifest `c4a1c98e459d5d135bdf0121efd0e5b022ad9ee5944a0149d20ebd9f8a5fc43e`

## originalIntent

Implement Todo 5, identity, ownership, authorization, and API application services for R14-R22. The observable result must enforce admin and app authentication, ownership transfer and audit, operation-boundary resource authorization, device/pond/rule/alarm management, history/latest/stats behavior, and the documented 400/401/404/409 conventions. The user gate requires positive and error paths, contract checks, real PostgreSQL/Timescale checks, race coverage where applicable, recorded evidence, and two independent approvals over one tree.

## desiredOutcome

An API surface that matches `docs/api/openapi.yaml` and `docs/api/admin-openapi.yaml`, keeps A/B resources isolated through moves and transfers, performs atomic confirmation and ownership updates, and has reproducible evidence for R14-R22 before Todo 5 is marked complete.

## userOutcomeReview

The recorded DSN-backed integration summary covers ownership transfer, snapshot history, filtering, token revocation, and batch atomicity, and the architecture and contract commands are recorded as passing. The current source contains fixes for missing expiry, deterministic latest selection, invalid pagination/filter values, owner transfer transactions, fail-closed history capability, no-shadow `latest: null`, missing-device move mapping, explicit not-found checks for filtered farm/pond queries, and removal of response fields disallowed by the frozen OpenAPI schemas. The final source review found no criterion-blocking defect. The evidence file is a summary rather than raw command output, and the final receipts now bind both reviews to the same tree manifest.

## blockers

[]

## checkedArtifactPaths

- `docs/README.md`
- `docs/PLAN.md`
- `docs/PLAN-DETAILS.md`
- `docs/ACCEPTANCE.md`
- `docs/api/openapi.yaml`
- `docs/api/admin-openapi.yaml`
- `docs/evidence/rebuild/ownership-api.txt`
- `.omo/evidence/todo5-independent-code-review.md`
- `.omo/evidence/todo5-raw-qa.log`
- `internal/appapi/handlers.go`
- `internal/appapi/server.go`
- `internal/adminapi/handlers.go`
- `internal/adminapi/server.go`
- `internal/core/admin_store.go`
- `internal/core/repos.go`
- `internal/core/m2_integration_test.go`
- `internal/appapi/history_test.go`
- `internal/core/history_http_test.go`

## verification

- `GOCACHE=/tmp/iolink-gatecheck go test ./internal/core -count=1`: pass locally.
- `GOCACHE=/tmp/iolink-gatecheck go test ./internal/appapi -run 'TestHistory' -count=1 -v`: pass locally.
- `git diff --check`: pass locally.
- `.omo/evidence/todo5-independent-code-review.md`: independent code-quality review APPROVE, blockers `[]`, same final tree manifest.
- `GOCACHE=/tmp/iolink-gatecheck go test ./internal/appapi ./internal/adminapi ./internal/core -count=1`: app/admin tests could not start their `httptest` listener in this sandbox (`listen tcp6 [::1]:0: operation not permitted`); this is an environment limitation, not counted as a pass.
- `.omo/evidence/todo5-raw-qa.log`: records claimed app/admin, real-DB integration, race/shuffle, `make verify`, manifest, and contract results, but supplies no raw output file path or tree manifest.
- `docs/evidence/rebuild/ownership-api.txt`: records the same commands and redacted environment; it labels WeChat/hardware `external_blocked`.

## removeAiSlopsAndProgrammingReview

I consulted `remove-ai-slops` and `programming` and performed a direct pass over the Todo 5 diff and changed Go paths. I found no criterion-blocking deletion-only or tautological test pattern, and no untyped escape hatch or broad exception equivalent. The current tests assert useful ownership, scoped history, filtered alarms, and batch behavior. The final fixes also cover invalid admin filters, filtered farm/pond not-found behavior, and the no-shadow device-detail shape.

## exactEvidenceGaps

- The raw QA summary does not include raw command output files, but the commands, redacted environment, and external-blocked classification are recorded in the ownership evidence.
- The local review shell cannot access Docker (`permission denied` on `/var/run/docker.sock`), so the DSN-backed claims remain recorded evidence rather than independently reproduced real-provider output.
- Full app/admin package execution is blocked by the sandbox IPv6 listener restriction; the recorded pass remains the available execution evidence and should be reproduced in a permitted test environment for an independent local run.

## nonBlockingNotes

- `farm_id` existence is supplied through the concrete core `FarmExists` capability; the handler keeps the optional consumer-side assertion so the admin package remains provider-independent. The composition root uses the core implementation.
- The local sandbox cannot reproduce the HTTP listener tests or access Docker, so the DSN-backed raw QA remains the source of real-provider evidence. This is an environment limitation and does not contradict the recorded container run.
- The independent code review marks `WATCH` for bounded scalability and maintainability notes; it identifies no acceptance blocker.
