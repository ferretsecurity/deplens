import json
from pathlib import Path
from unittest.mock import patch

import pytest

from audit_harvest.storage import ArtifactStore
from audit_harvest.subprocess_utils import ToolResult, ToolError, ToolNotFound
from audit_harvest.producers.repo_profile import (
    produce_repo_profile,
    _parse_purl,
    _detect_build_system,
    _detect_monorepo,
    _detect_entry_binaries,
)


MOCK_ENRY_OUTPUT = json.dumps({
    "Go": ["main.go"],
    "Markdown": ["README.md"],
})

MOCK_SCC_OUTPUT = json.dumps([
    {"Name": "Go", "Lines": 35, "Code": 28, "Comments": 0, "Blanks": 7, "Files": 1, "Complexity": 4}
])

MOCK_SBOM = {
    "bomFormat": "CycloneDX",
    "specVersion": "1.5",
    "components": [
        {
            "type": "library",
            "name": "gin",
            "version": "v1.9.1",
            "purl": "pkg:golang/github.com/gin-gonic/gin@v1.9.1",
            "scope": "required",
        }
    ],
    "services": [],
}


def _fake_result(stdout: str) -> ToolResult:
    return ToolResult(stdout=stdout, stderr="", returncode=0, wall_time_sec=0.01, cmd=[])


def test_parse_purl_golang():
    purl_type, key = _parse_purl("pkg:golang/github.com/gin-gonic/gin@v1.9.1")
    assert purl_type == "golang"
    assert key == "golang/github.com/gin-gonic/gin"


def test_parse_purl_npm_scoped():
    purl_type, key = _parse_purl("pkg:npm/%40nestjs/core@10.0.0")
    assert purl_type == "npm"
    assert "@nestjs/core" in key


def test_parse_purl_pypi():
    purl_type, key = _parse_purl("pkg:pypi/flask@3.0.0")
    assert purl_type == "pypi"
    assert key == "pypi/flask"


def test_parse_purl_maven():
    purl_type, key = _parse_purl("pkg:maven/org.springframework.boot/spring-boot-starter-web@3.2.0")
    assert purl_type == "maven"
    assert key == "maven/org.springframework.boot/spring-boot-starter-web"


def test_produce_repo_profile_requires_sbom(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()
    sbom_path = repo / "nonexistent_sbom.json"

    with pytest.raises(ValueError, match="A5 SBOM not found"):
        produce_repo_profile(repo, store, sbom_path=sbom_path)


def _make_run_tool_side_effect():
    """Returns enry/scc results for the first two calls, ToolNotFound for rg calls."""
    responses = [_fake_result(MOCK_ENRY_OUTPUT), _fake_result(MOCK_SCC_OUTPUT)]
    call_count = 0

    def side_effect(cmd, **kwargs):
        nonlocal call_count
        call_count += 1
        if call_count <= 2:
            return responses[call_count - 1]
        raise ToolNotFound(f"rg not found (call {call_count})")

    return side_effect


def test_produce_repo_profile_detects_gin_framework(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()
    (repo / "go.mod").write_text("module example\n\ngo 1.21\n")

    # Write SBOM to a file
    sbom_path = tmp_path / "sbom.cdx.json"
    sbom_path.write_text(json.dumps(MOCK_SBOM))

    with patch("audit_harvest.producers.repo_profile.run_tool", side_effect=_make_run_tool_side_effect()):
        record = produce_repo_profile(repo, store, sbom_path=sbom_path)

    content = Path(record.path).read_text()
    assert "Gin" in content
    assert "Go" in content


def test_produce_repo_profile_lists_manifests(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()
    (repo / "go.mod").write_text("module example\n\ngo 1.21\n")
    (repo / "go.sum").write_text("")

    sbom_path = tmp_path / "sbom.cdx.json"
    sbom_path.write_text(json.dumps({"bomFormat": "CycloneDX", "components": [], "services": []}))

    with patch("audit_harvest.producers.repo_profile.run_tool", side_effect=_make_run_tool_side_effect()):
        record = produce_repo_profile(repo, store, sbom_path=sbom_path)

    content = Path(record.path).read_text()
    assert "go.mod" in content


def test_detect_build_system_makefile(tmp_path):
    makefile = tmp_path / "Makefile"
    makefile.write_text(".PHONY: build test\nbuild:\n\tgo build ./...\ntest:\n\tgo test ./...\n")
    results = _detect_build_system(tmp_path)
    assert len(results) == 1
    entry = results[0]
    assert entry["tool"] == "make"
    assert "build" in entry["targets"]
    assert "test" in entry["targets"]


def test_detect_monorepo_finds_nested_manifest(tmp_path):
    services_api = tmp_path / "services" / "api"
    services_api.mkdir(parents=True)
    (services_api / "go.mod").write_text("module services/api\n\ngo 1.21\n")
    results = _detect_monorepo(tmp_path)
    paths = [r["path"] for r in results]
    assert any("go.mod" in p for p in paths)
    types = [r["type"] for r in results]
    assert "go.mod" in types


def test_detect_entry_binaries_go_cmd(tmp_path):
    cmd_server = tmp_path / "cmd" / "server"
    cmd_server.mkdir(parents=True)
    (cmd_server / "main.go").write_text("package main\nfunc main() {}\n")
    results = _detect_entry_binaries(tmp_path)
    files = [r["file"] for r in results]
    assert any("cmd/server/main.go" in f for f in files)
    langs = [r["language"] for r in results]
    assert "Go" in langs


def test_detect_entry_binaries_rg_no_matches(tmp_path):
    # Create a Go entry so we can verify it is still found via glob (no rg needed)
    cmd_server = tmp_path / "cmd" / "server"
    cmd_server.mkdir(parents=True)
    (cmd_server / "main.go").write_text("package main\nfunc main() {}\n")

    no_match_result = ToolResult(cmd=[], returncode=1, stdout="", stderr="", wall_time_sec=0.0)

    with patch("audit_harvest.producers.repo_profile.run_tool", side_effect=ToolError("no matches", no_match_result)):
        results = _detect_entry_binaries(tmp_path)

    assert isinstance(results, list)
    files = [r["file"] for r in results]
    assert any("cmd/server/main.go" in f for f in files)
