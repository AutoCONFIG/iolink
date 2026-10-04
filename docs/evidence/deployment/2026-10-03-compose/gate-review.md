# Server Compose Gate Review

- recommendation: **APPROVE**
- reviewed SHA: `a734e1afa4af0748a33e863327be7eb336902eda`
- comparison base: `74232902ff2d34a7dc83374b9804aa5c65ff1e3b`
- review mode: read-only independent gate review
- blockers: []

## originalIntent

Provide a test-server Compose file that can be copied with `.env` without the source tree, pulls `ghcr.io/autoconfig/iolink:latest` on startup, runs a persistent and healthy internal Timescale/PostgreSQL database, and keeps the auxiliary scope separate from pending M6b. Provide a development overlay that builds the current local frontend and Go source with dependency/build caches. Remove obsolete Compose filenames and document the operator UX.

## desiredOutcome

A user can copy `deploy/docker-compose.yaml` and `.env` to a server, configure required secrets, run documented migration/admin/start commands, and obtain a ready SPA/API with persistent admin state. A repository checkout can use `deploy/docker-compose.dev.yaml` to build and run the current source. Configuration, health/readiness, repeat migration, repeat initialization, restart persistence, and full verification evidence are reviewable without exposing secrets.

## userOutcomeReview

The final server model resolves to GHCR latest with `pull_policy: always`, has no application build stanza, keeps the DB internal, uses a named `pgdata` volume, waits for DB health, binds HTTP/MQTT to loopback by default, and applies bounded Docker JSON logs. Missing PG/root secrets fail Compose parsing. A standalone temporary directory containing only `docker-compose.yaml` and `.env` passed `docker compose config`.

The development model resolves to `iolinkd:local-dev`, `pull_policy: build`, context `..`/Dockerfile, and a loopback DB port. Dockerfile inspection and the source-build log show Node frontend build, frontend embedding, and Go compilation with BuildKit npm/Go module/build caches. Final-file model checks and live container inspection showed healthy DB/application, persistent named DB volume, expected log rotation, SPA asset delivery, readiness/health 200, and invalid login 401.

The supplied executable receipt records fresh DB startup, migration, admin initialization, repeat-init rejection, app readiness/SPA/login, restart/login persistence, repeat migration, source build, race/shuffle Go verification, and contract validation. I independently reproduced `make verify-contracts` (53 operations / 244 fixtures), final Compose model checks, standalone-copy behavior, live health/assets, and credential rejection. The evidence explicitly scopes out M6b, TLS, capacity, and future registry-release claims.

## checkedArtifactPaths

- `/tmp/iolink-server-compose/deploy/docker-compose.yaml`
- `/tmp/iolink-server-compose/deploy/docker-compose.dev.yaml`
- `/tmp/iolink-server-compose/deploy/.env.example`
- `/tmp/iolink-server-compose/Dockerfile`
- `/tmp/iolink-server-compose/.dockerignore`
- `/tmp/iolink-server-compose/Makefile`
- `/tmp/iolink-server-compose/docs/DEPLOY.md`
- `/tmp/iolink-server-compose/docs/evidence/deployment/2026-10-03-compose/README.md`
- `/tmp/iolink-server-compose/docs/evidence/deployment/2026-10-03-compose/verify.mjs`
- `/tmp/iolink-server-compose/docs/evidence/deployment/2026-10-03-compose/logs/verification.json`
- `/tmp/iolink-server-compose/docs/evidence/deployment/2026-10-03-compose/logs/source-build.log`
- `/tmp/iolink-server-compose/docs/evidence/deployment/2026-10-03-compose/logs/go-verify.log`
- `/tmp/iolink-server-compose/docs/evidence/deployment/2026-10-03-compose/logs/admin-repeat-rejected.log`
- `/tmp/iolink-server-compose/docs/evidence/deployment/2026-10-03-compose/logs/migration-repeat.log`
- `/tmp/iolink-server-compose/docs/evidence/deployment/2026-10-03-compose/logs/restart-ready.log`
- `/tmp/iolink-server-compose/.omo/evidence/server-compose/verification.json`
- `/tmp/iolink-server-compose/.omo/evidence/server-compose/model-development.log`

## directSkillChecks

### remove-ai-slops

I reviewed the scoped diff for obvious comments, broad/over-defensive logic, needless abstractions, dead code, duplication, complexity, hidden cost, and test overfit. The changed production surface is Compose/Docker/Make/documentation; no unnecessary parsing or normalization extraction, deletion-only behavior tests, tautological tests, or implementation-mirroring tests were introduced. The JS orchestrator is a bounded executable evidence script; its assertions exercise command exit codes and user-visible HTTP outcomes. Raw log whitespace is provenance data, not production behavior. No slop finding violates a stated criterion.

### programming

The only code-like changes are Dockerfile shell instructions, Make recipe edits, and the evidence JS runner. I checked the Dockerfile build boundaries, cache mounts, frontend embedding path, and shell fail-fast behavior. The runner uses explicit expected exit codes and redacts generated secrets before writing logs. No changed Go/TypeScript/Python production module requires the programming skill's language-specific refactor gates. No scoped criterion failure found.

## evidenceGaps

1. `/tmp/server-compose-code-review.md` was not present when this review was written; the independent code-review lane is expected to provide it separately. This is a reporting coverage gap only and does not prove a Compose criterion failure because the direct artifact review above covers the scoped behavior.
2. `docs/evidence/deployment/2026-10-03-compose/logs/verification.json` records the pre-rename equivalent `.yml` commands and source SHA `7423290...`; the README states final `.yaml` config was rerun, and I independently reran final-model checks. The evidence does not claim a GHCR network pull, only local-image verification.
3. The archived raw Docker logs retain terminal padding, and the follow-up evidence commit `a734e1a` now documents that the clean `git diff --check` was run on the implementation tree before raw capture. This is a note, not a user criterion failure.
4. No ulw-loop plan exists in this checkout, so no `currentAttemptDir` could be resolved; this report is written to the required fallback locations.

## blockers

None. No scoped user success criterion failed.

