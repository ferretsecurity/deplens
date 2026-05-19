import json
from pathlib import Path
from unittest.mock import patch

import pytest

from audit_harvest.storage import ArtifactStore
from audit_harvest.subprocess_utils import ToolResult
from audit_harvest.producers.entry_points import produce_entry_points
from audit_harvest.extractors.frameworks.go import extract_go_routes
from audit_harvest.extractors.frameworks.python import extract_python_routes
from audit_harvest.extractors.frameworks.javascript import extract_js_routes
from audit_harvest.extractors.frameworks.java import extract_java_routes


def test_extract_go_routes_gin(tmp_path):
    fixture = Path("tests/fixtures/go_simple")
    routes = extract_go_routes(fixture)
    assert len(routes) >= 3
    paths = [r["path"] for r in routes]
    assert "/users" in paths
    assert "/users/:id" in paths
    methods = [r["method"] for r in routes]
    assert "GET" in methods
    assert "POST" in methods


def test_extract_python_routes_flask(tmp_path):
    fixture = Path("tests/fixtures/python_flask")
    routes = extract_python_routes(fixture)
    assert len(routes) >= 3
    paths = [r["path"] for r in routes]
    assert "/users" in paths
    assert "/users/<int:user_id>" in paths


def test_extract_js_routes_express(tmp_path):
    fixture = Path("tests/fixtures/js_express")
    routes = extract_js_routes(fixture)
    assert len(routes) >= 3
    paths = [r["path"] for r in routes]
    assert "/users" in paths
    assert "/users/:id" in paths


def test_extract_java_routes_spring(tmp_path):
    fixture = Path("tests/fixtures/java_spring")
    routes = extract_java_routes(fixture)
    assert len(routes) >= 1
    # UserController has @RequestMapping("/users")
    assert any("/users" in r["path"] for r in routes)


def test_produce_entry_points_writes_artifact(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    fixture = Path("tests/fixtures/go_simple")

    record = produce_entry_points(fixture, store)

    assert record.name == "entry_points"
    content = json.loads(Path(record.path).read_bytes())
    assert "meta" in content
    assert "entry_points" in content
    assert len(content["entry_points"]) >= 3


def test_produce_entry_points_all_have_required_fields(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    fixture = Path("tests/fixtures/go_simple")

    record = produce_entry_points(fixture, store)
    content = json.loads(Path(record.path).read_bytes())

    for ep in content["entry_points"]:
        assert "kind" in ep
        assert "method" in ep
        assert "path" in ep
        assert "handler" in ep
        assert "file" in ep
        assert ep["kind"] in ("http", "cli", "grpc", "worker", "cron")
