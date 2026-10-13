# V1b / D14 independent final code review

- recommendation: **APPROVE**
- codeQualityStatus: **WATCH**
- Exact candidate tree: `a5b3259aca2cbacd7e0f4510545ddfd0b5501293`
- Base: `de499f4`
- Reviewed export: `/tmp/iolink-v1b-r2-20261013`
- Reviewer: `/root/v1b_code_review`, not an implementer.
- Date: 2026-10-13
- Blockers: **none** for this recorded-evidence tooling milestone.

This is a fresh review of the repaired frozen tree. The prior REQUEST_CHANGES report at `.omo/evidence/v1b-code-review.md` remains valid for the older tree `48fba0f591b80fd45b0df7d78298b36c935e2464` and is not rewritten as an approval.

## Findings by severity

### CRITICAL

None.

### HIGH

None remaining. The original M6d receipt-shape regression is resolved: `WholeReceipt.evidence` at `scripts/architecture_gate.py:68` supports the recorded mapping or nonempty bundle-path form, and the latter is checked through the same repository-confined regular-file boundary at `scripts/architecture_gate.py:191`. `PartialReceipt.evidence` remains a strict mapping at line 75. The real M6d CLI now exits 0 without changing its historical receipt.

### MEDIUM

None.

### LOW

1. **Strict programming-style follow-ups remain, with no observed acceptance failure.** The two-variant evidence discriminator uses `isinstance`/`elif` at `scripts/architecture_gate.py:191`, while the programming skill prefers exhaustive `match` for union variants. The six-argument evaluator adapter at line 160 and the limited Ruff rule set are also departures from that skill's strict style baseline. They are bounded in this small evidence evaluator and do not cause another correctness or regression failure in the reviewed scope. The evidence README accurately states that full-library Ruff ALL and full legacy-checker type checking are not claimed as passed.

## Independent checks on this exact tree

Commands used the host's absolute Python executable:

```text
/media/yun/706bc403-c76c-4fdd-8a3f-d954b6189048/iolink/.venv/contracts/bin/python
```

All code commands ran in `/tmp/iolink-v1b-r2-20261013`, with bytecode/cache generation disabled where applicable.

| Check | Actual result |
|---|---|
| `python -B -m pytest -q -p no:cacheprovider scripts` | 107 passed in 7.05s, exit 0 |
| `python -B scripts/check_architecture_manifests.py --all` | 55-requirement validation PASS, exit 0 |
| Actual CLI `--gate M0`, `M5`, `M6a`, `M6b`, `M6c`, `M6d` | All exit 0 |
| SHA256 before/after all those CLI invocations | All five existing gate JSON files and acceptance-map unchanged |
| `python -B scripts/check_contracts.py` | 96 target operations and 568 synthetic request/response fixtures PASS, exit 0 |
| `ruff check --select E4,E7,E9,F,I --no-cache` on the new evaluator and two test files | All checks passed, exit 0 |

The actual M6b JSON returns only `R36.a`/`R37.a` as scope, with these historical identities:

- candidate: `03354381b5954087d82d0d3f807cad9993b94b8d`
- verified_source: `ef6510ffa5e43fd9796352ae324e09dfc3820c68`
- web_commit: `e84092326d9979e066c611e2be8eb6b9736e09f3`

These match the inspected verification record and both historical report fields. The unchanged manifest retains whole R36/R37 as not_run. M6d returns whole R41/R42 scope, retaining its original bundle receipt bytes.

Reviewed the repaired diff against both the original candidate and the supplied base; inspected the frozen source/tests, the real M6b and M6d receipts, the historical M6b verification/review files, and `docs/evidence/rebuild/reconciliation/2026-10-13-gates/` artifacts. In particular, the corrected CLI regression copies the actual repository's M6d JSON, and the bundle tests separately reject missing and outside-root files while checking that no receipt is repaired or rewritten.

## Skill-perspective check

Loaded and consulted `omo:remove-ai-slops`, `omo:programming`, and the Python reference before the initial review, and reapplied those criteria to this repaired diff.

The remove-ai-slops pass finds no useless removal-only, tautological, prompt-text, or production-constant-mirroring tests. File absence and byte preservation are observable requirements with a demonstrated pre-change failure. The added M6d regression covers a real repository format that exposed a missed path; it does not merely assert that an extra union variant was added. Adversarial receipt/verification values, explicit rejecting or mismatched reports, real filesystem aliases and absent/outside artifacts exercise boundary behavior. Production parsing is needed at this recorded-evidence boundary; no unrelated extraction, normalization or speculative framework was added.

The programming pass finds typed Pydantic boundary models, strict exit-code values, immutable model shells, specific exception handling and a typed success/failure result with exhaustive caller handling. No new casts, type-ignore suppression, broad catch-all or untyped public escape hatch was found. The narrow style departures are recorded as LOW above rather than represented as a full strict-tool pass. No remaining skill-perspective issue warrants blocking this scope.

## Approval scope and residual limits

Approval covers the read-only gate CLI repair and explicit local M6b `.a` receipt-consistency evaluation. Whole-gate statuses still control whole acceptance; missing/malformed/blocked/partial-only whole receipts and inconsistent partial evidence remain rejected. Partial scope requires matched source/web verification records, successful nonempty local artifacts and two distinct approving report files naming the same candidate. Tests retain rejection for aliases, bad paths, rejecting/conflicting reports, wrong candidates, failed/malformed exit codes and missing evidence.

This checker does not authenticate authors or establish cryptographic provenance, candidate/source Git ancestry, truly independent reviewers, or fresh runtime business acceptance. Historical TODO3/M5 binding gaps, external service/hardware/License delivery checks and deferred new business functions remain outside this milestone. Whole R36/R37 and future `.b` are not certified by this approval.

No business source was edited during review. No database, browser or container was used. The report uses the required fallback artifact path because `ulw-loop status` reported `ULW_LOOP_PLAN_MISSING` in the previous review turn.
