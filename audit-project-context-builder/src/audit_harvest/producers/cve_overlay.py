from __future__ import annotations

import hashlib
import json
import time
from pathlib import Path

from audit_harvest.storage import ArtifactRecord, ArtifactStore
from audit_harvest.subprocess_utils import ToolError, ToolNotFound, ToolTimeout, _resolver, run_tool


def _load_reachable_purls(sbom_appsec_path: Path) -> set[str] | None:
    if not sbom_appsec_path.exists():
        return None
    try:
        data = json.loads(sbom_appsec_path.read_bytes())
    except (json.JSONDecodeError, OSError):
        return None

    reachable: set[str] = set()

    for comp in data.get("components", []):
        evidence = comp.get("evidence", {})
        if evidence.get("callstack") or evidence.get("occurrences"):
            purl = comp.get("purl", "")
            if purl:
                reachable.add(purl)

    return reachable


def produce_cve_overlay(
    repo_path: Path,
    store: ArtifactStore,
    sbom_path: Path,
    sbom_appsec_path: Path | None = None,
) -> ArtifactRecord:
    src_hash = hashlib.sha256(sbom_path.read_bytes()).hexdigest()

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
    except ToolNotFound:
        raw = {"results": []}
    except ToolTimeout as e:
        raise RuntimeError(f"osv-scanner timed out: {e}") from e

    reachable_purls: set[str] | None = None
    if sbom_appsec_path is not None:
        reachable_purls = _load_reachable_purls(sbom_appsec_path)

    findings = _parse_osv_output(raw, reachable_purls)

    overlay = {
        "meta": {
            "artifact_id": "cve_overlay",
            "built_at": time.time(),
            "producer_version": "0.1.0",
            "source_hash": src_hash,
        },
        "findings": findings,
    }

    return store.write(
        "cve_overlay",
        json.dumps(overlay).encode(),
        source_hash=src_hash,
    )


def _parse_osv_output(raw: dict, reachable_purls: set[str] | None) -> list[dict]:
    findings = []
    for result in raw.get("results", []):
        for pkg in result.get("packages", []):
            pkg_info = pkg.get("package", {})
            name = pkg_info.get("name", "")
            version = pkg_info.get("version", "")
            purl = pkg_info.get("purl", "")
            for vuln in pkg.get("vulnerabilities", []):
                vuln_id = vuln.get("id", "")
                aliases = vuln.get("aliases", [])
                db_specific = vuln.get("database_specific", {})
                severity = db_specific.get("severity", "UNKNOWN").upper()
                if reachable_purls is None:
                    reachable = None
                else:
                    reachable = purl in reachable_purls
                findings.append({
                    "package_name": name,
                    "package_version": version,
                    "vulnerability_id": vuln_id,
                    "severity": severity,
                    "aliases": aliases,
                    "purl": purl,
                    "reachable": reachable,
                })
    return findings
