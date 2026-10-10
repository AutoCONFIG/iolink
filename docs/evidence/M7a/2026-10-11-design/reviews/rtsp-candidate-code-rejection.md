# M7a corrected design independent code review — 2026-10-11

- **Frozen commit:** `657d665b85aba6d346a7f15e588cabe73f118dc0`
- **Tree:** `73821b3036b330c6aab74e9d6ae8e4a3348dcc8f`
- **Design decision:** **REJECT**
- **codeQualityStatus:** **BLOCK**
- **recommendation:** **REQUEST_CHANGES**
- **Reviewer:** independent read-only code/design reviewer; no production edits, no child reviewers.
- **Review checkout:** `/tmp/iolink-m7a-corrected-design-code-657d665`, independently created detached worktree, clean at verification.
- **Goal/success criteria:** corrected M7a design must be secure, internally consistent and implementable while preserving R43, R36.b/R37.b/R39.b, tenant/farm/role and License semantics. Safe provider artifacts and malicious-server runtime evidence must precede RTSP activation; foundation may be developed with video disabled. Design approval never implies R43 implementation or real-camera acceptance.

## Scope and evidence identity

Read `docs/README.md` first, then `docs/CONTRIBUTING.md`, `PLAN.md`, `PLAN-DETAILS.md`, `EXTENSIONS.md`, `ACCEPTANCE.md` and `IMPLEMENTED.md`. Reviewed the complete design, DDL, deployment attachment, video OpenAPI and both scripts. The immediate corrective commit changes only `docs/design/M7a-video.md` and `docs/deploy/M7a-video.md` (35 additions, 3 deletions); the broader design/script diff was also inspected against `47e074c277bcfaa05b59156f435a46e4d0c08b50`.

Evidence directory: `.omo/evidence/m7a-corrected-design-code-20261011/`.

- `identity.json`: frozen commit/tree, clean checkout, scoped file hashes and independently downloaded source hashes.
- `frozen-design.diff`: full six-file design/script diff; `corrective.diff`: immediate security revision.
- `contracts.log`, `pytest.log`, `ddl-check.sql`, `ddl.log`, `diff-check.log`: independent command/SQL results.
- `notepad.md`: review notes, provenance, scope and limitations.
- `RtpServer.cpp`, `RtspPlayer.cpp`, `PlayerProxy.cpp`, `HlsMaker.cpp`, `HlsMakerImp.cpp`: independently fetched raw files at upstream SHA `46220e6a866592c140d719ca2981bd2276344f5e`.

`omo-agent-toolkit ulw-loop status --json` returned `ULW_LOOP_PLAN_MISSING`, so no `currentAttemptDir` exists. Canonical fallback report is `.omo/evidence/m7a-corrected-design-20261011-code-review.md`; identical requested delivery is `.omo/evidence/m7a-corrected-design-code-20261011.md`.

## Findings by severity

### CRITICAL

None found within this design review.

### HIGH

**H1 — Normal fixed-provider HLS segments cannot be represented by the proposed mapping table.**

- Location: `docs/design/video-proposal.sql:133`; gateway behavior `docs/design/M7a-video.md:96`–`102`.
- Fixed ZLM `src/Record/HlsMakerImp.cpp:159`–`164` always constructs live TS names as `YYYY-MM-DD/HH/MM-SS_index.ts`. `HlsMaker.cpp:149` writes that same relative name into the manifest. The DDL only accepts `^[A-Za-z0-9_-]+[.]ts$`, excluding every such slash-containing provider path.
- Independent PG counterexample: inserting `2026-10-11/00/00-01_0.ts` for an otherwise valid session fails with SQLSTATE `23514`; a basename-only fixture is accepted. See `ddl-check.sql` and `ddl.log`.
- Consequence: the specified gateway cannot persist its session-to-provider segment map for normal ZLM output, so the baseline RTSP and GB HLS playback path fails. Stripping the path loses the actual provider location and can collide across date/hour directories.
- Required correction: define and store the strictly validated relative provider path beneath the fixed stream root, or explicitly require a provider patch producing a compatible flat namespace. Keep traversal/absolute URI/encoding/query restrictions and prove ordinary fixed-build HLS manifest→mapping→segment retrieval before approving the design. Do not merely broaden the regex without boundary rules.
- Source: https://github.com/ZLMediaKit/ZLMediaKit/blob/46220e6a866592c140d719ca2981bd2276344f5e/src/Record/HlsMakerImp.cpp#L156 ; https://github.com/ZLMediaKit/ZLMediaKit/blob/46220e6a866592c140d719ca2981bd2276344f5e/src/Record/HlsMaker.cpp#L149 .

