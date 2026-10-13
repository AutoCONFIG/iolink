# Baseline HTTP 500 safety: independent gate review

recommendation: REJECT

Exact candidate tree: `144a9abf4f4d1ada23b6c11febe3ff7577df7f49`

Base: `9c63186`

Export: `/tmp/iolink-http-errors-20261014`

Reviewer: `/root/latest_gate_review`, nonediting except this report.

## Blockers

- violatedCriterion: C3 — a missing provider capability must be recorded as actual capability absence, without fabricating an execution error.
  Observation: when the history adapter lacks `UserTelemetryRepo`, the changed app handler calls `providerFailure(c, errors.New("history authorization unavailable"))`. This creates an error that no provider returned and logs `HTTP provider operation failed` with `err=*errors.errorString`. The milestone brief and evidence README explicitly require no fabricated error for capability absence. The management-side missing-capability paths correctly use an unavailable diagnostic without an `err` field, but the app test currently requires the fabricated error class and therefore gives false confidence for this criterion.
  evidencePointer: `internal/appapi/handlers.go:255`; `internal/appapi/http_errors.go:10`; `internal/appapi/provider_error_test.go:94` and `:181`; `docs/evidence/deployment/2026-10-13-http-provider-errors/README.md:5`.
  Required fix: record a constant capability-unavailable diagnostic through the injected context-aware logger, return the same contractual 500/internal_error, and make the history-capability test assert absence of an error field/category rather than the invented execution error. Preserve true provider-error logging. Present the fixed candidate for fresh same-tree independent double review.

## Original intent and desired outcome

originalIntent: continue correcting and testing existing IoLink functionality under AGENTS.md, with safe internal debug diagnostics, comprehensive relevant tests and independent double review before publishing each small milestone.

desiredOutcome: for this bounded baseline HTTP 500 substep, replace raw/provider-specific public error text with the contractual `internal_error`, record genuine provider failure categories with generated request correlation through injected loggers, and distinguish actual capability absence without fabricating errors. Preserve existing 4xx, License, authorization and dedicated latest behavior.

userOutcomeReview: the candidate improves public error privacy and contract consistency across the changed handlers, and actual PostgreSQL failures are safely correlated. One diagnostic branch fails the explicit capability-absence criterion and blocks approval. This report does not certify the complete original 35-operation live HTTP matrix, whole R02.c, the 96 extended operations, browser behavior or all project acceptance.

## Criteria matrix

| Criterion | Result | Evidence |
|---|---|---|
| C1: changed genuine 500 responses use stable contractual internal_error | PASS | production diff; independently replayed 37 management and 18 app scenarios; seven actual PostgreSQL failure surfaces |
| C2: raw provider/request secrets stay out of public responses and dump; correlation is server generated | PASS for exercised changed paths | actual TCP HTTP/dump assertions; injected poison fixture; real SQLSTATE correlation |
| C3: capability absence records only the actual missing capability, without fabricated execution error | FAIL | app history-capability branch and observed diagnostic above |
| C4: 4xx/License/authorization/latest behavior and unrelated drafts are preserved | PASS by bounded diff inspection | no edits to those mappings or database semantics; app server only two login error-response hunks |
| C5: source-bound tests and real integration support the stated bounded result | PASS for focused evidence | all 1,268 export blobs match; independent focused shuffled race with real DSN and independent contract checker |

## Independent verification

- Matched every tracked exported blob to `git ls-tree -rz` and `git hash-object` for the exact candidate: 1,268 matched, zero mismatches.
- Inspected all changed production hunks and the new helpers, consumer fixtures, HTTP tests and actual PostgreSQL test. `internal/appapi/server.go` differs only in the two login failure handling hunks, with unrelated playback drafts excluded.
- Independently ran from the candidate export:

```text
GIN_MODE=release IOLINK_TEST_PG_DSN=<dedicated Timescale DSN> go test -race -shuffle=on -count=1 -v ./internal/adminapi ./internal/appapi ./internal/core -run 'TestBaselineAdminHTTPProviderFailuresUseSafeInternalError|TestBaselineHTTPProviderFailuresUseSafeInternalError|TestBaselineHTTPProviderErrorsWithRealPostgres'
```

  Exit 0; adminapi 11.293s, appapi 1.084s, core 2.384s. Shuffle seeds 1791907984301596279, 1791907984300563068, 1791907984444276315. The real PG case ran without SKIP in 1.32s. The passing history-capability test still encodes the C3 defect.
