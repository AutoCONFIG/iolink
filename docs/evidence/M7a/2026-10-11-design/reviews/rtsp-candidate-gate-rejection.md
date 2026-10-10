# M7a corrected design — independent final gate

- **recommendation: REJECT**
- **Reviewed frozen SHA:** `657d665b85aba6d346a7f15e588cabe73f118dc0`.
- **Date / reviewer:** 2026-10-11 Asia/Shanghai; independent `/root/m7a_corrected_design_gate`.
- **Snapshot:** `/tmp/iolink-m7a-gate-657d665`, detached and clean. Root checkout was not changed. Only this report and its evidence files were written in root; no production file edited, no delegate used.
- **Goal boundary:** implementation design review only; no R43, media deployment, real-camera or WeChat completion approval.
- **Notepad path:** none supplied. This report is the durable review record.
- `omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`. Requested report is here; fallback copy is `.omo/evidence/m7a-corrected-design-gate-review.md`.

## originalIntent

Allow M7a development to proceed from a concrete, secure video design while preserving R43 and R36.b/R37.b/R39.b semantics. The revision must resolve the selected stock ZLM's redirect/control URI, credential logging and internal reconnect behavior. Domain, credential encryption and persistence can be implemented while video remains disabled; enabling RTSP requires a versioned hardened build and operational proof. This gate must also check that the frozen GB/HLS, transaction and deployment design can actually be implemented.

## desiredOutcome

A consistent API/DDL/deployment baseline suitable for implementation, including tenant/resource authorization, feature guards, atomic state/job intent, reconciliation of unknown external outcomes, encrypted credentials, private media entrypoints and bounded authenticated playback. The currently selected provider must support the specified deployment boundaries. Design approval must leave actual R43 acceptance pending.

## blockers

### B1 — C-PORT-PAIR: frozen GB port allocation and DDL conflict with the selected provider's required RTP/RTCP pair

- **violatedCriterion:** `C-PORT-PAIR` — the explicitly frozen GB network and database boundary must be implementable with the pinned provider and correctly describe its 20 simultaneous receivers. This is the deployment detail requirement in `docs/EXTENSIONS.md` M7 and the M7a preimplementation design consistency gate in `docs/ACCEPTANCE.md` (final section), concretized at `docs/deploy/M7a-video.md:11`, `:16`, `:27`. The coordinator also explicitly directed resolving this design inconsistency before implementing its DDL.
- **observation:** `video_streams.rtp_port` permits any integer 30000–30019, while deployment allocates those 20 single UDP ports to up to 20 independent streams. Fixed upstream `RtpServer::start` always binds both the selected port and `port+1` even when an explicit nonzero port is provided. Adjacent allocations therefore overlap; the allowed highest RTP port additionally binds RTCP 30020 outside the frozen firewall/NAT range. A safe nonoverlapping allocation within the stated 20-port interval offers at most ten complete pairs, not the stated 20 receivers.
- **evidencePointer:** frozen `docs/deploy/M7a-video.md:11`, `:16`, `:27`; `docs/design/video-proposal.sql:91`, `:103`; archived `.omo/evidence/m7a-corrected-design-gate-20261011/RtpServer.cpp:137` and `:153`; upstream SHA `46220e6a866592c140d719ca2981bd2276344f5e`, URL `https://github.com/ZLMediaKit/ZLMediaKit/blob/46220e6a866592c140d719ca2981bd2276344f5e/src/Rtp/RtpServer.cpp#L137`.
- **independent reproduction:** after applying frozen migrations 001..012 and proposal in a new isolated real PG16/Timescale database, the original SQL fixture passed. I then inserted two active GB streams with RTP 30018 and 30019. Both inserts succeeded. The resulting required RTCP ports were 30019 and 30020. Exact SQL and output are in `port-pair-repro.sql` and `verification.txt` in the evidence directory. This is a demonstrated schema/provider mismatch, not an executed ZLM playback test.
- **required correction:** freeze nonoverlapping RTP/RTCP pairs consistently across deployment, DDL and negative fixtures. For 20 receivers the coordinator's proposed even RTP ports 30000..30038 with reserved RTCP `RTP+1`, firewall/NAT 30000..30039 and a database even-port constraint is a concrete solution. Verify odd/out-of-range/duplicate active RTP rejection, 20 valid distinct pairs and the updated proposal on real PG. Re-review the resulting new frozen SHA; this report does not approve a future edit.

## userOutcomeReview

