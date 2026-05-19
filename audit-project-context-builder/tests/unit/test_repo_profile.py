import json
from pathlib import Path
from unittest.mock import patch

import pytest

from audit_harvest.storage import ArtifactStore
from audit_harvest.subprocess_utils import ToolResult
from audit_harvest.producers.repo_profile import produce_repo_profile, _parse_purl


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


def test_produce_repo_profile_detects_gin_framework(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()
    (repo / "go.mod").write_text("module example\n\ngo 1.21\n")

    # Write SBOM to a file
    sbom_path = tmp_path / "sbom.cdx.json"
    sbom_path.write_text(json.dumps(MOCK_SBOM))

    with patch("audit_harvest.producers.repo_profile.run_tool") as mock_run:
        mock_run.side_effect = [
            _fake_result(MOCK_ENRY_OUTPUT),  # enry call
            _fake_result(MOCK_SCC_OUTPUT),   # scc call
        ]
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

    with patch("audit_harvest.producers.repo_profile.run_tool") as mock_run:
        mock_run.side_effect = [
            _fake_result(MOCK_ENRY_OUTPUT),
            _fake_result(MOCK_SCC_OUTPUT),
        ]
        record = produce_repo_profile(repo, store, sbom_path=sbom_path)

    content = Path(record.path).read_text()
    assert "go.mod" in content
