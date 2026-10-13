# Existing HTTP baseline: final bounded independent gate review

recommendation: APPROVE

codeQualityStatus: WATCH

blockers: []

Exact candidate tree: `91926edee43fb1d05e3ea32e914f44149259d5c7`

Base: `4fffc4b` (v0.0.36)

Export: `/tmp/iolink-baseline-http-r2-20261014`

Reviewer: `/root/latest_gate_review`; no implementation, fixture, exported-source, archived-evidence or staging edits. Only this report was written.

**Approval scope:** the actual original-35 positive workflows and named negative cases in this evidence set, plus the three fixes identified by the initial reviews. This is not approval of all possible baseline failure paths, all inputs, whole R02.c/R02 or the later platform matrix. The known follow-ups below must remain open in IMPLEMENTED.

## Findings and resolved blockers

No blocker remains within that stated test/fix scope.

1. The initial gate's C1 common-error discrepancy is resolved. Both contracts now encode their already documented common response identifiers with per-response enums; affected baseline handlers/auth paths emit those values. Generic Error, License-specific and camera-specific envelopes remain separately scoped. The former `bad id`, `invalid credentials` and `tenant action forbidden` counterexamples are independently rejected with `enum`, rather than accepted by string-only schemas.
2. The initial code review's malformed-query finding is resolved for the identified class. Standard `ParseQuery` errors are checked before using defaults/filters. Six original management collection and four app query surfaces now return 400/invalid_request for the exercised bad escapes; they do not silently discard those pairs.
3. The initial code review's WeChat trust-boundary finding is resolved. Non-2xx HTTP responses fail before identity JSON can be trusted, including responses carrying an openid. The recognized 40029 invalid-code sentinel yields 401/unauthorized; the other exercised exchange failures yield 502/upstream_error. The actual failed exchanges create no user and retain safe response/downstream request correlation.

## Original intent and desired outcome

originalIntent: continue repairing and testing existing IoLink functionality autonomously, use actual data/operation/error evidence and independent double review, and keep incomplete acceptance visible.

desiredOutcome: close these observed contract/input/upstream failures in the existing management-25/app-10 workflow and its named adverse cases using source-bound real TCP HTTP/PostgreSQL tests, while preserving input rejection, zero writes, viewer denial, ownership, rule behavior and diagnostic privacy.

userOutcomeReview: the final snapshot satisfies this bounded outcome. Independent replay executes both positive matrices, the existing negative/data-state cases and all new query/upstream cases against actual handlers/storage. The coordinator is explicitly continuing the shared-auth and broader input matrix; this approval cannot be used to mark V1d, the original baseline's exhaustive acceptance, whole R02 or all 96 live operations complete.

## Acceptance matrix

| Criterion | Result in approved scope | Evidence |
|---|---|---|
| C1: selected positive workflows and named adverse responses match the current contract, including fixed common machine codes | PASS | actual 25+10 workflows, strict response validator, independently rejected former counterexamples |
| C2: declared ambiguous JSON/range/query cases reject before relevant writes/exchange | PASS | live unknown/case/duplicate/trailing/null/nonpositive/overflow cases, SQL state checks, admin6/app4 bad-escape requests |
| C3: tested authorization/ownership and atomic mutations retain their required outcomes | PASS | viewer write 403/state unchanged, cross-resource 404, invalid-request digests, batch rollback; broader auth-storage failure scope excluded |
| C4: actual success data, pagination/filtering and enabled:false behavior are correct in tested cases | PASS | ingested latest/history/alarm, device pages and empty totals, disabled/reenabled rule, explicit existing Enabled:true regression fixtures |
| C5: exercised real provider failures and CLI/logging boundaries preserve privacy and correlation | PASS | genuine SQL failures, seven controlled HTTP WeChat/real-PG failures, safe stdin diagnostics and no failed-login user rows |
| C6: exact-source regression/provenance and limitations are accurately recorded | PASS | 1,331 matching blobs, independent runs, waited terminal full-suite result, same-tree companion approval, retained initial rejections/failures/type limitations |

## Independent final-source verification

- Compared every tracked exported blob against NUL-delimited `git ls-tree -rz` entries and `git hash-object`: **1,331 matched**, zero mismatches.
- Inspected the complete initial production/test diff and final correction delta, including schemas, shared non-camera auth responses, query helpers, WeChat adapter, strict input/pagination SQL and unchanged permission/atomicity assertions. Next-platform drafts, playback and user AGENTS edits are excluded from the snapshot.
- The positive matrix selections contain **25 unique management** and **10 unique app method/route pairs**, counted directly in the actual workflow source. These workflows run against `/user/v1` for the tenant-facing management resource family and `/api/v1` for app resources; this is not a platform-administrator permission bypass.
- Independently ran from the final export:

```text
GIN_MODE=release IOLINK_TEST_PG_DSN=<dedicated Timescale DSN> IOLINK_CONTRACT_PYTHON=<repository contracts Python> go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestBaseline'
```

  Exit 0, core **77.743s**, shuffle seed **1791912479294510393**. **17 top-level tests** passed, no SKIP/FAIL/DATA RACE. Management positive matrix 8.69s; app positive matrix 3.31s. The full named baseline test set ran, including zero-write/batch, permissions, real SQL failures, rule state, device filters, login privacy, bad encoding and upstream failures.
