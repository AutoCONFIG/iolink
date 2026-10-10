# M7a independent code quality / design review — 2026-10-11

- **Verdict: REJECT**
- **codeQualityStatus: BLOCK**
- **recommendation: REQUEST_CHANGES**
- **Reviewed full SHA:** `97bbe588bd82e6024522eddead7daa1a8f2dfd3b`
- **Diff base:** `47e074c277bcfaa05b59156f435a46e4d0c08b50`
- **Frozen worktree:** `/tmp/iolink-m7a-design-review-20261011`; clean before and after verification.
- **Review scope:** approval to implement the design only. This does not approve video functionality, deployment, R43 completion, or external camera/WeChat acceptance.
- **Notepad:** no designated notepad was supplied; none was created or modified.
- `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`, so this report uses `.omo/evidence/m7a-design-code-review.md`; an identical dated copy is `.omo/evidence/m7a-design-code-review-20261011.md` as requested by the coordinator.

## Goal, authority, and inspected scope

Supply a second independent review of the frozen M7a design before implementation. Success requires a finite implementation plan for R43 and the applicable R36.b/R37.b/R39.b authorization and License boundaries, complete API/schema/deployment contracts, and truthful separation of design checks from operational video evidence.

Read `AGENTS.md`, `docs/README.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, and `docs/CONTRIBUTING.md`. User scope takes priority: distinct platform administrators, no automatic business tenants, mini API demonstration only, user visual acceptance, and tag CI release policy. The current change does not release a tag or add a runnable video deployment.

Inspected all ten files in the design commit: `docs/IMPLEMENTED.md`, `docs/README.md`, `docs/api/video-openapi.yaml`, `docs/deploy/M7a-video.md`, `docs/design/M7a-video.md`, `docs/design/video-proposal.sql`, `docs/evidence/M7a/2026-10-09/README.md`, `docs/evidence/M7a/2026-10-09/proposal-check.sql`, `scripts/check_contracts.py`, and `scripts/test_video_contracts.py`. Added files were read in full, and modifications were compared to the parent commit. Also consulted the existing `jobs` schema in migration 005 and the pinned upstream ZLM source. The existing independent gate report's conclusion was not consumed to make this judgment.

Complete diff and review identity are preserved in:

- `.omo/evidence/m7a-design-code-review-20261011/frozen-design.diff`
- `.omo/evidence/m7a-design-code-review-20261011/identity.json`

## Findings by severity

### CRITICAL

None.

### HIGH

**H1 — The selected unmodified ZLM provider cannot enforce the frozen RTSP pinning and redirect policy.**

Project references: `docs/design/M7a-video.md:115`, `docs/design/M7a-video.md:116`, `docs/design/M7a-video.md:192`, `docs/deploy/M7a-video.md:13`, and `docs/deploy/M7a-video.md:24`.

The design promises that each connection uses a validated numeric IP, rejects source redirects and external SDP control URLs, and rechecks targets on reconnect. The pinned upstream source `46220e6a866592c140d719ca2981bd2276344f5e` does not provide that behavior: `src/Rtsp/RtspPlayer.cpp:191` handles 301/302 by calling `play(Location)` unconditionally at line 196. That new call parses and connects to the supplied host at lines 74–77 and 117, bypassing the original IP decision. A global allowed CIDR/port ACL can still permit another target in that range, so it is insufficient to preserve the original pin. `Content-Base` is trusted at line 209; `src/Rtsp/Rtsp.cpp:132` and `:382` accept absolute track/session control URLs unchanged. The latter source inspection proves a policy mismatch; it does not by itself prove that those control strings cause a second socket connection.

The generic instruction to refuse a source when the adapter cannot guarantee safety leaves the baseline provider unable to enable RTSP safely. Resolve this before freezing the design: specify a reproducible hardened ZLM build or another concrete enforcement mechanism that rejects redirects before following them, checks Content-Base and all control URI origins against the original numeric IP/port/scheme, and keeps authentication restricted to that origin. Specify `retry_count=0` for `addStreamProxy` so all retries return through application authorization/DNS/License checks: upstream `server/WebApi.cpp:1348` otherwise defaults to unlimited retries, and `src/Player/PlayerProxy.cpp:333` replays the stored URL internally. Require malicious RTSP server integration evidence before enabling the provider. No exploit or live RTSP test was executed in this review; this finding is based on the exact selected source.

Local source evidence: `.omo/evidence/m7a-design-code-review-20261011/RtspPlayer.cpp`, `Rtsp.cpp`, `WebApi.cpp`, and `PlayerProxy.cpp`. SHA256 hashes and the upstream SHA are in `source-manifest.json` in that directory.

**H2 — The deployment design does not enforce source URI and credential redaction in the selected media provider.**

Project references: `docs/design/M7a-video.md:124`, `docs/deploy/M7a-video.md:29`, and `docs/deploy/M7a-video.md:30`.

The stated no-URI/no-credential logging boundary is not satisfied by stock ZLM. `src/Rtsp/RtspPlayer.cpp:98` emits the parsed source URL, username, and password at Debug level, and line 772 emits RTSP request URLs at Trace level. `src/Player/PlayerProxy.cpp:205` logs the pull URL on successful playback, and line 333 logs it on retries. `server/main.cpp:143` defaults to Debug; lines 255–263 install both console and file log sinks. Passing credentials separately can avoid some plaintext fields, but source URI logging still occurs. Rotating those logs does not redact them.

Freeze an explicit provider logging control, such as removing/sanitizing the raw protocol/URI/credential log sites in the required hardened source build, with API request debug disabled and a raw-error policy. Require a source with distinctive test credentials/URI markers and verify neither provider stdout nor files nor API errors contain them at the permitted operating log levels. This is a deployment/security blocker for the claimed baseline. It is not a claim that credentials were actually exposed during this design-only run.

Local source evidence: `.omo/evidence/m7a-design-code-review-20261011/RtspPlayer.cpp`, `PlayerProxy.cpp`, and `zlm-server-main.cpp`.

### MEDIUM

None. The generic schema fixtures are appropriately labeled synthetic and are not presented as handler or media behavior coverage.

### LOW

None requiring action for this design review.

## Skill perspective and overfit/slop pass

**The required skill-perspective check ran.** Read `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md`, `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/programming/SKILL.md`, and the programming Python reference before judging tests or maintainability. Applied their test relevance, boundary parsing, abstraction, typing, and module-size criteria as a read-only reviewer.

No deletion-only tests, tests that merely pin a requested removal, prose/prompt tests, tautologies, or needless production extraction/normalization were found in this diff. The source-variant and credential exclusion tests exercise the actual machine-consumed OpenAPI validators with adversarial objects; the mini/media route checks pin routing/security declarations consumed by clients. The fixture test supplies an independent example and validates the resulting sample. These are useful static contract checks and provide no video-runtime proof. No new untyped escape hatch or speculative production abstraction was introduced; the pre-existing dynamic fixture/resolve helpers were not treated as a reason for an unrelated typing migration. `check_contracts.py` has 111 pure LOC and `test_video_contracts.py` 81.

**Perspective verdict:** the added/changed Python code and tests do not violate either skill perspective in a way requiring a finding for this goal. The provider security gaps above arise from the selected upstream behavior and the incomplete deployment enforcement, not from test slop.

## Positive design checks and remaining proof limits

- The 22 video operations distinguish user, mini, and media surfaces; platform ADMIN has no business playback grant, and user-selected tenant/farm fields are rejected. Camera/GB responses have closed schemas excluding source credentials.
- Playback uses an independent HS256 audience/key, DB token hash, immutable maximum 300-second expiry, live membership/tenant/farm/camera checks, and bounded in-flight cancellation. License expiry deliberately differs from tenant revocation, matching EXTENSIONS.
- Session/config changes include audit and durable job intent in the same transaction; external side effects are outside locks; lease/CAS, unknown reconciliation, stop/new-session race handling, and last-session cleanup are specified.
- Proposal SQL uses the existing tenant-bearing jobs foundation and composite camera/farm/pond/GB-channel/stream/session foreign keys. Source exclusivity, positive versions, token hash length, five-minute TTL, revocation timestamp pairing, and partial RTP/registration rejection have meaningful negative SQL fixtures.
- REGISTER digest, replay/transaction retransmission, peer identity, bounded XML/catalog collection, UDP PS INVITE/ACK/BYE and cancellation races have explicit scope. Missing devices, TLS/NAT/ACL, and WeChat qualification are correctly external_blocked; their absence is not a design blocker.
- The original evidence README reports a real PG16/Timescale proposal run, but only embeds its summary output. I inspected the SQL fixture and did not independently rerun it because this lane was instructed to await the coordinator's dedicated isolated database. Therefore this review does **not** certify that historical DB execution. The contract checks below were independently rerun and have durable logs.
- Production video handlers/providers/migration 013 are absent as stated. Runtime media, actual camera, browser playback, and WeChat acceptance were not executed and are not marked passed.

## Independently executed verification

All commands ran in the frozen worktree, with `PYTHONDONTWRITEBYTECODE=1` and pytest cache disabled. Complete command arguments, output, environment versions, exit codes, and clean status are in `.omo/evidence/m7a-design-code-review-20261011/verification.log`.

- `git rev-parse HEAD` → exact reviewed SHA above.
- `git status --short` before and after → empty.
- `git diff --check 97bbe588^ 97bbe588` → exit 0.
- Root `.venv/contracts/bin/python -m pytest -p no:cacheprovider scripts/test_video_contracts.py scripts/test_m6a_contracts.py -q` → **22 passed**.
- `make verify-contracts DOCS_PYTHON=<root>/.venv/contracts/bin/python` → **96 target operations, 568 synthetic request/response fixtures PASS**, including all 22 video operations.
- Python 3.14.6, pytest 9.1.1, PyYAML 6.0.3, openapi-spec-validator 0.9.0, openapi-schema-validator 0.9.0.
- Downloaded and inspected the exact pinned upstream files named above; no dependency or executable code in the frozen snapshot was changed.

## Blockers and recommendation

1. Freeze a concrete provider implementation/build contract that enforces RTSP target pinning, redirect/control origin rejection, and application-owned reconnect checks for the selected ZLM source (H1).
2. Freeze enforceable ZLM source/credential/raw-error logging controls and the corresponding integration evidence gate (H2).

**REQUEST_CHANGES / REJECT for `97bbe588bd82e6024522eddead7daa1a8f2dfd3b`.** A subsequent corrected commit must be reviewed at its own frozen full SHA. This report does not evaluate or approve a later revision.
