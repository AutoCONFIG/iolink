# V1c current-source evidence: independent gate review

recommendation: REJECT

Exact candidate tree: `10ae4fb60edefeee0622ca5354ebe63554fc6a9a`

Base: `fbfa7ba` (v0.0.35)

Export: `/tmp/iolink-v1-current-20261013`

Reviewer: `/root/latest_gate_review`, read-only except this report.

## Blockers

- violatedCriterion: C2 — preserve historical evidence and keep source/command/output identity accurate when documenting reproduction.
  Observation: the added README directly instructs executing `run_ops_qa.py` from its archived evidence directory. That script defines `OUT = Path(__file__).resolve().parent` and repeatedly writes logs, SQL fixtures, recovery probes, shutdown observations and summary JSON into that same directory. A rerun overwrites the historical bundle while keeping fixed historical source identifiers and an existing binary path. It can therefore destroy the evidence that this audit is meant to preserve and bind new observations to stale declarations.
  evidencePointer: `docs/evidence/rebuild/reconciliation/2026-10-13-current-software/README.md:23`; `run_ops_qa.py:11-15`, `:20-25`, `:150`.
  Required fix: retain the original script bytes as a nonexecutable historical capture, remove the in-place execution instruction, and explain that a future QA run needs a fresh output directory and independently recorded source/build identity. A prose/capture correction does not require a test pinning filenames or wording. Present a new candidate for both independent reviews.

## Original intent and desired outcome

originalIntent: continue verifying existing functionality autonomously, preserve actual evidence and historical review identities, and correct the implementation/status ledger rather than claim unverified features are complete.

desiredOutcome: a bounded V1c reconciliation of all 55 requirement rows and fresh software operations evidence tied to known source snapshots, with honest whole-row versus partial-scope states and unchanged historical gate receipts.

userOutcomeReview: the existing observations and status corrections support that bounded reconciliation. The reproduction instruction creates a concrete risk to preservation and source identity and must be corrected before this candidate is approved. Nothing in this report certifies all V1, R01–R55, deferred M7/M8 features or external acceptance.

## Checked criteria

| Criterion | Result | Evidence |
|---|---|---|
| C1: candidate source identity is known and product code is unchanged | PASS | 1,250 exported blobs matched; no internal/cmd/web/deploy/scripts product diff from base; internal/cmd hashes match approved final latest-reading source |
| C2: historical receipts/evidence are preserved and documented reproduction cannot misbind outputs | FAIL | receipts remain byte-identical, but the archived-script execution guidance rewrites the new history bundle as described above |
| C3: whole rows do not receive partial-scope false passes | PASS in this diff | R01/R02/R39 become not_run; R03/R04 replace planned prose with historical/current regression references; provider-operation promotion limited to performed R02 contract/R03 migration/R04 configuration |
| C4: real operations evidence supports only the stated bounded claims | PASS | raw health/dependency/recovery responses; 12-table byte-identical comparison; explicit exclusions for counters, capacity, 100GiB/RPO, TLS and external providers |
| C5: gate tooling remains read-only and expected whole/partial states are reproducible | PASS | independent 107 Python tests; actual CLI checks and before/after hash equality |

## Independent verification and observed evidence

