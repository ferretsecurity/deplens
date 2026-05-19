from __future__ import annotations

import json
from pathlib import Path
from unittest.mock import patch

import pytest

from audit_harvest.producers.cve_overlay import produce_cve_overlay, _load_reachable_purls
from audit_harvest.storage import ArtifactStore
from audit_harvest.subprocess_utils import ToolResult


def _fake_result(stdout: str = "", returncode: int = 0) -> ToolResult:
    return ToolResult(
        stdout=stdout, stderr="", returncode=returncode,
        wall_time_sec=0.01, cmd=[]
    )


_OSV_TWO_PACKAGES = {
    "results": [
        {
            "packages": [
                {
                    "package": {
                        "name": "express",
                        "version": "4.17.1",
                        "purl": "pkg:npm/express@4.17.1",
                    },
                    "vulnerabilities": [
                        {
                            "id": "GHSA-aaaa-bbbb-cccc",
                            "aliases": ["CVE-2022-0001"],
                            "database_specific": {"severity": "HIGH"},
                        }
                    ],
                },
                {
                    "package": {
                        "name": "lodash",
                        "version": "4.17.20",
                        "purl": "pkg:npm/lodash@4.17.20",
                    },
                    "vulnerabilities": [
                        {
                            "id": "GHSA-dddd-eeee-ffff",
                            "aliases": ["CVE-2022-0002"],
                            "database_specific": {"severity": "MEDIUM"},
                        }
                    ],
                },
            ]
        }
    ]
}

_APPSEC_SBOM_WITH_EVIDENCE = {
    "bomFormat": "CycloneDX",
    "specVersion": "1.5",
    "components": [
        {
            "type": "library",
            "name": "express",
            "version": "4.17.1",
            "purl": "pkg:npm/express@4.17.1",
            "evidence": {
                "callstack": [{"frames": []}],
            },
        },
        {
            "type": "library",
            "name": "lodash",
            "version": "4.17.20",
            "purl": "pkg:npm/lodash@4.17.20",
        },
    ],
}


def test_reachability_true_false(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()

    sbom_record = store.write("sbom", b"{}", source_hash="h1")

    appsec_path = tmp_path / "sbom_appsec.cdx.json"
    appsec_path.write_text(json.dumps(_APPSEC_SBOM_WITH_EVIDENCE))

    with patch("audit_harvest.producers.cve_overlay.run_tool") as mock_run:
        mock_run.return_value = _fake_result(stdout=json.dumps(_OSV_TWO_PACKAGES))
        record = produce_cve_overlay(
            repo, store, Path(sbom_record.path), sbom_appsec_path=appsec_path
        )

    content = json.loads(Path(record.path).read_bytes())
    findings = content["findings"]
    assert len(findings) == 2

    express_finding = next(f for f in findings if f["package_name"] == "express")
    lodash_finding = next(f for f in findings if f["package_name"] == "lodash")

    assert express_finding["reachable"] is True
    assert lodash_finding["reachable"] is False


def test_reachability_null_when_no_appsec(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()

    sbom_record = store.write("sbom", b"{}", source_hash="h1")

    with patch("audit_harvest.producers.cve_overlay.run_tool") as mock_run:
        mock_run.return_value = _fake_result(stdout=json.dumps(_OSV_TWO_PACKAGES))
        record = produce_cve_overlay(repo, store, Path(sbom_record.path))

    content = json.loads(Path(record.path).read_bytes())
    for finding in content["findings"]:
        assert finding["reachable"] is None


def test_reachability_null_when_file_missing(tmp_path):
    store = ArtifactStore(tmp_path / "store")
    repo = tmp_path / "repo"
    repo.mkdir()

    sbom_record = store.write("sbom", b"{}", source_hash="h1")
    missing_path = tmp_path / "nonexistent_appsec.cdx.json"

    with patch("audit_harvest.producers.cve_overlay.run_tool") as mock_run:
        mock_run.return_value = _fake_result(stdout=json.dumps(_OSV_TWO_PACKAGES))
        record = produce_cve_overlay(
            repo, store, Path(sbom_record.path), sbom_appsec_path=missing_path
        )

    content = json.loads(Path(record.path).read_bytes())
    for finding in content["findings"]:
        assert finding["reachable"] is None
