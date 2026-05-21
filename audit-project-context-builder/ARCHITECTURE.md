This document captures the permanent architectural knowledge for Stage 1 (audit-project-context-builder). For Phase 2 sprint tasks and open implementation questions, see `PLAN.md`. For cross-stage conventions that all stages must follow, see `docs/parent-plugin-conventions.md`.

---

## §1 Context and scope

Stage 1 is the first of five stages in a white-box security audit agent pipeline. The pipeline stages are: Stage 1 (Discover), Stage 2 (Tooling), Stage 3 (Code Review), Stage 4 (Verify), Stage 5 (Report). Each stage ships as an independent Claude Code plugin.

Stage 1 produces 14 persistent, hash-tracked artifacts that downstream stages consume instead of repeatedly re-reading source files. The artifacts cover repo profiling (A1), entry points (A2), trust boundaries (A3), gate matrix (A4), SBOM (A5), CVE overlay (A6), symbol graph (A7), code property graph (A8), sources/sinks (A9), tests inventory (A10), git signals (A11), framework conventions (A12), AGENTS.md echo (A13), and the index (A14).

MVP scope (Phase 1): A1, A2, A4, A5, A6, A7, A14 — 7 artifacts.

Phase 2 adds: A3, A8, A9, A10, A11, A12, A13.

Out of scope through Stage 1: finding vulnerabilities, running SAST, producing fix guidance. Those are Stages 2-5.

The authoritative artifact list and downstream schema contracts live in `interfaces/harvest-outputs.schema.json`.

---

## §2 Locked shape decisions

Five decisions are locked. Do not relitigate them.

1. **Ship as a Claude Code plugin.** Stage 1 is the first of five stages; each stage ships as its own plugin, independently versioned. Stage 1's shape sets the precedent for Stages 2-5.

2. **Sub-agent orchestrator (not the parent agent) runs all producers.** Stage 1's orchestrator is a Claude Code sub-agent with its own context, invoked by the parent (or directly by a human) with the repo path. The sub-agent runs producers, makes strategy decisions, writes artifacts, and hands back. It does not run inline in the parent agent's context.

3. **Deterministic producers do facts; LLM judgment is used only where determinism is infeasible.** The producer code (Python) handles all extraction, hashing, caching, and SBOM/CPG/repomap generation deterministically. The sub-agent uses LLM judgment for strategy selection (which producers, what budgets, which paths to deep-scan) and invokes LLM-driven producers (A3, A4 ambiguous cases, A9 layer 3, A13) with cite-or-omit grounding required on every emitted claim.

4. **Read-side MCP tools are non-negotiable.** Downstream stages (and the sub-agent itself) query repomap and CPG via MCP tools, not by reading JSON files directly. The MCP server lives in this plugin.

5. **Two-maturity-stage dependency architecture.** How system-level tools are delivered and isolated follows a two-stage model (see §6.6). Maturity Stage 1 uses PATH tools. Maturity Stage 2 uses Docker. No implementation task in Phase 1 may contradict this model.

Each decision is elaborated in §6.

---

## §3 Plugin structure

### Directory tree

```
audit-project-context-builder/
├── .claude-plugin/
│   └── plugin.json                     # plugin metadata
├── .mcp.json                           # declares the audit-harvest-mcp server
├── agents/
│   └── harvest-agent.md                # the orchestrator sub-agent
├── commands/
│   └── audit-discover.md               # /audit-discover <repo> for direct human use
├── README.md
├── docs/
│   ├── parent-plugin-conventions.md    # written first; conventions for all stages
│   ├── decisions/
│   │   ├── producer-surfaces.md        # per-producer invocation choice
│   │   └── ...                         # one file per resolved decision
│   └── research-notes/                 # findings from investigation tasks
├── interfaces/
│   └── harvest-outputs.schema.json     # JSON Schema for all artifacts; published contract
├── src/audit_harvest/
│   ├── __init__.py
│   ├── storage.py                      # content-hash + mtime cache
│   ├── subprocess_utils.py             # tool invocation wrapper
│   ├── producers/
│   │   ├── repo_profile.py             # A1
│   │   ├── entry_points.py             # A2
│   │   ├── trust_boundaries.py         # A3 (Phase 2)
│   │   ├── gate_matrix.py              # A4
│   │   ├── sbom.py                     # A5
│   │   ├── cve_overlay.py              # A6
│   │   ├── repomap/                    # A7 (vendored from Aider)
│   │   ├── cpg/                        # A8 (Phase 2)
│   │   ├── sources_sinks.py            # A9 (Phase 2)
│   │   ├── tests_inventory.py          # A10 (Phase 2)
│   │   ├── git_signals.py              # A11 (Phase 2)
│   │   ├── framework_conventions/      # A12 (Phase 2; curated templates)
│   │   ├── agents_md.py                # A13 (Phase 2)
│   │   ├── index.py                    # A14
│   │   └── registry/
│   │       ├── frameworks.yaml
│   │       ├── secret_managers.yaml
│   │       ├── cwe_rules.yaml              # CWE applicability signal definitions (A4)
│   │       └── cwe_loader.py               # loads cwe_rules.yaml into frozen CweRule dataclasses
│   ├── extractors/
│   │   ├── frameworks/                 # per-framework route extractors (A2)
│   │   ├── ts_utils.py                 # shared tree-sitter helpers used by all A2 extractors
│   │   ├── sbom_runners.py             # cdxgen/syft subprocess wrappers
│   │   └── git.py                      # git log parsing
│   ├── mcp_server/
│   │   ├── server.py                   # the audit-harvest-mcp server
│   │   └── tools/
│   │       ├── read_side/              # repomap.query, cpg.*, index.*
│   │       │   ├── repomap_tools.py
│   │       │   ├── cpg_tools.py        # Phase 2
│   │       │   └── index_tools.py
│   │       └── producers/              # producer-side MCP tools
│   │           └── ...
│   └── llm/
│       ├── client.py                   # Anthropic client + retry/grounding
│       └── prompts/                    # versioned prompt files
├── pyproject.toml
├── tests/
│   ├── fixtures/                       # tiny test repos (per language)
│   ├── integration/
│   └── unit/
└── golden/                             # known-good artifact outputs for regression
```

