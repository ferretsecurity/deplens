from __future__ import annotations

import json
import time
from pathlib import Path

from audit_harvest.storage import ArtifactRecord, ArtifactStore
from audit_harvest.subprocess_utils import ToolError, _resolver, run_tool


def produce_cve_overlay(
    repo_path: Path,
    store: ArtifactStore,
    sbom_path: Path,
) -> ArtifactRecord:
    try:
        result = run_tool(
            [
                _resolver.resolve("osv-scanner"),
                "--format", "json",
                "--sbom", str(sbom_path),
            ],
            cwd=repo_path,
            timeout_sec=120,
        )
        raw = json.loads(result.stdout)
    except ToolError as e:
        # osv-scanner exits 1 when vulnerabilities are found — stdout still has valid JSON
        raw = json.loads(e.result.stdout) if e.result.stdout else {"results": []}

    findings = _parse_osv_output(raw)

    overlay = {
        "meta": {
            "artifact_id": "cve_overlay",
            "built_at": time.time(),
            "producer_version": "0.1.0",
            "source_hash": str(sbom_path),
        },
        "findings": findings,
    }

    return store.write(
        "cve_overlay",
        json.dumps(overlay).encode(),
        source_hash=str(sbom_path),
    )


def _parse_osv_output(raw: dict) -> list[dict]:
    findings = []
    for result in raw.get("results", []):
        for pkg in result.get("packages", []):
            pkg_info = pkg.get("package", {})
            name = pkg_info.get("name", "")
            version = pkg_info.get("version", "")
            for vuln in pkg.get("vulnerabilities", []):
                vuln_id = vuln.get("id", "")
                aliases = vuln.get("aliases", [])
                db_specific = vuln.get("database_specific", {})
                severity = db_specific.get("severity", "UNKNOWN").upper()
                purl = pkg_info.get("purl", "")
                findings.append({
                    "package_name": name,
                    "package_version": version,
                    "vulnerability_id": vuln_id,
                    "severity": severity,
                    "aliases": aliases,
                    "purl": purl,
                })
    return findings
