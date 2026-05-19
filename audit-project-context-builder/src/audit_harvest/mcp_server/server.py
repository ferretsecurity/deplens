"""audit-harvest-mcp: MCP server exposing read-side and producer tools."""
from __future__ import annotations

from fastmcp import FastMCP

mcp = FastMCP("audit-harvest")

# Import tool modules after mcp is defined to trigger @mcp.tool() registration.
# This ordering is intentional to avoid circular-import errors.
from audit_harvest.mcp_server.tools import index_tools, repomap_tools, run_tools  # noqa: E402


def main() -> None:
    mcp.run()
