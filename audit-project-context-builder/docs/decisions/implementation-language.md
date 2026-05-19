# Decision: Implementation Language

**Choice:** Python 3.11+

**Rationale:**
- Aider `repomap.py` is Apache-2.0 Python; direct vendoring saves ~1 day vs subprocess or port.
- `tree-sitter` Python bindings are mature and well-tested.
- MCP Python SDK (fastmcp) has strong support.
- `subprocess` ergonomics are fine for shelling out to cdxgen/osv-scanner.

**Considered:** Go (consistent with security-code-scan SAST tools), TypeScript/Node.
Go would require porting Aider repomap or subprocess calls; TypeScript adds same polyglot tax.
