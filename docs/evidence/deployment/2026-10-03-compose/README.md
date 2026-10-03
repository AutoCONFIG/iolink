# Server and source-build Compose verification

2026-10-03 Asia/Shanghai. Scope: user's Compose request and R04 startup wiring;
this auxiliary milestone does not approve M6b or R30/R31/R33 external deployment.

Server file: `deploy/docker-compose.yaml`, application pull_policy always,
GHCR latest default, pinned PG16/TimescaleDB, persistent pgdata, loopback
HTTP/MQTT and no public database port. Development override:
`deploy/docker-compose.dev.yaml`, current-source frontend/Go Docker build,
dependency/build caches and loopback database port. Old prod file removed.
No Redis is required; MQTT is embedded. Internal detailed log dump remains
under development; this increment bounds Docker stdout/stderr to10MB×3.

Environment: Linux x86_64, Docker29.8.2, Compose5.5.1, dedicated
`iolink-compose-verify` project with fresh volume, ports18089/11889/55449.
Web gitlink is unchanged c54237e9d1a424bc9cb0b3f700014e2e966cf2cc.
Secrets generated in memory and redacted before preserving artifacts.

| Verification | Observed result |
|---|---|
| Missing credentials | Compose rejects PG password/root key absence |
| Both Compose models | config --quiet exit0; dev uses local image/build policy |
| Docker source build | exit0; frontend and Go compiled inside image |
| Database health and migrations | fresh DB healthy; migrate exit0 |
| Admin initialization on stdin | exit0; repeat init correctly exit1 |
| Application readiness/health/SPA/admin login | HTTP200; token presence verified without recording it |
| Restart persistence | ready and same admin login HTTP200 |
| Repeat migration | exit0 |
| make verify with real isolated DSN | build/vet/all race/shuffle tests exit0, no DB skips |
| make docs-tools && make verify-contracts | 53 operations/244 fixtures pass |
| git diff --check on the implementation tree before raw Docker capture | exit0; captured Docker CLI lines retain terminal padding in archived logs |

Exact Docker commands/output and metadata are in `logs/verification.json`;
`verify.mjs` preserves the executable orchestrator. Initial verification used
the equivalent .yml names before the requested rename; final .yaml models
were checked again. Rerun first setup only with a fresh project/volume.

Tests use the locally built image through the server file with --pull never.
They do not claim a future registry release was pulled. Frozen snapshot
double review and subsequent GHCR publication are recorded separately.
