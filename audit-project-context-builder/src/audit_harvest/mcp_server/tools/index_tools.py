"""MCP tools: harvest_index_list, harvest_index_get."""
from __future__ import annotations

import json
from pathlib import Path

from audit_harvest.mcp_server.server import mcp
from audit_harvest.storage import ArtifactStore
from audit_harvest.producers.index import produce_index


@mcp.tool()
def harvest_index_list(store_root: str, max_age_days: int = 7) -> dict:
    """List all Stage 1 artifacts with their hashes and staleness status."""
    store = ArtifactStore(Path(store_root))
    return produce_index(store, max_age_days)


@mcp.tool()
def harvest_index_get(store_root: str, artifact_name: str) -> dict:
    """Read the content of a specific Stage 1 artifact by name."""
    store = ArtifactStore(Path(store_root))
    record = store.get(artifact_name)
    if record is None:
        return {"error": f"artifact '{artifact_name}' not found"}
    artifact_path = Path(record.path)
    if not artifact_path.exists():
        return {"error": "artifact file missing"}
    return {"content": artifact_path.read_text(errors="replace")}
