from pathlib import Path
from collections.abc import Mapping
from typing import Final

import pytest
import yaml
from openapi_schema_validator import OAS30Validator, OAS30WriteValidator

from check_contracts import fixture, resolve


ROOT: Final = Path(__file__).resolve().parents[1]


def video_schema(name: str) -> OAS30Validator:
    raw = yaml.safe_load((ROOT / "docs/api/video-openapi.yaml").read_text())
    validator = OAS30WriteValidator if name.endswith("Request") else OAS30Validator
    return validator(resolve(raw, raw)["components"]["schemas"][name])


def test_source_variants_accept_rtsp_and_gb() -> None:
    validator = video_schema("CameraRequest")
    assert validator.is_valid({
        "name": "Camera", "pond_id": 1,
        "source": {"kind": "rtsp", "uri": "rtsp://192.168.10.20:554/live"},
    })
    assert validator.is_valid({
        "name": "Camera", "pond_id": 1,
        "source": {"kind": "gb28181", "device_id": 2, "channel_id": "34020000001320000001"},
    })


@pytest.mark.parametrize("source", [
    {"kind": "rtsp"},
    {"kind": "rtsp", "uri": "http://192.168.10.20/live"},
    {"kind": "rtsp", "uri": "rtsp://user:password@192.168.10.20/live"},
    {"kind": "rtsp", "uri": "rtsp://192.168.10.20/live?token=secret"},
    {"kind": "rtsp", "uri": "rtsp://192.168.10.20/live#fragment"},
    {"kind": "rtsp", "uri": "rtsp://192.168.10.20/live", "device_id": 2},
    {"kind": "rtsp", "uri": "rtsp://192.168.10.20/live", "credentials": {"username": "user"}},
    {"kind": "gb28181", "device_id": 2},
    {"kind": "gb28181", "device_id": 2, "channel_id": "invalid"},
    {"kind": "gb28181", "device_id": 0, "channel_id": "34020000001320000001"},
    {"kind": "gb28181", "device_id": 2, "channel_id": "34020000001320000001", "uri": "rtsp://host/live"},
    {"kind": "future"},
])
def test_invalid_source_variants_rejected(source: Mapping[str, str | int | Mapping[str, str]]) -> None:
    assert not video_schema("CameraRequest").is_valid({"name": "Camera", "pond_id": 1, "source": source})


def test_camera_request_rejects_client_tenant_and_farm() -> None:
    request = {"name": "Camera", "pond_id": 1, "source": {"kind": "rtsp", "uri": "rtsp://host/live"}}
    for field in ("tenant_id", "farm_id"):
        assert not video_schema("CameraRequest").is_valid({**request, field: 2})


def test_camera_response_cannot_contain_source_credentials() -> None:
    camera = {"id": 1, "pond_id": 1, "name": "Camera", "source_kind": "rtsp", "source_version": 1, "status": "offline"}
    validator = video_schema("Camera")
    assert validator.is_valid(camera)
    for field in ("uri", "username", "password", "credential_cipher", "source", "peer_address"):
        assert not validator.is_valid({**camera, field: "secret"})


def test_grant_requires_https_and_poll_does_not_return_token() -> None:
    session = {"id": "8e64b082-fb25-4f17-9f01-ae29b98033e1", "camera_id": 1, "state": "pending", "expires_at": "2026-10-09T00:05:00Z"}
    validator = video_schema("PlaybackGrant")
    assert validator.is_valid({"session": session, "manifest_url": "https://example.test/media/v1/token/session/index.m3u8"})
    assert not validator.is_valid({"session": session, "manifest_url": "http://example.test/media"})
    assert not video_schema("PlaybackSession").is_valid({**session, "token": "secret"})


def test_mini_surface_cannot_provision_and_media_needs_path_token() -> None:
    raw = yaml.safe_load((ROOT / "docs/api/video-openapi.yaml").read_text())
    for path, methods in raw["paths"].items():
        if path.startswith("/api/v1/"):
            assert "put" not in methods
            assert "/gb-devices" not in path
            if path.endswith("/cameras"):
                assert "post" not in methods
            for operation in methods.values():
                assert operation["security"] == [{"MiniBearer": []}]
        if path.startswith("/media/"):
            assert "{token}" in path and "{session_id}" in path
            assert methods["get"]["security"] == []
            assert "425" in methods["get"]["responses"]
            assert "503" in methods["get"]["responses"]
        assert not path.startswith("/admin/")


def test_fixture_supports_oneof_and_examples_without_weakening_validation() -> None:
    schema = {
        "oneOf": [
            {"type": "string", "pattern": "^[0-9]{20}$", "example": "34020000001320000001"},
            {"type": "integer", "minimum": 1},
        ]
    }
    sample = fixture(schema)
    OAS30Validator(schema).validate(sample)
    assert sample == "34020000001320000001"
    assert not OAS30Validator(schema).is_valid("bad")
