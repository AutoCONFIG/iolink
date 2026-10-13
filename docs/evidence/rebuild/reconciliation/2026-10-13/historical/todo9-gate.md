# Todo 9 final gate review

recommendation: APPROVE
reviewed_at: 2026-09-28
reviewer: independent final gate reviewer
review_scope: current shared working tree for Todo 9 / M5 / R28-R33
review_tree: shared working tree

## originalIntent

Deliver M5 operations and notification release controls with reproducible R28-R33 software evidence, including a consistent backup/restore check that compares the migration version and required data summaries. External-only checks must remain `external_blocked`, and completion requires two independent reviews of the same tree.

## desiredOutcome

The release gate must fail closed when the restored schema migration state differs from the source backup, and the evidence must be reproducible from the checked-in scripts.

## userOutcomeReview

Most M5 software checks and external-blocked labels are present and reproducible. The runner now separates software success from skipped integration (`software=passed`, `integration=skipped_external_blocked`). The restore script requires metadata, parses and compares PostgreSQL version, TimescaleDB version, and `migration_rows`, and fails on mismatch. The previously identified R31 and runner-state blockers are closed.

## blockers

[]

## checked_artifacts

- `docs/README.md`
- `docs/PLAN.md`
- `docs/PLAN-DETAILS.md`
- `docs/ACCEPTANCE.md`
- `docs/IMPLEMENTED.md`
- `docs/CONTRIBUTING.md`
- `docs/evidence/rebuild/m5-operations.txt`
- `docs/evidence/rebuild/gates/M5.json`
- `docs/evidence/acceptance/R28/README.md` through `docs/evidence/acceptance/R33/README.md`
- `scripts/run_external_case.py`
- `scripts/capacity_smoke.py`
- `deploy/backup.sh`
- `deploy/restore.sh`
- `.github/workflows/ci.yml`
- `.gitlab-ci.yml`

## reproduced verification

- `python3 -m pytest -q scripts/test_capacity_smoke.py scripts/test_check_architecture_manifests.py`: 31 passed.
- `python3 scripts/check_architecture_manifests.py --all`: PASS.
- `make verify`, `make web-verify`, and contract checks: PASS per current evidence.
- `bash -n deploy/backup.sh deploy/restore.sh`: PASS.
- No-DSN runner classification: software checks pass while integration is explicitly `skipped_external_blocked`; external remains `external_blocked`.
- Pinned Timescale isolated restore replay after the fix: PASS, including migration metadata comparison and `restore verified` (reported by the executor and recorded in the current M5 evidence).
- Direct source inspection confirms `deploy/restore.sh` requires `.meta`, validates and compares PostgreSQL/Timescale versions and numeric `migration_rows`, and exits nonzero on mismatch.

## remove-ai-slops and programming pass

The direct pass found no criterion-linked overfit/slop blocker in the M5 tests or production code. The restore migration assertion is now present and is a required observable check.

## exact evidence gaps

No criterion-linked evidence gap remains for the software scope. Real WeChat, public TLS/firewall, 100 GiB RTO, timed release interruption, and sustained 500-device capacity checks remain `external_blocked` as documented.
