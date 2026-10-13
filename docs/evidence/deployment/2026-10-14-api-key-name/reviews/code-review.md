# API key name boundary — independent code review

Date: 2026-10-14. Reviewer: `/root/api_key_name_code_review`; did not edit the candidate or its tests. Disposition: **APPROVE**.

- `codeQualityStatus`: **CLEAR**
- `recommendation`: **APPROVE**
- `blockers`: **[]**
- `reportPath`: `.omo/evidence/api-key-name-code-review.md`

## Scope, authority and identity

Goal: repair existing R41 API key issuance so the published `APIKeyCreate.name.maxLength: 128` accepts 128 Unicode characters and rejects 129 without partial writes, while retaining credential privacy. Success criteria include actual authenticated HTTP status and response contracts, matching persisted name/encrypted secret/issuance audit, and unchanged state on rejection.

Read `docs/README.md`, `docs/CONTRIBUTING.md`, the relevant PLAN/EXTENSIONS/ACCEPTANCE requirements, and `docs/api/admin-openapi.yaml:2041`. The contract at line 2045 specifies minLength 1/maxLength 128; an independent OAS30Validator request oracle accepts ASCII128 and Unicode128 and rejects both 129 cases. The existing PostgreSQL constraint at `internal/migrate/sql/012_open_api.sql:4` also counts characters. No intentional byte bound was found for this field.

Reviewed Git **tree** `ed39d630a9fc346e3076504ab761c7e1ed7bcddf`, against base commit `fa802deb10a9c6a1da8dd4e13c4a5065296357b0` (independently resolved `v0.0.39^{commit}`). Execution used `/tmp/iolink-api-key-name-20261014-v2`, not the dirty root checkout. Independently recomputed Git blob hashes for all 1,470 exported tracked blobs before and after verification: zero mismatches. No extra Go files were present. Gitlink `web` remains `39299dfb0d7771690117a2765fb766a9b715f554`; the 24 copied ignored `web/dist` assets matched the root bytes. Unrelated AGENTS/playback edits are excluded from this review.

Full pinned diff has 15 paths: two Go files and 13 archived evidence files. Production changes consist of the `unicode/utf8` import and the guard at `internal/core/api_keys.go:23`; new behavioral coverage is `internal/core/api_key_name_contract_test.go`. No API schema, migration, other name field or public signature changed. The decoded `source-diff.json` applies in reverse with `git apply --check --reverse` against the exact export (exit 0); its patch order differs from the default sorted Git diff, which does not change the patch content.

| Changed source | SHA256 |
|---|---|
| internal/core/api_keys.go | a83690deedb1dcb7182dd520d64f7962147ae967e2e43d2575a44eb5ec7a8076 |
| internal/core/api_key_name_contract_test.go | bd911f69bc332c61b82380e12197ce945a3d74e273562b6c1ffc1fbf98759c3d |

`omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`, so the requested fallback report path is used. No current-scope notepad was supplied; the archived README includes the worker's debug journal, which was inspected as evidence rather than authority.

## Skill perspective check

**Ran** the `remove-ai-slops` and `programming` checks after directly loading:

- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/remove-ai-slops/SKILL.md`
- `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.32/skills/programming/SKILL.md`
- Programming Go README, testing reference and data modeling reference.

**The diff violates neither perspective.** The production change uses the standard library at the existing shared issuance validation seam; it introduces no extraction, parsing layer, normalization, defensive layer, abstraction, dependency or untyped escape hatch. Caller inspection found the guard is used by `IssueAPIKey`, reached by the actual user API handler and service callers. The existing transaction/authorization/encryption/audit flow is unchanged.

The test is a contract regression, not a deletion test, prose test, implementation constant pin or tautology. Its table values come from the published boundary; success is observed through actual HTTP and PostgreSQL. Separate subtests cover accepted and rejected boundaries, then metadata privacy. No production `RuneCount` result supplies an expected outcome. Restoring only the old guard independently made this unchanged test fail on the user-visible defect. The new file is 95 pure LOC; the production file is 218 pure LOC by the skill's nonblank/noncomment measure. No new oversized module or speculative helper was introduced.

Explicit overfit/slop pass: excessive or useless tests, deletion-only tests, tests merely verifying a requested removal, tautological tests, implementation-mirroring tests, and brittle prompt/prose tests are all absent from this diff. Unnecessary production data extraction, parsing and normalization are also absent. The persistence and credential assertions exercise the applicable R41 behavior for the newly accepted boundary requests.

## Verification and evidence inspected

Environment independently checked: Go 1.26.8 linux/amd64, Python 3.14.6, PostgreSQL 16.15, TimescaleDB 2.30.2. Assigned isolated test host: `127.0.0.1:33127`, container `iolink-http-contract-pg-20261013`, role `iolink`; credentials omitted. Fixture inspection confirmed unique per-test databases with cleanup, production migrations and services, real login JWTs, a signed local test License with `openapi`, the test vault and a real ephemeral loopback TCP server.

Inspected `key_open_contract_fixture_test.go:110`: status is assigned from `resp.StatusCode`, compared with the independent expected status via `t.Errorf`, and actual response bytes/status/content type are sent to the contract validator. The test's early return on an unexpected success-path status cannot hide failure. `http_contract_helpers_test.go:16` passes the actual response via stdin and fails on validator rejection. `state` at fixture line 162 derives a PostgreSQL digest; new rejection checks also compare actual key and issued-audit counts.

Independent commands executed in the exact export, with the supplied disposable `IOLINK_TEST_PG_DSN`, `GIN_MODE=release`, and `IOLINK_CONTRACT_PYTHON=/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.venv/contracts/bin/python`:

```sh
go build ./...
go vet ./...
go test -race -shuffle=on -count=1 -timeout=3m -v ./internal/core -run '^(TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity|TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites)$'
```

Build and vet each exited 0 with no diagnostic output. The two selected real-PG tests passed, no skips, exit 0, package duration **7.368s**, shuffle seed **1791923374382544535**. Actual outcomes:

| Scenario | HTTP status | Key delta | Issued audit delta |
|---|---:|---:|---:|
| ASCII128 | 201 | 1 | 1 |
| Unicode128 (384 UTF-8 bytes) | 201 | 1 | 1 |
| ASCII129 | 400 | 0 | 0 |
| Unicode129 (387 UTF-8 bytes) | 400 | 0 | 0 |

The green test also checked exact returned/stored names, nonempty encrypted storage and nonce, a 32-byte one-time secret, rejection state equality, GET list/audit credential omission, and the real diagnostic sink. Existing invalid-body/resource cases also passed. The fixture configures console=error, while `internal/observability/logging.go:122` independently configures the JSON file dump at debug level. Privacy checks read that actual dump for the exercised requests; this does not certify unexercised paths or external environments.

Independently reproduced RED using an **in-memory Go overlay**, leaving all candidate source and tests unchanged. Baseline source was read using `git show fa802de:internal/core/api_keys.go`; a byte comparison proved it equals the candidate with only the import/guard reversed. Linux `memfd_create` held baseline source and the Go overlay JSON, then the same boundary test ran with:

```sh
go test -overlay=/proc/<review-wrapper-pid>/fd/<overlay-fd> -race -shuffle=on -count=1 -timeout=3m -v ./internal/core -run '^TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity$'
```

Actual Go exit **1**, package duration **2.482s**, shuffle seed **1791923456061593514**, specifically Unicode128 actual HTTP400 versus expected201; ASCII128 and both 129 cases remained correct. The review wrapper exited 0 only after asserting the expected failing Go exit and unchanged candidate bytes. This provides direct evidence that the test catches the original bug.

Independent request-schema oracle loaded `APIKeyCreate` from the exact exported YAML and ran `OAS30Validator.iter_errors` on the four real request shapes. Output:

```text
ascii_128: characters=128 bytes=128 valid=True keywords=[]
unicode_128: characters=128 bytes=384 valid=True keywords=[]
ascii_129: characters=129 bytes=129 valid=False keywords=['maxLength']
unicode_129: characters=129 bytes=387 valid=False keywords=['maxLength']
independent APIKeyCreate request schema boundary oracle PASS
```

Archived artifacts inspected under `docs/evidence/deployment/2026-10-14-api-key-name/`: README, source hashes/raw patch JSON, frozen file list, environment, request-schema, red/red-toggle/green, final-focused, run script, format and diff-check. Root supplemental artifacts inspected: `snapshot-build-vet.log`, `snapshot-related.log`, `snapshot-contracts.log`. The related log ends in exit-success output at 57.233s, with **26 top-level PASS and one explicit browser SKIP**; the static contract log reports **96 target operations / 621 synthetic fixtures**. Those wider runs were inspected, not rerun by this reviewer. The independent build/vet runs above provide separate terminal evidence despite the build/vet archive's terse output. The previous milestone's 33-package result was not counted as evidence for this guard.

Project-wide strict linters/nilaway/LSP/security scanners were not run by this reviewer; no such new gate or tooling change is in scope. Compiler/vet and the relevant race/real-PG/HTTP contract gates passed. No full repository test run, real browser, external License delivery, hardware, complete R41/R02 phase acceptance or all possible Unicode input combinations are certified.

## Findings by severity

- **CRITICAL:** None.
- **HIGH:** None.
- **MEDIUM:** None.
- **LOW:** None.

No misleading milestone completion or skipped-as-passed claim was found in the reviewed archive. The archive explicitly retains review/release pending and bounded software scope. Browser and external acceptance remain outside this approval.

**Blockers: none. Recommendation: APPROVE the pinned tree for this small API key name boundary milestone.**

## Independent GREEN output

The following captured output is part of this report artifact, not a planned command.

```text
-test.shuffle 1791923374382544535
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128
    api_key_name_contract_test.go:42: POST /api-keys characters=128 bytes=128 status=201 key_delta=1 issued_audit_delta=1
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128
    api_key_name_contract_test.go:42: POST /api-keys characters=128 bytes=384 status=201 key_delta=1 issued_audit_delta=1
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129
    api_key_name_contract_test.go:42: POST /api-keys characters=129 bytes=129 status=400 key_delta=0 issued_audit_delta=0
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129
    api_key_name_contract_test.go:42: POST /api-keys characters=129 bytes=387 status=400 key_delta=0 issued_audit_delta=0
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys
    api_key_name_contract_test.go:85: GET /api-keys status=200 one_time_secret_absent=true
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit
    api_key_name_contract_test.go:85: GET /api-keys/audit status=200 one_time_secret_absent=true
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity
    api_key_name_contract_test.go:101: diagnostic_dump_credentials_and_request_name_absent=true
--- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity (2.55s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128 (0.26s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128/response_contract (0.25s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128 (0.26s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128/response_contract (0.25s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129 (0.24s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129/response_contract (0.24s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129 (0.24s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129/response_contract (0.23s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys (0.24s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/response_contract (0.23s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit (0.24s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit/response_contract (0.24s)
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/unknown
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/unknown/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/trailing
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/trailing/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_name
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_name/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_scopes
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_scopes/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_resources
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_resources/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_resource_array
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_resource_array/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/zero_resource
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/zero_resource/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/foreign_resource
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/foreign_resource/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/unknown_nested
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/unknown_nested/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/duplicate_name
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/duplicate_name/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/duplicate_scopes
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/duplicate_scopes/response_contract
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/wrong_case
=== RUN   TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/wrong_case/response_contract
--- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites (3.75s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/unknown (0.24s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/unknown/response_contract (0.23s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/trailing (0.23s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/trailing/response_contract (0.23s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_name (0.23s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_name/response_contract (0.23s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_scopes (0.24s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_scopes/response_contract (0.24s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_resources (0.23s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_resources/response_contract (0.23s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_resource_array (0.25s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/null_resource_array/response_contract (0.24s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/zero_resource (0.23s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/zero_resource/response_contract (0.23s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/foreign_resource (0.25s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/foreign_resource/response_contract (0.24s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/unknown_nested (0.23s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/unknown_nested/response_contract (0.23s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/duplicate_name (0.23s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/duplicate_name/response_contract (0.23s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/duplicate_scopes (0.24s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/duplicate_scopes/response_contract (0.23s)
    --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/wrong_case (0.24s)
        --- PASS: TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites/wrong_case/response_contract (0.24s)
PASS
ok  	git.hyhy.fun/rsplab/iolink/internal/core	7.368s
reviewer_terminal_exit_code=0
```

## Independent baseline-guard RED output

```text
INDEPENDENT BASELINE GUARD RUN: in-memory Go overlay from fa802de, exact candidate test unchanged
-test.shuffle 1791923456061593514
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128
    api_key_name_contract_test.go:42: POST /api-keys characters=128 bytes=128 status=201 key_delta=1 issued_audit_delta=1
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128
    api_key_name_contract_test.go:38: POST /api-keys status=400 want=201
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128
    api_key_name_contract_test.go:42: POST /api-keys characters=128 bytes=384 status=400 key_delta=0 issued_audit_delta=0
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129
    api_key_name_contract_test.go:42: POST /api-keys characters=129 bytes=129 status=400 key_delta=0 issued_audit_delta=0
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129
    api_key_name_contract_test.go:42: POST /api-keys characters=129 bytes=387 status=400 key_delta=0 issued_audit_delta=0
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys
    api_key_name_contract_test.go:85: GET /api-keys status=200 one_time_secret_absent=true
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit
=== RUN   TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit/response_contract
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit
    api_key_name_contract_test.go:85: GET /api-keys/audit status=200 one_time_secret_absent=true
=== NAME  TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity
    api_key_name_contract_test.go:101: diagnostic_dump_credentials_and_request_name_absent=true
--- FAIL: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity (2.42s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128 (0.25s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_128/response_contract (0.23s)
    --- FAIL: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128 (0.23s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_128/response_contract (0.23s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129 (0.24s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/ascii_129/response_contract (0.23s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129 (0.24s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/unicode_129/response_contract (0.23s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys (0.24s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/response_contract (0.24s)
    --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit (0.24s)
        --- PASS: TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity/secret_privacy/api-keys/audit/response_contract (0.23s)
FAIL
FAIL	git.hyhy.fun/rsplab/iolink/internal/core	2.482s
FAIL
BASELINE_GUARD_EXIT=1
baseline guard mutation proves the regression; candidate file unchanged
reviewer_wrapper_exit_code=0
```
