import json
from pathlib import Path
from unittest.mock import patch

import pytest

from audit_harvest.storage import ArtifactStore
from audit_harvest.producers.entry_points import produce_entry_points
from audit_harvest.extractors.frameworks.go import extract_go_routes
from audit_harvest.extractors.frameworks.python import extract_python_routes
from audit_harvest.extractors.frameworks.javascript import extract_js_routes
from audit_harvest.extractors.frameworks.java import extract_java_routes

_FIXTURES = Path(__file__).parent.parent / "fixtures"
GO_FIXTURE = _FIXTURES / "go_simple"
PYTHON_FIXTURE = _FIXTURES / "python_flask"
JS_FIXTURE = _FIXTURES / "js_express"
JAVA_FIXTURE = _FIXTURES / "java_spring"


def test_extract_go_routes_gin(tmp_path):
    routes = extract_go_routes(GO_FIXTURE)
    assert len(routes) >= 3
    paths = [r["path"] for r in routes]
    assert "/users" in paths
    assert "/users/:id" in paths
    methods = [r["method"] for r in routes]
    assert "GET" in methods
    assert "POST" in methods


def test_extract_python_routes_flask(tmp_path):
    routes = extract_python_routes(PYTHON_FIXTURE)
    assert len(routes) >= 3
    paths = [r["path"] for r in routes]
    assert "/users" in paths
    assert "/users/<int:user_id>" in paths


def test_extract_js_routes_express(tmp_path):
    routes = extract_js_routes(JS_FIXTURE)
    assert len(routes) >= 3
    paths = [r["path"] for r in routes]
    assert "/users" in paths
    assert "/users/:id" in paths


def test_extract_java_routes_spring(tmp_path):
    routes = extract_java_routes(JAVA_FIXTURE)
    assert len(routes) >= 1
    # UserController: @GetMapping("/{id}") under @RequestMapping("/users") → GET /users/{id}
    assert any("/users" in r["path"] for r in routes)
    assert any("GET" == r["method"] and "/users" in r["path"] for r in routes)
    assert any("/{id}" in r["path"] or "{id}" in r["path"] for r in routes)
    assert all(r["handler"] for r in routes)


def test_produce_entry_points_writes_artifact(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    fixture = GO_FIXTURE

    record = produce_entry_points(fixture, store)

    assert record.name == "entry_points"
    content = json.loads(Path(record.path).read_bytes())
    assert "meta" in content
    assert "entry_points" in content
    assert len(content["entry_points"]) >= 3


def test_produce_entry_points_all_have_required_fields(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    fixture = GO_FIXTURE

    record = produce_entry_points(fixture, store)
    content = json.loads(Path(record.path).read_bytes())

    for ep in content["entry_points"]:
        assert "kind" in ep
        assert "method" in ep
        assert "path" in ep
        assert "handler" in ep
        assert "file" in ep
        assert ep["kind"] in ("http", "cli", "grpc", "worker", "cron")


def test_produce_entry_points_empty_repo(tmp_path):
    """A repo with no source files produces an artifact with zero entry points."""
    store = ArtifactStore(tmp_path / ".audit")
    record = produce_entry_points(tmp_path, store)
    data = json.loads(Path(record.path).read_text())
    assert data["entry_points"] == []


def test_go_routes_variable_name_agnostic(tmp_path):
    """Tree-sitter extractor must find routes regardless of router variable name."""
    (tmp_path / "main.go").write_text(
        'package main\n'
        'import "github.com/gorilla/mux"\n'
        'func main() {\n'
        '    myrouter := mux.NewRouter()\n'
        '    myrouter.HandleFunc("/api/items", ListItems)\n'
        '    myrouter.HandleFunc("/api/orders", ListOrders)\n'
        '}\n'
    )
    routes = extract_go_routes(tmp_path)
    paths = [r["path"] for r in routes]
    assert "/api/items" in paths
    assert "/api/orders" in paths
