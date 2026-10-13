# Existing 35-operation HTTP contract: independent gate review

recommendation: REJECT

Exact candidate tree: `dffad35ac6a5adc871621929c5a2bea22d9e724e`

Base: `4fffc4b` (v0.0.36)

Export: `/tmp/iolink-baseline-http-20261014`

Reviewer: `/root/latest_gate_review`; read-only implementation/test/evidence inspection. Only this report was written.

## Blockers

- violatedCriterion: C1 — the original 35 operations and their exercised negative responses must satisfy the current authoritative contract, including fixed common error identifiers.
  Observation: the contract's common responses specify `invalid_request`, `unauthorized`, `forbidden`, `not_found` and `conflict`, but baseline handlers/middleware still emit diagnostic phrases instead. For example, app `GET /ponds/0` reaches the new nonpositive-ID guard returning `{"error":"bad id"}` rather than `invalid_request`; management unknown/wrong credentials return `invalid credentials` rather than `unauthorized`; denied management writes return `tenant action forbidden` rather than `forbidden`. Other affected 404 and 409 paths have the same issue. The schema points all responses at an unconstrained string-valued Error, so the new response validator and status-only negative tests accept these incompatible values. Passing counts therefore cannot establish the requested contract closure.
  evidencePointer: candidate `internal/appapi/handlers.go:99`; `internal/adminapi/auth.go:29`, `:40`, `:206`; additional baseline mappings in `internal/adminapi/handlers.go`; `docs/api/openapi.yaml` and `docs/api/admin-openapi.yaml`, `components.responses.BadRequest/Unauthorized/Forbidden/NotFound/Conflict`; `scripts/validate_http_response.py`; `scripts/test_validate_http_response.py:test_generic_error_string_accepted_when_schema_has_no_enum`.
  Required fix: align the baseline response/middleware identifiers with the declared values, encode those values in the relevant response schemas or equivalent strict machine validation, and add failing-then-passing checks for the actual incompatible cases. Preserve viewer write 403, cross-resource 404, rollback/zero-write and secret exclusion assertions. Do not change permissions or replace fixed contract semantics with arbitrary strings to make tests pass. Present the corrected exact snapshot for both independent reviews.

## Original intent and desired outcome

originalIntent: continue testing and fixing existing IoLink features autonomously under AGENTS.md, use actual operation/data/error evidence and independent double review, and avoid declaring incomplete project requirements complete.

desiredOutcome: this milestone closes the current original management 25 plus app 10 operation contract scope through real authenticated TCP HTTP and PostgreSQL, including strict input rejection, no rejected writes, permissions, pagination, rule state, login privacy and safe actual storage failures. Whole R02 and all 96 operations remain outside that completion claim.

userOutcomeReview: the candidate contains substantial valid fixes and real positive/negative tests. The common-error discrepancy prevents the claim that the original 35 operations conform to their complete current contract. Product data/permission tests otherwise passed in independent replay; no failure was concealed by treating skipped or failed setup as passed.

## Criteria matrix

| Criterion | Result | Evidence |
|---|---|---|
| C1: all 35 selected operation responses satisfy the current contract, including negative codes | FAIL | incompatible fixed error values described above; validator accepts them because schema is only string |
| C2: strict baseline request decoding rejects ambiguity before writes/exchange | PASS for declared exercised cases | actual unknown/case/duplicate/trailing/null/range cases, state digests, no user provisioning |
| C3: authentication/ownership/viewer permissions and atomic state are retained | PASS in exercised scope | actual 401/403/cross-tenant 404; 15 management viewer mutations denied with unchanged state; app viewer confirm denied; batch rollback |
| C4: real success workflows, pagination/resource filters and rule enable state are observed | PASS | 25+10 unique positive operations; actual ingested latest/history/alarm, disabled→enabled rule, filtered pages and empty/null responses |
| C5: provider/privacy diagnostics and validator stdin/output are safe | PASS in exercised scope | actual SQL failures, login storage/upgrade checks, public user projection, Python CLI privacy tests; semantic C1 remains separate |
| C6: exact source and relevant regression evidence are honest | PASS with stated limits | 1,312 matching blobs; independent real-PG/HTTP and Python/contracts; full suite inspected plus terminal result reported; failed prerequisites/typecheck clearly retained |

## Independent verification

- Compared every tracked final-export blob against NUL-delimited Git tree entries and `git hash-object`: **1,312 matched**, zero mismatches. Unrelated playback and user AGENTS changes are excluded. The web gitlink stays at the existing source; generated dist/embed prerequisites are separately prepared, not claimed as tracked-source changes.
- Inspected all production and test diffs, anonymous/narrow capability ports, SQL tenant/farm scopes, transaction boundaries, strict JSON decoders and response projection. The app resource-scope regression changes only array-to-page decoding and adds the total check while retaining exact role visibility expectations. Viewer permission expectations are not weakened.
- Counted unique positive method/route selections directly in the two workflow matrices: management **25**, app **10**. This count alone is not treated as approval; both matrices and their negative scenarios were independently executed.
- Independently ran from the exact export:

```text
GIN_MODE=release IOLINK_TEST_PG_DSN=<dedicated Timescale DSN> IOLINK_CONTRACT_PYTHON=<repository contracts Python> go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestBaseline'
```

  Exit 0, core **73.155s**, shuffle seed **1791910722408550136**. **15 top-level tests** passed; no SKIP/FAIL/DATA RACE. Management positive matrix 11.18s; app positive matrix 3.42s. Executed actual schema checks, strict request/no-write cases, viewer 403, cross-resource 404, rule disabled/reenabled behavior, real relation failures, paging/filtering/empty responses and login failure/privacy cases. Their passing C1 assertions are incomplete because wrong error strings satisfy the broad schema.
