"""Read-only validation of recorded gate evidence, not fresh runtime acceptance.

Local receipt consistency cannot establish cryptographic provenance or reviewer
independence beyond two distinct report files. Historical metadata is preserved.
"""
# /// script
# requires-python = ">=3.13"
# dependencies = ["pydantic==2.13.4"]
# ///
# How to run: imported by check_architecture_manifests.py; tests via pytest.

import re
from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from typing import Annotated, Final, Literal, TypeVar

from pydantic import (
    BaseModel,
    ConfigDict,
    Field,
    StrictInt,
    StrictStr,
    ValidationError,
    model_validator,
)
from pydantic_core import PydanticCustomError

Sha = Annotated[StrictStr, Field(pattern=r"^[0-9a-f]{40}$")]
Text = Annotated[StrictStr, Field(min_length=1)]
PARTS: Final = ("R36.a", "R37.a")
RECEIPT: Final = "docs/evidence/rebuild/gates/M6b.json"
VERDICT: Final = re.compile(
    r"^(?:-\s*)?(?:Verdict|recommendation)\s*:\s*(APPROVE|REJECT|REQUEST_CHANGES|BLOCK)\b",
    re.IGNORECASE,
)
IDENTITY: Final = re.compile(
    r"^(?:-\s*)?(?:Exact candidate|reviewedSha)\s*:\s*([0-9a-f]{40})\b"
)


class FrozenModel(BaseModel):
    model_config = ConfigDict(frozen=True, extra="ignore")


class PartialProfile(FrozenModel):
    """Only the historically verified M6b .a software phase is supported."""

    model_config = ConfigDict(frozen=True, extra="forbid")
    scope: tuple[Text, ...]
    receipt: Literal["docs/evidence/rebuild/gates/M6b.json"]

    @model_validator(mode="after")
    def exact_scope(self) -> "PartialProfile":
        if len(self.scope) != 2 or set(self.scope) != set(PARTS):
            raise PydanticCustomError(
                "gate_scope", "scope must be exactly R36.a and R37.a"
            )
        return self


class ReceiptBase(FrozenModel):
    gate: Text
    requirements: tuple[Text, ...]


class WholeReceipt(ReceiptBase):
    evidence: dict[StrictStr, StrictStr] | Text
    status: Literal["complete"] = "complete"
    verified_scope: tuple[Text, ...] | None = None
    blocked_by: tuple[Text, ...] = ()


class PartialReceipt(ReceiptBase):
    evidence: dict[StrictStr, StrictStr]
    status: Literal["complete"]
    verified_scope: tuple[Text, ...]
    candidate: Sha
    verified_source: Sha
    web_commit: Sha
    verification: Text
    reviews: tuple[Text, Text]
    blocked_by: tuple[Text, ...]


class VerificationResult(FrozenModel):
    name: Text
    code: StrictInt
    artifact: Text


class Verification(FrozenModel):
    source: Sha
    web: Sha
    results: Annotated[tuple[VerificationResult, ...], Field(min_length=1)]


class GateSuccess(FrozenModel):
    gate: str
    requirements: tuple[str, ...]
    scope: tuple[str, ...]
    evidence: dict[str, str]
    receipt: str | None = None
    candidate: str | None = None
    verified_source: str | None = None
    web_commit: str | None = None
    reviews: tuple[str, ...] = ()


class GateFailure(FrozenModel):
    errors: tuple[str, ...]


@dataclass(frozen=True, slots=True)
class EvidenceError(Exception):
    path: str
    reason: str

    def __str__(self) -> str:
        return f"{self.path}: {self.reason}"


def _file(root: Path, relative: str, base: Path | None = None) -> Path:
    """Resolve a nonempty regular evidence file confined to the repository."""
    part = Path(relative)
    if part.is_absolute() or ".." in part.parts:
        raise EvidenceError(relative, "evidence path must be repository-relative")
    path = ((base or root) / part).resolve()
    if not path.is_relative_to(root.resolve()):
        raise EvidenceError(relative, "evidence path escapes repository")
    if not path.is_file() or path.stat().st_size == 0:
        raise EvidenceError(relative, "missing or empty evidence file")
    return path


Model = TypeVar("Model", bound=BaseModel)


def _read(path: Path, model: type[Model]) -> Model:
    return model.model_validate_json(path.read_bytes())