**The specific RTSP security correction is adequate as a design requirement.** I independently downloaded and inspected the pinned source. `RtspPlayer.cpp:191` follows 301/302 with `play(Location)`, `:209` accepts Content-Base, `Rtsp.cpp:132` and `:382` return absolute track/session control URLs, and `sendRtspRequest` computes Authorization from the requested URL. Source/credential logging is present at `RtspPlayer.cpp:98`/`:772`, `PlayerProxy.cpp:205`/`:333`; WebApi defaults omitted retry_count to -1, and PlayerProxy retries internally. The revised design explicitly mandates all-3xx rejection, parsed control-origin checks before storage and at final send, removal of unsafe logs at all levels, `retry_count=0`, worker-owned DNS/auth/License rechecks, versioned patches/build recipes, approved digest/capability checks and malicious-source runtime evidence. It rejects unmodified or mismatched builds and does not claim those patches exist already. These requirements address the actual selected source behavior and permit ordinary numeric-origin RTSP instead of declaring every source unsupported.

**Tenant/RBAC and License:** design and API separate user, mini and media surfaces; ADMIN gets no tenant detail/playback grant; camera parent scope is server-derived; owner/admin configure while other permitted users see only authorized farms; GB catalogs remain owner/admin-only. Session GET/DELETE is creator-only. Live authorization/version checks, tenant stop, membership/support expiry, source changes and password revocation invalidate playback. License checks cover configuration/catalog/session creation and worker startup, allow existing sessions only to their original maximum expiry, and preserve read/stop recovery paths. These are suitable planned R36.b/R37.b/R39.b behaviors, not tested handlers.

**Transactions and worker outcomes:** session/stream/job/audit intent is atomic; locks have a stated order; provider operations run outside locks; lease/CAS/source version fence completion; unknown results reconcile rather than blindly replay; shared-stream stop checks active sessions and serializes with new starts; expiry/retry/shutdown bounds are specified. Existing migration 005 supplies tenant-bearing durable jobs. No implementation is being certified by reviewing these requirements.

**Credentials and HLS:** AES-256-GCM with derived domain key, nonce and tenant/entity/version AAD covers camera and GB credential storage; bad key/AAD/tag fails closed; no plaintext fallback or token storage. Gateway uses separate signed audience, maximum 300-second grants, DB token hashes, live DB authorization and bounded in-flight cancellation. Finite HLS parsing, segment mapping, response size/time limits, denied external URI/KEY/MAP/byterange/master/fMP4 and private ZLM ingress are explicit. No source URI or credential is included in response schemas.

**GB scope:** manual deployment-unique device IDs, peer-bound registration/dialog identity, replay and transaction retransmission semantics, bounded XML/catalog atomic replacement, UDP PS/SSRC INVITE/ACK/BYE/cancel handling and restart reconciliation are specified. TCP RTP, H265/transcoding/PTZ/recording are correctly outside the baseline. The port-pair blocker above must be corrected before this DDL/deployment plan is frozen.

**Status truthfulness:** `docs/IMPLEMENTED.md:40` says no video production implementation and design review pending; R43–R45 remain unimplemented. The eight-group manual QA matrix in the design is explicitly a future test requirement. Missing real RTSP/GB cameras, TLS/NAT/ACL and WeChat qualification/device evidence stay `external_blocked`.

## Direct remove-ai-slops / programming pass

Consulted both requested skills under `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/`. Applied their criteria directly to this commit's two-document diff, the whole supplied video design/API/DDL, and the two static Python validator/test files. No cleanup was performed because this is a read-only gate.

- No new production extraction, parser, normalizer, generic interface, fallback, speculative dependency or implementation abstraction was introduced by the correction. The capability-specific application ports and composition-root adapter choices match project architecture. The future protocol URI parser is necessary at an untrusted network boundary.
- No deletion-only, requested-removal-only, prose/phrase pin, tautological, or implementation-mirroring test was added by the two-document correction. The existing source variant, credential exclusion, tenant/farm rejection and mini/media routing tests exercise machine-consumed OpenAPI schemas and security declarations. They can fail for actual contract regressions and are correctly labeled static coverage.
- Fixture examples/oneOf tests check the real helper and validator, rather than returning a mocked success. The SQL negative cases catch specific constraint failures and deliberately raise if rejection is missing. The rollback check is meaningful evidence for a stated database transaction requirement. The port reproduction shows why green narrow fixtures do not prove all provider constraints.
- No excessive/useless test burden or scope drift identified for this goal. `test_video_contracts.py` is 81 pure LOC; `check_contracts.py` is 111. Pre-existing dynamic YAML helpers were not converted as part of a document-only fix. Source/build and real-surface gates remain required; schema tests are not used as runtime proof.
- Maintenance/style NOTE: OpenAPI repeats generic success/error boilerplate; counts are useful summaries only and cannot establish security or operational correctness. This does not violate a stated acceptance criterion and is not a blocker.
- Lint/typecheck/full Go suite/static security scan: N/A to the two-document correction; not run or represented as passed. The meaningful applicable schema and real PG checks were run below.

