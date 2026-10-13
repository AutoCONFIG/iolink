# /// script
# requires-python = ">=3.14"
# dependencies = ["pytest==9.1.1", "pydantic==2.13.4", "PyYAML==6.0.3", "openapi-schema-validator==0.9.0", "openapi-spec-validator==0.9.0"]
# ///
# How to run: .venv/contracts/bin/python -m pytest -q scripts/test_validate_http_response.py
"""Exercise fixed-contract validation and the real stdin CLI privacy boundary."""

import json
import subprocess
import sys
from pathlib import Path
from typing import Final

import pytest
from pydantic import JsonValue
from validate_http_response import ResponseInput, Result, validate_response

SCRIPT: Final = Path(__file__).with_name("validate_http_response.py")
SECRET: Final = "private-provider-password-JWT-xyz"


@pytest.mark.parametrize(
    ("spec", "route", "status", "body"),
    [
        ("license", "/license", 401, {"error": "unauthorized"}),
        ("open", "/open/v1/ponds", 401, {"error": "unauthorized"}),
        ("setup", "/status", 503, {"error": "unavailable"}),
        (
            "video",
            "/user/v1/cameras",
            401,
            {"code": "unauthorized", "message": "unauthorized"},
        ),
    ],
)
def test_fixed_additional_contracts_validate_real_error_shapes(
    spec: str, route: str, status: int, body: dict[str, JsonValue]
) -> None:
    payload = {
        "spec": spec,
        "method": "GET",
        "route": route,
        "status": status,
        "content_type": "application/json",
        "has_body": True,
        "body": body,
    }
    result = validate_response(ResponseInput.model_validate(payload))
    assert result.valid
    assert result.issues == ()


def error_response() -> ResponseInput:
    return ResponseInput(
        spec="app",
        method="GET",
        route="/water/latest",
        status=500,
        content_type="application/json; charset=utf-8",
        has_body=True,
        body={"error": "internal_error"},
    )


@pytest.mark.parametrize(
    "body", [1.5, [1.5], {"nested": [True, None, "Infinity", 1.5]}]
)
def test_input_accepts_finite_json_without_rejecting_strings(body: JsonValue) -> None:
    # Given
    payload = json.dumps({**error_response().model_dump(), "body": body})
    # When
    value = ResponseInput.model_validate_json(payload)
    # Then
    assert value.body == body


@pytest.mark.parametrize("spec", ["admin", "app"])
def test_valid_internal_error_when_contract_response_exists(spec: str) -> None:
    # Given
    value = error_response().model_copy(update={"spec": spec, "route": "/ponds"})
    # When
    result = validate_response(value)
    # Then
    assert result.valid
    assert result.issues == ()


@pytest.mark.parametrize(
    ("body", "keyword"),
    [
        ({}, "required"),
        ({"error": "internal_error", "detail": SECRET}, "additionalProperties"),
        ({"error": 123}, "type"),
        ({"error": {SECRET: SECRET}}, "type"),
    ],
)
def test_error_response_rejected_when_schema_violated(
    body: dict[str, JsonValue], keyword: str
) -> None:
    # Given
    value = error_response().model_copy(update={"body": body})
    # When
    result = validate_response(value)
    # Then
    assert not result.valid
    assert keyword in {issue.keyword for issue in result.issues}
    assert SECRET not in result.model_dump_json()


@pytest.mark.parametrize("has_body", [True, False])
def test_latest_null_requires_present_json_body(has_body: bool) -> None:
    # Given
    value = error_response().model_copy(
        update={"status": 200, "body": None, "has_body": has_body}
    )
    # When
    result = validate_response(value)
    # Then
    assert result.valid is has_body


def test_latest_rejected_when_timestamp_format_invalid() -> None:
    # Given
    metrics = (
        "temperature",
        "dissolved_oxygen",
        "ph",
        "turbidity",
        "salinity",
        "signal",
    )
    body: dict[str, JsonValue] = {
        "device_no": "test-device",
        "pond_id": 1,
        "ts": SECRET,
        "report_interval": 60,
        "timestamps": {metric: None for metric in metrics},
        **{metric: None for metric in metrics},
    }
    value = error_response().model_copy(update={"status": 200, "body": body})
    # When
    result = validate_response(value)
    # Then
    assert not result.valid
    assert "format" in {issue.keyword for issue in result.issues}
    assert SECRET not in result.model_dump_json()


