"""Pipeline integration test: exercises harvest_* MCP tool functions directly.

Calls the same Python functions the MCP server exposes, in the dependency-locked
order A5 -> A6 -> A1 -> A7 -> A2 -> A4 -> A14. No MCP transport involved.

Bucket B steps (A5 via cdxgen, A6 via osv-scanner) are skipped gracefully when
the binaries are absent. A1 is also skipped when A5 was skipped (it needs the SBOM).
All other steps (A7, A2, A4, A14) run unconditionally -- they need no external tools.

Set AUDIT_HARVEST_DIR via monkeypatch so fixtures are never written to.
"""
import json
from pathlib import Path

import pytest

_FIXTURES_DIR = Path(__file__).parent.parent / "fixtures"
FIXTURES = [
    _FIXTURES_DIR / "go_simple",
    _FIXTURES_DIR / "python_flask",
    _FIXTURES_DIR / "js_express",
    _FIXTURES_DIR / "java_spring",
]


@pytest.mark.integration
@pytest.mark.parametrize("fixture", FIXTURES, ids=[f.name for f in FIXTURES])
def test_full_mcp_tool_pipeline(fixture, tmp_path, monkeypatch):
    """Run the full harvest pipeline via MCP tool functions in dependency order."""
    monkeypatch.setenv("AUDIT_HARVEST_DIR", str(tmp_path))

    from audit_harvest.mcp_server.tools.run_tools import (
        harvest_check_prerequisites,
        harvest_run_cve_overlay,
        harvest_run_entry_points,
        harvest_run_gate_matrix,
        harvest_run_index,
        harvest_run_repo_profile,
        harvest_run_repomap,
        harvest_run_sbom,
    )

    repo = str(fixture)

    # Step 0 — prerequisites
    prereqs = harvest_check_prerequisites()
    assert isinstance(prereqs["ok"], bool)
    bucket_a_ok = prereqs["ok"]  # git + enry + scc all present
    bucket_b_ok = len(prereqs["missing_bucket_b"]) == 0

    # Step 1 — A5 SBOM (non-blocking: skipped if cdxgen absent)
    sbom_produced = False
    if bucket_b_ok:
        result = harvest_run_sbom(repo)
        assert result["status"] == "ok", result
        assert "artifact" in result
        sbom_produced = True

    # Step 2 — A6 CVE overlay (non-blocking: skipped if A5 skipped or osv-scanner absent)
    if sbom_produced:
        result = harvest_run_cve_overlay(repo)
        assert result["status"] == "ok", result

    # Step 3 — A1 repo profile (requires A5 + Bucket A tools enry/scc)
    if sbom_produced and bucket_a_ok:
        result = harvest_run_repo_profile(repo)
        assert result["status"] == "ok", result
        assert "artifact" in result

    # Step 4 — A7 repomap (independent)
    result = harvest_run_repomap(repo)
    assert result["status"] == "ok", result
    repomap_data = json.loads(Path(result["artifact"]["path"]).read_text())
    assert "symbols" in repomap_data
    assert repomap_data["meta"]["total_symbols"] > 0

    # Step 5 — A2 entry points (independent)
    result = harvest_run_entry_points(repo)
    assert result["status"] == "ok", result
    ep_data = json.loads(Path(result["artifact"]["path"]).read_text())
    assert "entry_points" in ep_data

    # Step 6 — A4 gate matrix (uses A2 + A5 from store if present)
    result = harvest_run_gate_matrix(repo)
    assert result["status"] == "ok", result
    gm_data = json.loads(Path(result["artifact"]["path"]).read_text())
    assert len(gm_data["rules"]) == 13

    # Step 7 — A14 index (must cover every artifact written above)
    result = harvest_run_index(repo)
    assert result["status"] == "ok", result
    index = result["index"]
    # produce_index snapshots the store before writing the index artifact itself,
    # so "index" is absent from the returned data. Minimum: repomap + entry_points + gate_matrix.
    assert len(index["artifacts"]) >= 3
    artifact_names = {a["name"] for a in index["artifacts"]}
    assert {"repomap", "entry_points", "gate_matrix"}.issubset(artifact_names)
    assert all(not a["stale"] for a in index["artifacts"])


@pytest.mark.integration
@pytest.mark.parametrize("fixture", FIXTURES, ids=[f.name for f in FIXTURES])
def test_second_run_returns_cached_artifacts(fixture, tmp_path, monkeypatch):
    """Running entry_points and repomap twice yields the same content_hash (cache hit)."""
    monkeypatch.setenv("AUDIT_HARVEST_DIR", str(tmp_path))

    from audit_harvest.mcp_server.tools.run_tools import (
        harvest_run_entry_points,
        harvest_run_repomap,
    )

    repo = str(fixture)

    ep1 = harvest_run_entry_points(repo)
    ep2 = harvest_run_entry_points(repo)
    assert ep1["artifact"]["content_hash"] == ep2["artifact"]["content_hash"]
    assert ep1["artifact"]["path"] == ep2["artifact"]["path"]

    rm1 = harvest_run_repomap(repo)
    rm2 = harvest_run_repomap(repo)
    assert rm1["artifact"]["content_hash"] == rm2["artifact"]["content_hash"]
    assert rm1["artifact"]["path"] == rm2["artifact"]["path"]
