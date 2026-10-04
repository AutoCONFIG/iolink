# M6c acceptance gate review — af20fcc

recommendation: APPROVE
confidence: high (0.88)
reviewed_sha: af20fcca8296a32438bc564079077fdcbf854af7
web_sha: 0aa7771acf0ef6322c9ac1e4c9839616646bb675
runtime_sha: 333fc427a4cd9c49715193176993c1e1827dedc6

## originalIntent
Deliver the M6c software milestone for R38 (signed License state and durable clock), R39.a (transactional quota/admission, restore and feature guards), and R40 (x86_64 offline Docker delivery), with integrated backend/admin web image and separate web-mini input. Keep future R39.b/M6d/M7/M8 entry points and issuer/customer-host/ARM64/WeChat/hardware acceptance explicitly external-blocked.

## desiredOutcome
A reviewable candidate with strict raw-byte parsing/signing, fail-closed authorization, stable HTTP errors, durable clock/recheck/pool/lock behavior, atomic audit and state updates, quota concurrency and restore semantics, secure setup, checksum-verified offline install/resume on one volume, simulator telemetry, and accurate SBOM/notices limitations.

## userOutcomeReview
The runtime implementation and evidence satisfy the stated M6c software outcome. `internal/license` strictly bounds and parses envelope/payload, verifies RSA-PSS SHA-256 over original payload bytes, rejects duplicate/unknown/trailing/non-UTC/invalid feature and key inputs, and supports the requested key and size boundaries. License status preserves signed fields and durable clock high-water/clock_error; import rejection audits retain the exact raw envelope digest and propagate audit failures. Device admission/recovery uses the license-row transaction lock, rechecks signature/deployment/clock/quota in-transaction, releases/rechecks quota for disable/restore, clears last_seen/shadow and increments session version, and fails closed when verification capability is unavailable. `RequireLicenseFeature` is exercised with a fake future executor across valid/permanent/overage and all denied states; overage still permits signed features while blocking new devices.

The offline bundle script pins local amd64 image IDs/manifests, includes the integrated iolinkd plus web assets, migrations, SBOM/dependency/license inventories, optional configs and checksums. Installer validation rejects checksum/path/image mismatches before Docker load, uses pull_policy=never and an internal network, stops before serving without License, resumes the same volume without setup input, reaches health/readiness with a supplied test License, and records simulator telemetry. The integrated screenshot and all 39 browser PNGs were inspected; widths are exactly 375/768/1440 and show the expected permission, loading/error, status, upload, quota and device states without visible horizontal overflow.

No blocker is tied to a stated M6c software criterion. R40 clean customer host 30-minute timing, actual issuer credentials, ARM64, WeChat, physical hardware, and R39.b future real feature entry points are accurately marked `external_blocked` and are deferred by the authoritative requirements.

## blockers
[]

## checkedArtifactPaths
- `AGENTS.md`, `docs/README.md`, `docs/PLAN.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, `docs/CONTRIBUTING.md`, `docs/design-m6c.md`
- `docs/evidence/M6c/2026-10-05/README.md`
- `.omo/evidence/m6c-license-delivery.md`, `.omo/evidence/m6c-manual-qa-final.md`
- `docs/evidence/acceptance/R38/README.md`, `R39/README.md`, `R40/README.md`
- `internal/license/{license.go,parsing.go,raw_input.go,signature.go}` and tests
- `internal/core/{license_runtime.go,license_clock.go,license_import.go,license_audit.go,admin_store.go,m6c_guard_test.go,m6c_license_test.go,m6c_transaction_test.go}`
- `internal/adminapi/{license.go,server_test.go}`
- `cmd/iolinkd/{main.go,license_cli_test.go}`, `cmd/license-sign/{main.go,main_test.go}`
- `internal/platform/{setup.go,setup_test.go}`
- `scripts/{build-offline-bundle.sh,install-offline.sh,offline-smoke.sh,uninstall-offline.sh}`
- `deploy/docker-compose.offline.yaml`, `cmd/offline-sbom/main.go`
- `web/src/api/admin.ts`, `web/src/views/SystemView.vue`, `web/src/views/DevicesView.vue`
- `docs/evidence/M6c/2026-10-05/browser/**/*.png` (39 files), `m6c-live-ui.png`
- all M6c logs/manifests/checksums/SBOM/notices under `docs/evidence/M6c/2026-10-05/`

## verification
- Evidence artifacts report `TMPDIR=$PWD/.tmp make verify`, isolated TimescaleDB race/shuffle integration, docs-tools/contracts, web tests/build, M6c Playwright (11 scenarios/39 screenshots), existing web E2E, Docker build, offline stop/resume, internal-network probe, simulator telemetry, and tamper rejection as PASS.
- Current review tree has no generated `internal/web/dist` or web `node_modules`; consequently an independent local `go test` and `npm test` cannot run here (`embed.go: pattern all:dist: no matching files`, `vitest: not found`). This is an environment reproduction gap, not a criterion failure, because the immutable candidate evidence includes the successful image build and web logs.
- `make verify-contracts` is not runnable in this checkout because `.venv/contracts/bin/python` is absent; the committed `m6c-contracts.log` records the candidate run as PASS.
- Direct programming/remove-ai-slops pass found no deletion-only/tautological tests, no unbounded secret logging, and no criterion-relevant dead or over-defensive code. Oversized Go modules remain maintainability notes only; no M6c criterion requires a split.

## exactEvidenceGaps
- External-blocked by authoritative scope: issuer-supplied production License, clean customer x86_64 30-minute installation timing, ARM64 build/acceptance, real WeChat credentials, physical devices, and future real R39.b entry points.
- Local reproduction lacks generated web build/dependencies and contract virtualenv as noted above.
- `architecture-gate.log` intentionally remains blocked until the two independent final approvals; this is the release process gate, not a software requirement failure.