def test_common_response_rejects_wrong_machine_error_code() -> None:
    # Given
    value = error_response().model_copy(update={"body": {"error": SECRET}})
    # When
    result = validate_response(value)
    # Then
    assert not result.valid
    assert "enum" in {issue.keyword for issue in result.issues}
    assert SECRET not in result.model_dump_json()


@pytest.mark.parametrize("has_body", [True, False])
def test_no_content_response_rejects_even_json_null_body(has_body: bool) -> None:
    # Given
    value = error_response().model_copy(
        update={
            "method": "POST",
            "route": "/alarms/{id}/confirm",
            "status": 204,
            "body": None,
            "has_body": has_body,
            "content_type": "",
        }
    )
    # When
    result = validate_response(value)
    # Then
    assert result.valid is (not has_body)


@pytest.mark.parametrize(
    ("field", "value", "keyword"),
    [
        ("content_type", SECRET, "content_type"),
        ("status", 418, "response_selection"),
        ("route", "/water/latest?secret=" + SECRET, "response_selection"),
        ("method", "DELETE", "response_selection"),
    ],
)
def test_response_rejected_when_selection_or_media_invalid(
    field: str, value: str | int, keyword: str
) -> None:
    # Given
    response = error_response().model_copy(update={field: value})
    # When
    result = validate_response(response)
    # Then
    assert not result.valid
    assert result.issues[0].keyword == keyword
    assert SECRET not in result.model_dump_json()


@pytest.mark.parametrize(
    ("payload", "exit_code", "expected_keyword"),
    [
        (error_response().model_dump_json(), 0, None),
        (
            error_response()
            .model_copy(update={"body": {"error": "internal_error", SECRET: SECRET}})
            .model_dump_json(),
            1,
            "additionalProperties",
        ),
        (
            error_response().model_copy(update={"spec": SECRET}).model_dump_json(),
            1,
            "input",
        ),
        (json.dumps({**error_response().model_dump(), "status": True}), 1, "input"),
        (json.dumps({**error_response().model_dump(), "has_body": "true"}), 1, "input"),
        ('{"body": "' + SECRET, 1, "input"),
        *[
            (
                json.dumps(
                    {
                        **error_response().model_dump(),
                        "spec": "admin",
                        "method": "POST",
                        "route": "/login",
                        "status": 200,
                        "body": {
                            "token": SECRET,
                            "expires_in": 1,
                            "platform_admin": True,
                            "extra": {"nested": [number]},
                        },
                    }
                ),
                1,
                "input",
            )
            for number in (float("nan"), float("inf"), float("-inf"))
        ],
        (
            json.dumps(
                {
                    **error_response().model_dump(),
                    "method": "POST",
                    "route": "/alarms/{id}/confirm",
                    "status": 204,
                    "has_body": False,
                    "body": {SECRET: SECRET},
                }
            ),
            1,
            "input",
        ),
    ],
    ids=[
        "valid-500",
        "extra-private-field",
        "unknown-spec",
        "boolean-status",
        "string-has-body",
        "malformed-json",
        "nested-nan",
        "nested-positive-infinity",
        "nested-negative-infinity",
        "absent-body-with-private-data",
    ],
)
def test_cli_exit_and_diagnostics_when_input_valid_or_private(
    payload: str, exit_code: int, expected_keyword: str | None
) -> None:
    # Given
    invocation = [sys.executable, str(SCRIPT)]
    # When
    completed = subprocess.run(
        invocation,
        input=payload,
        capture_output=True,
        text=True,
        timeout=20,
        check=False,
    )
    # Then
    result = Result.model_validate_json(completed.stdout)
    assert completed.returncode == exit_code
    assert result.valid is (exit_code == 0)
    if expected_keyword is not None:
        assert result.issues[0].keyword == expected_keyword
    assert SECRET not in completed.stdout + completed.stderr
    assert completed.stderr == ""
    assert '"valid":' in completed.stdout