The existing archived code review `.omo/evidence/m7a-design-code-review-20261011.md` explicitly covers both skill perspectives and overfit categories (deletion-only/removal-only/prose pins/tautology/needless extraction/normalization, typing and module size). Its identity and source manifest/log were inspected after I had independently formed the initial design/security judgment. It reviews old SHA `97bbe588...`, so its REJECT or skill prose is not a current approval and cannot replace this direct pass. Its H1/H2 provider concerns are explicitly addressed by the current design correction. The port-pair issue here was found from my own subsequent download/read of `RtpServer.cpp`, not from an archived reviewer conclusion. No other corrected-snapshot review conclusion was read.

## Checked artifact paths

Read at frozen SHA: `docs/README.md`, `docs/CONTRIBUTING.md`, `docs/PLAN.md`, `docs/PLAN-DETAILS.md`, `docs/EXTENSIONS.md`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`; `docs/design/M7a-video.md`, `docs/design/video-proposal.sql`, `docs/deploy/M7a-video.md`, `docs/api/video-openapi.yaml`; `scripts/test_video_contracts.py`, `scripts/check_contracts.py`, `Makefile`; `docs/evidence/M7a/2026-10-09/README.md`, `proposal-check.sql`; existing `internal/migrate/sql/005_persistence_foundation.sql` and all 001..012 files consumed by real PostgreSQL.

Reviewed `git show --format=fuller 657d665`, the relevant file history, frozen checksums and clean snapshot status. Upstream artifacts downloaded independently and archived here: `RtspPlayer.cpp`, `Rtsp.cpp`, `PlayerProxy.cpp`, `WebApi.cpp`, `RtpServer.cpp`; exact SHA256 values in `source-manifest.json`. Historical review context inspected only for the required coverage check: `.omo/evidence/m7a-design-code-review-20261011.md`, its `identity.json`, `source-manifest.json` and `verification.log`. The historical source hashes for the first four independently downloaded files match that manifest.

## Commands, actual results and exact evidence gaps

Full command summary/environment/output: `.omo/evidence/m7a-corrected-design-gate-20261011/verification.txt`.

- `git worktree add --detach /tmp/iolink-m7a-gate-657d665 657d665b85aba6d346a7f15e588cabe73f118dc0`; `git diff --check 657d665^ 657d665` — success, no whitespace error.
- Root `.venv/contracts/bin/python -m pytest scripts/test_video_contracts.py scripts/test_m6a_contracts.py -q` from frozen worktree — **22 passed**.
- Same Python `scripts/check_contracts.py` (the command used by `make verify-contracts`) — **6 schemas, 96 operations, 568 synthetic fixtures passed**, including 22 video operations. Python 3.14.6, pytest 9.1.1, PyYAML 6.0.3, OpenAPI validators 0.9.0.
- Two unique disposable databases on task-authorized `iolink-m7a-foundation-20261011-93c1`, PostgreSQL 16.15 / TimescaleDB 2.30.2: frozen migrations 001..012 + proposal **PASS**; original positive/negative/rollback fixture **PASS**; targeted port-pair mismatch **REPRODUCED**. Both databases deleted in `finally`. No other container operated, no service/database credentials recorded.
- No hardened source patch/build recipe/digest exists in the reviewed change, and no normal/malicious RTSP build tests were executed. This is expressly a future prerequisite to media activation, not a missing design-stage success artifact.
- No production video handler/worker, real ZLM streaming, GB simulator/device, browser or WeChat playback was run. Their planned matrix is not an executed QA report. This review grants none of R43/R36.b/R37.b/R39.b runtime acceptance.
- Historical design evidence has embedded database output rather than a full raw historical log; independently repeating its frozen SQL here closes the current proposal-execution uncertainty. It does not validate historical runtime claims.
- A revised port plan, DDL and negative fixtures are required next; only a new frozen snapshot can resolve B1. Other examined design boundaries have no cited criterion failure.

**Final recommendation: REJECT for 657d665b85aba6d346a7f15e588cabe73f118dc0. Correct B1 and re-review the new frozen design.**