**H2 — The proposed 20-stream RTP allocation ignores the provider's mandatory RTCP port.**

- Location: `docs/deploy/M7a-video.md:11`, `:27`; `docs/design/video-proposal.sql:91`, `:103`.
- Fixed ZLM `src/Rtp/RtpServer.cpp:149`–`156` binds both the explicit RTP port and `RTP+1` for RTCP. The deployment exposes only `30000–30019`, promises 20 independent receivers, and the DDL accepts all consecutive RTP values with uniqueness only on the RTP value.
- Independent PG counterexample: active stream rows with RTP `30000` and `30001` are both accepted. The second allocation conflicts with the first stream's RTCP socket. Even selecting only even values yields 10 complete pairs inside the documented firewall range, not 20.
- Consequence: the proposed allocation/NAT/firewall contract cannot deliver its stated capacity and can admit conflicting allocations before provider startup. This is a concrete correctness mismatch with the pinned implementation, not a future optimization.
- Required correction: specify 20 disjoint RTP/RTCP pairs consistently across allocation, DDL, port publication/NAT and firewall (for example even RTP `30000..30038`, RTCP `RTP+1`, UDP `30000..30039`), with rejection of odd/out-of-range/duplicate active allocation and provider evidence.
- Source: https://github.com/ZLMediaKit/ZLMediaKit/blob/46220e6a866592c140d719ca2981bd2276344f5e/src/Rtp/RtpServer.cpp#L137 .
- Provenance: the parent supplied this source pointer during review; I independently downloaded the pinned source, inspected the binding code and reproduced the DDL counterexample. No other reviewer conclusion was used.

### MEDIUM

None added. No useless test or needless production parsing/normalization was identified in the reviewed diff.

### LOW

None material to this admission decision.

## Independent checks and design assessment

| Area | Assessment at this snapshot |
|---|---|
| RTSP security correction | Design-level previous conflict closed: original ZLM prohibited; all 3xx rejected; Content-Base/session/track controls checked against the numeric pinned origin before credentials; final send guard; sensitive/raw provider logs removed; `retry_count=0`; repository-managed patch/build recipe with patch SHA256, image digest, capability probe and malicious-server evidence before activation. These are future acceptance requirements, not existing capability. |
| Module boundaries | Narrow domain/application ports, provider adapters and composition root are consistent with the modular monolith. No SDK dependency is prescribed in domain/application. Default-disabled video and startup failure on missing enabled capability are explicit. |
| Tenant/farm/role | Server derives parents, clients cannot choose tenant/farm, platform ADMIN denied user business surface, owner/admin configuration versus scoped reads, support expiry, creator-only session access and live permission/version checks are specified. PG composite FKs reject cross-tenant farm/pond/GB-channel combinations. Runtime role/ownership concurrency remains implementation acceptance. |
| License | New configuration/catalog/playback and worker starts guarded; existing playback bounded by original TTL; stop/delete and redacted metadata remain available. Disabled video never claims a ready provider. |
| Atomic jobs and unknown outcomes | Session/stream/start job/audit committed together, external work outside transaction, leases/CAS, deterministic stream identity, GB dialog persistence, reconciliation, compensation and stop/new-session serialization are specified. Manual real-PG camera+job rollback demonstrated atomic schema use; no production worker exists to prove its behavior. |
| HLS/token/revocation | Separate fixed-algorithm/audience token, stored hash, 300-second cap, claim/session/source/version consistency, live DB authorization on every request, bounded in-flight cancellation, no cache/referrer/log leaks and unsupported-manifest rejection are appropriate. H1 blocks provider compatibility. |
| GB | Registered identity, digest nonce/replay/transaction handling, source peer binding, XML/size limits, complete catalog replacement, missing-channel revocation, UDP PS/SSRC/dialog lifecycle and unknown/cancel paths are specified. H2 blocks the documented receiver capacity. |
| DDL | Applying real migrations 001–012 plus the frozen proposal succeeded. Positive RTSP/GB fixtures and negative parent/source/session checks succeeded, apart from the two deliberately confirmed compatibility counterexamples. Constraints are not asserted to replace application authorization. |
| Status/scope | `IMPLEMENTED.md` still marks R43–R45 unimplemented. Deployment attachment and optional compose are explicitly not runnable. No R43 or external success was inferred from static checks. |

