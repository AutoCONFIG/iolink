# R37.a: app V2 telemetry write permission fix

Date: 2026-10-01. Base: `764c5eb` (resolve exact base from source-identity.log). Branch: `codex/m6b-telemetry-permission-fix`. This is executor evidence, pending two fresh independent reviews of the resulting committed snapshot. Previous approvals of the 764c5eb baseline do not approve this defect/fix. No M6b stage completion is claimed.

## Observed defect and fix

Baseline production app handlers and core repository were reloaded from Git through Go overlays, with the same final regression fixtures. Login and tenant selection were real production app HTTP routes on ephemeral loopback listeners; only the external WeChat code exchange was injected with a deterministic fixture identity. Real PostgreSQL/TimescaleDB was used. Viewer/member/support POST returned 202 and created telemetry, water projection, shadow, alarm and outbox rows. Direct core calls also accepted forged or revoked roles and missing authenticated context.

The repository operation now requires matching tenant/user/role context, reloads active unexpired USER membership and tenant under transaction row locks, and asks the existing injected Casbin policy for telemetry/write. Only owner/admin receive this policy. A role denial occurs before every write and maps to HTTP 403; resource visibility still hides foreign/unassigned farms with 404. HTTP auth supplies the validated actor ID. MQTT ingestion uses its existing distinct device boundary.

## Environment and source

Go 1.26.8 linux/amd64; real TimescaleDB 2.30.0. Dedicated test instance: PostgreSQL role `iolink`, host `127.0.0.1`, port `55439`, seed database `iolink`, `sslmode=disable`. Test-only credentials were supplied through `IOLINK_TEST_PG_DSN`; `testdb.New` creates and drops a unique isolated database per scenario. No production database was used.

`source-identity.log` records the full base SHA and SHA256 for every changed production file, contract, and regression test. Frontend build assets were copied from the existing root's `web/dist` into this worktree's ignored `web/dist`; make's normal embedding/build step then ran. No fixed-port E2E was run.

Evidence was first captured under `.omo/evidence/m6b-v2-permission-fix/`, then copied to this committed directory. Raw logs are preserved with exact command invocations and exit statuses in `commands.log`. The manual harness `manual_qa_test.go.txt` invokes curl against a real running production app router and logs statuses/body plus full before/after SQL snapshots. JWTs are not printed.

## Scenario ledger

| Criterion/scenario | Invocation in commands.log | Binary observable | Raw artifact |
|---|---|---|---|
| Assigned viewer/member/support app login → select tenant → POST | RED, GREEN, CURL_RED, CURL_GREEN | baseline 202 with five-table mutation; fixed 403 `forbidden`, full JSON snapshots byte equal | red.log, green.log, manual-red.log, manual-green.log |
| Owner/admin positive app and direct core writes | GREEN, CURL_GREEN | HTTP 202; direct counts telemetry/sensor_data/shadow/alarm/outbox each 1 | green.log, manual-green.log |
| Same tenant hidden farm and cross tenant | GREEN | app POST 404 for each role; snapshots unchanged; direct cross tenant `ErrNotFound` | green.log |
| Forged actor, forged role, unknown claimed role, viewer/member/support direct boundary | GREEN | `errors.Is(ErrForbidden)`, exact complete snapshots equal | green.log |
| Revoked/expired membership, role downgrade, tenant disabled, farm revoked | GREEN | direct typed denial; next operation cannot write; role downgrade/revoked old app JWT 401 | green.log |
| Missing actor/role/tenant, wholly unscoped direct owner | GREEN | `ErrForbidden`, no partial persisted state | green.log |
| Missing policy capability | GREEN | typed `ErrForbidden`; no persisted state | green.log |
| M6a validated generic properties, invalid/mixed whole-batch rejection/history/alarms | GREEN, ADJACENT | named real DB regression PASS | green.log, adjacent.log |
| MQTT water/authentication/ACL and second product numeric/enum ingestion | ADJACENT | both actual MQTT broker tests PASS, protocol/access regression PASS | adjacent.log |
| Complete repository build/vet/race/shuffle tests with real DSN | VERIFY | make exit 0 and every tested package `ok` | verify.log |
| OpenAPI including POST V2 403, architecture manifests | CONTRACTS, ARCHITECTURE | exit 0; 53 operations/245 synthetic fixtures; 55 requirement manifests PASS | contracts.log, architecture.log |

`red.log` intentionally records failing assertions and exit 1: it is the pre-fix behavior oracle. `green.log`, `manual-green.log`, `adjacent.log`, `verify.log`, `contracts.log`, and `architecture.log` record successful reruns. RED and GREEN use identical HTTP/core regression tests. `manual-red.log` and `manual-green.log` use the same curl harness. The baseline overlay restores only the five changed production files and makes the new authorization helper an empty core package file; it does not alter the final worktree.

## Replaying the overlays

From any checkout containing this commit and the base commit, first provide the dedicated isolated `IOLINK_TEST_PG_DSN`. Generate overlay source files locally; source from Git is preferable to trusting copied source.

```bash
python3 - <<'PY'
import json, subprocess
from pathlib import Path
root = Path.cwd()
e = root / '.omo/evidence/m6b-v2-permission-fix'
e.mkdir(parents=True, exist_ok=True)
b = e / 'baseline'
b.mkdir(exist_ok=True)
replace = {}
for path in ['internal/core/repos.go', 'internal/core/service.go', 'internal/authorization/policy.go', 'internal/appapi/server.go', 'internal/appapi/telemetry_v2.go']:
    target = b / Path(path).name
    target.write_bytes(subprocess.check_output(['git', 'show', '764c5eb:' + path]))
    replace[str(root / path)] = str(target)
helper = b / 'telemetry_authorization.go'
helper.write_text('package core\n')
replace[str(root / 'internal/core/telemetry_authorization.go')] = str(helper)
(e / 'red-overlay.json').write_text(json.dumps({'Replace': replace}, indent=2) + '\n')
manual = root / 'docs/evidence/acceptance/R37/2026-10-01-v2-permission-fix/manual_qa_test.go.txt'
manual_replace = {str(root / 'internal/core/telemetry_manual_qa_test.go'): str(manual)}
(e / 'manual-overlay.json').write_text(json.dumps({'Replace': manual_replace}, indent=2) + '\n')
replace.update(manual_replace)
(e / 'manual-red-overlay.json').write_text(json.dumps({'Replace': replace}, indent=2) + '\n')
PY
```

Then run the command lines in `commands.log`. RED/CURL_RED are expected to fail. Go overlay adds the manual harness to the test binary without adding a production or permanent package file.

## Limits

WeChat external code exchange, physical devices, public MQTT and production deployment are `external_blocked`; the local HTTP/MQTT software scenarios above do not replace those external acceptance gates. This change does not implement future R37.b resources. Stage statuses remain owned by the central implementation ledger and are pending fresh dual review.

The final verification initially exposed that a raw manual harness ending in `.go` under docs was discovered by `go vet ./...`. It was renamed to `.go.txt` as replay-only evidence; `verify-harness-failure.log` records the failure. The complete relevant `make verify` scenario was rerun after correction.
