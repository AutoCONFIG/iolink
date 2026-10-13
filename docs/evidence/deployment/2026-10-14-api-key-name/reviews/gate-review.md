# API key name boundary — independent final gate

Date: 2026-10-14. Reviewer: `/root/api_key_name_gate_review`, independent noneditor. Read-only review; only this report leaf was written.

- recommendation: **APPROVE**
- blockers: **[]** (no failed stated criterion)
- confidence: **high within the bounded name-length fix**
- candidateTree: `ed39d630a9fc346e3076504ab761c7e1ed7bcddf`
- baseCommit: `fa802deb10a9c6a1da8dd4e13c4a5065296357b0` / `v0.0.39`
- exactExport: `/tmp/iolink-api-key-name-20261014-v2`
- notepadPath: none supplied for this scope; worker debug journal in the archived README was inspected.
- attemptDir: unavailable; `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`. Requested fallback report path used.

## Original intent and desired outcome

`originalIntent`: The supplied original brief calls for testing existing functionality through positive and error paths, actual PostgreSQL/TCP surfaces, and private diagnostics, then closing small milestones with two independent same-candidate reviews. This milestone repairs the existing R41 API-key name mismatch discovered in that work; browser work, external License delivery and new M7/M8 work are deferred.

`desiredOutcome`: A user can issue an API key with either an ASCII or CJK name of 128 characters under the current published contract. A 129-character name returns a contract-valid 400 response without creating or modifying key/audit state. Issuance retains matching persisted names, encrypted one-time secrets and audit records; subsequent metadata and audit reads do not expose credentials.

`userOutcomeReview`: The delivered production guard and new regression satisfy that outcome on the real authenticated user API and isolated PostgreSQL. The schema independently accepts both 128-character requests and rejects both 129-character requests. The guard operates at the shared `IssueAPIKey` seam; existing scope, ownership, authorization and transaction behavior is unchanged. This is approval of the small software milestone, not complete R41, R02, browser or external acceptance.

## Stated criteria and direct review

IDs below label the supplied milestone criteria; they do not invent new phase requirements.

| ID | Criterion | Verdict and evidence pointer |
|---|---|---|
| C1 | Follow authoritative character limit of 128 | PASS. `docs/api/admin-openapi.yaml:2041–2047`, migration `internal/migrate/sql/012_open_api.sql:4`, independent request-schema oracle below. The schema counts characters; the database already uses character length. |
| C2 | ASCII128 and Unicode128 succeed through authenticated real HTTP and persist names/credentials/audit | PASS. Independently reproduced 201, key delta 1 and issued-audit delta 1 for both; `internal/core/api_key_name_contract_test.go:16–68`, direct output below, archived `green.log` and `final-focused.log`. Actual response contract validation passed. |
| C3 | ASCII129 and Unicode129 reject atomically | PASS. Independently reproduced 400, key delta 0 and issued-audit delta 0 for both. The test also compares the database digest and checks error-body privacy; source validation precedes transaction opening. `api_key_name_contract_test.go:43–51`, `api_keys.go:61–67`, direct output below. |
| C4 | Preserve one-time secret and diagnostic privacy on the exercised boundaries | PASS. Tests decode the 32-byte issued secret, inspect encrypted storage and nonce, reject credential fields/secrets on list/audit reads, and read the actual diagnostic dump for names, passwords, JWTs and issued secrets. `api_key_name_contract_test.go:54–101`; direct nonempty-dump observation below. |
| C5 | Existing related behavior and guards remain intact | PASS. Independently executed seven real-PG/race tests, covering issuance, malformed inputs, scope/resource ownership, stale membership, rotation/revocation and five signed reads. All passed without skips. The diff changes only the name-length expression in production. |
| C6 | Review exact source and scoped changes; exclude unrelated dirty files | PASS. All 1,470 tracked export blobs matched the candidate tree before and after review. The 24 ignored `web/dist` assets matched root bytes. AGENTS/adminapi server/appapi server blobs equal the base; unrelated playback files are absent. Source diff/hashes verified. |
| C7 | Two independent noneditor approvals for the same candidate, with direct skill perspectives | PASS for this review pair. Companion `.omo/evidence/api-key-name-code-review.md` explicitly APPROVEs the same tree, records independent execution and direct programming/remove-ai-slops passes. This independent report also APPROVEs that tree. Release/merge/tag/push remain the parent's subsequent actions. |

## Exact identity, diff and artifacts

The pinned diff is 15 paths: two Go files and 13 archived evidence files. `internal/core/api_keys.go` adds `unicode/utf8` and replaces the byte count with `utf8.RuneCountInString(name)`. The other source path is the new behavioral test. No API schema, migration, other field, public signature, provider selection or frontend source change is present.

Source SHA256, checked against export and the archived source hashes:

```text
a83690deedb1dcb7182dd520d64f7962147ae967e2e43d2575a44eb5ec7a8076  internal/core/api_keys.go
bd911f69bc332c61b82380e12197ce945a3d74e273562b6c1ffc1fbf98759c3d  internal/core/api_key_name_contract_test.go
```