- Independently invoked the real candidate validator with three concrete incompatible responses: app GET `/ponds/{id}` 400 + `bad id`; management POST `/login` 401 + `invalid credentials`; management POST `/farms` 403 + `tenant action forbidden`. Each returned valid despite the contract's required identifiers. These probes do not print private request/response data; they use only fixed known error literals.
- Independently ran `python -m pytest -q scripts/test_validate_http_response.py`: exit 0, **29 passed in 2.57s**. The tests meaningfully cover malformed stdin, selection/media/body contradictions, nested nonfinite numbers, schema type/required/additional fields and no secret output. One test explicitly documents the current generic-string acceptance that permits the C1 discrepancy.
- Independently ran `scripts/check_contracts.py`: exit 0; **96 operations / 584 synthetic fixtures** passed. The schema changes add actual 403 declarations and remove a duplicate 404 key; they do not weaken role denial. The fixture count increased from 568 because of the added response declarations. Schema-only success does not prove the missing fixed error-value semantics.
- `gofumpt -l` on all changed Go files emitted no paths. `git diff --check` for this candidate diff passed.
- Inspected executor `snapshot-verify.log`: build/vet and 33 tested-package race results completed; root reported session **91217 exit 0** for this exact tree. Root also reported the corrected focused session **60451 exit 0**, 22 top-level tests including admin/app unit/fault cases plus the core baseline scope, and Python session **75708 exit 0**, 110 selected tests. Those are executor results, not a second independent whole-suite run; independently executed counts above remain distinct.

## Failed prerequisites and unavailable validation

`snapshot-build-prerequisite-failed.log` records missing `web/dist/index.html`; `snapshot-focused-prerequisite-failed.log` records core compilation failing with no embeddable dist while some separate package tests ran. Neither is a passed full/focused check. Root rebuilt/prepared dist from the unchanged web source and then reran the frozen source successfully. The evidence must retain the initial failures and distinguish them from the successful terminal runs.

`boundary-typecheck.log` records **12 strict type diagnostics**, including dynamic OpenAPI SDK types, the existing untyped resolver and mandatory exhaustive-match unreachable branches. Strict Python type checking did not pass and must not be represented as passed. In this review it is a maintenance/validation NOTE because the criterion includes honest limitations and the concrete contract failure is C1; runtime behavior is independently tested rather than inferred from typing. Base Ruff/no-excuse/Python tests do not erase those diagnostics.

## Required programming/remove-ai-slops direct pass

Loaded/consulted both skills and the Go/Python, error-handling and testing guidance; performed the direct pass over production changes and tests.

- Input decoding uses typed DTOs/known field sets, duplicate/case/trailing-object rejection and bounded reads. The management token pass adds required behavior beyond standard `DisallowUnknownFields`, which permits case-insensitive field matches and duplicate keys; the second typed decode is therefore not a redundant normalization layer. The app single-code parser similarly checks an actual wire boundary before identity exchange.
- Pond updates check the expected current farm under the transaction/row lock, retain tenant write authorization and rollback; no unsupported farm move is silently performed. User page SQL retains the existing farm-role scope and uses bound query arguments, stable ID order and accurate empty-page total. Provider no-row outcomes are normalized while real SQL errors remain failures. Enabled:false now enters SQL explicitly; older direct-call alarm fixtures set Enabled:true and keep their original alarm/transaction assertions.
- Public login uses an explicit ID/nickname projection. The Python validator uses fixed local spec selection, frozen typed boundary models, stdin for actual response data and only schema keyword/location diagnostics; real values and validator messages are not emitted. Unknown/dynamic SDK typing is explicitly reported instead of ignored.
- No deletion-only, prose-pin or tautological tests identified. The workflows use actual routes/core/storage, persist/requery state and compare meaningful rollback/no-write outcomes. The permission tests assert exact 403 and unchanged state, not schema permissiveness. The positive schema checks are currently insufficient for fixed common error identifiers; that false-confidence gap is the C1 blocker.
- No speculative provider abstraction or new business feature was added. New source modules stay within the pure-LOC ceiling; larger legacy files predate this change. No missing optional lint/security tool was claimed passed.
- At report creation the code reviewer artifact was not yet available. Root relayed the common-error concern, but I independently confirmed it against the candidate production branches, authoritative descriptions, actual test replay and direct validator probes. The evidence directory was inspected; my direct pass supports this report. The corrected snapshot needs fresh independent reports explicitly covering these perspectives.

## Checked paths and scope limits

Checked all changed admin/app input/auth/handler/page/error modules and tests, core admin store/repositories/user paging and integration helper/matrix/login/alarm fixtures, domain page port, Python response validator/tests, API schema diff, CONTRIBUTING and the complete baseline-http evidence README/log bundle. Additional final logs were inspected at the root evidence path outside the original candidate tree and require source binding in final metadata.

This is an existing-baseline milestone review, not whole R02.c/R02 or all 96 live operations acceptance. Browser QA remains user-owned; WeChat exchange is a controlled software identity, not real WeChat. Physical devices, final License delivery, 100GiB restore, long capacity and deferred M7/M8 work are excluded. No reviewer container/network/volume was created or cleaned; dedicated PG test databases are owned/cleaned by testdb.

No ulw-loop attempt directory is available in this session; the report uses the required fallback location.
