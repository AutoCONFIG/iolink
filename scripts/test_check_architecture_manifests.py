import importlib.util
from pathlib import Path
import sys

import pytest
import yaml


SCRIPT = Path(__file__).with_name("check_architecture_manifests.py")
SPEC = importlib.util.spec_from_file_location("architecture_checker", SCRIPT)
assert SPEC is not None and SPEC.loader is not None
checker = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = checker
SPEC.loader.exec_module(checker)
PROJECT = SCRIPT.parents[1]


@pytest.fixture
def manifests(tmp_path, monkeypatch):
    architecture = tmp_path / "docs/architecture"
    architecture.mkdir(parents=True)
    (tmp_path / "docs/ACCEPTANCE.md").write_text("baseline", encoding="utf-8")
    (tmp_path / "AGENTS.md").write_text(
        "绿地重建 MySQL QQ Telegram WhatsApp", encoding="utf-8"
    )
    evidence = tmp_path / "docs/evidence/rebuild"
    evidence.mkdir(parents=True)
    (evidence / "foundation-guidance.txt").write_text("fixture evidence", encoding="utf-8")
    data = {}
    for name in ("acceptance-map", "provider-matrix", "invariants"):
        source = PROJECT / f"docs/architecture/{name}.yaml"
        data[name] = yaml.safe_load(source.read_text(encoding="utf-8"))
    for item in data["acceptance-map"]["requirements"]:
        if item["software_status"] == "passed" or item["external_status"] == "passed":
            path = tmp_path / item["evidence"]
            path.mkdir(parents=True, exist_ok=True)
            (path / "result.txt").write_text("fixture evidence", encoding="utf-8")
    for item in data["acceptance-map"]["provider_operations"]:
        if item["status"] == "passed":
            path = tmp_path / item["evidence_path"]
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("fixture evidence", encoding="utf-8")
    for item in data["provider-matrix"]["providers"]:
        if item["status"] == "implemented":
            path = tmp_path / item["evidence_path"]
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("fixture evidence", encoding="utf-8")
    for item in data["provider-matrix"]["capabilities"]:
        if item["status"] == "implemented":
            path = tmp_path / item["evidence_path"]
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("fixture evidence", encoding="utf-8")
    monkeypatch.setattr(checker, "ROOT", tmp_path)
    return data


def validate(manifests):
    for name, data in manifests.items():
        (checker.ROOT / f"docs/architecture/{name}.yaml").write_text(
            yaml.safe_dump(data), encoding="utf-8"
        )
    return checker.validate_manifests()


@pytest.mark.parametrize(
    ("change", "expected"),
    [
        (lambda m: m["acceptance-map"]["requirements"][0].update(wave="M8d"), "wave"),
        (lambda m: m["provider-matrix"]["providers"][5].update(status="implemented", evidence_path="docs/evidence/rebuild/missing.txt"), "evidence"),
        (lambda m: m["provider-matrix"]["capabilities"][7].update(status="implemented", evidence_path="docs/evidence/rebuild/missing.txt"), "evidence"),
        (lambda m: m["provider-matrix"].update(capabilities=[]), "capabilities"),
        (lambda m: m["provider-matrix"].update(capabilities=None), "capabilities"),
        (lambda m: m["acceptance-map"]["requirements"][0].update(id="R99"), "R99"),
        (lambda m: m["acceptance-map"]["requirements"][1].update(id="R01"), "duplicate requirement"),
        (lambda m: m["acceptance-map"]["requirements"].pop(), "missing requirement R55"),
        (lambda m: m["provider-matrix"]["capabilities"][0].update(provider="unknown-db"), "unknown provider"),
        (lambda m: m["provider-matrix"]["providers"][0].update(id="unknown-db"), "undeclared provider"),
    ],
)
def test_rejects_invalid_manifest_when_adversarial_change(manifests, change, expected):
    # Given: a baseline with one adversarial change.
    change(manifests)
    # When: the checker validates the complete manifest.
    errors = validate(manifests)
    # Then: the contract violation is named explicitly.
    assert any(expected in error for error in errors), errors


@pytest.mark.parametrize("gate", ["M0", "M6a"])
def test_gate_fails_when_requirements_are_not_run(manifests, monkeypatch, capsys, gate):
    # Given: valid manifests recording not_run rather than accepted outcomes.
    assert validate(manifests) == []
    monkeypatch.setattr(sys, "argv", [str(SCRIPT), "--gate", gate])
    # When: the gate is evaluated through the CLI.
    result = checker.main()
    # Then: neither a successful exit nor JSON evidence is produced.
    output = capsys.readouterr()
    assert result != 0
    assert output.out == ""
    assert "not_run" in output.err


