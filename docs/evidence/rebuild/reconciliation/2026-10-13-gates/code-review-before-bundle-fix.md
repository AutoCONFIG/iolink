# V1b / D14 independent code review

- recommendation: **REQUEST_CHANGES**
- codeQualityStatus: **BLOCK**
- Exact candidate tree: `48fba0f591b80fd45b0df7d78298b36c935e2464`
- Base: `de499f4`
- Reviewed export: `/tmp/iolink-v1b-20261013`
- Reviewer: `/root/v1b_code_review`, not an implementer.
- Date: 2026-10-13

## Findings

### CRITICAL

None.

### HIGH

1. **The new whole-receipt parser rejects the existing valid M6d receipt.** `scripts/architecture_gate.py:65` requires every receipt's `evidence` to be a dictionary; `scripts/architecture_gate.py:181` now applies that shape to every existing whole-gate receipt. However, the unchanged `docs/evidence/rebuild/gates/M6d.json:5` records its evidence bundle as `"docs/evidence/M6d/2026-10-07/README.md"`, alongside source/web identities, checks and two review paths. Both R41 and R42 remain `software_status: passed` in the candidate manifest. On this exact export, the real CLI now exits 1:

   ```text
   ERROR: M6d gate blocked: 1 validation error for WholeReceipt
   evidence
     Input should be an object [type=dict_type, input_value='docs/evidence/M6d/2026-10-07/README.md', input_type=str]
   ```

   This is a regression in an existing supported gate caused by imposing a new receipt shape. The parent-status evaluator previously allowed this recorded gate. Preserving the historical JSON bytes while consistently evaluating its recorded bundle is part of this milestone's scope. The new tests cover synthetic mapping-shaped whole receipts, and the CLI success parametrization at `scripts/test_check_architecture_manifests.py:107` only covers M6a/M6c; therefore the 103-test green result does not establish compatibility with all current gate records.

   **Required before approval:** handle the existing M6d bundle receipt through an explicit supported shape while retaining fail-closed checks and keeping the receipt unchanged. Add a behavioral regression for that real receipt shape and run all supported real CLI gates. Do not overwrite the historical receipt or silently ignore malformed evidence to make the check pass.

### MEDIUM

None beyond the concrete regression above.

### LOW

1. **The strict programming-tool baseline remains incomplete, as already disclosed in the evidence.** Independently running Ruff `--select ALL` on the new module and its two test files reports 87 diagnostics. These include documentation/trailing-comma rules and expected pytest assert-rule conflicts, plus the six-argument `evaluate_gate` interface at `scripts/architecture_gate.py:159`. This is not evidence of another functional blocker. Keep the limited E4/E7/E9/F/I lint result distinct from a full strict-tool pass; the candidate README does so. A later tooling configuration should encode appropriate test-specific exceptions rather than imply that all strict checks currently pass.

## Skills and relevance review

Loaded and applied `omo:remove-ai-slops`, `omo:programming`, and the Python programming reference before judging maintainability or test relevance.

The remove-ai-slops perspective found no deletion-only, tautological, prompt-text, or implementation-constant-only tests in this change. Receipt byte preservation and absence of new files are observable requirements: the before-change log records three failures proving that the old successful CLI paths created or overwrote receipts. Adversarial JSON, candidate mismatch, rejecting verdicts, symlink/hardlink aliasing and missing artifacts exercise real boundary behavior. Parsing and Markdown field normalization occur at the recorded-evidence boundary and are needed to interpret the existing historical report formats; no unrelated extraction framework was introduced.

The programming perspective found typed Pydantic boundary models, a typed failure/success union, exhaustive result handling, specific caught exceptions and no added cast/type-ignore escape hatches. The strict lint/signature guidance has the LOW limitation above. The HIGH finding demonstrates a boundary-model correctness failure, not merely a style preference. Fixing it needs a real historical-shape regression rather than a prose or constant pin.

## Checks actually run and artifacts inspected

Used the host's absolute `.venv/contracts/bin/python` with `-B` against the frozen export; no business source was edited and no database/browser/container was used.

- `python -B -m pytest -q -p no:cacheprovider scripts`: **103 passed in 6.94s** on this candidate.
- Real `check_architecture_manifests.py --all`: exit 0.
- Real `--gate M0`, `M5`, `M6a`, `M6b`, `M6c`: exit 0.
- Real `--gate M6d`: **exit 1**, HIGH finding reproduced above.
- SHA256 before/after `--all`, M6b, M6a and M0 was unchanged for all five existing gate JSON files and `acceptance-map.yaml`.
- M6b output explicitly reports scope `R36.a`/`R37.a`, candidate `03354381b5954087d82d0d3f807cad9993b94b8d`, source `ef6510ffa5e43fd9796352ae324e09dfc3820c68` and web `e84092326d9979e066c611e2be8eb6b9736e09f3`.
- Inspected the actual M6b verification JSON and both referenced historical APPROVE reports; their source/web/candidate fields match the emitted values.
- Inspected `docs/evidence/rebuild/reconciliation/2026-10-13-gates/README.md`, `native-cli.log`, `all-python-tests.log`, `contracts.log`, `typecheck.log`, `ruff.log`, `no-excuse.log`, and the root `.debug-journal.md`. Their scoped claims do not claim fresh whole-project runtime acceptance.
- Independent Ruff ALL pass: **exit 1, 87 diagnostics**, explicitly not counted as passed.
- `omo-agent-toolkit ulw-loop status --json`: `ULW_LOOP_PLAN_MISSING`; report uses the required fallback path.

## Residual scope limits

The evaluator checks local recorded-evidence consistency. It does not establish cryptographic provenance, actual independent human authorship, candidate/source Git ancestry, or fresh runtime acceptance. Whole R36/R37 and future `.b` remain unpromoted. Existing TODO3/M5 historical source-binding gaps, external service/hardware/License delivery checks and deferred new business features are not closed by this review.

## Blockers

- Fix the M6d whole-receipt shape regression and show the unchanged historical receipt is accepted through the real CLI under the supported evidence rules.
- Obtain independent approvals on the new fixed candidate; this report does not approve the current tree.