- Independently observed all seven WeChat cases through actual TCP provider/app servers and real storage: HTTP200 invalid code ->401; HTTP200 busy ->502; HTTP503 with openid ->502; HTTP400 with invalid-code JSON ->502; malformed HTTP200 body ->502; missing-openid HTTP200 ->502; refused provider TCP/status0 ->502. Each case verified unchanged user rows, its exact public machine code, no secret/provider-body/URL leakage and identical generated response/downstream request IDs.
- Independently supplied the former three common-error counterexamples to the final candidate validator: all now return invalid with `enum`. The exact same inputs had been accepted on dffad35, as recorded in the initial report.
- Independently ran `python -m pytest -q scripts/test_validate_http_response.py`: exit 0, **29 passed in 2.73s**. CLI privacy, malformed input, finite numbers, media/selection/body contradictions, type/required/additional properties and wrong common machine-code tests passed.
- Independently ran `scripts/check_contracts.py`: exit 0, **96 operations / 584 synthetic fixtures** passed. This is static schema/fixture verification, not the complete 96-operation live matrix.
- `gofumpt -l` on all **37 changed Go files** produced no paths; `git diff --check` against the base was clean.
- Verified the archived initial gate report is byte-identical to `.omo/evidence/baseline-http-gate-review.md`. The unchanged generated dist prerequisite exists in the final export; index SHA256 matches README: `a2cf487d2b7cd9c2486ae55e386d12f519fbfaa99fb1dab4c21a1817dd2ee632`.

## Required full-suite wait and companion report

I waited for the final full-suite terminal result and the same-tree code review before issuing this approval. Root confirmed run **95867 exit 0** for final `make verify`: build/vet/race, **33 tested Go packages**, core **232.874s**. I inspected `final-snapshot-verify.log`: 33 successful package lines and no FAIL/DATA RACE. Root also confirmed focus run **8464 exit 0** and Python/static run **68247 exit 0**; final logs show 110 selected Python tests and 96/584 static fixtures. These are coordinator observations, not a second independent full-suite run. My independent runs/counts above remain separate.

Read `.omo/evidence/baseline-http-code-final.md`: explicit **APPROVE** for this exact 91926 tree, blockers empty, codeQuality WATCH, the same bounded scope and known follow-ups. It explicitly records both required skill perspectives and adversarial/overfit checks. Its report coverage was checked after my own direct pass; it did not substitute for it.

## Direct programming/remove-ai-slops pass

Both skills and Python/Go/error-handling/testing guidance were loaded/consulted during this reviewer session and applied to the complete original changes and final delta.

- Standard error-returning query parsing is used at a real untrusted boundary; no new parser framework or speculative dependency was added. Exact JSON field/duplicate/trailing checks address standard decoder permissiveness and remain necessary. The WeChat status guard checks the actual identity trust boundary and preserves real failures instead of fabricating success.
- Fixed response identifiers are machine-consumed enum constraints already specified by the contract. No permission test was relaxed to match schema permissiveness. Generic/License responses were not forced into a single global enum. Camera-specific auth envelopes retain their original branch; the shared non-camera path now uses common codes.
- Live response, persisted-state, rollback and no-secret assertions are meaningful. No deletion-only, prose-pin, tautological or private-call-count tests identified. Explicit Enabled:true changes preserve earlier alarm tests while the new false/reenable test proves the formerly ignored flag. New WeChat cases independently observe non-2xx precedence and no user provisioning.
- The Python CLI uses typed frozen boundary/result models, fixed local spec selection and stdin for actual response values; diagnostics expose only schema locations/keywords. Dynamic resolver/SDK typing is a disclosed limitation, not hidden with casts/ignores. There is no new unnecessary production extraction/normalization, provider SDK dependency in domain or unrelated business implementation.
- New modules remain within the pure-LOC ceiling; large legacy modules remain existing debt. No unavailable strict-lint/security/nilaway check is represented as passed.

## Notes and still-open acceptance

1. **Shared identity storage failures remain a follow-up:** existing token-version/authority lookup errors can still be collapsed into 401 in shared authentication (`internal/adminapi/auth.go:132`, `internal/appapi/server.go:320`, `:326`). Root reports an actual relation-failure reproduction in its next platform matrix, outside this snapshot's evidence. This approval does not establish that every authenticated provider failure produces 500.
2. **Empty optional numeric filters remain a follow-up:** management handlers can still treat explicitly empty farm_id/pond_id as absent (`internal/adminapi/handlers.go:318`, `:781`). Rejecting malformed escapes does not certify all decoded illegal values. This limitation must not be hidden by claiming an exhaustive negative matrix.
3. **Strict Python typing has not passed:** the twelve recorded diagnostics in `boundary-typecheck.log` remain open/disclosed. They concern dynamic SDK/legacy resolver types and exhaustive unreachable branches; basic Ruff/no-excuse and runtime tests do not count as strict typing success. No concrete additional runtime/privacy defect from them was found in this bounded review.

V1d/whole R02 remain ongoing. Browser interaction is user-owned; real WeChat credentials, hardware, customer License delivery, all 96 live operations, large recovery/long capacity and deferred M7/M8 features are excluded. Earlier dffad snapshot results, selector/prerequisite failures and rejected reports remain historical, not final approval. Final review/log artifacts outside the candidate must be archived with source binding before publishing the milestone record.

No reviewer container/network/volume was created or cleaned. The dedicated PG instance was reused; testdb owns the per-test database cleanup. No ulw-loop attempt directory is available, so this report uses the required fallback path.
