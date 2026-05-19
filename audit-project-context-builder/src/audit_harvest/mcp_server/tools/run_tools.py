"""MCP tools: harvest_run_* -- invoke each producer."""
from __future__ import annotations

import hashlib
import json
import os
from dataclasses import asdict
from pathlib import Path

from audit_harvest.mcp_server.server import mcp
from audit_harvest.storage import ArtifactStore
from audit_harvest.subprocess_utils import ToolPathResolver

_resolver = ToolPathResolver()

_BUCKET_A_TOOLS = ["git", "enry", "scc"]
_BUCKET_B_TOOLS = ["cdxgen", "osv-scanner"]


def _get_store(repo_path: str) -> tuple[ArtifactStore, Path]:
    repo = Path(repo_path).resolve()
    store_root = os.environ.get("AUDIT_HARVEST_DIR") or str(repo / ".audit" / "harvest")
    return ArtifactStore(Path(store_root)), repo


@mcp.tool()
def harvest_check_prerequisites() -> dict:
    """Check all required tools are on PATH. Call before any producer."""
    results = {}
    for tool in _BUCKET_A_TOOLS + _BUCKET_B_TOOLS:
        results[tool] = _resolver.check(tool)
    missing_a = [t for t in _BUCKET_A_TOOLS if results[t]["status"] == "missing"]
    missing_b = [t for t in _BUCKET_B_TOOLS if results[t]["status"] == "missing"]
    return {
        "ok": len(missing_a) == 0,
        "tools": results,
        "missing_bucket_a": missing_a,
        "missing_bucket_b": missing_b,
    }


@mcp.tool()
def harvest_run_sbom(repo_path: str) -> dict:
    """Run A5: generate CycloneDX SBOM via cdxgen. Must run before harvest_run_repo_profile."""
    from audit_harvest.producers.sbom import produce_sbom
    store, repo = _get_store(repo_path)
    record = produce_sbom(repo, store)
    return {"status": "ok", "artifact": asdict(record)}


@mcp.tool()
def harvest_run_cve_overlay(repo_path: str) -> dict:
    """Run A6: CVE overlay via osv-scanner. Requires A5 SBOM."""
    from audit_harvest.producers.cve_overlay import produce_cve_overlay
    store, repo = _get_store(repo_path)
    sbom_record = store.get("sbom")
    if sbom_record is None:
        return {"status": "error", "message": "Run harvest_run_sbom first (A5 required)"}
    record = produce_cve_overlay(repo, store, Path(sbom_record.path))
    return {"status": "ok", "artifact": asdict(record)}


@mcp.tool()
def harvest_run_repo_profile(repo_path: str) -> dict:
    """Run A1: produce repo_profile. Always writes an artifact; degraded if tools or SBOM missing."""
    from audit_harvest.producers.repo_profile import produce_repo_profile
    store, repo = _get_store(repo_path)
    reason: str | None = None

    sbom_record = store.get("sbom")
    if sbom_record is None:
        reason = "A5 SBOM not found -- run harvest_run_sbom first"
    else:
        try:
            record = produce_repo_profile(repo, store, Path(sbom_record.path))
            return {"status": "ok", "artifact": asdict(record)}
        except Exception as e:
            reason = str(e)

    content = f"# Repository Profile\n\nGeneration failed: {reason}\n"
    src_hash = hashlib.sha256(reason.encode()).hexdigest()
    record = store.write("repo_profile", content.encode(), source_hash=src_hash)
    return {"status": "degraded", "artifact": asdict(record), "reason": reason}


@mcp.tool()
def harvest_run_repomap(repo_path: str) -> dict:
    """Run A7: extract symbol map via tree-sitter."""
    from audit_harvest.producers.repomap.producer import produce_repomap
    store, repo = _get_store(repo_path)
    record = produce_repomap(repo, store)
    return {"status": "ok", "artifact": asdict(record)}


@mcp.tool()
def harvest_run_entry_points(repo_path: str) -> dict:
    """Run A2: extract HTTP/CLI entry points."""
    from audit_harvest.producers.entry_points import produce_entry_points
    store, repo = _get_store(repo_path)
    record = produce_entry_points(repo, store)
    return {"status": "ok", "artifact": asdict(record)}


@mcp.tool()
def harvest_run_gate_matrix(repo_path: str) -> dict:
    """Run A4: CWE applicability gate matrix. Requires A2 + A5."""
    from audit_harvest.producers.gate_matrix import produce_gate_matrix
    store, repo = _get_store(repo_path)
    ep_record = store.get("entry_points")
    entry_points: dict = {"entry_points": []}
    if ep_record:
        try:
            entry_points = json.loads(Path(ep_record.path).read_text())
        except (json.JSONDecodeError, OSError):
            entry_points = {"entry_points": []}
    sbom_record = store.get("sbom")
    sbom_components: list = []
    if sbom_record:
        try:
            sbom_data = json.loads(Path(sbom_record.path).read_text())
            sbom_components = sbom_data.get("components", [])
        except (json.JSONDecodeError, OSError):
            sbom_components = []
    result = produce_gate_matrix(repo, entry_points, sbom_components)
    src_hash = hashlib.sha256(str(repo).encode()).hexdigest()
    record = store.write("gate_matrix", json.dumps(result, indent=2).encode(), source_hash=src_hash)
    return {"status": "ok", "artifact": asdict(record)}


@mcp.tool()
def harvest_run_index(repo_path: str, max_age_days: int = 7) -> dict:
    """Run A14: build artifact index. Run after all other producers."""
    from audit_harvest.producers.index import produce_index
    store, repo = _get_store(repo_path)
    data = produce_index(store, max_age_days=max_age_days)
    src_hash = hashlib.sha256(str(repo).encode()).hexdigest()
    store.write("index", json.dumps(data, indent=2).encode(), source_hash=src_hash)
    return {"status": "ok", "index": data}