### What lives where, by audience

| Audience | Reads from | Purpose |
|---|---|---|
| Parent security agent | `agents/harvest-agent.md` | Invokes Stage 1 as a sub-agent |
| Stage 1 sub-agent (LLM) | `agents/harvest-agent.md`, MCP tool catalog, `interfaces/harvest-outputs.schema.json` | Orchestration logic + tool surface |
| Downstream stages (LLM) | MCP read-side tools, `.audit/harvest/` artifacts, `interfaces/harvest-outputs.schema.json` | Consume Stage 1's output |
| Human running directly | `commands/audit-discover.md` | Optional CLI entry point |
| Claude Code (implementing) | `src/`, `docs/decisions/`, `docs/research-notes/`, `tests/` | Implementation surface |

### Five core conventions

1. The sub-agent (`agents/harvest-agent.md`) is production code, not a scratch file. Review, version, and test it the same way you would a critical function.

2. Every producer module exposes a `produce(repo_path, store)` entry point. The surface choice (MCP tool vs. script) is wrapping, not interface.

3. All subprocess calls go through `subprocess_utils.run_tool()` — never `subprocess.run()` directly.

4. All LLM calls go through `llm/client.py` — never direct `anthropic.Client` instantiation.

5. All artifact writes are atomic (write to temp file in same directory, then `os.replace`); validated against `interfaces/harvest-outputs.schema.json` before the write is committed.

---

## §4 Sub-agent orchestrator

### §4.1 What the sub-agent decides

| Decision | Source of judgment | Notes |
|---|---|---|
| **Strategy selection by repo size** | LLM judgment over deterministic measurements | Walk repo, measure LOC + file count + language mix deterministically. Then pick from the token-economics table: <50k LOC → eager full extraction; 50k–500k → top-N PageRank; 500k–5M → per-package; >5M → route-to-subtree-first. The measurements are deterministic; the picking is LLM judgment because edge cases exist. |
| **Per-producer budget** | LLM judgment | E.g., set repomap token budget based on strategy. |
| **Producer order and skip logic** | LLM with deterministic preconditions | Dependency order is fixed (A5 before A6; A4 needs A2+A5). Discretionary: whether to run A6's osv-scanner if A5 came up empty, whether to attempt A3 if A2 found zero auth boundaries. |
| **Gate matrix ambiguous cases** | LLM call inside the A4 producer | Producer code handles deterministic gates; only ambiguous cases hit the LLM, with grounding. |
| **Custom-wrapper labeling (A9 layer 3)** | LLM call inside the A9 producer | Same pattern. |
| **Trust-boundary inference (A3)** | LLM call inside the A3 producer | Always grounded. |
| **AGENTS.md echo-or-generate (A13)** | LLM call inside the A13 producer | Read if present; generate from A1+A7 otherwise. |
| **Staleness / refusal** | Deterministic from A14 hashes | Sub-agent reads `index.json`, refuses to hand off stale artifacts on a security run. |
| **Failure surfacing** | Sub-agent | When a producer fails, sub-agent reports the gap honestly rather than masking it. |

### §4.2 What the sub-agent does NOT do

