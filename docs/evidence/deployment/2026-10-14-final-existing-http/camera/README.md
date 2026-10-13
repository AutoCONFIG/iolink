# Existing camera HTTP contract verification

Date: 2026-10-14. Scope: the seven mounted camera operations, five on
/user/v1/cameras and two read operations on /api/v1/cameras. No GB, media,
playback HTTP, browser, hardware, real WeChat or delivery License acceptance is
included. This is working-tree evidence; the root owns exact-source verification
and independent release reviews.

## Environment and invocation

The root-owned container iolink-http-contract-pg-20261013 exposes the dedicated
PostgreSQL endpoint 127.0.0.1:33127. postgres-environment.log records the actual
server version and available Timescale default version. go-version.log and
python-version.log record installed runtimes. The root's disposable DSN is used
through IOLINK_TEST_PG_DSN; its password is not recorded here. testdb.New creates
and drops a migrated database for each fixture.

```bash
IOLINK_CONTRACT_PYTHON="$PWD/.venv/contracts/bin/python" GIN_MODE=release \
go test -race -shuffle=on -count=1 -v ./internal/core \
  -run '^TestCameraContract' \
  > .omo/evidence/camera-contract/after.log 2>&1

GIN_MODE=release go test -race -shuffle=on -count=1 \
  ./internal/adminapi ./internal/appapi \
  > .omo/evidence/camera-contract/http-unit-after.log 2>&1
```

Both commands exited 0. after.log contains five passing top-level tests and
53 passing subtests, covering 101 actual TCP responses checked against the
current video OpenAPI, with explicit status-specific machine-code checks.
There are no SKIP outcomes. The two HTTP packages pass their race/shuffle tests.

## Production-shaped runtime boundary

The camera service uses the real camerapg store, credential cipher and durable
License clock. A locally signed test License with video enabled supplies only
the dependency needed to reach the current configuration guard; this is not
License acceptance. Availability remains nil, matching cmd/iolinkd/camera.go.
No fake Availability is injected.

Actual POST create and PUT replacement therefore return 503 unavailable and
leave the captured camera business state unchanged. They are not claimed as
successful configuration or media-runtime functionality. The durable License
clock may observe its high-water mark in its separate transaction; zero-write
assertions cover cameras, streams, sessions, stop jobs and camera audits.

## Observed scenarios

- Both user and mini list/detail operations return schema-valid, source-redacted
  camera objects. List cursor fields, assigned-farm scope and foreign/missing
  resources are exercised through real authentication and PostgreSQL.
- Owner/admin read same-tenant cameras; member/viewer/support read only their
  assigned farm. Limited roles cannot create, replace or disable (403 with no
  camera-state change). Foreign mutations and reads return 404.
- Malformed/duplicated/unknown query parameters, invalid IDs, malformed,
  duplicate/case-mismatched/trailing JSON and credential-bearing RTSP URI input
  are rejected with 400. Missing identity is 401, live expired membership is 401,
  and a real platform ADMIN token is rejected from business camera access (403).
- Real DELETE persists camera disablement, seeded session-metadata revocation,
  one durable stop intent and one camera audit atomically. Repeated disable is
  idempotent and disabled camera details become 404. No playback/media provider
  is invoked; only the existing metadata invalidation behavior is certified.
- Six real relation-failure scenarios return 500 internal_error. A missing jobs
  relation after the disable mutation proves transaction rollback restores the
  entire captured business snapshot.

## Reproduced diagnostic defect and smallest fix

initial.log passed the initial read/configuration/disable/role/input scenarios.
diagnostic-before.log exited 1: all six SQL failure cases already returned a safe
schema-valid 500, but none had a correlated safe error-class dump record. The
failure scenario invocation was the same DSN/race flags with selector
^TestCameraContractProviderFailure.

The two owned cameras.go handlers now funnel provider/capability failures through
a small injected-slog cameraFailure helper before the existing cameraError
mapper. It records a constant message, stage=http_provider and safe err class
under the operation's request context. Ordinary invalid/forbidden/not-found/
conflict decisions do not become provider error logs. Existing response shapes
and authorization behavior are unchanged.

after.log proves each actual fault's dump record is ERROR with safe class
*errors.errorString and request_id equal to the response X-Request-ID. It also
checks that dumps exclude actual JWTs, credentials, RTSP URI and internal faulty
relation names. camerapg deliberately normalizes provider errors; the HTTP layer
does not reconstruct an invented SQLSTATE. Existing composition-root QueryTracer
remains the separate source of SQLSTATE diagnostics and was not modified here.

## Owned files and handoff

- internal/core/camera_contract_helpers_test.go
- internal/core/camera_contract_test.go
- internal/core/camera_contract_errors_test.go
- internal/adminapi/cameras.go
- internal/appapi/cameras.go

All new test files remain below 250 pure lines. No core production, camerapg,
schema, staging, commit or container lifecycle changes were performed by this
worker. Other agents' edits and existing playback drafts were preserved. The
root now owns the frozen combined candidate, full verification, two independent
reviews, release and dedicated container cleanup.