def test_gate_emits_json_only_when_recorded_evidence_passes(manifests, monkeypatch, capsys):
    # Given: both M6a requirements with actual recorded evidence.
    evidence = checker.ROOT / "docs/evidence/acceptance"
    for item in manifests["acceptance-map"]["requirements"]:
        if item["wave"] == "M6a":
            item["software_status"] = "passed"
            path = evidence / item["id"]
            path.mkdir(parents=True)
            (path / "result.txt").write_text("independent acceptance result", encoding="utf-8")
    assert validate(manifests) == []
    monkeypatch.setattr(sys, "argv", [str(SCRIPT), "--gate", "M6a"])
    # When: the valid gate runs.
    result = checker.main()
    # Then: the CLI returns machine-readable evidence.
    output = capsys.readouterr()
    assert result == 0
    assert '"gate": "M6a"' in output.out
    assert '"R34"' in output.out and '"R35"' in output.out
    assert output.err == ""


def test_rejects_duplicate_provider_operation(manifests):
    manifests["acceptance-map"]["provider_operations"].append(
        manifests["acceptance-map"]["provider_operations"][0].copy()
    )
    errors = validate(manifests)
    assert any("duplicate requirement operation R01" in error for error in errors), errors


def test_rejects_contradictory_agents_scope(manifests):
    (checker.ROOT / "AGENTS.md").write_text(
        "绿地重建 MySQL QQ Telegram WhatsApp\n必须保留生产数据", encoding="utf-8"
    )
    errors = validate(manifests)
    assert any("contradictory greenfield/provider rule" in error for error in errors), errors


def test_rejects_passed_operation_without_evidence(manifests):
    manifests["acceptance-map"]["provider_operations"][0].update(
        status="passed", evidence_path="docs/evidence/rebuild/missing.txt"
    )
    errors = validate(manifests)
    assert any("passed provider operation lacks" in error for error in errors), errors


def test_rejects_operation_level_above_provider_metadata(manifests):
    manifests["acceptance-map"]["provider_operations"][0]["required_level"] = "L4"
    errors = validate(manifests)
    assert any("exceeds provider capability metadata" in error for error in errors), errors


def test_rejects_unsupported_provider_wave(manifests):
    manifests["provider-matrix"]["providers"][0]["supports"] = ["M99"]
    errors = validate(manifests)
    assert any("finite release waves" in error for error in errors), errors


def test_rejects_implemented_capability_without_evidence(manifests):
    manifests["provider-matrix"]["capabilities"][0].update(
        status="implemented", evidence_path="docs/evidence/rebuild/missing.txt"
    )
    errors = validate(manifests)
    assert any("implemented capability lacks" in error for error in errors), errors


def test_rejects_missing_invariant_field(manifests):
    manifests["invariants"]["invariants"][0].pop("negative_test")
    errors = validate(manifests)
    assert any("invariants[0]: missing fields negative_test" in error for error in errors), errors


def test_rejects_universal_port(manifests):
    manifests["invariants"]["invariants"][0]["port"] = "Repository"
    errors = validate(manifests)
    assert any("forbidden universal port" in error for error in errors), errors


def test_rejects_undeclared_consumer_port(manifests):
    manifests["invariants"]["invariants"][0]["port"] = "FooPort"
    errors = validate(manifests)
    assert any("not a declared consumer-owned port" in error for error in errors), errors


def test_rejects_missing_requirement_invariant(manifests):
    manifests["invariants"]["invariants"].pop()
    errors = validate(manifests)
    assert any("missing invariant R55" in error for error in errors), errors


def test_rejects_provider_sdk_import_in_boundary(manifests):
    boundary = checker.ROOT / "internal/domain"
    boundary.mkdir(parents=True)
    (boundary / "leak.go").write_text(
        'package domain\nimport "github.com/jackc/pgx/v5"\n', encoding="utf-8"
    )
    errors = validate(manifests)
    assert any("provider SDK import in domain/application boundary" in error for error in errors), errors


@pytest.mark.parametrize(
    ("field", "value", "expected"),
    [
        ("consumer_owned_ports", {"name": "FooPort"}, "consumer_owned_ports must be a nonempty list"),
        ("consumer_owned_ports", "FooPort", "consumer_owned_ports must be a nonempty list"),
        ("provider_import_roots", "internal/domain", "provider_import_roots must be a list"),
        ("provider_import_roots", {"root": "internal/domain"}, "provider_import_roots must be a list"),
    ],
)
def test_rejects_malformed_topology_without_crashing(manifests, field, value, expected):
    manifests["invariants"]["topology"][field] = value
    errors = validate(manifests)
    assert any(expected in error for error in errors), errors