## Skill-perspective check

**Ran.** Explicitly loaded `/home/yun/.codex/plugins/cache/sisyphuslabs/omo/5.1.29/skills/remove-ai-slops/SKILL.md` and `.../programming/SKILL.md`, plus the programming Python reference, before judging test relevance and maintainability. Applied the requested read-only perspectives; did not execute a cleanup/refactor workflow or spawn agents.

Reviewed both `scripts/test_video_contracts.py` and the changed `scripts/check_contracts.py` logic. Assertions cover machine-consumed OpenAPI input/output restrictions, source variants, secrecy, HTTPS grants, security declarations and fixture validation. They do not pin natural-language prose, a requested deletion or a removed implementation; their expectations are independently supplied inputs. No deletion-only, tautological or implementation-constant-only new tests were found. Example/oneOf handling and distinct read/write validators are needed by the schema boundary; no production data extraction/normalization, explicit `Any`, `type: ignore` or needless abstraction was introduced. Both scripts are below the skill's 250 pure-LOC ceiling. Thus **neither skill perspective is violated by the reviewed diff**. These static tests remain incapable of proving provider interoperability, which is why H1/H2 required source and real-PG checks.

## Verification results and limits

1. From the clean detached worktree: `PYTHONDONTWRITEBYTECODE=1 /media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.venv/contracts/bin/python scripts/check_contracts.py` — exit 0, **96 operations / 568 synthetic fixtures PASS** (`contracts.log`).
2. `PYTHONDONTWRITEBYTECODE=1 <same-python> -m pytest -q -p no:cacheprovider scripts/test_video_contracts.py scripts/test_m6a_contracts.py` — exit 0, **22 passed** (`pytest.log`).
3. Real isolated PostgreSQL **16.15**, TimescaleDB **2.30.2**, permitted container `iolink-m7a-foundation-20261011-93c1`, test role `iolink_test`: random database, applied 001–012 and frozen proposal; executed `ddl-check.sql`; cleaned only that database (`ddl.log`). Cross-tenant FK, source exclusivity, missing source, 300/301-second TTL, state/revoked/hash/version constraints and camera+job rollback checked. H1/H2 counterexamples confirmed. This does not test future production adapters or workers.
4. Pinned raw-source review independently fetched with `curl --fail --silent --show-error --location https://raw.githubusercontent.com/ZLMediaKit/ZLMediaKit/46220e6a866592c140d719ca2981bd2276344f5e/<source-path>`; hashes in `identity.json`. No ZLM binary was built or executed, so socket behavior beyond source analysis and actual playback are not reported as runtime PASS.
5. `git diff --check 47e074c277bcfaa05b59156f435a46e4d0c08b50 657d665b85aba6d346a7f15e588cabe73f118dc0 -- <six scope files>` — exit 0 (`diff-check.log`).

Production Go tests, lint/type checking and full UI QA: **N/A for this documentation/contract admission review**, no production implementation changed. Safe-build RTSP malicious-server tests, provider/network integration and real RTSP/GB camera, public TLS/ACL and WeChat playback: **not run / unverified; external requirements remain `external_blocked` where inputs are missing**. No external devices are required to establish H1/H2 or perform this design review. Earlier reviewer reports/conclusions were not read.

## Blockers and conclusion

1. Resolve H1's fixed-provider HLS path versus DDL/gateway mapping conflict.
2. Resolve H2's RTP/RTCP pair versus 20-stream allocation/DDL/deployment conflict.
3. Re-review the corrected frozen snapshot independently; this report is valid only for `657d665b85aba6d346a7f15e588cabe73f118dc0` and does not approve any newer commit or worktree edits.

The hardened RTSP build requirement is a sufficient design-level correction to the earlier provider safety mismatch. It must remain an activation gate with video disabled until actual pinned patch/build/digest and adversarial runtime evidence exist. The complete current design still requires changes for H1/H2. **R43 remains unimplemented and is not accepted.**
