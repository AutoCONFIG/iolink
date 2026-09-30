from pathlib import Path

import yaml
from openapi_schema_validator import OAS30Validator

from check_contracts import resolve


ROOT = Path(__file__).resolve().parents[1]


def _doc(name):
    path = ROOT / "docs" / "api" / name
    raw = yaml.safe_load(path.read_text())
    return raw, resolve(raw, raw)


def _validator(doc, name):
    return OAS30Validator(doc["components"]["schemas"][name])


def test_m6a_v2_routes_are_explicit_and_keep_v1_paths():
    app, resolved = _doc("openapi.yaml")
    assert "/water/latest" in app["paths"]
    assert "/devices/{device_no}/telemetry" in app["paths"]
    assert "/devices/{device_no}/history" in app["paths"]
    for route in ("/devices/{device_no}/telemetry", "/devices/{device_no}/history"):
        operation = resolved["paths"][route]["post" if route.endswith("telemetry") else "get"]
        assert operation["servers"] == [{"url": "http://localhost:8080/api/v2"}]


def test_model_field_fixture_accepts_numeric_and_enum_fields_and_rejects_invalid_shape():
    _, doc = _doc("admin-openapi.yaml")
    validator = _validator(doc, "ModelField")
    numeric = {
        "identifier": "temperature",
        "type": "number",
        "unit": "C",
        "minimum": -20,
        "maximum": 80,
        "readable": True,
        "writable": False,
        "nullable": False,
    }
    enum = {
        "identifier": "mode",
        "type": "string",
        "unit": "",
        "enum_values": ["auto", "manual"],
        "readable": True,
        "writable": True,
        "nullable": False,
    }
    assert validator.is_valid(numeric)
    assert validator.is_valid(enum)
    assert not validator.is_valid({**numeric, "type": "float"})
    assert not validator.is_valid({key: value for key, value in numeric.items() if key != "identifier"})


def test_immutable_model_and_assignment_fixtures_require_version_and_fields():
    _, doc = _doc("admin-openapi.yaml")
    model = {
        "fields": [
            {
                "identifier": "temperature",
                "type": "number",
                "unit": "C",
                "readable": True,
                "writable": False,
                "nullable": False,
            }
        ]
    }
    assignment = {"product_id": 7, "model_version": 2}
    assert _validator(doc, "ProductModelCreateReq").is_valid(model)
    assert _validator(doc, "DeviceProductAssignmentReq").is_valid(assignment)
    assert not _validator(doc, "DeviceProductAssignmentReq").is_valid({"product_id": 7})


def test_telemetry_fixture_rejects_empty_properties():
    _, doc = _doc("openapi.yaml")
    validator = _validator(doc, "TelemetryV2Request")
    assert validator.is_valid({"ts": "2026-09-28T09:00:00Z", "properties": {"mode": "auto"}})
    assert not validator.is_valid({"ts": "2026-09-28T09:00:00Z", "properties": {}})
