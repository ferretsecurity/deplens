# Decision: Producer Invocation Surface

**Choice:** MCP tools as default. Each producer exposed as `harvest_run_<name>`.

**Rationale:**
- Consistent surface with read-side tools (all MCP).
- Structured error types and schemas visible to LLM.
- Single server startup amortizes overhead.

**Per-producer record:**
| Producer | MCP Tool | Notes |
|---|---|---|
| A5 sbom | `harvest_run_sbom` | Must run first (A1 depends on it) |
| A6 cve_overlay | `harvest_run_cve_overlay` | Requires A5 |
| A1 repo_profile | `harvest_run_repo_profile` | Requires A5 SBOM |
| A7 repomap | `harvest_run_repomap` | Independent |
| A2 entry_points | `harvest_run_entry_points` | Independent |
| A4 gate_matrix | `harvest_run_gate_matrix` | Requires A2 + A5 |
| A14 index | `harvest_run_index` | Must run last |

**Producer execution order (dependency-locked):**
1. `harvest_run_sbom` — A5 (required by A1, A4)
2. `harvest_run_cve_overlay` — A6 (requires A5)
3. `harvest_run_repo_profile` — A1 (requires A5)
4. `harvest_run_repomap` — A7 (independent)
5. `harvest_run_entry_points` — A2 (independent)
6. `harvest_run_gate_matrix` — A4 (requires A2, A5)
7. `harvest_run_index` — A14 (requires all above)
