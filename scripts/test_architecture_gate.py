# /// script
# requires-python = ">=3.13"
# dependencies = ["pydantic==2.13.4", "pytest==9.1.1"]
# ///
# How to run: .venv/contracts/bin/python -m pytest scripts/test_architecture_gate.py

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Literal, assert_never

import pytest
from architecture_gate import GateFailure, GateSuccess, PartialProfile, evaluate_gate
from pydantic import ValidationError

SHA = "a" * 40
SOURCE = "b" * 40
WEB = "c" * 40
RECEIPT = "docs/evidence/rebuild/gates/M6b.json"
EVIDENCE = {
    "R36": "docs/evidence/acceptance/R36/",
    "R37": "docs/evidence/acceptance/R37/",
}


@dataclass(frozen=True, slots=True)
class Fixture:
    root: Path
    profile: PartialProfile
    receipt: Path
    verification: Path
    reviews: tuple[Path, Path]
    artifact: Path

    def evaluate(self) -> GateSuccess | GateFailure:
        return evaluate_gate(
            self.root,
            "M6b",
            ("R36", "R37"),
            {"R36": "not_run", "R37": "not_run"},
            EVIDENCE,
            self.profile,
        )


@pytest.fixture
def recorded(tmp_path: Path) -> Fixture:
    receipt = tmp_path / RECEIPT
    receipt.parent.mkdir(parents=True)
    bundle = tmp_path / "docs/evidence/acceptance/R37/phase"
    bundle.mkdir(parents=True)
    artifact = bundle / "run.log"
    artifact.write_text("real fixture result\n", encoding="utf-8")
    verification = bundle / "verification.json"
    verification.write_text(
        json.dumps(
            {
                "source": SOURCE,
                "web": WEB,
                "results": [{"name": "scenario", "code": 0, "artifact": "run.log"}],
            }
        ),
        encoding="utf-8",
    )
    reviews = (bundle / "code.md", bundle / "gate.md")
    reviews[0].write_text(
        f"- Verdict: **APPROVE** for phase.\n- Exact candidate: **{SHA}**\n",
        encoding="utf-8",
    )
    reviews[1].write_text(
        f"recommendation: **APPROVE**\nreviewedSha: `{SHA}`\n", encoding="utf-8"
    )
    receipt.write_text(
        json.dumps(
            {
                "gate": "M6b",
                "status": "complete",
                "requirements": ["R36", "R37"],
                "evidence": EVIDENCE,
                "verified_scope": ["R36.a", "R37.a"],
                "candidate": SHA,
                "verified_source": SOURCE,
                "web_commit": WEB,
                "verification": str(verification.relative_to(tmp_path)),
                "reviews": [str(path.relative_to(tmp_path)) for path in reviews],
                "blocked_by": [],
                "historical_metadata": {"keep": True},
            }
        ),
        encoding="utf-8",
    )
    profile = PartialProfile(scope=("R36.a", "R37.a"), receipt=RECEIPT)
    return Fixture(tmp_path, profile, receipt, verification, reviews, artifact)


def test_accepts_partial_evidence_without_changing_receipt_or_parent_status(
    recorded: Fixture,
) -> None:
    # Given: valid recorded .a evidence while whole requirements remain not_run.
    before = recorded.receipt.read_bytes()
    # When: evaluating the partial profile.
    result = recorded.evaluate()
    # Then: scope and identities are explicit, and receipt bytes are untouched.
    assert isinstance(result, GateSuccess)
    assert result.scope == ("R36.a", "R37.a")
    assert (result.candidate, result.verified_source, result.web_commit) == (
        SHA,
        SOURCE,
        WEB,
    )
    assert recorded.receipt.read_bytes() == before


@pytest.mark.parametrize(
    ("field", "value"),
    [
        ("gate", "M6c"),
        ("status", "pending"),
        ("requirements", ["R36"]),
        ("verified_scope", ["R36.a", "R37.b"]),
        ("verified_scope", ["R36.a", "R36.a"]),
        ("candidate", "short"),
        ("verified_source", "wrong"),
        ("web_commit", "invalid"),
        ("blocked_by", ["R37"]),
        ("reviews", ["missing.md", "absent.md"]),
        ("verification", "../outside.json"),
    ],
)
def test_rejects_inconsistent_receipt(
    recorded: Fixture, field: str, value: str | list[str]
) -> None:
    # Given: one broken structured receipt field.
    payload = json.loads(recorded.receipt.read_text(encoding="utf-8"))
    payload[field] = value
    recorded.receipt.write_text(json.dumps(payload), encoding="utf-8")
    # When: evaluating its phase.
    result = recorded.evaluate()
    # Then: no success can be claimed.
    assert isinstance(result, GateFailure)


@pytest.mark.parametrize(
    ("field", "value"), [("source", "d" * 40), ("web", "e" * 40), ("results", [])]
)
def test_rejects_inconsistent_verification(
    recorded: Fixture, field: str, value: str | list[str]
) -> None:
    # Given: a mismatching or empty verification record.
    payload = json.loads(recorded.verification.read_text(encoding="utf-8"))
    payload[field] = value
    recorded.verification.write_text(json.dumps(payload), encoding="utf-8")
    # When: evaluating its phase.
    result = recorded.evaluate()
    # Then: mismatched provenance cannot pass.
    assert isinstance(result, GateFailure)