- Symbol-level reasoning. That is repomap/CPG tool territory.
- Pre-reading every function. That is the anti-pattern this design explicitly rejects.
- Inventing artifact content. Every claim in an emitted artifact has either a deterministic source (a tool's output, a regex match) or a grounded LLM citation (`file:line` verified by retrieval). If neither, the claim does not get emitted.
- Fix suggestions, vulnerability findings, or CVE attribution. Out of scope for Stage 1.

### §4.3 Sub-agent prompt structure

The `agents/harvest-agent.md` prompt covers, in order:

1. **Identity and scope.** What Stage 1 is, what it produces, what it does not do.
2. **Tool catalog.** The MCP tools (read-side always; producer tools) and bash invocations.
3. **Decision tree.** The strategy table from §4.1, in prompt-readable form.
4. **Producer order and dependencies.** A topological description so the sub-agent does not run A6 before A5.
5. **Hand-off format.** What goes in `.audit/harvest/`, what the three-tier bundle (always-on / read-on-relevance / tool-only) looks like, how to report status back to the parent.
6. **Failure protocol.** When a producer fails: log, mark in `index.json`, continue if non-blocking, halt and surface if blocking.
7. **Grounding discipline.** Sub-agent's own outputs to the parent must be cite-anchored to artifact paths or tool outputs — same discipline as producer-internal LLM calls.

### §4.4 Why the orchestrator is a sub-agent, not a skill

A sub-agent gets its own context, so producer-by-producer logs, intermediate decisions, and grounding-failure diagnostics do not leak into the parent agent's context window. The parent sees only the hand-off (artifact paths + always-on bundle + status). This is the right partition: Stage 1's working memory is throw-away once artifacts are committed.

---

## §5 Cross-cutting infrastructure

### §5.1 ArtifactStore (`storage.py`)

`ArtifactStore(root_dir)` class with these key methods:

- `write(artifact_id, content, metadata) -> ArtifactRecord` — writes atomically, updates `current` symlink only on successful completion of a full run.
- `get(artifact_id) -> ArtifactRecord | None`
- `is_fresh(artifact_id, max_age_seconds) -> bool`

`ArtifactRecord` fields: `name`, `path`, `content_hash`, `source_hash`, `last_built_at`, `producer_version`.

Root directory convention: the `root_dir` passed to `ArtifactStore` is `<repo>/.audit/harvest/<git-sha>/` where `<git-sha>` comes from `git rev-parse HEAD` of the audited repo. The storage layer does not compute the SHA — the sub-agent orchestrator computes it at startup and passes it in.

`current` symlink: updated atomically only on successful completion of the full run. Dirty working trees use `dirty-<unix-timestamp>` as the directory name and never update `current`. Atomic symlink replacement on POSIX uses `os.symlink` to a temp name then `os.replace`.

### §5.2 ToolPathResolver + run_tool (`subprocess_utils.py`)

`ToolPathResolver` is the single location that changes when migrating from Maturity Stage 1 (PATH tools) to Stage 2 (Docker). In Maturity Stage 1, `resolver.path("cdxgen")` returns the bare binary name for PATH resolution. In Maturity Stage 2, it returns the path as seen inside the container.

`run_tool(resolver, args: list[str], cwd: Path, timeout_sec: int, expect_json: bool = False) -> ToolResult`

Captures stdout, stderr, returncode, and wall-time. Logs every invocation to `<storage>/runs/<timestamp>.jsonl` for audit trail.

Typed exception hierarchy:
- `ToolError` (base)
- `ToolTimeout`
- `ToolNotFound`

Never propagates raw `CalledProcessError`. Hard rule: `args` must be `list[str]`; `shell=True` is never used. The audit agent runs on attacker-influenced repos — a malicious filename must not escape into a shell.

### §5.3 LLM client with grounded-output enforcement (`llm/client.py`)

`call_with_grounding(prompt, retrieval_fn) -> GroundedOutput`

Every claim in the output carries a `{"file": "...", "lines": [a, b], "evidence": "..."}` reference.

Post-call validator behavior: the validator opens each cited file, checks the lines exist, and either passes or drops the unverified claim. It does not raise an exception on failure — it degrades gracefully. Every dropped claim is appended to `llm-grounding-failures.jsonl` in the artifact store root.

This is an invariant of the system, not an optional feature. It is the single technical mechanism preventing hallucinated symbols from compounding across stages.

### §5.4 JSON Schema registry (`interfaces/harvest-outputs.schema.json`)

Single source of truth for all artifact output shapes. Every producer validates its output against the schema before calling `store.write()`. A schema-violating artifact write raises before disk commit.

---

## §6 Prerequisite decisions and outcomes

### §6.1 Parent-plugin conventions

Each stage is an independent Claude Code plugin. The five core conventions are listed in §3. See `docs/parent-plugin-conventions.md` for the full specification.

### §6.2 Implementation language

Outcome: Python 3.11+.

Rationale:
- Aider `repomap.py` is Apache-2.0 Python — direct vendoring saves approximately one engineering day.
- `tree-sitter` Python bindings are mature.
- `fastmcp` and the Anthropic MCP SDK have strong Python support.

See `docs/decisions/implementation-language.md`.

### §6.3 Storage location

Outcome: in-repo at `.audit/harvest/<git-sha>/` relative to the audited repo root. Override: `AUDIT_HARVEST_DIR` env var.

```
<repo-root>/
└── .audit/
    └── harvest/
        ├── abc1234567890abcdef1234567890abcdef12345678/  ← clean working tree: full git SHA
        │   ├── repo_profile/   ← A1
        │   ├── sbom/           ← A5
        │   └── ...
        ├── dirty-1716123456/   ← dirty working tree: unix timestamp
        └── current -> abc1234567890abcdef1234567890abcdef12345678/  ← symlink; not updated for dirty runs
```

Git SHA rules: clean tree uses full 40-char SHA; dirty tree uses `dirty-<unix-timestamp>`; `current` symlink updated only on successful clean-tree runs.

See `docs/decisions/storage-location.md`.

### §6.4 Target languages

Outcome:

| Language | A2 Framework extractors | A7 tree-sitter grammar | cdxgen profile |
|---|---|---|---|
| Go | stdlib net/http, gin, chi, echo, fiber | go | default |
| Python | Flask, FastAPI, Django | python | default |
| JS/TS | Express, NestJS, Next.js API routes | javascript, typescript | default |
| Java | Spring @RestController, @Controller | java | default |

See `docs/decisions/target-languages.md`.

### §6.5 CodeQL

Out of scope through Phase 2.

### §6.6 Dependency maturity model

Outcome: two-bucket model.

Bucket A (required — abort if missing):

| Tool | Purpose |
|---|---|
| git | SHA computation, blame signals |
| enry | Language detection |
| scc | Line counts per language |
| rg (ripgrep) | Fast pattern search |

Bucket B (optional — log gap, continue):

| Tool | Purpose |
|---|---|
| cdxgen | CycloneDX SBOM generation |
| osv-scanner | CVE overlay |

`ToolPathResolver` is the upgrade seam: switching to Docker changes only this class, not any producer code.

Maturity Stage 1 startup sequence:
1. `harvest_check_prerequisites()` — validates both buckets, returns structured report.
2. Sub-agent reads the report and decides whether to proceed.
3. Bucket B gaps are noted in A14 index as `coverage_gaps`.

### §6.7 Maturity Stage 2 (Docker) — target state

The `.mcp.json` `command` switches from `uv run --project ${PLUGIN_DIR} audit-harvest-mcp` to `docker run <image> audit-harvest-mcp`. The MCP tool interface is unchanged. Implementation deferred to Phase 2.

Six open design questions to be answered before implementing:
1. How does the container access the audited repo's filesystem? (bind mount)
2. What is the image build and distribution strategy?
3. How are tool versions pinned per-image?
4. How does `ToolPathResolver` detect whether it is in Docker or PATH mode?
5. What is the fallback if Docker is not available?
6. How are GPU/memory resource limits applied?

See `PLAN.md` §6.0 for the implementation task.

### §6.8 Preflight check contract

`harvest_check_prerequisites()` MCP tool:

Input: none.

Output:
```json
{
  "bucket_a": {"git": true, "enry": true, "scc": true, "rg": true},
  "bucket_b": {"cdxgen": true, "osv-scanner": false},
  "missing_a": [],
  "missing_b": ["osv-scanner"],
  "install_hints": {"osv-scanner": "go install github.com/google/osv-scanner/cmd/osv-scanner@latest"}
}
```

Sub-agent aborts if any `missing_a` entry is present. Proceeds with `missing_b` gaps logged in A14 `coverage_gaps`.

### §6.9 Serena (LSP-over-MCP)

Outcome: optional, not bundled, graceful fallback required.

The `.mcp.json` ships with Serena commented out:
```json
// "serena": {
//   "command": "uvx",
//   "args": ["serena", "--context", "ide-assistant", "--transport", "stdio", "<repo-path>"]
// }
```

Users opt in by uncommenting. Every producer that uses Serena must have a fallback to repomap + ripgrep when Serena is absent.

Per-stage usage:

| Stage | Artifact used first | Then Serena for | What Serena does NOT replace |
|---|---|---|---|
| Stage 1 A2 (entry points) | Static framework extraction | find_symbol to resolve handler symbols static extraction could not pin | The route enumeration itself — Serena cannot enumerate all HTTP routes |
| Stage 1 A9 (sources/sinks, Phase 2) | CPG-derived patterns (A8) + framework rules | find_referencing_symbols to find all call sites of known sources | Categorization of functions as sources/sinks |
| Stage 3 (Code Review) | A7 repomap for orientation, A9 for known sources/sinks | find_referencing_symbols, go_to_definition to trace call chains without reading whole files | A8 CPG taint — Serena gives call edges, CPG gives data-flow edges |
| Stage 4 (Verify) | A8 CPG taint paths, A3 trust boundaries, A2 entry points | find_references + go_to_definition to verify specific symbol claims | Reachability from the taint perspective — use A8; use Serena for call graph confirmation alongside it |

Language server prerequisites: Java 11+ (handled automatically by Serena via Eclipse JDT); Go: `go install golang.org/x/tools/gopls@latest`; Python: `pip install pyright`; JS/TS: `npm install -g typescript-language-server typescript`.

### §6.10 Cross-stage interface contract

The authoritative artifact consumption table (which artifacts are always-on vs. file-read vs. MCP-tool) lives in `PLAN_global.md §Cross-Stage Interface Contract` and in `docs/parent-plugin-conventions.md §6`. It is not duplicated here.

### §6.11 A1 sub-artifact conventions

The following three sub-artifacts are part of the cross-stage contract. Downstream stages depend on these file names and type labels.

| Sub-artifact | File name | Type in index.json |
|---|---|---|
| SCC raw output | `repo_profile_scc.json` | `sub-artifact` |
| Framework detection results | `repo_profile_frameworks.json` | `sub-artifact` |
| Language breakdown | `repo_profile_languages.json` | `sub-artifact` |

These are referenced from A14 `index.json` with `"type": "sub-artifact"`.

---

## §7 Producer catalog (Phase 1 — 7 artifacts)

### §7.1 A1 `repo_profile.md`

Token budget: target ≤2k tokens for the rendered Markdown summary. Raw detail goes into sub-artifacts and is referenced from A14 index — never embedded in A1.

Eight sections with their sources:

1. **Languages** — primary language(s) with LOC percentages and total code-line count. Source: `enry` for classification; `scc` for LOC/comment/blank split and per-language cyclomatic complexity summary. Sub-artifact: `repo_profile_scc.json`.

2. **Frameworks** — web framework(s), task queues, gRPC/RPC layer. Source: SBOM purl registry lookup over the cdxgen SBOM (A5). If A5 is not yet available, fall back to file-existence + import-pattern detection. Sub-artifact: `repo_profile_frameworks.json`.

3. **Build / test tooling** — build tool and test framework(s). Source: file-existence check + cdxgen `type` field + SBOM purl name match against known test-framework registry. Manifest files checked: Makefile, package.json scripts, pyproject.toml, pom.xml, build.gradle, Cargo.toml.

4. **Infrastructure** — databases, caches, message brokers. Source: cdxgen `services[]` field populated from docker-compose/k8s/Skaffold manifests. If `services[]` is empty, omit this section.

5. **Monorepo** — presence of workspace config. Source: file-existence check for pnpm-workspace.yaml, Cargo.toml [workspace], go.work, nx.json.

6. **Entry binaries** — high-level list only; full structured records are in A2. Source: `dockerfile-parse` for Dockerfile CMD/ENTRYPOINT/EXPOSE; `package.json` bin; `pyproject.toml` [project.scripts]; `go.mod` module name + presence of cmd/*/main.go; Cargo.toml [[bin]]; pom.xml mainClass; K8s manifests; serverless configs. Emit: binary name + source file (up to 8 entries).

7. **Manifests** — list of manifest files found with their paths. Source: file-tree walk for well-known names (go.mod, package.json, pom.xml, Cargo.toml, pyproject.toml, requirements.txt, build.gradle, Gemfile, composer.json).

8. **Secret posture** — what secret manager does this project use, if any? Not finding leaked secrets. Source (two-step): (a) SBOM purl match against the secret-manager registry (§7.3); (b) if a match is ambiguous (e.g., boto3 is used for multiple AWS services), run `rg --json` to confirm the SDK is actually imported in source. One ripgrep invocation per matched SDK. Emit: manager name + evidence source (`sbom` | `sbom+ripgrep`).

### §7.2 Framework purl registry

Maintained in `src/audit_harvest/producers/registry/frameworks.yaml`. Human-curated reference data; never LLM-generated.

```yaml
# format: purl_type/namespace_and_name: {label, category}
golang/github.com/gin-gonic/gin:           {label: Gin,         category: web}
golang/github.com/labstack/echo/v4:        {label: Echo,        category: web}
golang/github.com/gofiber/fiber/v2:        {label: Fiber,       category: web}
golang/github.com/gorilla/mux:             {label: Gorilla Mux, category: web}
golang/github.com/go-chi/chi:              {label: Chi,         category: web}
golang/github.com/beego/beego/v2:          {label: Beego,       category: web}
golang/github.com/cloudwego/hertz:         {label: Hertz,       category: web}
golang/google.golang.org/grpc:             {label: gRPC,        category: rpc}
pypi/django:                               {label: Django,      category: web}
pypi/flask:                                {label: Flask,       category: web}
pypi/fastapi:                              {label: FastAPI,     category: web}
pypi/starlette:                            {label: Starlette,   category: asgi}
pypi/tornado:                              {label: Tornado,     category: web}
pypi/aiohttp:                              {label: aiohttp,     category: web}
pypi/sanic:                                {label: Sanic,       category: web}
pypi/celery:                               {label: Celery,      category: task-queue}
npm/express:                               {label: Express,     category: web}
npm/fastify:                               {label: Fastify,     category: web}
npm/koa:                                   {label: Koa,         category: web}
npm/@nestjs/core:                          {label: NestJS,      category: web}
npm/next:                                  {label: Next.js,     category: web}
npm/nuxt:                                  {label: Nuxt,        category: web}
npm/@hapi/hapi:                            {label: Hapi,        category: web}
maven/org.springframework.boot/spring-boot-starter-web: {label: Spring Boot, category: web}
maven/org.springframework/spring-webflux:  {label: Spring WebFlux, category: web-reactive}
maven/io.quarkus/quarkus-core:             {label: Quarkus,     category: web}
maven/io.micronaut/micronaut-core:         {label: Micronaut,   category: web}
maven/io.vertx/vertx-core:                 {label: Vert.x,      category: web}
cargo/actix-web:                           {label: Actix-web,   category: web}
cargo/axum:                                {label: Axum,        category: web}
cargo/rocket:                              {label: Rocket,      category: web}
cargo/warp:                                {label: Warp,        category: web}
cargo/tokio:                               {label: Tokio,       category: runtime}
```

Purl parsing rules:
- `pkg:golang/github.com/gin-gonic/gin@v1.9.1` → type=`golang`, key=`golang/github.com/gin-gonic/gin`
- `pkg:npm/@nestjs/core@10.0.0` → type=`npm`, key=`npm/@nestjs/core` (keep @ prefix)
- `pkg:maven/org.springframework.boot/spring-boot-starter-web@3.2.0` → type=`maven`, key=`maven/org.springframework.boot/spring-boot-starter-web`
- `pkg:pypi/flask@3.0.2` → type=`pypi`, key=`pypi/flask`
- `pkg:cargo/axum@0.8.1` → type=`cargo`, key=`cargo/axum`
- Strip version (`@...`) before matching. Lowercase both key and registry entry.

For JS/TS: if `component.scope == "required"` (cdxgen babel-parser AST result), emit `(imported in source)` alongside the framework name. If scope is absent or not `required`, emit `(declared in manifest)`.

### §7.3 Secret-manager purl registry

Maintained in `src/audit_harvest/producers/registry/secret_managers.yaml`. Human-curated; never LLM-generated.

```yaml
# format: purl_type/name_or_namespace_name: {manager}
pypi/boto3:                                           {manager: aws_secrets_manager, ambiguous: true}
pypi/aioboto3:                                        {manager: aws_secrets_manager, ambiguous: true}
npm/@aws-sdk/client-secrets-manager:                  {manager: aws_secrets_manager}
maven/software.amazon.awssdk/secretsmanager:          {manager: aws_secrets_manager}
golang/github.com/aws/aws-sdk-go-v2/service/secretsmanager: {manager: aws_secrets_manager}
cargo/aws-sdk-secretsmanager:                         {manager: aws_secrets_manager}
pypi/hvac:                                            {manager: hashicorp_vault}
npm/node-vault:                                       {manager: hashicorp_vault}
maven/io.github.jopenlibs/vault-java-driver:          {manager: hashicorp_vault}
golang/github.com/hashicorp/vault/api:                {manager: hashicorp_vault}
cargo/vaultrs:                                        {manager: hashicorp_vault}
pypi/azure-keyvault-secrets:                          {manager: azure_key_vault}
npm/@azure/keyvault-secrets:                          {manager: azure_key_vault}
maven/com.azure/azure-security-keyvault-secrets:      {manager: azure_key_vault}
golang/github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets: {manager: azure_key_vault}
pypi/google-cloud-secret-manager:                     {manager: gcp_secret_manager}
npm/@google-cloud/secret-manager:                     {manager: gcp_secret_manager}
maven/com.google.cloud/google-cloud-secretmanager:    {manager: gcp_secret_manager}
golang/cloud.google.com/go/secretmanager:             {manager: gcp_secret_manager}
pypi/doppler-sdk-python:                              {manager: doppler}
npm/@dopplerhq/node-sdk:                              {manager: doppler}
npm/@1password/sdk:                                   {manager: onepassword}
pypi/onepassword-sdk:                                 {manager: onepassword}
pypi/infisical-python:                                {manager: infisical}
npm/@infisical/sdk:                                   {manager: infisical}
```

Ambiguous-entry confirmation rule: for `ambiguous: true` entries (boto3, aioboto3), after SBOM match run one ripgrep pass: `rg --json "secretsmanager|SecretsManager" --type py <repo>`. If no match: emit `posture: ambiguous — boto3 present but secretsmanager usage not confirmed`. If match: emit `posture: aws_secrets_manager (confirmed via source import)`.

### §7.4 A5 + A6 — SBOM and CVE overlay

A5 mechanism: `cdxgen --output sbom.cdx.json --output-format cyclonedx <repo>` as default. Output is CycloneDX JSON. If a target ecosystem is not covered by cdxgen, `syft` is used as a fallback. For JS/Java/TS, `cdxgen --profile appsec` produces `atom` reachability evidence that is folded into A6.

A6 mechanism: `osv-scanner --format json --sbom sbom.cdx.json`. Output is a CVE findings JSON. Reachability evidence for JS/Java/TS is noted where available from the cdxgen appsec profile. Both producers are non-blocking: if cdxgen or osv-scanner is absent, the gap is recorded in A14 `coverage_gaps` and the run continues.

### §7.5 A7 — repomap

Vendor rationale: Aider's `repomap.py` provides PageRank-weighted code structure graphs. Vendoring (not pip install) avoids upstream breakage and allows tight integration with the `ArtifactStore` cache layer.

Constraints:
- Files to copy: `repomap.py`, `special.py`, tree-sitter `tags.scm` files per language from `aider/queries/`.
- License attribution: Aider is Apache 2.0; `THIRD_PARTY_LICENSES.md` must be kept current.
- ArtifactStore integration: repomap output is written via `store.write()` like any other artifact.
- Do not rewrite the PageRank logic. The vendored algorithm is a known-good implementation; any apparent simplification is wrong.

Token budget (from `agents/harvest-agent.md`):

| Source files | Repomap budget |
|---|---|
| < 200 | 8192 tokens |
| 200-2000 | 4096 tokens |
| 2000-10000 | 2048 tokens |
| > 10000 | 1024 tokens |

MCP exposure: `harvest_repomap_query(package, budget_tokens)` — downstream stages call this, never read the raw repomap file.

### §7.6 A2 — Entry points

Output JSON schema (per `interfaces/harvest-outputs.schema.json`):
```json
{
  "kind": "http|cli|grpc|worker|cron",
  "method": "GET|POST|...|null",
  "path": "/api/v1/...",
  "handler": "HandlerFunctionName",
  "file": "relative/path/to/file.go",
  "line": 42,
  "framework": "gin"
}
```

Per-language extraction mechanism:

| Language | Mechanism |
|---|---|
| Go | tree-sitter patterns for `gin.GET`, `http.HandleFunc`, `chi.Route`, `echo.GET`, `fiber.Get` |
| Python | tree-sitter decorator patterns for `@app.route`, `@router.get`, `@api_view` |
| JS/TS | tree-sitter patterns for `app.get`, `router.get`, `export default function` (Next.js) |
| Java | tree-sitter annotations: `@GetMapping`, `@PostMapping`, `@RequestMapping`, `@RestController` |
| Universal | Dockerfile `ENTRYPOINT`/`CMD`, K8s CronJob `schedule` field |

Phase 2 upgrade: OWASP Noir integration for deeper dynamic dispatch discovery.

Rule: if the handler function cannot be statically resolved (dynamic dispatch), emit `"handler": null` — never guess.

### §7.7 A4 — Gate matrix

Output JSON schema:
```json
{
  "cwe_id": "CWE-89",
  "applicable": true,
  "confidence": "HIGH|MEDIUM|LOW",
  "evidence": ["uses parameterized queries in db/queries.go:42"]
}
```

Detection mechanism:

CWE applicability rules live in `registry/cwe_rules.yaml`. Each entry defines:
- `sbom_signals`: SBOM component substrings to match (e.g., `pg`, `psycopg2`).
- `rg_pattern` / `rg_file_types`: optional ripgrep pattern and file-type filters for source confirmation.
- `no_web_means_false`: when true, the CWE is inapplicable if no web framework was detected in A1.
- `negative_requires_two`: when true, two independent negative signals are required before marking `applicable: false`.

`cwe_loader.py` loads the YAML once at startup (`@lru_cache`) and returns a tuple of immutable `CweRule` dataclasses. The producer iterates all rules through a single generic `_evaluate_rule()` function — no per-CWE branches in the producer code.

`_evaluate_rule()` logic:
1. If `no_web_means_false` and no web framework detected: return `applicable: false`.
2. Check SBOM signals via `_sbom_has_purl_matching()`.
3. If `rg_pattern` defined, confirm via `_rg_match()` (ripgrep with Python fallback).
4. Combine signal evidence into applicability and confidence.

Special cases encoded in the YAML data:
- **CWE-89 (SQL Injection):** SBOM driver match alone returns `needs_verification`; requires both SBOM match AND ripgrep SQL-pattern match for `applicable: true`.
- **CWE-611 (XXE):** no `rg_pattern`; XML library presence alone sets `applicable: true` with MEDIUM confidence.

LLM judgment (via `call_with_grounding`) is used only for cases the deterministic signals cannot resolve.

Thirteen mandatory CWE classes (all must appear in every gate matrix output):
- CWE-22 (Path Traversal)
- CWE-78 (OS Command Injection)
- CWE-79 (XSS)
- CWE-89 (SQL Injection)
- CWE-94 (Code Injection)
- CWE-200 (Information Exposure)
- CWE-287 (Improper Authentication)
- CWE-352 (CSRF)
- CWE-434 (Unrestricted Upload)
- CWE-502 (Deserialization)
- CWE-611 (XXE)
- CWE-798 (Hardcoded Credentials)
- CWE-918 (SSRF)

Rule for `applicable: false`: requires two independent pieces of negative evidence (e.g., no database dependency in SBOM AND no SQL-pattern entry points). One negative signal is not sufficient.

### §7.8 A14 — Index

Output: both a human-readable Markdown table and a machine-readable `index.json` mirror.

`index.json` schema:
```json
{
  "built_from_commit": "<sha>",
  "built_at": "<ISO8601>",
  "artifacts": [
    {
      "artifact_id": "A1",
      "path": "repo_profile/repo_profile.md",
      "type": "artifact|sub-artifact",
      "source_hash": "<sha256>",
      "producer_version": "0.1.0"
    }
  ],
  "coverage_gaps": ["osv-scanner not found — A6 skipped"]
}
```

Staleness threshold: configurable, not hardcoded. Default: 7 days (604800 seconds). Downstream stages check `index.json` age before invoking Stage 1 again.

---

## §8 Producer execution order and MCP surface

### §8.1 Producer dependency graph

```
A5 (sbom) ──────────────┐
                         ├──► A6 (cve_overlay)
                         └──► A1 (repo_profile)
A7 (repomap) ────────────────── (independent)
A2 (entry_points) ──────┐
A5 (sbom) ──────────────┴──► A4 (gate_matrix)
A1, A2, A4, A5, A6, A7 ────► A14 (index)
```

Per-producer MCP tool name:

| Artifact | MCP tool | Dependencies |
|---|---|---|
| A5 | `harvest_run_sbom` | none |
| A6 | `harvest_run_cve_overlay` | A5 |
| A1 | `harvest_run_repo_profile` | A5 |
| A7 | `harvest_run_repomap` | none |
| A2 | `harvest_run_entry_points` | none |
| A4 | `harvest_run_gate_matrix` | A2, A5 |
| A14 | `harvest_run_index` | all above |

Execution order: A5 → A6 → A1 → A7 (parallel with A2) → A2 → A4 → A14.

### §8.2 Full MCP tool surface (Phase 1)

The `audit-harvest-mcp` server exposes:
- `harvest_check_prerequisites` — validate Bucket A and B tools
- `harvest_run_sbom` — produce A5
- `harvest_run_cve_overlay` — produce A6
- `harvest_run_repo_profile` — produce A1
- `harvest_run_repomap` — produce A7
- `harvest_run_entry_points` — produce A2
- `harvest_run_gate_matrix` — produce A4
- `harvest_run_index` — produce A14
- `harvest_index_list` — list all artifacts in the store for a repo
- `harvest_index_get` — get metadata for a specific artifact
- `harvest_repomap_query` — query the repomap graph for a package or file

### §8.3 Sub-agent entry points

- `agents/harvest-agent.md` — the sub-agent orchestrator; invoked by the parent agent when audit context is needed.
- `commands/audit-discover.md` — the `/audit-discover <repo-path>` command; direct human invocation.

---

## §9 Three-tier hand-off model

Three tiers, in order of how downstream stage sub-agents consume artifacts.

### Tier 1 — Always-on bundle (≤5k tokens)

Injected into every downstream sub-agent's system context at startup. Contents:
- A1 `repo_profile.md` summary (≤2k tokens)
- A4 gate matrix (≤1k tokens)
- A14 `index.json` (artifact paths and coverage gaps)
- A12 framework conventions summary (Phase 2 — not in MVP)

Phase 1 note: the `export-always-on` bundle generator is a Phase 2 deliverable. In Phase 1, the orchestrator assembles this bundle manually by reading A1, A4, and A14.

### Tier 2 — Read-on-relevance (file paths)

Sub-agent uses the Read tool on paths from `index.json`:
- A2 `entry_points.json`
- A3 trust boundaries (Phase 2)
- A5 `sbom.json`
- A6 `cve_overlay.json`
- A9 sources/sinks (Phase 2)
- A10 tests inventory (Phase 2)
- A11 git signals (Phase 2)

### Tier 3 — Tool-only MCP surfaces

Sub-agent calls structured query tools, never reads underlying files:
- A7 via `harvest_repomap_query(package, budget_tokens)`
- A8 via `harvest_cpg_taint()`, `harvest_cpg_callgraph()`, etc. (Phase 2)

---

## §10 Failure-mode handling

| Failure mode | Handling |
|---|---|
| Bucket A tool missing | Abort; report specific missing tool and install hint |
| Bucket B tool missing | Log in A14 `coverage_gaps`; continue without that artifact |
| cdxgen produces empty SBOM | Write empty `sbom.json`; note in A14; A6 and A1 proceed with reduced accuracy |
| osv-scanner error | Write failure marker; A6 entry in `coverage_gaps`; A1 continues |
| repomap OOM or timeout | Write partial repomap with truncation note; A7 marked as `degraded` in index |
| repo_profile LLM grounding failures | Drop failed claims to `llm-grounding-failures.jsonl`; write A1 with remaining verified content |
| Catastrophic failure (git not found, not a git repo) | Abort immediately; return structured error to calling agent |

---

## §11 Framework registry maintenance

The framework purl registry (§7.2) and secret-manager purl registry (§7.3) are human-curated reference data.

Maintenance rules:
- Review cadence: quarterly, or when a real audit surfaces an unrecognized framework.
- File locations: `src/audit_harvest/producers/registry/frameworks.yaml` (framework), `src/audit_harvest/producers/registry/secret_managers.yaml` (secret managers).
- Curation policy: human-only; never LLM-generated. Every entry must have a verified purl and at least one real-world usage example.
- Pull request requirement: registry updates require a brief description of the framework/tool and a link to its package registry entry.

### CWE rules registry (`registry/cwe_rules.yaml`)

Same curation discipline as the framework and secret-manager registries:
- Human-curated only; never LLM-generated.
- Review trigger: when an audit surfaces a CWE not covered, or when signal quality (false-positive rate on SBOM or ripgrep patterns) warrants tuning.
- Modifying `sbom_signals` or `rg_pattern` for an existing entry is non-breaking.
- Adding or removing CWE IDs affects what downstream stages see in the always-on bundle — coordinate with Stage 2 owners before removing an entry.
- Boolean flags (`no_web_means_false`, `negative_requires_two`) require a security engineer sign-off before changing; they encode confidence semantics, not just detection logic.
- New entries must document signal quality evidence (true-positive rate on at least one real fixture).
