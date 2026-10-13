# API key name boundary — implementation verified, review pending

Scope: R41 existing API key issuance, `internal/core/api_keys.go` and new `internal/core/api_key_name_contract_test.go` only. No index, commit, deployment or shared-resource cleanup.

Authority: docs/README.md → PLAN/PLAN-DETAILS/EXTENSIONS → admin-openapi.yaml APIKeyCreate name minLength 1/maxLength 128 → ACCEPTANCE R41. No intentional byte bound found in these authority files.

No active currentAttemptDir: `omo-agent-toolkit ulw-loop status --json` returned ULW_LOOP_PLAN_MISSING. Evidence belongs here.

## Result

Confirmed and repaired an existing R41 interface defect: the core counted UTF-8 bytes although `APIKeyCreate.name.maxLength: 128` counts characters. Production changes are only an import and replacement of the byte count with `utf8.RuneCountInString`. No contract or other name field changed.

Real authenticated TCP requests before the fix returned ASCII128=201, Unicode128=400, ASCII129=400, Unicode129=400. After the fix, both 128-character names return 201 and persist matching names, encrypted secrets and one issuance audit each; both 129-character names return 400 with zero key/audit changes and an unchanged database state digest. The same valid Unicode request was reproduced red twice, including a rollback of only the guard, and green twice.

This is a bounded software fix. Independent review and release belong to the caller; no M6d/R41/R42 phase completion, browser, external License delivery or hardware acceptance is claimed.

## Environment and invocations

`environment.log` records Go 1.26.8, Python 3.14.6, PostgreSQL 16.15 and TimescaleDB 2.30.2. The already-running isolated container is `iolink-http-contract-pg-20261013`, host `127.0.0.1:33127`, role `iolink`; credentials are omitted. Existing `testdb.New` allocates/drops a unique test database. The fixture logs in through actual `/user/v1/login`, installs a local signed test License with `openapi`, configures a local test vault, then serves production user/open routes on a real ephemeral loopback TCP listener. `GIN_MODE=release`. Read-only database version inspection used `docker exec iolink-http-contract-pg-20261013 psql -U iolink -d iolink -Atc "SELECT version(); SELECT extversion FROM pg_extension WHERE extname='timescaledb';"`.

For all runs, export the assigned `IOLINK_TEST_PG_DSN` and `IOLINK_CONTRACT_PYTHON=/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.venv/contracts/bin/python`.

RED, GREEN and reverted-guard RED invocation: `bash .omo/evidence/api-key-name-boundary/run-boundary.sh`, which runs `go test -race -shuffle=on -count=1 -timeout=3m -v ./internal/core -run '^TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity$'`.

Final invocation, with the same DSN/Python and `GIN_MODE=release`:

```sh
go test -race -shuffle=on -count=1 -timeout=3m -v ./internal/core -run '^(TestAPIKeyNameContract_countsCharactersAndPreservesAtomicity|TestKeyOpenContractInvalidIssuanceHasNoKeyOrAuditWrites|TestKeyOpenContractManagementAndFiveSignedReads|TestM6dIssueAndAuthenticateOpenKey|TestM6dResourceValidation_rejectsInvalidAndForeignWithoutWrites|TestM6dRotation_rejectsChangedResourceOwnershipWithoutWrites|TestM6dAdministration_rejectsStaleMembershipWithoutPartialWrite)$'
```

Final result: seven top-level tests passed, no skips, exit 0, `ok git.hyhy.fun/rsplab/iolink/internal/core 13.060s`. No full repository PostgreSQL suite was run for this bounded fix.

Request-schema oracle invocation captured in `request-schema.log`:

```sh
.venv/contracts/bin/python - <<'PY'
from pathlib import Path
import yaml
from openapi_schema_validator import OAS30Validator
spec = yaml.safe_load(Path('docs/api/admin-openapi.yaml').read_text())
validator = OAS30Validator(spec['components']['schemas']['APIKeyCreate'])
for label, name, expected in [('ascii_128', 'a'*128, True), ('unicode_128', '界'*128, True), ('ascii_129', 'a'*129, False), ('unicode_129', '界'*129, False)]:
    errors = list(validator.iter_errors({'name':name, 'scopes':['ponds:read']}))
    valid = not errors
    print(f'{label}: characters={len(name)} bytes={len(name.encode())} valid={valid} keywords={[error.validator for error in errors]}')
    assert valid == expected
print('APIKeyCreate request schema boundary oracle PASS')
PY
```

## Evidence ledger

All artifact paths below are relative to `.omo/evidence/api-key-name-boundary/`.