- Independent replay observed the faulty history-capability JSON diagnostic: `msg="HTTP provider operation failed"`, `stage="http_provider"`, `err="*errors.errorString"`, and a generated matching request ID. By contrast, management missing-member capability diagnostics omit `err` and use `HTTP provider capability unavailable`.
- The real test authenticates both APIs before individually making farms/alarms/alarm_rules unavailable, restores each relation afterwards, and verifies seven 500/internal_error responses with matching `postgres_42P01` dump records. Raw relation name, test password and tokens are rejected by dump assertions. Core/testdb handles dedicated test-database cleanup. I reused the coordinator-owned dedicated PG instance and did not create or clean a container.
- Independently ran the repository venv `scripts/check_contracts.py` from the export: exit 0; 96 operations and 568 synthetic fixtures passed. This checks contract schemas/fixtures, not the entire live handler matrix or capability diagnostic semantics.
- `git diff --check 9c63186 144a9abf4f4d1ada23b6c11febe3ff7577df7f49`: exit 0. `gofumpt -l` on all ten changed Go files emitted no filenames.
- Inspected before/after and real-PG executor logs. The before tests reproduce raw/noncontract public errors and missing provider diagnostics. At report time `snapshot-verify.log` was still incomplete, ending after camerapg; I do not report the pending full build/vet/race command as completed. The fixed milestone must capture its terminal full-suite result.

## Required programming/remove-ai-slops direct pass

Both skill instructions were loaded and consulted in this reviewer session, including Go README/error-handling/testing references. I applied their criteria directly to this entire production/test diff.

- New error helpers centralize the existing outer HTTP failure boundary without introducing interfaces/dependencies, domain/provider coupling, speculative layers or extra parsing. Constant messages and safe structured fields use the composition-root logger and request context. Real errors remain error values, enabling the existing redactor to retain only safe categories/SQLSTATE.
- The app capability branch creates an unnecessary synthetic error, which violates the stated observation requirement and the programming perspective's truthful boundary handling. The history-capability assertion mirrors that wrong choice instead of observing the required absence; this is the C3 blocker, not a stylistic preference.
- The 55 HTTP scenarios cover distinct affected operations/provider branches, including fallback and actor ports, and inspect actual network response and dump files. Real PostgreSQL integration complements fake consumer failures. No deletion-only, prose-pin, tautological self-comparison or removal-only tests found. The error-value assertions target the machine-consumed external contract; the diagnostic assertions target the requested privacy/correlation behavior.
- New files stay below the 250 pure-LOC limit; oversized legacy handlers/server files predate this change. No new goroutine/sleep, transaction semantics, handwritten external contract parser or unavailable-tool pass claim was introduced. Existing redaction is reused rather than reimplemented.
- At report creation the independent code report was not yet present. The evidence directory was inspected and the direct pass above supports this report; no additional blocker is inferred from that temporary report absence. The fixed candidate needs fresh same-tree code/gate reports with explicit programming/slop coverage.

## Checked artifacts and limits

Checked changed admin `auth.go`, `handlers.go`, `http_errors.go`, `provider_error_test.go`, `provider_error_fixtures_test.go`; changed app `handlers.go`, `server.go`, `http_errors.go`, `provider_error_test.go`; core `provider_error_http_test.go`; relevant `InternalError` references in `docs/api/admin-openapi.yaml` and `docs/api/openapi.yaml`; `docs/evidence/deployment/2026-10-13-http-provider-errors/{README.md,admin-before.log,admin-after.log,app-before.log,app-after.log,real-pg-http.log,contracts.log,snapshot-verify.log}`.

The coordinator's later R02.c matrix drafts and unrelated working-tree edits were not part of this review. Unchanged authentication/extension handlers are not claimed fully audited by this bounded step. Browser testing remains user-owned; real WeChat, hardware, customer License and new M7/M8 features are excluded. This rejection remains historical and cannot serve as an approval for a fixed future tree.

No ulw-loop attempt directory is available in this session; the report uses the required fallback path.
