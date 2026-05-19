"""MCP tool: harvest_repomap_query."""
from __future__ import annotations

import json
from pathlib import Path

from audit_harvest.mcp_server.server import mcp
from audit_harvest.storage import ArtifactStore


@mcp.tool()
def harvest_repomap_query(store_root: str, budget_tokens: int = 8000) -> dict:
    """Query the repomap artifact. Returns the A7 repo map from the stored artifact.

    If the artifact has not been produced yet, returns an error message.
    Pass budget_tokens to control the token budget when regenerating.
    """
    store = ArtifactStore(Path(store_root))
    record = store.get("repomap")
    if record is None:
        return {"error": "repomap not yet produced — run harvest_run_repomap first"}
    artifact_path = Path(record.path)
    if not artifact_path.exists():
        return {"error": "repomap artifact file missing"}
    try:
        return json.loads(artifact_path.read_text(errors="replace"))
    except json.JSONDecodeError:
        return {"error": "repomap artifact is not valid JSON"}
