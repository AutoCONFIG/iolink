# First-run setup final gate review

recommendation: APPROVE
blockers: []

Date: 2026-10-08 Asia/Shanghai. Independent read-only gate reviewer.
Snapshot: root `360bc8742ce9392c9cfdcd25f295050540f5fee7`, web `b340caedd038d1c80504aeffec8de7927b2871ba`. Both identities independently read before/after checks. Root source clean apart from pre-existing `.tmp/`; web clean. Review covers root base `c85a0db` and web base `66c5262`, including original first-run change and submitted fixes.

ULW status previously returned `ULW_LOOP_PLAN_MISSING`; the prescribed `.omo/evidence/<goal>-gate-review.md` fallback is used. This receipt supersedes the old gate REJECT only for the new snapshot, not by inheriting approval from any old stage.

## originalIntent

Default startup should open initialization/configuration, require no separate migration/admin CLI, reliably create a self-chosen administrator, and include direct web changes submitted to the remote. User explicitly owns browser testing and asked to avoid expanding testing beyond making initialization work.

## desiredOutcome

The empty database migrates automatically, serves the setup page, requires the installation credential for the atomic one-time administrator/audit creation, protects business HTTP routes until initialized, and directs initialized instances to login. Failed, invalid, duplicate and concurrent requests must not leave partial accounts. Web tests/typecheck/build pass and the web commit is remotely retrievable. New setup contract also promises proper input/body restrictions and checking status before repeating writes with an unknown result.

## userOutcomeReview

APPROVE for the authorized software scope. Composition root, transactional bootstrap adapter, HTTP gate, frontend setup route/page/API and Vite proxy now form the intended installation path. All previously identified functional blockers are resolved in the inspected source and reproduced checks. No skipped DB test was counted as passed. This is not browser acceptance, production TLS certification, full external R04/R40 acceptance, or an approval of unrelated historical stages.

## Criterion review and reproduced evidence

| Criterion | Result and actual evidence |
|---|---|
| R03/R04 automatic migration and empty startup | `cmd/iolinkd/main.go` runs `migrate.Up` before serving; errors exit. Archived empty-volume Docker logs show healthy app without CLI migrate, status required=true → initialize204 → required=false → ready. Startup wiring unchanged by corrective commit. |
| R04 installation credential and input rejection | Live HTTP/real Timescale suite executes missing/wrong key, other-host/port Origin, invalid Origin structures, invalid/unknown/trailing JSON, empty/short credentials and oversize body; all reject and SQL account count remains zero. |
| R04 one winner and atomic rollback | Live race/shuffle tests execute concurrent initialize, repeat409, single admin/audit and trigger-induced audit failure with zero accounts. |
| R04 unknown DB state fails closed/business gate | Real suite closes pool and receives503; normal lifecycle observes protected business handler blocked before initialization and available after. Composition root protects admin/open/app v1/v2 paths. |
| SETUP-ORIGIN | New contract explicitly permits same Host including port for http/https to support TLS termination; implementation additionally rejects userinfo/path/query/fragment. HTTPS-on-HTTP proxy-origin test now intentionally succeeds204; other-host/port/structure cases reject403. Installation key remains mandatory; no cookie authentication/CORS grant added. Earlier B1 is resolved by an explicit appropriate contract policy and matching implementation, not an ignored failure. |
| SETUP-BODY-LIMIT | Production limit4096 retained; fixture is now valid JSON preceded by4097 spaces. Normal suite returns400; independent mutation removing only `MaxBytesReader` produces204 and FAIL, proving meaningful coverage. |
| SETUP-UNKNOWN-RESULT | `web/src/api/setup.ts` reads status before every POST; skips write when initialized, probes status after network/5xx error, recognizes committed result, and prevents another write if status is unavailable. 52-test suite includes lost-response and unavailable-status assertions. |
| Vite development flow | Actual current Vite server plus temporary HTTP backend: GET `/setup/v1/status` → HTTP200, `application/json`, `{"required":true}`. Configured prefixes `/setup/v1`, `/admin`, `/api`. This directly resolves old SPA HTML response failure. |
| Frontend automation | Independently ran `npm test --prefix web` (6 files/52 tests), `npm run typecheck --prefix web`, `npm run build --prefix web`: all exit0. Existing Zod annotations/chunk warnings retained as notes. |
| Static contracts/architecture | Independently reran `make verify-contracts` (69 operations/343 synthetic fixtures) and `python3 scripts/check_architecture_manifests.py --all` (55 requirements): exit0. Static fixtures not treated as live handler QA. |
| Web remote traceability | `git -C web ls-remote origin refs/heads/main` independently returns exact `b340caedd038d1c80504aeffec8de7927b2871ba`. Root candidate is locally committed. |
| IMPLEMENTED truthful status | S4 still “已完成待核验”, identifies initial rejection/corrective review and user-owned browser QA. No old stage approval accepted as this delta's approval. |

## Exact commands and independent run results

