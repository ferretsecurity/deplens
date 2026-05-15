# Parent Plugin Conventions

*Canonical reference for Stages 2–5 plugin scaffolding. Established by Stage 1
(`audit-project-context-builder`). Do not re-derive — follow or explicitly deviate with
a recorded decision.*

*Last updated: 2026-05-14*

---

## 1. Plugin shape (every stage)

Each stage is an independently versioned Claude Code plugin with this structure:
```
audit-stage<N>-<name>/
├── .claude-plugin/
│   └── plugin.json          # name, version, description
├── .mcp.json                # declares this stage's MCP server
├── agents/
│   └── stage<N>-<name>.md   # the sub-agent orchestrator prompt
├── commands/
│   └── audit-<name>.md      # optional: /<cmd> for direct human invocation
├── README.md
├── docs/
│   ├── PLAN.md              # this stage's implementation plan
│   ├── decisions/           # one .md per resolved [DECISION NEEDED]
│   └── research-notes/      # findings from [RESEARCH NEEDED] tasks
├── interfaces/
│   └── stage<N>-outputs.schema.json   # JSON Schema for this stage's artifacts
├── src/audit_stage<N>/      # Python producers + MCP server
│   ├── producers/
│   ├── mcp_server/
│   └── llm/
├── pyproject.toml
├── tests/
└── golden/
```

## 2. Artifact storage convention (every stage)

Artifacts are stored **inside the audited repo**, gitignored, under:
```
<repo>/.audit/
  stage1/
    <git-sha>/         ← 8-char short SHA of the audited repo's HEAD
      <artifact files>
      index.json       ← manifest: built_from_commit, built_at, artifact hashes, stale flags
    current -> <git-sha>/   ← symlink; updated on successful completion only
  stage2/
    <git-sha>/
      ...
    current -> <git-sha>/
  ...
```

**Git SHA rules:**
- Clean working tree: `git rev-parse --short=8 HEAD` of the **audited repo** (not the plugin repo).
- Dirty working tree: `dirty-<unix-timestamp>`. Do NOT create/update `current` for dirty runs.

**`current` symlink discipline:**
- Updated ONLY on successful completion of the stage's full run.
- Never updated on partial runs, errors, or dirty-state runs.
- Downstream stages always read from `current/` — they never need to know the SHA.

**Gitignore:** the sub-agent checks on first run that `.audit/` is in the audited repo's
`.gitignore` (or user global gitignore) and warns if not.

## 3. MCP server convention (every stage)

Each stage has one MCP server (`audit-stage<N>-mcp`) declared in `.mcp.json`:

```json
{
  "mcpServers": {
    "audit-stage<N>": {
      "command": "uv",
      "args": ["run", "--project", "${PLUGIN_DIR}/src", "audit-stage<N>-mcp"]
    }
  }
}
```

**Read-side tools are non-negotiable.** Every stage exposes MCP tools so downstream
stages can query its outputs without reading raw files. Minimum read-side tools:
- `stage<N>.index.list()` — list all artifacts in current run
- `stage<N>.index.get(artifact_name)` — get artifact record with path and hash

Stage 1 additionally exposes `repomap.query()` and (Phase 2) `cpg.*` tools.

**All MCP servers from all stages are declared in the parent plugin's `.mcp.json`** so
any sub-agent can reach any stage's tools without going through the parent orchestrator.

**Maturity Stage 2 (Docker):** when Docker migration happens (triggered by Joern integration
in Stage 1 Phase 2), the `.mcp.json` command switches from `uv run --project` to
`docker run ... ghcr.io/yourorg/audit-stage<N>-mcp:latest`. MCP tool interface unchanged.

## 4. Sub-agent orchestrator convention (every stage)

Each stage's orchestrator is a Claude Code **sub-agent** (not a skill). Sub-agent file:
`agents/stage<N>-<name>.md`.

**Why sub-agent, not skill:** sub-agents get their own context. Producer-by-producer logs,
intermediate decisions, and grounding-failure diagnostics do not leak into the parent agent's
context window. The parent sees only the hand-off: artifact paths + always-on bundle + status.

**Mandatory sections in every stage sub-agent prompt:**
1. Identity and scope (what this stage does; what it does NOT do)
2. Tool catalog (MCP tools available, bash tools available, file paths)
3. Startup sequence (discovery → always-on bundle check → prerequisite check)
4. Decision tree (strategy choices the sub-agent makes with LLM judgment)
5. Read sequence (which artifacts to read, in which order, using which mechanism)
6. Write convention (where outputs go, how to update the `current` symlink)
7. Failure protocol (log → mark in index.json → continue if non-blocking; halt if blocking)
8. Grounding discipline (every factual claim in output must cite a file:line or tool result)

