# V1b / D14 independent gate review

recommendation: REJECT
reviewedSha: `48fba0f591b80fd45b0df7d78298b36c935e2464` (Git candidate tree)
base: `de499f4`
surface: `/tmp/iolink-v1b-20261013`

## Original intent and desired outcome

originalIntent: Repair existing acceptance tooling while continuing existing-feature verification under AGENTS.md; retain historical evidence and independently review the same candidate.

desiredOutcome: `--gate` emits read-only evaluation; existing whole-stage gates retain their valid recorded behavior; the explicit M6b profile certifies only R36.a/R37.a using consistent local historical source/web/results and two distinct approving reports. The full R36/R37 requirements and future .b resources remain not_run. This is local evidence consistency, not new business reacceptance or cryptographic authenticity.

## Criteria and blockers

- G1: No receipt creation or modification during evaluation.
- G2: Existing full-parent gate evaluation remains usable with the repository's established historical receipts, while not-passed/blocked/partial evidence cannot certify a whole stage.
- G3: M6b approval is restricted to the declared .a scope and consistent, nonempty local evidence, including two distinct reports approving the same historical candidate.
- G4: Missing, malformed, mismatching, rejecting, duplicate and escaping evidence fails closed.
- G5: Meaningful regression tests and observed real CLI behavior support the result without promoting deferred/external acceptance.

blockers:

1. violatedCriterion: G2
   evidencePointer: `scripts/architecture_gate.py:65`, `scripts/architecture_gate.py:181`, `docs/evidence/rebuild/gates/M6d.json:5`.
   observation: The new WholeReceipt requires a requirement-to-path dictionary for `evidence`. The preserved existing M6d receipt instead contains its approved bundle README as a string. Native `--gate M6d` now exits 1 with a Pydantic dict_type error even though R41/R42 are passed. This is a regression caused by the new parser, not missing evidence or a partial receipt. Support the established bundle receipt explicitly, preserve its bytes, and cover the real repository receipt through the CLI.

## Independently reproduced behavior

Commands used the repository's absolute `.venv/contracts/bin/python` and ran inside the exported candidate.

- `python -m pytest -q scripts`: exit 0, **103 passed in 6.96s**.
- Native `--all`: exit 0.
- Native `--gate M0`, `M5`, `M6a`, `M6b`, `M6c`: exit 0.
- Native `--gate M6d`: exit 1, stdout empty, stderr:

```text
ERROR: M6d gate blocked: 1 validation error for WholeReceipt
evidence
  Input should be an object [type=dict_type, input_value='docs/evidence/M6d/2026-10-07/README.md', input_type=str]
```

- SHA256 of every existing gate JSON plus acceptance-map before/after all seven CLI invocations: unchanged.
- R36 and R37 software_status after replay: both not_run.
- M6b emitted scope R36.a/R37.a and retained historical candidate `03354381b5954087d82d0d3f807cad9993b94b8d`, source `ef6510ffa5e43fd9796352ae324e09dfc3820c68`, web `e84092326d9979e066c611e2be8eb6b9736e09f3`.

## Artifact and scope inspection

Checked candidate diff vs base; `scripts/architecture_gate.py`, both evaluator tests, checker wiring/tests, acceptance-map, all gate receipts, V1b README and before/after/CLI/typecheck/contracts logs. Checked the M6b verification JSON and explicit verdict/candidate fields in its referenced code/gate approval reports. Inspected M6d approved evidence bundle README and its historical receipt. The red regression log shows actual old receipt creation/overwrite failures. Contract log correctly labels 96 operations/568 synthetic fixtures rather than real handler acceptance.

userOutcomeReview: The read-only and M6b subset outcomes are supported. Existing M6d becomes unusable through the same advertised CLI, so this candidate does not yet satisfy the complete tooling outcome.

## Programming / remove-ai-slops direct pass

Applied both skills independently. New production code has typed Pydantic boundaries, explicit result variants and bounded error handling; confinement/alias checks are required by stated criteria. Extraction separates gate evaluation from the existing manifest checker and is warranted. Tests use real temporary files and exercise distinct input classes rather than merely asserting production text or copying implementation decisions. Receipt-byte assertions observe an essential external side effect, and the red log proves they fail on the old bug; they are not deletion-only or tautological tests. No unrelated business code changes or new provider scaffolding appeared in this candidate. The uncovered M6d receipt is a concrete coverage gap despite the green count.

The current independent code-review report was still in progress when this blocking finding was returned; its programming/slop coverage is not asserted. My direct pass supports the coverage stated above. Report absence is not an additional blocker.

## Notes and exact evidence gaps

- `git diff --check` reports whitespace in archived raw CLI/pytest logs; not a functional criterion failure.
- Executor typecheck evidence covers the new production module, not the legacy untyped checker or all tests; README accurately limits that claim.
- No database/browser/external service testing is required for this Python tooling change. External WeChat, hardware, License delivery, TLS/capacity/recovery and deferred M7a/M8 remain outside this approval.
- Historical author independence and report cryptographic authenticity are not verified by this local consistency tool and are not claimed.

No source files were edited. Only this review report was written.
