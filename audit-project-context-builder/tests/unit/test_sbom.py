import json
from pathlib import Path
from unittest.mock import patch

import pytest

from audit_harvest.storage import ArtifactStore
from audit_harvest.subprocess_utils import ToolResult
from audit_harvest.producers.sbom import produce_sbom
from audit_harvest.producers.cve_overlay import produce_cve_overlay


def _fake_result(stdout: str = "", returncode: int = 0) -> ToolResult:
    return ToolResult(
        stdout=stdout, stderr="", returncode=returncode,
        wall_time_sec=0.01, cmd=[]
    )


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

MOCK_OSV_OUTPUT = {
    "results": [
        {
            "packages": [
                {
                    "package": {"name": "gin", "version": "v1.9.1", "ecosystem": "Go"},
                    "vulnerabilities": [
                        {
                            "id": "GHSA-xxxx-yyyy-zzzz",
                            "aliases": ["CVE-2024-0001"],
                            "database_specific": {"severity": "HIGH"},
                        }
                    ],
                }
            ]
        }
    ]
}


def test_produce_sbom_writes_artifact(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()

    with patch("audit_harvest.producers.sbom.run_tool") as mock_run:
        # cdxgen writes to the output path; we simulate by writing the file
        def fake_cdxgen(cmd, cwd, timeout_sec, **kwargs):
            out_path = Path(cmd[cmd.index("--output") + 1])
            out_path.parent.mkdir(parents=True, exist_ok=True)
            out_path.write_text(json.dumps(MOCK_SBOM))
            return _fake_result()

        mock_run.side_effect = fake_cdxgen
        record = produce_sbom(repo, store)

    assert record.name == "sbom"
    content = json.loads(Path(record.path).read_bytes())
    assert content["bomFormat"] == "CycloneDX"
    assert len(content["components"]) == 1


def test_produce_sbom_skips_when_fresh(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()

    # Write a pre-existing artifact so is_fresh returns True
    store.write("sbom", json.dumps(MOCK_SBOM).encode(), source_hash="any")

    with patch("audit_harvest.producers.sbom.run_tool") as mock_run:
        with patch("audit_harvest.producers.sbom._source_hash", return_value="any"):
            record = produce_sbom(repo, store)

    mock_run.assert_not_called()
    assert record.name == "sbom"


def test_produce_cve_overlay_parses_osv_output(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()

    # Pre-write an SBOM for the overlay to consume
    sbom_record = store.write("sbom", json.dumps(MOCK_SBOM).encode(), source_hash="h1")

    with patch("audit_harvest.producers.cve_overlay.run_tool") as mock_run:
        mock_run.return_value = _fake_result(stdout=json.dumps(MOCK_OSV_OUTPUT))
        record = produce_cve_overlay(repo, store, sbom_path=Path(sbom_record.path))

    assert record.name == "cve_overlay"
    content = json.loads(Path(record.path).read_bytes())
    assert content["meta"]["artifact_id"] == "cve_overlay"
    assert len(content["findings"]) == 1
    finding = content["findings"][0]
    assert finding["package_name"] == "gin"
    assert finding["vulnerability_id"] == "GHSA-xxxx-yyyy-zzzz"
    assert finding["severity"] == "HIGH"


def test_produce_cve_overlay_empty_when_no_vulns(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()

    sbom_record = store.write("sbom", json.dumps(MOCK_SBOM).encode(), source_hash="h1")

    with patch("audit_harvest.producers.cve_overlay.run_tool") as mock_run:
        mock_run.return_value = _fake_result(stdout=json.dumps({"results": []}))
        record = produce_cve_overlay(repo, store, sbom_path=Path(sbom_record.path))

    content = json.loads(Path(record.path).read_bytes())
    assert content["findings"] == []
