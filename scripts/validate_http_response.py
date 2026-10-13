#!/usr/bin/env python3
# /// script
# requires-python = ">=3.14"
# dependencies = ["pydantic==2.13.4", "PyYAML==6.0.3", "openapi-schema-validator==0.9.0", "openapi-spec-validator==0.9.0"]
# ///
# How to run: make docs-tools; .venv/contracts/bin/python scripts/validate_http_response.py < response.json
"""Validate an in-memory HTTP response without exposing response data in diagnostics."""

import math
import sys
from pathlib import Path
from typing import ClassVar, Final, Literal, Self, assert_never

import yaml
from check_contracts import resolve
from jsonschema.exceptions import SchemaError
from openapi_schema_validator import OAS30ReadValidator, OAS30Validator
from pydantic import (
    BaseModel,
    ConfigDict,
    Field,
    JsonValue,
    StrictBool,
    StrictInt,
    TypeAdapter,
    ValidationError,
    model_validator,
)
from pydantic_core import PydanticCustomError

ROOT: Final = Path(__file__).resolve().parents[1]
JSON_MAP: Final = TypeAdapter(dict[str, JsonValue])
SCHEMA_PATH: Final = TypeAdapter(tuple[str | int, ...])
KEYWORD: Final = TypeAdapter(str)


def _check_finite_json(value: JsonValue) -> None:
    match value:
        case float() as number:
            if not math.isfinite(number):
                raise PydanticCustomError("finite_number", "JSON number must be finite")
        case list() as items:
            for item in items:
                _check_finite_json(item)
        case dict() as fields:
            for item in fields.values():
                _check_finite_json(item)
        case str() | int() | None:
            return
        case unreachable:
            assert_never(unreachable)


class ResponseInput(BaseModel):
    """Only stdin carries the real response, including potentially private values."""

    model_config: ClassVar[ConfigDict] = ConfigDict(
        frozen=True, extra="forbid", strict=True, allow_inf_nan=False
    )
    spec: Literal["admin", "app"]
    method: Literal["GET", "POST", "PUT", "DELETE"]
    route: str
    status: StrictInt = Field(ge=100, le=599)
    content_type: str
    has_body: StrictBool
    body: JsonValue

    @model_validator(mode="after")
    def check_body_presence(self) -> Self:
        _check_finite_json(self.body)
        if not self.has_body and self.body is not None:
            raise PydanticCustomError("body_presence", "body presence is inconsistent")
        return self


class Media(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True)
    json_schema: dict[str, JsonValue] = Field(alias="schema")


class Response(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True)
    content: dict[str, Media] = Field(default_factory=dict)


class Operation(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True)
    responses: dict[str, Response]


class Document(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True)
    paths: dict[str, JsonValue]


class Issue(BaseModel):
    """Schema locations only; instance paths and validator messages are forbidden."""

    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True)
    keyword: str
    schema_path: tuple[str | int, ...] = ()


class Result(BaseModel):
    model_config: ClassVar[ConfigDict] = ConfigDict(frozen=True)
    valid: bool
    issues: tuple[Issue, ...] = ()


def failure(keyword: str) -> Result:
    return Result(valid=False, issues=(Issue(keyword=keyword),))


def validate_response(value: ResponseInput) -> Result:
    """Resolve the fixed local contract and evaluate the selected status response."""
    match value.spec:
        case "admin":
            filename = "admin-openapi.yaml"
        case "app":
            filename = "openapi.yaml"
        case unreachable:
            assert_never(unreachable)
    try:
        with (ROOT / "docs/api" / filename).open(encoding="utf-8") as source:
            raw_document = JSON_MAP.validate_python(yaml.safe_load(source))
        document = Document.model_validate(resolve(raw_document, raw_document))
        methods = JSON_MAP.validate_python(document.paths[value.route])
        operation = Operation.model_validate(methods[value.method.lower()])
        response = operation.responses[str(value.status)]
    except KeyError:
        return failure("response_selection")
    except (
        OSError,
        UnicodeError,
        yaml.YAMLError,
        ValidationError,
        AssertionError,
        RecursionError,
    ):
        return failure("contract")
    if not response.content:
        return Result(valid=True) if not value.has_body else failure("unexpected_body")
    if not value.has_body:
        return failure("missing_body")
    if value.content_type.split(";", 1)[0].strip().lower() != "application/json":
        return failure("content_type")
    try:
        schema = response.content["application/json"].json_schema
        validator = OAS30ReadValidator(
            schema, format_checker=OAS30Validator.FORMAT_CHECKER
        )
        issues = tuple(
            Issue(
                keyword=KEYWORD.validate_python(error.validator, strict=True),
                schema_path=SCHEMA_PATH.validate_python(
                    tuple(error.absolute_schema_path)
                ),
            )
            for error in validator.iter_errors(value.body)
        )
    except (KeyError, SchemaError, ValidationError):
        return failure("contract")
    return Result(valid=not issues, issues=issues)


def main() -> int:
    """Print safe JSON for both successful validation and malformed stdin."""
    try:
        value = ResponseInput.model_validate_json(sys.stdin.buffer.read())
    except (ValidationError, OSError):
        result = failure("input")
    else:
        result = validate_response(value)
    print(result.model_dump_json())
    return 0 if result.valid else 1


if __name__ == "__main__":
    raise SystemExit(main())