## 5. Always-on bundle and startup sequence (every stage)

**Stage 1 produces an always-on bundle** at `<repo>/.audit/stage1/current/always-on-bundle.md`
(≤5k tokens). It contains:
- A1 `repo_profile.md` (full, ≤2k tokens)
- A4 gate matrix summary (top applicable CWE classes, ~200 tokens)
- Top-10 entry points from A2 (~300 tokens)
- Top-20 SBOM components with critical CVEs from A5/A6 (~500 tokens)
- A12 framework conventions (~500 tokens)
- A13 agents.md (~300 tokens)
- A14 index summary (~200 tokens)

**The parent orchestrator injects this bundle into every stage sub-agent's system context**
before invocation. Sub-agents do not re-read the individual artifact files for always-on
content — they consume the pre-assembled bundle.

**Every stage sub-agent's startup sequence:**
```
Verify <repo>/.audit/stage1/current/index.json exists and is not stale.
→ If missing: halt. Tell user to run Stage 1 first.
→ If stale: halt. Tell user to re-run Stage 1.
[If not Stage 2] Verify prior stage's current/index.json exists and is not stale.
Call this stage's check_prerequisites() MCP tool.
→ Halt on any blocking missing tool.
Begin stage-specific work per the read sequence in PLAN_global.md §4.
```

## 6. Cross-stage interface contract

The authoritative artifact consumption table (which stage reads which artifact, via which
mechanism) lives in `PLAN_global.md §4`. Stage plans must reference it, not duplicate it.

**Three consumption mechanisms:**
- **Always-on:** in the system context bundle; sub-agent has it without reading anything.
  Artifacts: A1, A4 summary, A12, A13, A14 summary.
- **File read:** sub-agent uses Read tool on the path from index.json.
  Artifacts: A2, A3, A5, A6, A9, A10, A11, and inter-stage artifacts (stage2/tool_results.json etc.)
- **MCP tool:** sub-agent calls a structured query tool. Never reads raw underlying files.
  Artifacts: A7 (repomap.query), A8 (cpg.taint, cpg.callgraph, cpg.callers, cpg.slice).

## 7. Grounding discipline (every stage)

Every factual claim in every artifact produced by any stage must either:
- Have a deterministic source (tool output, regex match, file content) — cite the source.
- Have a grounded LLM citation: `{"file": "path", "lines": [a, b], "evidence": "..."}` —
  verified by opening the file and checking the lines exist.

Claims that fail retrieval verification are **dropped**, not emitted. Log dropped claims to
`<storage>/llm-grounding-failures.jsonl`.

This applies to: Stage 1 (A3, A4 ambiguous cases, A9 layer 3, A13), Stage 3 (all findings),
Stage 4 (all verified findings).

## 8. Serena (LSP) optional dependency

Serena (`oraios/serena`, MIT) is an optional MCP server providing semantic symbol navigation.
Declared in the parent `.mcp.json` as a commented-out entry; users opt in by uncommenting
and installing language servers.

Stages that benefit: Stage 1 (A2 handler resolution, A9 call sites), Stage 3 (call-chain
tracing without reading whole files), Stage 4 (call-graph reachability confirmation).

Fallback when absent: repomap.query() for navigation, ripgrep for reference finding.
Sub-agent prompts must handle both paths explicitly — never assume Serena is present.

## 9. Plugin distribution

All stage plugins are distributed via the **private GitHub self-hosted marketplace**:
```bash
/plugin marketplace add <your-org/skills-repo>
/plugin install audit-stage<N>-<name>@skills-repo
```

No global PyPI publish for Phases 1–2. Backlog item: publish to PyPI and switch to `uvx`.

## 10. Decisions that belong in this file (not in individual stage plans)

When a decision is made that affects multiple stages, record it here in a "Decision log"
appendix. Individual stage plans cite this file rather than re-recording the reasoning.

Current cross-stage decisions:
- D1: Sub-agent (Shape B) over skill for every stage — see §4 above.
- D2: In-repo `.audit/` storage with git-SHA directories — see §2 above.
- D3: Python + `uv run --project` for all stage MCP servers (Maturity Stage 1).
- D4: `current` symlink convention — see §2 above.
- D5: Grounding discipline on all LLM-produced artifacts — see §7 above.
- D6: Always-on bundle assembled by parent; sub-agents consume bundle, not raw artifacts.
- D7: Serena optional, never bundled, graceful fallback required — see §8 above.