- Verified every tracked blob in the exact export against NUL-delimited Git tree entries and `git hash-object`: 1,250 matched, zero mismatches.
- Independently ran from the export, using the root repository contracts venv: `python -m pytest -q scripts`: exit 0, 107 passed in 7.17s.
- Independently ran `scripts/check_architecture_manifests.py`: `--all` exit 0; `--gate M0` exit 1 with R01 not_run; `--gate M6c` exit 1 with R39 not_run; `--gate M6b` exit 0 with scope exactly R36.a/R37.a; `--gate M5` exit 0 as the historical software gate. No result was represented as fresh whole business/external acceptance.
- Hashed acceptance-map and every gate JSON before/after these actual CLI invocations: all hashes unchanged. Parsed all 55 rows: 17 software passed, 38 not_run.
- Compared `source-rows.log`, `restored-rows.log`, and `restore-after-reject.log`: files are byte-identical, all 12 named tables nonempty. Restore flag, nonisolated name and bad checksum transcripts each contain exit 2. Shutdown JSON records SIGTERM, exit 0, 0.003169s under a 15s bound.
- Inspected raw HTTP transcripts: normal health/ready 200; dirty migration health 200/ready 503; database denied health/ready 503; recovery probes preserve four intermediate 503 responses then 200, approximately 0.426s. Metrics prove process/Go text availability; capacity smoke proves only 100 health requests, not device workload acceptance.
- Counted 44 top-level PASS lines in the five-package real-DSN focused race transcript; no top-level SKIP/FAIL. Inspected named migration rollback/checksum/concurrency, provider isolation/configuration, dependency and worker cases. These are executor observations, not a new independent runtime replay.
- Inspected environment, binary SHA, source summary and QA-to-final diff. Independently confirmed 6ed→8edf changes only the two management response values and their test assertion. Current candidate and approved final latest-reading implementation have identical internal/cmd object hashes. Web gitlink is the declared `39299dfb0d7771690117a2765fb766a9b715f554`. New QA records do not retroactively establish unknown TODO3/M5 candidate identities.
- Inspected cleanup transcripts: the two QA databases were dropped; temporary container/host paths were absent, shared task container retained for review. This reviewer created no container or database and did not rerun the archived mutation script.

## Direct programming/remove-ai-slops perspective

Loaded and directly applied `omo:programming` and `omo:remove-ai-slops`; the preceding latest-review turns had loaded their complete skill/reference instructions. This candidate has no product-source changes and no new regression tests. Existing Python gate tests exercise parsed gate inputs, statuses and file preservation; no new prose pins or deletion-only tests were introduced.

The archived QA script drives actual CLI/HTTP/SQL surfaces with bounded timeouts, expected exit/status assertions, secret exclusion and cleanup. Its intended historical-capture purpose needs no reusable framework, extraction, parsing layer or expanded test suite. The C2 blocker is a concrete side-effect/source-binding issue: documentation treats a script that rewrites the evidence directory as an executable reproduction entry point. Keeping the original script as a capture and separating future execution output addresses the actual issue without creating implementation-mirroring tests. The script is an untyped, fixed-environment one-run capture, not a strict reusable product tool; no unavailable type/lint gates are claimed passed.

`git diff --check` flags whitespace in archived raw SQL output, diff context and CLI transcripts. Those are preserved observations and do not violate this milestone's outcome; record as NOTE rather than alter original transcripts to make formatting pass.

At report time the final code-review artifact had not yet been written. Root relayed the script issue; I verified it directly in the candidate README and script rather than treating the relay as proof. My own direct skill-perspective checks and evidence inspection support the findings above. The fixed candidate must receive fresh independent code/gate reports with explicit programming/slop coverage.

## Checked paths and remaining limits

Checked `docs/architecture/acceptance-map.yaml`, `docs/ACCEPTANCE.md`, `docs/IMPLEMENTED.md`, historical R03/R04/S4/S5/M6c references and gate JSONs; the complete changed-file list; and the new current-software README, row audit, manual-QA matrix, archived QA script, environment/source/CLI/HTTP/metrics/recovery/shutdown/backup/restore/comparison/focused-test/cleanup artifacts.

The original historical TODO3/M5 source binding gaps remain explicitly unknown; this new date/source evidence does not repair them retrospectively. Whole R01/R02/R39 pending scope remains blocked. Browser QA is user-owned; real WeChat/hardware/customer License, TLS, business counters, high-load draining, 100GiB restore/RPO, interrupted deployment recovery and full device capacity are not certified. Product code, deferred M7/M8 work, and full-project acceptance remain outside this review.

No ulw-loop attempt directory was available in this session, so this report uses the required fallback evidence path.
