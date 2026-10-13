# V1c current-source evidence: independent code review

Exact candidate: `10ae4fb60edefeee0622ca5354ebe63554fc6a9a`

Base: `fbfa7ba` (v0.0.35)

Export: `/tmp/iolink-v1-current-20261013`

Reviewer: `/root/latest_code_review`; read-only inspection and validation, with no product/evidence edits or database replay.

codeQualityStatus: WATCH

recommendation: REQUEST_CHANGES

## Findings

### CRITICAL

None.

### HIGH

None.

### MEDIUM

1. **The advertised replay entry overwrites the historical evidence it is meant to preserve.** `docs/evidence/rebuild/reconciliation/2026-10-13-current-software/run_ops_qa.py:11` sets `OUT` to the archived script's own directory, and line 27 writes command results directly to existing artifact names. The subsequent fixture, HTTP, recovery, shutdown and row-comparison writes use that same directory. `README.md:23` recommends executing this file directly. Following that instruction after rebuilding the temporary environment replaces the already reviewed evidence bundle in place. The script also does not refresh `environment.log`, which contains the original binary SHA, so the resulting mixture could retain the old source/binary identity alongside new observations. This undermines the goal of binding a dated result without rewriting history. Preserve the exact capture as an explicitly non-executable archival text record and remove the direct execution recommendation, or make a separately maintained replay tool require a fresh output directory and record its own source/environment identity. The immutable capture option is sufficient for this evidence-only milestone; no new general-purpose QA tool is required. I did not destructively replay the script to demonstrate this: the fixed path and unconditional `write_text` calls show the overwrite directly.

### LOW

None.

## Independent checks and inspected artifacts

- Verified all 1,250 export blobs against the exact candidate tree: zero mismatches. Git diff confirms the candidate changes only the acceptance map and new evidence files; no product code or existing gate receipt changes. Compared the 6ed-to-8edf operational source paths: main/cmd, migration, persistence, platform, operations, notifications and backup/restore are unchanged. The saved `qa-to-final-source.diff` contains the actual three latest-response/test string replacements, not a purported same-binary identity.
- Independently ran `PYTHONDONTWRITEBYTECODE=1 <repository contracts Python> -m pytest -q -p no:cacheprovider scripts` from the export: exit 0, **107 passed in 7.24s**.
- Independently ran the actual architecture CLI from the export. `--all` returned 0; M0 returned 1 for R01 not_run; M6c returned 1 for R39 not_run; M6b returned 0 with only R36.a/R37.a and the preserved source/candidate/web/review bindings; M5 returned 0 for its recorded software scope. SHA256 checks before/after showed the acceptance map and all historical gate receipts unchanged.
- Parsed all 55 unique requirement rows and the operation rows. The only whole-row status demotions are R01/R02/R39. R03/R04 retain their existing software status with actual historical/current references replacing planned text. Provider-operation promotions are confined to the actually logged contract validation, migration and startup/configuration software checks. Other full requirements and external states are unchanged.
- Inspected the manual QA matrix and referenced CLI, HTTP, SQL, recovery, signal, backup/restore and cleanup outputs. `source-rows.log`, `restored-rows.log` and `restore-after-reject.log` are byte-identical, with twelve nonempty table counts and full-row ordered JSON digests. The three restore refusals show exit 2. Dirty readiness, disconnected health/readiness and the intermediate recovery 503 responses are retained. SIGTERM timing and the 100-request health-only sample are explicitly bounded, not claimed to establish high-load drain or device capacity.
- Inspected the earlier failed QA attempts and their corrections; the README does not count those attempts as passed. Inspected `focused-race.log`, the final full Go log and the environment/cleanup artifacts. These are executor observations, not newly repeated database or process runs by this reviewer. The final full Go artifact records build/vet and 33 tested-package race results; the focused log records actual database tests without SKIP.
- `git diff --check` reports whitespace in raw diff/psql/transcript artifacts. These are preserved captures (including diff context space before original tabs and psql column headers), not changed product-code formatting. They are not the approval blocker.

## Required skill-perspective check

Loaded and applied the `remove-ai-slops` and `programming` perspectives, including the Python reference for the archived QA script and the earlier Go/error/testing/logging guidance.

- **Overfit/slop review:** no new deletion-only, prose-pin or tautological tests in this candidate. Existing architecture tests exercise the actual parser/evaluator/CLI and write-protection behavior. QA assertions observe command exits, live HTTP statuses and actual restored data, rather than merely inspecting intended configuration. Whole-row SQL extraction and limited response parsing are needed for those observations; no speculative production abstraction or normalization was added.
- **Programming review:** no product implementation changes, new dependency or provider boundary changes. The ad hoc capture currently has untyped helpers and fixed mutable run state; it should not be presented as a maintained reusable executable. The concrete boundary failure is its ability to replace the evidence/identity record on replay. Preserving its original bytes as a clearly labeled non-executable capture resolves this milestone's issue without retroactively editing the historical script or expanding the task into tool development. If a replay tool is later shipped, its typing, resource namespace, output ownership and identity recording need their own implementation gate.

The runnable replay presentation violates the programming perspective's ownership/boundary discipline. No separate production-code or useless-test violation of either perspective was identified.

## Scope/provenance conclusions

The actual operations binary is bound to the recorded 6ed tree and its binary SHA, not retrospectively to 8edf. The final-code regression is separately bound to 8edf. Historical TODO3/M5 missing Git identities remain missing; this new dated evidence does not invent them. R01's missing full cache/CI acceptance, R02.c's incomplete target-handler matrix, R39.b's future real entry points, and external WeChat/hardware/TLS/large recovery/long capacity checks remain open. Product source and the latest-reading approval are not reapproved by this evidence review.

The state corrections and the observed bounded operational results otherwise have support in the referenced artifacts. This report does not approve the whole V1/M0/M5 or R01-R55 scope. No ulw-loop attempt directory exists (`ULW_LOOP_PLAN_MISSING`), so the report uses the documented fallback path.

## Blockers

- Remove the archived evidence overwrite/replay path while preserving the original capture bytes; then present the revised fixed evidence candidate to both independent reviewers.