| Criterion / exact scenario | Invocation | Binary observable | Captured artifact |
|---|---|---|---|
| Request contract character semantics: ASCII/Unicode 128/129 | contracts Python loads current `admin-openapi.yaml`, constructs `OAS30Validator(APIKeyCreate)`, calls `iter_errors({'name': name, 'scopes': ['ponds:read']})` for `'a'*128`, `'界'*128`, `'a'*129`, `'界'*129` | 128 valid True; 129 valid False with only maxLength keyword | `request-schema.log` |
| Regression fails before fix and when only guard restored | boundary invocation above, `unicode_128` | actual HTTP400 versus expected201, zero key/audit delta, test exit1 both times | `red.log`, `red-toggle.log` |
| ASCII128 and Unicode128 issue valid keys | boundary/final invocations above, `ascii_128`, `unicode_128` | HTTP201; exact returned/stored name; nonempty key ID; decoded one-time secret 32 bytes; nonempty encrypted storage/nonce; key_delta=1 and issued_audit_delta=1 | `green.log`, `final-focused.log` |
| ASCII129 and Unicode129 reject atomically | same invocations, `ascii_129`, `unicode_129` | HTTP400; key_delta=0, issued_audit_delta=0, before/after database digest equal, no input/secret in error body | `green.log`, `final-focused.log` |
| Actual HTTP response contracts | each scenario's `response_contract`, production response bytes/status/content type piped only via stdin into existing `scripts/validate_http_response.py` | PASS for each actual 201/400/200 response, no validator issues | `green.log`, `final-focused.log` |
| One-time secret privacy on metadata | `secret_privacy/api-keys`, `secret_privacy/api-keys/audit` GET requests | HTTP200; issued secrets and secret/encrypted_secret/secret_nonce fields absent; response schema PASS | `green.log`, `final-focused.log` |
| Request/credential log privacy for exercised boundaries | same boundary test, closes injected observability sink and checks actual diagnostic dump | JWTs, fixture password, issued secrets and request names absent | `green.log`, `final-focused.log` |
| Existing issuance, rotation, scope/ownership, auth and management behavior | final seven-test invocation above | seven top-level PASS, exit0, no skipped test | `final-focused.log` |
| Minimal source and formatting | `gofumpt -l internal/core/api_keys.go internal/core/api_key_name_contract_test.go`; `git diff --check -- internal/core/api_keys.go` | no format filenames or diff errors, exit0 | `format.log`, `diff-check.log`, `source.diff`, `frozen-files.txt`, `source-sha256.log` |

Privacy scope: fixture logger uses the existing error-level sink. This checks absence from the real sink during these requests; it does not claim a new diagnostic emission or an unavailable external environment.

## Frozen ownership

`frozen-files.txt` lists exactly the two assigned Go files. `source-sha256.log` binds their final bytes; `source.diff` captures the guard change and new regression test. Source checksums:

```text
a83690deedb1dcb7182dd520d64f7962147ae967e2e43d2575a44eb5ec7a8076  internal/core/api_keys.go
bd911f69bc332c61b82380e12197ce945a3d74e273562b6c1ffc1fbf98759c3d  internal/core/api_key_name_contract_test.go
```

No index write, commit, deployment or manual container/network/volume cleanup was performed. Unrelated edits and the shared root debug journal were preserved. A transient unused-import diagnostic during the deliberate rollback was resolved before the RED rerun; final compilation and race run passed.

## Debug journal

Hypotheses:
1. Core counts UTF-8 bytes: ASCII128 succeeds, Unicode128 fails despite same contract character count; rune-count substitution toggles failure.
2. HTTP parser or OpenAPI validator rejects Unicode independently: Unicode128 remains rejected after changing core guard; inspect actual response and request-schema result.
3. Fixture auth/license/vault or database blocks issuance: ASCII128 also fails or lacks persisted key/audit; verify real auth and successful ASCII case.

Retained artifacts: new regression test, minimal guard change, red/green logs, request-schema/environment logs, frozen source file list/hashes, final report here. Runtime resources were only automatically allocated/closed by existing isolated fixtures. No debugging instrumentation remains.

Existing shared `.debug-journal.md` and unrelated edits remain untouched.

## Root archive

The process record above retains its original `.omo/evidence/api-key-name-boundary/` paths. All named log/script/hash artifacts are copied here. The original `source.diff` is preserved losslessly as the `text` value in `source-diff.json`, so patch context whitespace is represented as JSON escapes rather than being confused with source whitespace errors. Decode that value to recover the original patch bytes. The recorded script can be run from the repository root as `bash docs/evidence/deployment/2026-10-14-api-key-name/run-boundary.sh` with the assigned disposable DSN exported.

Base `fa802de` / v0.0.39. Implementation commit `29c85ed`, approved tree `ed39d630a9fc346e3076504ab761c7e1ed7bcddf`, published as v0.0.40. Two independent non-editors explicitly approved the same complete tree: [code review](reviews/code-review.md), [gate review](reviews/gate-review.md). Both independently matched all1,470 tracked blobs; no AGENTS/playback drafts were included.

Exact-source `make build`, `go vet ./...`, related PG/TCP `go test -race -shuffle=on -count=1 -v ./internal/core -run '^TestAPIKeyNameContract|^TestKeyOpenContract|^TestM6d'`: session79636 terminalexit0, core57.233s,26 top-level PASS. The one optional M6dBrowser test skipped because the user owns browser acceptance; it is explicitly not a passed scenario. `snapshot-build-vet.log`, `snapshot-related.log`, `snapshot-contracts.log` archive actual outputs; static96/621 passed. The previous milestone's complete33-package gate is baseline evidence, not verification of this changed guard. The scope-specific real seven-test independent race run also passed without skips.

The observability file handler captures DEBUG and above, while Level:error configures console output. Gate review independently observed16 actual dump records including8 HTTP records and absence of request names/credentials, so the recorded privacy check is not an empty-output claim. Full R41/R02 and delivery/browser acceptance remain outside this small boundary approval.
