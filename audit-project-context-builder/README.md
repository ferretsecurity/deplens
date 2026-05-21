# audit-project-context-builder

Stage 1 of the white-box security audit pipeline. Profiles a code repository and produces 7 hash-tracked artifacts consumed by downstream audit stages (2-5).

## MCP Servers

This plugin includes the `audit-harvest` MCP server, configured in `.mcp.json`.

## Optional: LSP Support

For enhanced code navigation, you can add a Serena entry to your Claude Code settings to enable LSP-over-MCP. If Serena is installed and configured, add the following to your `.mcp.json`:

```json
{
  "serena": {
    "command": "serena",
    "args": ["--language-server"]
  }
}
```

If Serena is not available, the plugin gracefully falls back to repomap and ripgrep for code queries.