- `GIN_MODE=release IOLINK_TEST_PG_DSN=<isolated, omitted> go test -race -shuffle=on -count=1 -v ./internal/setupapi ./internal/platform ./cmd/iolinkd`: PASS, setupapi5.412s/platform1.718s/cmd4.153s, no SKIP. Disposable `iolink-todo9-pg` PG16/Timescale instance; each fixture creates/drops its own random DB; credentials held only in child environment.
- `npm test --prefix web && npm run typecheck --prefix web && npm run build --prefix web`: PASS, 52 tests, production build1817 modules, exit0.
- `make verify-contracts && python3 scripts/check_architecture_manifests.py --all`: PASS, exit0.
- Current Vite runtime probe, using `createServer` from installed vite and current config, temporary backend bound8080, actual GET through Vite: `{"status":200,"contentType":"application/json","body":"{\"required\":true}","configuredProxyPrefixes":["/setup/v1","/admin","/api"]}`, exit0. Both temporary servers closed. Existing5173 caused Vite to choose another free port; no fixed-port collision was treated as success.
- Ephemeral Go overlay removing only `r.Body = http.MaxBytesReader(w, r.Body, 4096)` then `go test -overlay <temp> -count=1 -v ./internal/setupapi -run '^TestBootstrapWhenInvalidThenNoAccounts$/oversize$'`: expected FAIL, `server_test.go:100: 204`, mutation exit1. Overlay removed; source unchanged. This is adversarial test validation, not a failing production suite.

## Direct programming / remove-ai-slops pass

Consulted both skills during this review session (full skill criteria and relevant Go/TS references), and directly re-read original production diff plus corrective diff and tests. Independent pass remains required even when code reviewer also covers skills.

- No excessive deletion-only tests, removal-only tests, prose pins, tautological assertions or implementation-derived expected results. Bodylimit false-confidence case is fixed and mutation-verified.
- New tests distinguish actual externally observable outcomes: prohibited writes, recognized committed initialization, unauthorized/invalid requests, SQL account/audit outcomes. API SDK spies are narrow for error-state behavior; real wire test and actual Vite probe supplement them.
- The extra GET is required by unknown-result contract, not unnecessary extraction/parsing/normalization. JSON/Zod remains at input/response boundaries; shared transactional bootstrap is reused rather than copied. No provider SDK enters domain/application; typed sentinel errors and `errors.Is`, request contexts/timeouts and root-injected safe logger remain.
- Existing lifecycle multi-action test and exact JSON wire-string key-order assertion create small maintenance coupling; NOTE only. Existing oversized composition root and stale CLI-only comment are inherited/taste/maintenance findings and fail no stated user success criterion. No mandatory new architecture/tool migration inferred from skill preferences.
- Corrective Origin predicate has many boundary checks but each corresponds to the explicitly documented accepted Origin form. No new speculative abstraction or normalization layer was introduced.

## Code review coverage check

Inspected `.omo/evidence/first-run-setup-code-review.md`: original-snapshot report explicitly includes “Skill perspective check”, overfit/slop coverage, deletion-only/removal-only/prose/tautology/implementation-mirroring criteria, unnecessary parsing/abstraction scrutiny, bodylimit mutation finding and existing module-size note. The report's functional findings were independently checked against corrective snapshot; its old REJECT is not treated as current approval. Revised same-snapshot code report was not yet present at this receipt's write time; direct skill pass and complete checked evidence support this gate conclusion per role instructions. Dual-review completion still requires the second reviewer's explicit current-snapshot receipt.

## Checked artifact paths

- Authority: `AGENTS.md`, `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/ACCEPTANCE.md`, `docs/DEPLOY.md`, `docs/IMPLEMENTED.md`, `docs/api/setup-openapi.yaml`.
- Source: root and web full diff; `cmd/iolinkd/main.go`, `internal/domain/bootstrap.go`, `internal/platform/bootstrap.go`, `internal/platform/web_bootstrap.go`, `internal/setupapi/server.go`, `internal/setupapi/server_test.go`, `internal/testdb/testdb.go`, `internal/observability/redaction.go`, `scripts/check_contracts.py`, `scripts/embed-frontend.sh`, `Makefile`, `web/src/api/setup.ts`, `web/src/router/index.ts`, `web/src/views/SetupView.vue`, `web/src/styles/main.css`, `web/tests/setup-api.test.ts`, `web/vite.config.ts`.
- Executor directory `docs/evidence/deployment/2026-10-08-web-setup/`: `README.md`, `backend.log`, `full-verify.log`, `contracts.log`, `architecture.log`, `docker-build.log`, `docker-build-final.log`, `docker-start.log`, `docker-start-final.log`, `docker-reset.log`, `docker-cleanup.log`, `docker-cleanup-final.log`, `status-before.json`, `init-status.txt`, `status-after.json`, `readyz.txt`, `review-fixes-backend.log`, `review-fixes-web.txt`.
- Reviewer artifacts: `.omo/evidence/first-run-setup-code-review.md`; prior gate receipt; `.omo/evidence/first-run-setup-review/identity.json`, `snapshot.diff`, `web-snapshot.diff`, `origin-probe.log`, `body-limit-probe.log`, `vite-proxy-probe.json`, `backend.log`. Old negative probes describe original snapshot only and are superseded by reproduced fixes, not mislabeled PASS.

## Exact evidence gaps / notes

- No dedicated notepad supplied. README and linked raw DB/full-suite/Docker files supply executor QA matrix; final scope checks reproduced directly.
- Browser QA is expressly user-owned. No browser PASS is asserted.
- Full Go suite and Docker empty-volume HTTP run were archived for original functional candidate; corrective source changes are input restriction, frontend retry safety and Vite proxy. Current three relevant backend packages, frontend build, actual Vite route and static gates were rerun independently. Exact rebuilt corrective Docker snapshot not re-run by this reviewer; archived Docker logs are startup-chain evidence, not a claim the final image digest was tested.
- Web summary is an excerpt rather than full original npm output; independent current npm reproduction covers that evidence gap.
- Old Docker logs contain trailing whitespace; no all-green whitespace claim.
- New code-review receipt pending as described above; this gate receipt alone does not certify that dual-review process is already complete.
- External WeChat/hardware, customer offline installation, public TLS, browser appearance, remote root push/CI/image publication remain outside this requested initialization review and are not silently marked accepted.