def _review(root: Path, relative: str, candidate: str) -> Path:
    path = _file(root, relative)
    lines = [
        line.replace("**", "").replace("`", "").strip()
        for line in path.read_text(encoding="utf-8").splitlines()
    ]
    verdicts = [
        match.group(1).upper() for line in lines if (match := VERDICT.match(line))
    ]
    identities = [match.group(1) for line in lines if (match := IDENTITY.match(line))]
    if not verdicts or any(verdict != "APPROVE" for verdict in verdicts):
        raise EvidenceError(relative, "explicit approval verdict missing or rejecting")
    if not identities or any(identity != candidate for identity in identities):
        raise EvidenceError(relative, "review candidate does not match receipt")
    return path


def evaluate_gate(
    root: Path,
    gate: str,
    parents: tuple[str, ...],
    statuses: Mapping[str, str],
    evidence: Mapping[str, str],
    profile: PartialProfile | None = None,
) -> GateSuccess | GateFailure:
    """Evaluate recorded evidence without creating or modifying any files."""
    try:
        if profile is not None:
            return _partial(root, gate, parents, evidence, profile)
        blocked = tuple(
            f"{rid}: software_status={statuses.get(rid, 'missing')}"
            for rid in parents
            if statuses.get(rid) != "passed"
        )
        if blocked:
            return GateFailure(errors=blocked)
        relative = f"docs/evidence/rebuild/gates/{gate}.json"
        path = root / relative
        if path.exists() or path.is_symlink():
            record = _read(_file(root, relative), WholeReceipt)
            if record.gate != gate or record.requirements != parents:
                raise EvidenceError(relative, "gate or parent requirements mismatch")
            if record.blocked_by or (
                record.verified_scope is not None and record.verified_scope != parents
            ):
                raise EvidenceError(
                    relative, "partial or blocked receipt cannot certify whole gate"
                )
            if isinstance(record.evidence, str):
                _file(root, record.evidence)
            elif record.evidence != {rid: evidence[rid] for rid in parents}:
                raise EvidenceError(relative, "requirement evidence mismatch")
        return GateSuccess(
            gate=gate,
            requirements=parents,
            scope=parents,
            evidence={rid: evidence[rid] for rid in parents},
        )
    except (EvidenceError, OSError, UnicodeError, ValidationError, KeyError) as error:
        return GateFailure(errors=(str(error),))


def _partial(
    root: Path,
    gate: str,
    parents: tuple[str, ...],
    evidence: Mapping[str, str],
    profile: PartialProfile,
) -> GateSuccess:
    if gate != "M6b" or len(parents) != 2 or set(parents) != {"R36", "R37"}:
        raise EvidenceError(
            profile.receipt, "partial profile only applies to M6b R36/R37"
        )
    record = _read(_file(root, profile.receipt), PartialReceipt)
    if record.gate != gate or record.requirements != parents or record.blocked_by:
        raise EvidenceError(profile.receipt, "gate, parents or blocked status mismatch")
    if len(record.verified_scope) != 2 or set(record.verified_scope) != set(
        profile.scope
    ):
        raise EvidenceError(profile.receipt, "verified scope does not match profile")
    if any(record.evidence.get(rid) != evidence.get(rid) for rid in parents):
        raise EvidenceError(profile.receipt, "requirement evidence mismatch")
    verification_path = _file(root, record.verification)
    verification = _read(verification_path, Verification)
    if (
        verification.source != record.verified_source
        or verification.web != record.web_commit
    ):
        raise EvidenceError(
            record.verification, "verification source or web identity mismatch"
        )
    for result in verification.results:
        if result.code != 0:
            raise EvidenceError(
                record.verification, f"verification failed: {result.name}"
            )
        _file(root, result.artifact, verification_path.parent)
    reviews = tuple(
        _review(root, review, record.candidate) for review in record.reviews
    )
    if reviews[0].samefile(reviews[1]):
        raise EvidenceError(profile.receipt, "two distinct review files are required")
    return GateSuccess(
        gate=gate,
        requirements=parents,
        scope=profile.scope,
        evidence={rid: evidence[rid] for rid in parents},
        receipt=profile.receipt,
        candidate=record.candidate,
        verified_source=record.verified_source,
        web_commit=record.web_commit,
        reviews=record.reviews,
    )