The `source-diff.json.text` value equals the original `.omo/evidence/api-key-name-boundary/source.diff` losslessly. Candidate-wide `git diff --check base tree` passed. All 1,470 tracked blobs were rehashed after verification with zero mismatches. Unrelated root dirt was never used as the execution source.

Checked authority and source paths: `docs/README.md`, `docs/CONTRIBUTING.md`, relevant portions of `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md:55`, `docs/ACCEPTANCE.md:80`, `docs/api/admin-openapi.yaml:2041`, `internal/migrate/sql/012_open_api.sql`, both changed Go files, `internal/adminapi/api_key_input.go`, `internal/adminapi/api_keys.go`, `internal/core/key_open_contract_fixture_test.go`, `key_open_contract_boundary_test.go`, `http_contract_helpers_test.go`, `m6d_http_helpers_test.go`, `m6d_resource_validation_test.go`, `m6d_resource_http_test.go`, `internal/testdb/testdb.go`, `internal/observability/logging.go`, `dump_handler.go`, and safe-field behavior in `redaction.go`.

Checked executor artifacts under `docs/evidence/deployment/2026-10-14-api-key-name/`: `README.md` (including its scenario/evidence matrix), `environment.log`, `source-sha256.log`, `source-diff.json`, `frozen-files.txt`, `request-schema.log`, `red.log`, `red-toggle.log`, `green.log`, `final-focused.log`, `run-boundary.sh`, `format.log`, `diff-check.log`. Supplemental root artifacts inspected: `snapshot-build-vet.log`, `snapshot-related.log`, `snapshot-contracts.log`. The wider related log ends in PASS at 57.233s with 26 top-level PASS and one explicitly skipped browser test. I inspected that skip and did not count it as success. The prior milestone's complete 33-package run is not verification of this new guard.

Checked companion report: `.omo/evidence/api-key-name-code-review.md`, including its attached independent GREEN/RED outputs, same-tree identity and direct skill review. Its claims were checked against the source, archived artifacts and my own execution; its prose was not substituted for direct verification.

## Independent execution and real-surface matrix

All commands below ran in the exact export. Environment: assigned disposable PostgreSQL at `127.0.0.1:33127`, role `iolink` (credentials omitted); `IOLINK_CONTRACT_PYTHON` set to the existing absolute contracts Python path; `GIN_MODE=release`. Fixture inspection confirms unique per-test databases, automatic cleanup, real migrations and services, actual `/user/v1/login` JWTs, local signed test License and vault prerequisites, and ephemeral TCP HTTP listeners. These are local software prerequisites, not external License delivery evidence.

```sh
go build ./...
go vet ./...
go test -race -shuffle=on -count=1 -timeout=3m -v ./internal/core -run '^(TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity|TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites|TestKeyOpenContractManagementAndFiveSignedReads|TestM6dIssueAndAuthenticateOpenKey|TestM6dResourceValidation_rejectsInvalidAndForeignWithoutWrites|TestM6dRotation_rejectsChangedResourceOwnershipWithoutWrites|TestM6dAdministration_rejectsStaleMembershipWithoutPartialWrite)$'
make verify-contracts DOCS_PYTHON='/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.venv/contracts/bin/python'
gofumpt -l internal/core/api_keys.go internal/core/api_key_name_contract_test.go
```

Build and vet exited 0 without diagnostics. The seven-test run exited 0, no skips, package duration **12.974s**, shuffle seed **1791923338856254495**. Contracts passed **96 target operations / 621 synthetic fixtures**. Formatting listed no files. An initial make invocation using only `IOLINK_CONTRACT_PYTHON` failed because this Make target uses `DOCS_PYTHON`; direct script execution and the corrected make invocation both passed. No failed startup attempt was counted as a passed run.

Independent real-surface results (actual response contracts passed in every row):

| Scenario | Actual result |
|---|---|
| ASCII128 POST | 201; key delta 1; issued-audit delta 1; matching returned/stored name; valid one-time credential and encrypted storage |
| Unicode128 POST, 384 UTF-8 bytes | 201; key delta 1; issued-audit delta 1; matching returned/stored name; valid one-time credential and encrypted storage |
| ASCII129 POST | 400; both deltas 0; database state digest unchanged; private error response |
| Unicode129 POST, 387 UTF-8 bytes | 400; both deltas 0; database state digest unchanged; private error response |
| GET key list | 200; issued secrets and credential fields absent |
| GET key audit | 200; issued secrets and credential fields absent |
| Existing invalid issuance matrix | Unknown/trailing/duplicate/wrong-case/null fields, zero/foreign resources, duplicate scopes reject with unchanged key/audit state |
| Existing issuance and signed reads | Issuance, five real signed resource reads, rotation, old-key rejection, new-key acceptance, revocation and audit all pass |
| Core policy/ownership failures | Foreign/invalid resource, ownership change before rotation and stale membership reject without partial write |

The independent schema oracle loads the exact exported `APIKeyCreate` and uses OAS30Validator on actual request shapes:

```text
ascii_128 128 128 True []
unicode_128 128 384 True []
ascii_129 129 129 False ['maxLength']
unicode_129 129 387 False ['maxLength']
request schema oracle PASS
```

I inspected both archived RED runs: Unicode128 alone failed with actual400/want201 and exit1. The companion also directly reproduced this using an unchanged candidate test and an in-memory baseline-source overlay, then attached its output. My direct GREEN run independently reproduced the repaired behavior.

To resolve whether diagnostic absence was vacuous, I ran the named boundary test again while an external read-only observer sampled its temporary file. Go exited 0, package duration **3.548s**, shuffle seed **1791923407673436513**. Observed output:

```text
observed_dump_signal={"bytes": 3539, "records": 16, "http_records": 8, "db_records": 0, "levels": ["DEBUG", "INFO", "WARN"], "known_names_absent": true}
exit_code=0
```

This confirms the actual exercised dump contained records. `observability.New` creates a debug-level file handler; Level:error selects the console threshold. Thus the error-console fixture does not silently remove INFO/WARN/DEBUG records from the file. The regression itself checks JWTs/passwords/issued secrets/request names; the external observation prints only aggregate safe metadata. This does not prove every possible logging path.

## Direct programming and remove-ai-slops pass

Consulted the exact available skills: `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/programming/SKILL.md`, its Go README/testing/error-handling references, and `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/remove-ai-slops/SKILL.md`.

The production deletion ladder stops at standard-library substitution in the existing guard. Caller search confirms one shared `IssueAPIKey` validation seam; no sibling handler patch is needed. The range guard remains at a trust boundary and returns the existing typed sentinel. Context, authorization, resource ownership, encryption and transaction/audit operations are preserved. No source logging fields or messages were added. Production and new test pure LOC are 218 and 95, respectively.

| Requested overfit/slop class | Direct verdict |
|---|---|
| Excessive or useless tests | Absent. Four rows distinguish accepted/rejected contract boundaries; reused integration fixtures exercise the actual requested HTTP/Persistence/privacy outcome. |
| Deletion-only or requested-removal tests | Absent. No removal behavior or textual absence pin exists; the tests observe issuance/rejection and credential privacy. |
| Tautological tests | Absent. Expected HTTP outcomes are fixed from the independently validated contract. Actual status, response bytes and SQL outcomes come from the running services. |
| Implementation-mirroring tests | Absent. No assertion checks imports, source expressions or calls to RuneCount. RuneCount in the test is diagnostic only, not the expected-outcome oracle. |
| Unnecessary production extraction/parsing/normalization | Absent. One standard-library length expression replaces the incorrect byte expression. No new helper, parser, normalizer, interface or dependency. |

Also inspected obvious comments, defensive redundancy, complexity, abstraction, layer boundaries, dead code, duplication, performance and missing behavior coverage. No new maintenance burden, false-confidence test or scope drift violates a stated criterion. Given/When/Then markers describe the fixture and each observable subtest. Stateful subtests deliberately run sequentially; tests use ephemeral servers and isolated databases and passed with race/shuffle enabled. Credential/privacy checks observe consequences of this same issuance flow and are not unrelated assertions.

The companion report explicitly records the same skill perspectives, no production extraction/parsing/normalization, behavioral regression versus deletion/prose/constant pins/tautology, no production RuneCount-derived expected result, measured file sizes and real HTTP/PostgreSQL coverage. My direct pass above supplies every requested class independently; report coverage never replaces that pass.

## Exact evidence gaps and notes

No required artifact or failed behavior remains for C1–C7. No new source fix is requested.

- Full repository PostgreSQL testing was not rerun by this gate. My direct seven related tests plus exact-source build/vet/contracts and inspected wider 26-test evidence support this one validation seam. Earlier full-suite evidence is not claimed for the changed guard.
- Browser acceptance is explicitly skipped in the wider log and deferred by the supplied brief. There is no browser pass, complete R41/R02 phase claim, external License delivery, hardware or capacity claim here.
- The new test does not assert a nonempty dump; I resolved that potential false-confidence gap by observing its actual nonempty dump. Privacy is bounded to exercised requests, not every future logging call.
- The state digest covers key metadata, audit count and selected device state; it is not a whole-database snapshot. The guard precedes transactions, and direct key/issued-audit counts plus preserved transaction code support the stated rejection atomicity.
- Project-wide strict linters/nilaway/LSP/security scanners were not introduced or run. Repository-relevant compiler/vet, formatter, actual contracts and selected race/PostgreSQL gates passed. An unrequested tooling conversion is not a criterion blocker.
- Unicode grapheme clusters, every name field, all Unicode combinations and unrelated hardening were not certified. The production code now counts Unicode code points consistently with the current schema and database.
- The archived README still says independent review/release are pending. It is a historical executor record, not a false completed-stage claim. This report and the companion supply the review pair; the parent must preserve the pinned source identity for release.

**Final recommendation: APPROVE the exact pinned tree for the small existing API-key name boundary milestone. No blockers.**