@pytest.mark.parametrize("code", [1, True, "0"])
def test_rejects_failed_or_malformed_exit_code(
    recorded: Fixture, code: int | bool | str
) -> None:
    # Given: a failed or noninteger command result.
    payload = json.loads(recorded.verification.read_text(encoding="utf-8"))
    payload["results"][0]["code"] = code
    recorded.verification.write_text(json.dumps(payload), encoding="utf-8")
    # When: evaluating its phase.
    result = recorded.evaluate()
    # Then: strict successful exit is required.
    assert isinstance(result, GateFailure)


@pytest.mark.parametrize(
    "text",
    [
        f"Verdict: REJECT.\nExact candidate: {SHA}\nLater history says APPROVE.\n",
        f"recommendation: REQUEST_CHANGES\nreviewedSha: {SHA}\nVerdict: APPROVE\n",
        f"The candidate {SHA} was inspected and APPROVE is mentioned.\n",
        f"recommendation: APPROVE\nreviewedSha: {'d' * 40}\n",
    ],
)
def test_rejects_reports_without_explicit_candidate_approval(
    recorded: Fixture, text: str
) -> None:
    # Given: a nonapproval, conflicting verdict, or wrong candidate report.
    recorded.reviews[0].write_text(text, encoding="utf-8")
    # When: evaluating its phase.
    result = recorded.evaluate()
    # Then: prose mentioning approval is insufficient.
    assert isinstance(result, GateFailure)


def test_rejects_duplicate_review_alias(recorded: Fixture) -> None:
    # Given: two paths resolving to the same approving report.
    recorded.reviews[1].unlink()
    recorded.reviews[1].symlink_to(recorded.reviews[0].name)
    # When: evaluating its phase.
    result = recorded.evaluate()
    # Then: one report cannot count twice.
    assert isinstance(result, GateFailure)


def test_rejects_hardlinked_duplicate_review(recorded: Fixture) -> None:
    # Given: two filenames for one review file inode.
    recorded.reviews[1].unlink()
    recorded.reviews[1].hardlink_to(recorded.reviews[0])
    # When: evaluating its phase.
    result = recorded.evaluate()
    # Then: aliases cannot count as distinct reviews.
    assert isinstance(result, GateFailure)


@pytest.mark.parametrize("broken", ["absent", "empty", "outside"])
def test_rejects_unusable_command_artifact(
    recorded: Fixture, broken: Literal["absent", "empty", "outside"]
) -> None:
    # Given: a command artifact absent, empty, or symlinked outside the repo.
    recorded.artifact.unlink()
    match broken:
        case "empty":
            recorded.artifact.touch()
        case "outside":
            outside = recorded.root.parent / f"{recorded.root.name}-outside.log"
            outside.write_text("outside", encoding="utf-8")
            recorded.artifact.symlink_to(outside)
        case "absent":
            pass
        case unreachable:
            assert_never(unreachable)
    # When: evaluating its phase.
    result = recorded.evaluate()
    # Then: invalid artifact location cannot pass.
    assert isinstance(result, GateFailure)


@pytest.mark.parametrize("content", ["{", "[]", "null"])
def test_rejects_malformed_receipt_without_repair(
    recorded: Fixture, content: str
) -> None:
    # Given: malformed JSON or the wrong document shape.
    recorded.receipt.write_text(content, encoding="utf-8")
    # When: evaluating its phase.
    result = recorded.evaluate()
    # Then: malformed evidence fails without rewriting it.
    assert isinstance(result, GateFailure)
    assert recorded.receipt.read_text(encoding="utf-8") == content


@pytest.mark.parametrize(
    "scope", [("R36.a", "R37.b"), ("R36", "R37"), ("R36.a",), ("R36.a", "R36.a")]
)
def test_profile_refuses_wider_or_incomplete_phase(scope: tuple[str, ...]) -> None:
    # Given: a declaration exceeding or missing the supported phase.
    # When/Then: parsing rejects it at the boundary.
    with pytest.raises(ValidationError):
        PartialProfile(scope=scope, receipt=RECEIPT)


def test_rejects_missing_receipt(recorded: Fixture) -> None:
    # Given: a declared partial profile whose receipt is absent.
    recorded.receipt.unlink()
    # When: evaluating the declared phase.
    result = recorded.evaluate()
    # Then: absence cannot turn into a generated acceptance record.
    assert isinstance(result, GateFailure)
    assert not recorded.receipt.exists()


def test_rejects_valid_but_wrong_candidate(recorded: Fixture) -> None:
    # Given: a valid SHA different from the review candidate.
    payload = json.loads(recorded.receipt.read_text(encoding="utf-8"))
    payload["candidate"] = "f" * 40
    recorded.receipt.write_text(json.dumps(payload), encoding="utf-8")
    # When: evaluating the phase.
    result = recorded.evaluate()
    # Then: SHA syntax alone cannot certify a mismatched candidate.
    assert isinstance(result, GateFailure)


def test_profile_cannot_be_applied_to_another_gate(recorded: Fixture) -> None:
    # Given: the M6b profile with a caller requesting M6c.
    # When: evaluating the unrelated gate.
    result = evaluate_gate(
        recorded.root, "M6c", ("R38", "R39", "R40"), {}, {}, recorded.profile
    )
    # Then: phase bypass is scoped to its allowed gate only.
    assert isinstance(result, GateFailure)
