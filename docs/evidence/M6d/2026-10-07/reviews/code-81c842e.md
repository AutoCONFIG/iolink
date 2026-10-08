# M6d R41–R42 independent code-quality review — source 81c842e

- **codeQualityStatus:** WATCH
- **recommendation:** APPROVE
- **reportPath:** `docs/evidence/M6d/2026-10-07/reviews/code-81c842e.md`
- **blockers:** none
- **reviewed tree:** source `81c842e6903814cee187e67a699485ef6f934148`; product source `657807e195bcfc80b940706c58ad166a2de7a56f`; web `66c52624f6d97c9dd7fe29b05b64b3db39c4a72d`
- **notepadPath:** none; `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`, so this report uses the required fallback evidence path.

## Goal and scope

The goal is M6d/R41–R42: tenant/resource-scoped API-key issue/list/rotate/revoke/audit; one-time 32-byte secrets encrypted for HMAC verification; live tenant, membership, role, resource and License checks; safe diagnostics; canonical HMAC `/open/v1` signing with the published vector and inclusive ±300-second window; durable ten-minute nonce replay protection; shared deployment-configurable token-bucket defaults of 60/minute and burst 10; published contracts; and the production management UI. Invalid input, replay, scope, tenant, License and rate violations must fail closed without partial writes, and secrets/signed payloads must not enter logs or audit.

The exact source repair diff from prior rejected source `865e9b28f531c1402bcf96a53c8663ad0e2dab89` to `657807e195bcfc80b940706c58ad166a2de7a56f` was inspected: `internal/core/open_auth.go`, `internal/core/open_auth_test.go`, `internal/core/m6d_persistence_http_test.go`, `internal/core/api_keys.go`, `internal/domain/api_keys.go`, `internal/adminapi/api_keys.go`, `internal/adminapi/api_key_error_test.go`, and `docs/api/open-openapi.yaml`. The nested web diff from `3c1d860e2cca0b6ed8207142f5506784f5fef452` to `66c52624f6d97c9dd7fe29b05b64b3db39c4a72d` was inspected: `src/api/admin.ts`, `src/domain/api-key.ts`, `tests/api-key-api.test.ts`, and `e2e/m6d.spec.ts`. The locked tree is clean and detached; no product files were changed during this review.

## Findings by severity

### CRITICAL

None.

### HIGH

None. The prior H1 replay defect is fixed at `internal/core/open_auth.go:163-167`: the nonce insert is committed before returning `OpenRateLimitError`, while the rate bucket remains unconsumed. The exact production HTTP regression in `internal/core/m6d_persistence_http_test.go:69-95` passes: 429, same nonce 401 after refill, fresh nonce 200. The prior UTF-8, internal-error, and demo assertion findings are also repaired and green in the referenced logs.

### MEDIUM

1. **The exact-snapshot full gate is still explicitly unproven.** `docs/evidence/M6d/2026-10-07/README.md:28-35` records the full `make verify` row as `running`, and `logs/final-3-go-verify.log:1-29` contains build/vet/race output but no browser-enabled `TestM6dBrowser_realManagementFlow` result. The focused database/race/shuffle repair run and web run are green (`logs/865-followup-green.log:113-163`, `logs/865-followup-web.log:1-103`), and contracts/architecture/Docker are green (`logs/final-3-contracts-docker.log:1-7` and its build tail). This is an evidence-completeness watch item: do not mark M6d complete until the exact source/web pair has a non-skipped full and production-browser artifact.

2. **The API-key use-case boundary remains a long positional parameter list.** `internal/core/api_keys.go:56` and `internal/adminapi/server.go:107-112` pass tenant, name, scopes, resources and actor as separate values. This is the prior M5 maintainability note; it has no demonstrated runtime or security failure, but a small issue/value object would prevent future call-site mis-ordering.

3. **The new stored-key parser test partly derives its expected value from another production parser.** `web/tests/api-key-api.test.ts:12-15` sets `stored = parseAPIKey(key)` and then compares `parseStoredAPIKeys([stored])` to `[stored]`. The malformed-shape assertions are useful, and the production code now parses localStorage through Zod (`web/src/domain/api-key.ts:31-47`), but the success assertion should use an independent literal camelCase fixture/expected value to avoid sibling-parser drift producing false confidence. This is a test-quality issue only.

No deletion-only, prose-pinning, tautological production, broad-catch, untyped escape-hatch, needless provider abstraction, or unrequired production normalization was found in the repaired diff. The changed demo E2E now checks one-time alert disappearance, revoke state, and audit navigation (`web/e2e/m6d.spec.ts:10-27`) instead of asserting a canned secret literal.

## Skill-perspective check

This check **ran** before judging maintainability and test relevance. I loaded `omo:remove-ai-slops/SKILL.md`, `omo:programming/SKILL.md`, the Go error-handling/testing/concurrency/data-modeling references, the TypeScript reference, and `omo:review-work/SKILL.md`. The repaired source follows the required typed sentinel mapping (`domain.ErrInvalidAPIKey` to 400 and unknown errors to stable 500), parse-at-boundary UTF-8 checks, and transaction/error handling. The only violations found are the medium maintainability/test-quality items above; neither is a release blocker under the requested severity policy.

## Verification and evidence audit

| Check | Result | Evidence |
|---|---|---|
| Reproduce prior failures before repair | **PASS as red baseline** | `logs/865-followup-red.log:1-15` shows invalid UTF-8, nonce-after-429, and infrastructure-error failures on source 865e9b2. |
| Replay after 429 / token preservation | **PASS** | `logs/865-followup-green.log:150-154`; production HTTP test `internal/core/m6d_persistence_http_test.go:69-95`. |
| Invalid UTF-8 query/path rejection | **PASS** | `logs/865-followup-green.log:113-114`; `internal/core/open_auth.go:39-50,74-76` and `open_auth_test.go:18-29`. |
| Internal API-key store failure mapping | **PASS** | `logs/865-followup-green.log:159-163`; `internal/adminapi/api_keys.go:39-51` and `api_key_error_test.go:12-22`. |
| Focused real Timescale/Race/shuffle M6d tests | **PASS** | `logs/865-followup-green.log:1-163`; no skipped M6d focused rows except the separately gated browser row at 130-132. |
| Web unit tests and production build | **PASS**: 46 tests; vue-tsc/build exit 0 | `logs/865-followup-web.log:1-59`. |
| Existing browser regression | **PASS**: 17 scenarios | `logs/865-followup-web.log:61-103`. |
| Contracts, architecture, Docker | **PASS** | `logs/final-3-contracts-docker.log:1-7` and the recorded successful image build. |
| Exact-snapshot full `make verify` including real browser | **UNPROVEN / running** | `docs/evidence/M6d/2026-10-07/README.md:28-35`; no final non-skipped browser output in `logs/final-3-go-verify.log`. |

## Verdict

The prior high-severity replay defect and the three follow-up medium defects are repaired in the exact source/web pair, with focused real-HTTP/Timescale and web evidence. No CRITICAL or HIGH finding remains, so this code-quality review **approves** the snapshot with the three MEDIUM watch items above. Stage completion still requires the pending exact-snapshot full/browser evidence and the separately required independent gate approval.
