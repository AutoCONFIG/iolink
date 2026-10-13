# /// script
# requires-python = ">=3.13"
# dependencies = ["pydantic==2.13.4", "pytest==9.1.1"]
# ///
# How to run: .venv/contracts/bin/python -m pytest scripts/test_architecture_gate*.py

import json
from pathlib import Path

import pytest
from architecture_gate import GateFailure, GateSuccess, evaluate_gate
from test_architecture_gate import EVIDENCE, Fixture

pytest_plugins = ("test_architecture_gate",)


@pytest.mark.parametrize("bundle", ["bundle.md", "absent.md", "../outside.md"])
def test_whole_gate_reads_historical_bundle_receipt(tmp_path: Path, bundle: str) -> None:
    receipt = tmp_path / "docs/evidence/rebuild/gates/M6d.json"
    receipt.parent.mkdir(parents=True)
    (tmp_path / "bundle.md").write_text("historical evidence", encoding="utf-8")
    receipt.write_text(json.dumps({
        "gate": "M6d", "requirements": ["R41", "R42"],
        "evidence": bundle, "reviews": {"code": "old-code.md", "gate": "old-gate.md"},
    }), encoding="utf-8")
    before = receipt.read_bytes()
    result = evaluate_gate(tmp_path, "M6d", ("R41", "R42"),
                           {"R41": "passed", "R42": "passed"},
                           {"R41": "evidence/R41/", "R42": "evidence/R42/"})
    assert isinstance(result, GateSuccess if bundle == "bundle.md" else GateFailure)
    assert receipt.read_bytes() == before


def test_requires_full_parent_status_without_partial_profile(recorded: Fixture) -> None:
    # Given: the .a receipt but no profile, while parent status is not_run.
    # When: evaluating the whole gate.
    result = evaluate_gate(
        recorded.root,
        "M6b",
        ("R36", "R37"),
        {"R36": "not_run", "R37": "not_run"},
        EVIDENCE,
    )
    # Then: partial receipt cannot promote whole requirement acceptance.
    assert isinstance(result, GateFailure)


def test_partial_receipt_cannot_certify_whole_gate_even_with_passed_status(
    recorded: Fixture,
) -> None:
    # Given: whole status passed but a receipt proving only the .a subset.
    # When: evaluating without a partial profile.
    result = evaluate_gate(
        recorded.root,
        "M6b",
        ("R36", "R37"),
        {"R36": "passed", "R37": "passed"},
        EVIDENCE,
    )
    # Then: subset proof cannot become full-requirement proof.
    assert isinstance(result, GateFailure)


def test_whole_gate_does_not_create_receipt(tmp_path: Path) -> None:
    # Given: passed whole requirements without an existing receipt.
    # When: evaluating the whole gate.
    result = evaluate_gate(
        tmp_path,
        "M0",
        ("R01",),
        {"R01": "passed"},
        {"R01": "docs/evidence/acceptance/R01/"},
    )
    # Then: JSON result is possible without touching the filesystem.
    assert isinstance(result, GateSuccess)
    assert result.scope == ("R01",)
    assert list(tmp_path.iterdir()) == []


@pytest.mark.parametrize("valid", [True, False])
def test_whole_gate_preserves_existing_receipt(recorded: Fixture, valid: bool) -> None:
    # Given: a valid whole receipt or corrupted bytes.
    recorded.receipt.write_text(
        json.dumps(
            {
                "gate": "M6b",
                "requirements": ["R36", "R37"],
                "evidence": EVIDENCE,
                "historical": "retain",
            }
        )
        if valid
        else "{",
        encoding="utf-8",
    )
    before = recorded.receipt.read_bytes()
    # When: evaluating the whole gate.
    result = evaluate_gate(
        recorded.root,
        "M6b",
        ("R36", "R37"),
        {"R36": "passed", "R37": "passed"},
        EVIDENCE,
    )
    # Then: only valid evidence succeeds, but neither input is overwritten.
    assert isinstance(result, GateSuccess if valid else GateFailure)
    assert recorded.receipt.read_bytes() == before
