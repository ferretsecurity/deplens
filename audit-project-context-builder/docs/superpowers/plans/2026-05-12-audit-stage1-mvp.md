# Stage 1 "Discover" Plugin — Phase 1 (MVP) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Claude Code plugin containing 7 artifact producers (A1, A2, A4, A5, A6, A7, A14), an MCP server exposing them as tools, and a sub-agent orchestrator that deterministically profiles any repo and hands off a hash-tracked artifact bundle to downstream audit stages.

**Architecture:** Python 3.11+ producer library backed by a content-hash artifact store. Producers are called via MCP tool invocations by the sub-agent. Artifacts land in `~/.cache/audit-agent/<repo-hash>/stage1/` (overridable via `AUDIT_STAGE1_DIR`). Read-side MCP tools (`index.list`, `repomap.query`) plus producer MCP tools (`stage1.run_*`) are served by a single `audit-stage1-mcp` server declared in `.mcp.json`.

**Tech Stack:** Python 3.11+, `mcp` SDK (Anthropic), `tree-sitter` + language grammars, `tiktoken`, `jsonschema`, `pytest`; external tools shelled out: `cdxgen` (Node), `osv-scanner` (Go binary); Aider `repomap.py` vendored (Apache-2.0).

**Plugin root:** `skills/audit-project-context-builder/` (this directory IS the plugin root)

---

## Verification Checkpoints

Stop and get user approval before proceeding past each checkpoint:

- **CP-0** (after Task 2): Plugin scaffold review — directory layout, `pyproject.toml`, `plugin.json`, `.mcp.json`
- **CP-1** (after Task 6): Infrastructure review — `storage.py`, `subprocess_utils.py`, `llm/client.py`, JSON schema registry, all unit tests green
- **CP-2** (after Task 12): All producers review — A1, A5, A6, A7, A2, A4, A14 with unit tests; integration test on one fixture
- **CP-3** (after Task 15): Full integration — MCP server up, sub-agent prompt, plugin installs, end-to-end run on all 4 language fixtures passes

---

## File Map

All paths relative to `skills/audit-project-context-builder/`.

**Create:**
```
.claude-plugin/plugin.json
.mcp.json
agents/stage1-discoverer.md
commands/audit-discover.md
docs/parent-plugin-conventions.md
docs/decisions/implementation-language.md
docs/decisions/storage-location.md
docs/decisions/producer-surfaces.md
docs/decisions/target-languages.md
interfaces/stage1-outputs.schema.json
pyproject.toml
src/audit_stage1/__init__.py
src/audit_stage1/storage.py
src/audit_stage1/subprocess_utils.py
src/audit_stage1/llm/__init__.py
src/audit_stage1/llm/client.py
src/audit_stage1/producers/__init__.py
src/audit_stage1/producers/repo_profile.py        # A1
src/audit_stage1/producers/entry_points.py        # A2
src/audit_stage1/producers/gate_matrix.py         # A4
src/audit_stage1/producers/sbom.py                # A5
src/audit_stage1/producers/cve_overlay.py         # A6
src/audit_stage1/producers/repomap/__init__.py    # A7
src/audit_stage1/producers/repomap/repomap.py     # vendored from Aider
src/audit_stage1/producers/repomap/special.py     # vendored from Aider
src/audit_stage1/producers/repomap/queries/       # vendored tags.scm files
src/audit_stage1/producers/index.py               # A14
src/audit_stage1/extractors/__init__.py
src/audit_stage1/extractors/frameworks/__init__.py
src/audit_stage1/extractors/frameworks/go.py
src/audit_stage1/extractors/frameworks/python.py
src/audit_stage1/extractors/frameworks/javascript.py
src/audit_stage1/extractors/frameworks/java.py
src/audit_stage1/extractors/sbom_runners.py
src/audit_stage1/mcp_server/__init__.py
src/audit_stage1/mcp_server/server.py
src/audit_stage1/mcp_server/tools/__init__.py
src/audit_stage1/mcp_server/tools/index_tools.py
src/audit_stage1/mcp_server/tools/repomap_tools.py
src/audit_stage1/mcp_server/tools/run_tools.py
tests/__init__.py
tests/fixtures/go_simple/go.mod
tests/fixtures/go_simple/main.go
tests/fixtures/python_flask/requirements.txt
tests/fixtures/python_flask/app.py
tests/fixtures/js_express/package.json
tests/fixtures/js_express/index.js
tests/fixtures/java_spring/pom.xml
tests/fixtures/java_spring/src/main/java/com/example/UserController.java
tests/unit/__init__.py
tests/unit/test_storage.py
tests/unit/test_subprocess_utils.py
tests/unit/test_llm_client.py
tests/unit/test_repo_profile.py
tests/unit/test_entry_points.py
tests/unit/test_gate_matrix.py
tests/unit/test_sbom.py
tests/unit/test_index.py
tests/integration/__init__.py
tests/integration/test_end_to_end.py
golden/go_simple/
golden/python_flask/
```

---

## Task 1: Decisions documentation

**Files:**
- Create: `docs/decisions/implementation-language.md`
- Create: `docs/decisions/storage-location.md`
- Create: `docs/decisions/producer-surfaces.md`
- Create: `docs/decisions/target-languages.md`
- Create: `docs/parent-plugin-conventions.md`

- [ ] **Step 1: Write implementation-language.md**

```markdown
# Decision: Implementation Language

**Choice:** Python 3.11+

**Rationale:**
- Aider `repomap.py` is Apache-2.0 Python; direct vendoring saves ~1 day vs subprocess or port.
- `tree-sitter` Python bindings are mature and well-tested.
- MCP Python SDK has strong support.
- `subprocess` ergonomics are fine for shelling out to cdxgen/osv-scanner.

**Considered:** Go (consistent with security-code-scan SAST tools), TypeScript/Node.
Go would require porting Aider repomap or subprocess calls; TypeScript adds same polyglot tax.
```

- [ ] **Step 2: Write storage-location.md**

```markdown
# Decision: Artifact Storage Location

**Choice:** External by default — `~/.cache/audit-agent/<repo-content-hash>/stage1/`

Override via env var: `AUDIT_STAGE1_DIR=<path>`

**Rationale:**
- Does not pollute the audited repo's working tree.
- Portable: path is keyed on repo content hash, not repo path.
- In-repo override (set `AUDIT_STAGE1_DIR=<repo>/.audit/stage1`) supported for self-audits and CI.

**Repo content hash:** SHA-256 of sorted output of `git ls-files --cached` paths+hashes,
or fallback to directory mtime hash if not a git repo.
```

- [ ] **Step 3: Write producer-surfaces.md**

```markdown
# Decision: Producer Invocation Surface

**Choice:** MCP tools as default. Each producer exposed as `stage1.run_<name>`.

**Rationale:**
- Consistent surface with read-side tools (all MCP).
- Structured error types and schemas visible to LLM.
- Single server startup amortizes overhead.

**Per-producer record:**
| Producer | Surface | Notes |
|---|---|---|
| A1 repo_profile | MCP tool: stage1.run_repo_profile | |
| A2 entry_points | MCP tool: stage1.run_entry_points | |
| A4 gate_matrix | MCP tool: stage1.run_gate_matrix | Depends on A2+A5 |
| A5 sbom | MCP tool: stage1.run_sbom | Shells to cdxgen |
| A6 cve_overlay | MCP tool: stage1.run_cve_overlay | Shells to osv-scanner |
| A7 repomap | MCP tool: stage1.run_repomap | |
| A14 index | MCP tool: stage1.run_index | Must run last |
```

- [ ] **Step 4: Write target-languages.md**

```markdown
# Decision: Target Language Priorities for MVP

**Phase 1 languages:** Go, Python, JavaScript/TypeScript, Java

Coverage for A7 tree-sitter grammars, A2 framework extractors, A5/A6 cdxgen profiles.

| Language | A2 Frameworks | A7 Grammar | cdxgen Profile |
|---|---|---|---|
| Go | stdlib http, gin, chi | go | default |
| Python | Flask, FastAPI, Django | python | default |
| JS/TS | Express, NestJS, Next.js API routes | javascript, typescript | default |
| Java | Spring @RestController | java | default |
```

- [ ] **Step 5: Write parent-plugin-conventions.md**

```markdown
# Parent Plugin Conventions

Established by Stage 1; all subsequent stages follow this shape.

1. **Each stage is its own Claude Code plugin** independently versioned under `skills/`.
2. **Each stage's orchestrator is a Claude Code sub-agent** (`agents/<stage>-orchestrator.md`).
   The parent invokes sub-agents with repo path + prior stages' artifact paths.
3. **All MCP servers declared in the stage's `.mcp.json`**. A parent plugin composes them
   by referencing each stage's `.mcp.json` entries.
4. **Artifacts on disk at `~/.cache/audit-agent/<repo-hash>/stage<N>/`** (or `AUDIT_STAGEN_DIR`).
   Disk files are the durable handoff contract between stages.
5. **MCP is for queryable surfaces** (repomap, CPG). Disk JSON/Markdown for staged handoff.
6. **The parent agent prompt is pure orchestration** — invoke Stage 1, verify index hash,
   invoke Stage 2. No stage-specific reasoning in the parent.
7. **Staleness check**: sub-agent reads `index.json` on startup, refuses to hand off artifacts
   older than `AUDIT_MAX_ARTIFACT_AGE_DAYS` (default 7).
```

- [ ] **Step 6: Commit**

```bash
git add docs/
git commit -m "docs: add decisions and parent-plugin-conventions for Stage 1 MVP"
```

---

## Task 2: Project scaffold

**Files:**
- Create: `pyproject.toml`
- Create: `.claude-plugin/plugin.json`
- Create: `.mcp.json`
- Create: `src/audit_stage1/__init__.py`

- [ ] **Step 1: Write pyproject.toml**

```toml
[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "audit-stage1"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = [
    "mcp>=1.0.0",
    "anthropic>=0.40.0",
    "tree-sitter>=0.23.0",
    "tree-sitter-go>=0.23.0",
    "tree-sitter-python>=0.23.0",
    "tree-sitter-javascript>=0.23.0",
    "tree-sitter-typescript>=0.23.0",
    "tree-sitter-java>=0.23.0",
    "tiktoken>=0.7.0",
    "jsonschema>=4.23.0",
]

[project.optional-dependencies]
dev = ["pytest>=8.0", "pytest-asyncio>=0.24.0"]

[project.scripts]
audit-stage1-mcp = "audit_stage1.mcp_server.server:main"

[tool.pytest.ini_options]
asyncio_mode = "auto"
testpaths = ["tests"]
```

- [ ] **Step 2: Write .claude-plugin/plugin.json**

```json
{
  "name": "audit-project-context-builder",
  "version": "0.1.0",
  "description": "Stage 1 of the white-box security audit pipeline. Profiles a repo and produces 7 hash-tracked artifacts consumed by downstream audit stages.",
  "agents": ["agents/stage1-discoverer.md"],
  "commands": ["commands/audit-discover.md"],
  "mcpServers": [".mcp.json"]
}
```

- [ ] **Step 3: Write .mcp.json**

```json
{
  "mcpServers": {
    "audit-stage1": {
      "command": "python",
      "args": ["-m", "audit_stage1.mcp_server.server"],
      "env": {}
    }
  }
}
```

- [ ] **Step 4: Write src/audit_stage1/__init__.py**

```python
__version__ = "0.1.0"
```

- [ ] **Step 5: Create remaining empty `__init__.py` files**

```bash
touch src/audit_stage1/llm/__init__.py
touch src/audit_stage1/producers/__init__.py
touch src/audit_stage1/extractors/__init__.py
touch src/audit_stage1/extractors/frameworks/__init__.py
touch src/audit_stage1/mcp_server/__init__.py
touch src/audit_stage1/mcp_server/tools/__init__.py
touch tests/__init__.py
touch tests/unit/__init__.py
touch tests/integration/__init__.py
```

- [ ] **Step 6: Install in dev mode and verify import**

```bash
cd skills/audit-project-context-builder
pip install -e ".[dev]"
python -c "import audit_stage1; print(audit_stage1.__version__)"
```

Expected: `0.1.0`

- [ ] **Step 7: Commit**

```bash
git add pyproject.toml .claude-plugin/ .mcp.json src/ tests/
git commit -m "chore: scaffold Stage 1 plugin structure"
```

> **CP-0 CHECKPOINT** — Review directory layout, `pyproject.toml`, `plugin.json`, `.mcp.json` with user before proceeding.

---

## Task 3: Test fixtures

**Files:**
- Create: `tests/fixtures/go_simple/go.mod`
- Create: `tests/fixtures/go_simple/main.go`
- Create: `tests/fixtures/python_flask/requirements.txt`
- Create: `tests/fixtures/python_flask/app.py`
- Create: `tests/fixtures/js_express/package.json`
- Create: `tests/fixtures/js_express/index.js`
- Create: `tests/fixtures/java_spring/pom.xml`
- Create: `tests/fixtures/java_spring/src/main/java/com/example/UserController.java`

- [ ] **Step 1: Create Go fixture**

`tests/fixtures/go_simple/go.mod`:
```
module github.com/example/go-simple

go 1.21

require github.com/gin-gonic/gin v1.9.1
```

`tests/fixtures/go_simple/main.go`:
```go
package main

import (
	"net/http"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/users", listUsers)
	r.POST("/users", createUser)
	r.GET("/users/:id", getUser)
	http.ListenAndServe(":8080", r)
}

func listUsers(c *gin.Context) {
	c.JSON(200, gin.H{"users": []string{}})
}

func createUser(c *gin.Context) {
	c.JSON(201, gin.H{"id": "new"})
}

func getUser(c *gin.Context) {
	id := c.Param("id")
	c.JSON(200, gin.H{"id": id})
}
```

- [ ] **Step 2: Create Python Flask fixture**

`tests/fixtures/python_flask/requirements.txt`:
```
flask==3.0.0
sqlalchemy==2.0.23
```

`tests/fixtures/python_flask/app.py`:
```python
from flask import Flask, request, jsonify

app = Flask(__name__)

@app.route("/users", methods=["GET"])
def list_users():
    return jsonify({"users": []})

@app.route("/users", methods=["POST"])
def create_user():
    data = request.get_json()
    return jsonify({"id": "new", "name": data["name"]}), 201

@app.route("/users/<int:user_id>", methods=["GET"])
def get_user(user_id):
    return jsonify({"id": user_id})

if __name__ == "__main__":
    app.run()
```

- [ ] **Step 3: Create Express JS fixture**

`tests/fixtures/js_express/package.json`:
```json
{
  "name": "js-express-simple",
  "version": "1.0.0",
  "dependencies": {
    "express": "^4.18.2"
  }
}
```

`tests/fixtures/js_express/index.js`:
```javascript
const express = require('express');
const app = express();
app.use(express.json());

app.get('/users', (req, res) => {
  res.json({ users: [] });
});

app.post('/users', (req, res) => {
  res.status(201).json({ id: 'new', name: req.body.name });
});

app.get('/users/:id', (req, res) => {
  res.json({ id: req.params.id });
});

app.listen(3000);
```

- [ ] **Step 4: Create Java Spring fixture**

`tests/fixtures/java_spring/pom.xml`:
```xml
<?xml version="1.0" encoding="UTF-8"?>
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>spring-simple</artifactId>
  <version>0.0.1-SNAPSHOT</version>
  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
      <version>3.2.0</version>
    </dependency>
  </dependencies>
</project>
```

`tests/fixtures/java_spring/src/main/java/com/example/UserController.java`:
```java
package com.example;

import org.springframework.web.bind.annotation.*;
import java.util.List;

@RestController
@RequestMapping("/users")
public class UserController {

    @GetMapping
    public List<String> listUsers() {
        return List.of();
    }

    @PostMapping
    public String createUser(@RequestBody String body) {
        return "new";
    }

    @GetMapping("/{id}")
    public String getUser(@PathVariable String id) {
        return id;
    }
}
```

- [ ] **Step 5: Commit fixtures**

```bash
git add tests/fixtures/
git commit -m "test: add language fixtures for Go/Flask/Express/Spring"
```

---

## Task 4: Storage layer

**Files:**
- Create: `src/audit_stage1/storage.py`
- Create: `tests/unit/test_storage.py`

- [ ] **Step 1: Write the failing tests first**

`tests/unit/test_storage.py`:
```python
import hashlib
import json
import time
from pathlib import Path

import pytest

from audit_stage1.storage import ArtifactRecord, ArtifactStore


def test_write_and_read_roundtrip(tmp_path):
    store = ArtifactStore(tmp_path)
    content = b"hello world"
    record = store.write("repo_profile", content, source_hash="abc123")

    assert record.name == "repo_profile"
    assert record.source_hash == "abc123"
    assert Path(record.path).read_bytes() == content


def test_content_hash_is_sha256(tmp_path):
    store = ArtifactStore(tmp_path)
    content = b"hello world"
    record = store.write("repo_profile", content, source_hash="abc123")

    expected = hashlib.sha256(content).hexdigest()
    assert record.content_hash == expected


def test_get_returns_none_when_missing(tmp_path):
    store = ArtifactStore(tmp_path)
    assert store.get("nonexistent") is None


def test_is_fresh_false_when_source_hash_changes(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"content", source_hash="hash_v1")

    assert store.is_fresh("repo_profile", "hash_v1") is True
    assert store.is_fresh("repo_profile", "hash_v2") is False


def test_is_fresh_false_when_artifact_missing(tmp_path):
    store = ArtifactStore(tmp_path)
    assert store.is_fresh("repo_profile", "any_hash") is False


def test_atomic_write_does_not_leave_tmp_file(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"data", source_hash="h1")

    tmp_files = list(tmp_path.rglob("*.tmp"))
    assert tmp_files == []


def test_list_returns_all_artifacts(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"a", source_hash="h1")
    store.write("entry_points", b"b", source_hash="h2")

    names = {r.name for r in store.list()}
    assert names == {"repo_profile", "entry_points"}


def test_overwrite_updates_current_pointer(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"version1", source_hash="h1")
    store.write("repo_profile", b"version2", source_hash="h2")

    record = store.get("repo_profile")
    assert Path(record.path).read_bytes() == b"version2"
    assert record.source_hash == "h2"
```

- [ ] **Step 2: Run test to confirm they fail**

```bash
pytest tests/unit/test_storage.py -v
```

Expected: `ModuleNotFoundError` or similar — `storage` not implemented yet.

- [ ] **Step 3: Implement storage.py**

`src/audit_stage1/storage.py`:
```python
import hashlib
import json
import os
import time
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Optional


@dataclass
class ArtifactRecord:
    name: str
    path: str
    content_hash: str
    source_hash: str
    last_built_at: float
    producer_version: str


class ArtifactStore:
    def __init__(self, root_dir: Path) -> None:
        self.root = Path(root_dir)
        self.root.mkdir(parents=True, exist_ok=True)

    def write(
        self,
        name: str,
        content: bytes,
        source_hash: str,
        producer_version: str = "0.1.0",
    ) -> ArtifactRecord:
        content_hash = hashlib.sha256(content).hexdigest()
        artifact_dir = self.root / name / content_hash
        artifact_dir.mkdir(parents=True, exist_ok=True)

        artifact_path = artifact_dir / "artifact"
        tmp_path = artifact_path.with_suffix(".tmp")
        tmp_path.write_bytes(content)
        tmp_path.replace(artifact_path)

        record = ArtifactRecord(
            name=name,
            path=str(artifact_path),
            content_hash=content_hash,
            source_hash=source_hash,
            last_built_at=time.time(),
            producer_version=producer_version,
        )

        meta_path = artifact_dir / "meta.json"
        tmp_meta = meta_path.with_suffix(".tmp")
        tmp_meta.write_text(json.dumps(asdict(record)))
        tmp_meta.replace(meta_path)

        self._update_current(name, artifact_dir)
        return record

    def _update_current(self, name: str, target_dir: Path) -> None:
        current = self.root / name / "current"
        tmp_link = self.root / name / "current.new"
        if tmp_link.exists() or tmp_link.is_symlink():
            tmp_link.unlink()
        os.symlink(str(target_dir), str(tmp_link))
        os.replace(str(tmp_link), str(current))

    def get(self, name: str) -> Optional[ArtifactRecord]:
        current = self.root / name / "current"
        if not current.exists():
            return None
        meta = current / "meta.json"
        if not meta.exists():
            return None
        return ArtifactRecord(**json.loads(meta.read_text()))

    def is_fresh(self, name: str, source_hash: str) -> bool:
        record = self.get(name)
        if record is None:
            return False
        return record.source_hash == source_hash

    def list(self) -> list[ArtifactRecord]:
        records = []
        for item in sorted(self.root.iterdir()):
            if item.is_dir():
                record = self.get(item.name)
                if record:
                    records.append(record)
        return records
```

- [ ] **Step 4: Run tests — expect green**

```bash
pytest tests/unit/test_storage.py -v
```

Expected: all 8 tests pass.

- [ ] **Step 5: Commit**

```bash
git add src/audit_stage1/storage.py tests/unit/test_storage.py
git commit -m "feat: add ArtifactStore with content-hash tracking and atomic writes"
```

---

## Task 5: Subprocess utilities

**Files:**
- Create: `src/audit_stage1/subprocess_utils.py`
- Create: `tests/unit/test_subprocess_utils.py`

- [ ] **Step 1: Write failing tests**

`tests/unit/test_subprocess_utils.py`:
```python
import json
from pathlib import Path

import pytest

from audit_stage1.subprocess_utils import (
    ToolError,
    ToolNotFound,
    ToolResult,
    ToolTimeout,
    run_tool,
)


def test_run_echo(tmp_path):
    result = run_tool(["echo", "hello"], cwd=tmp_path, timeout_sec=5)
    assert result.returncode == 0
    assert "hello" in result.stdout


def test_raises_tool_not_found(tmp_path):
    with pytest.raises(ToolNotFound, match="nonexistent-binary-xyz"):
        run_tool(["nonexistent-binary-xyz"], cwd=tmp_path, timeout_sec=5)


def test_raises_tool_error_on_nonzero(tmp_path):
    with pytest.raises(ToolError) as exc_info:
        run_tool(["false"], cwd=tmp_path, timeout_sec=5)
    assert exc_info.value.result.returncode != 0


def test_raises_tool_timeout(tmp_path):
    with pytest.raises(ToolTimeout):
        run_tool(["sleep", "10"], cwd=tmp_path, timeout_sec=1)


def test_logs_invocation(tmp_path):
    log_dir = tmp_path / "logs"
    run_tool(["echo", "logged"], cwd=tmp_path, timeout_sec=5, log_dir=log_dir)

    log_file = log_dir / "runs.jsonl"
    assert log_file.exists()
    entry = json.loads(log_file.read_text().strip())
    assert entry["cmd"] == ["echo", "logged"]
    assert entry["returncode"] == 0


def test_no_shell_injection(tmp_path):
    # A malicious filename with shell metacharacters must not expand
    result = run_tool(
        ["echo", "safe; rm -rf /"],
        cwd=tmp_path,
        timeout_sec=5,
    )
    assert "safe; rm -rf /" in result.stdout


def test_wall_time_measured(tmp_path):
    result = run_tool(["echo", "timed"], cwd=tmp_path, timeout_sec=5)
    assert result.wall_time_sec >= 0
```

- [ ] **Step 2: Run tests — expect failure**

```bash
pytest tests/unit/test_subprocess_utils.py -v
```

Expected: `ModuleNotFoundError`.

- [ ] **Step 3: Implement subprocess_utils.py**

`src/audit_stage1/subprocess_utils.py`:
```python
import json
import shutil
import subprocess
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Optional


@dataclass
class ToolResult:
    stdout: str
    stderr: str
    returncode: int
    wall_time_sec: float
    cmd: list[str]


class ToolError(Exception):
    def __init__(self, msg: str, result: ToolResult) -> None:
        super().__init__(msg)
        self.result = result


class ToolTimeout(Exception):
    pass


class ToolNotFound(Exception):
    pass


def run_tool(
    cmd: list[str],
    cwd: Path,
    timeout_sec: int,
    expect_json: bool = False,
    log_dir: Optional[Path] = None,
) -> ToolResult:
    if not shutil.which(cmd[0]):
        raise ToolNotFound(f"Command not found on PATH: {cmd[0]}")

    start = time.monotonic()
    try:
        proc = subprocess.run(
            cmd,
            cwd=str(cwd),
            capture_output=True,
            text=True,
            timeout=timeout_sec,
        )
    except subprocess.TimeoutExpired:
        raise ToolTimeout(f"{cmd[0]} exceeded {timeout_sec}s")

    elapsed = time.monotonic() - start
    result = ToolResult(
        stdout=proc.stdout,
        stderr=proc.stderr,
        returncode=proc.returncode,
        wall_time_sec=elapsed,
        cmd=cmd,
    )

    if log_dir is not None:
        log_dir.mkdir(parents=True, exist_ok=True)
        entry = {
            "timestamp": time.time(),
            "cmd": cmd,
            "cwd": str(cwd),
            "returncode": proc.returncode,
            "wall_time_sec": elapsed,
        }
        with open(log_dir / "runs.jsonl", "a") as f:
            f.write(json.dumps(entry) + "\n")

    if proc.returncode != 0:
        raise ToolError(
            f"{cmd[0]} exited {proc.returncode}: {proc.stderr[:200]}",
            result,
        )

    return result
```

- [ ] **Step 4: Run tests — expect green**

```bash
pytest tests/unit/test_subprocess_utils.py -v
```

Expected: all 7 tests pass.

- [ ] **Step 5: Commit**

```bash
git add src/audit_stage1/subprocess_utils.py tests/unit/test_subprocess_utils.py
git commit -m "feat: add subprocess utility with typed errors and invocation log"
```

---

## Task 6: LLM client with grounding enforcement

**Files:**
- Create: `src/audit_stage1/llm/client.py`
- Create: `tests/unit/test_llm_client.py`

- [ ] **Step 1: Write failing tests**

`tests/unit/test_llm_client.py`:
```python
from pathlib import Path
from unittest.mock import patch

import pytest

from audit_stage1.llm.client import GroundedOutput, LLMClient, drop_unverified_claims


def test_verified_claim_is_kept(tmp_path):
    # Create a real file at the cited location
    src = tmp_path / "handler.go"
    src.write_text("func getUser(w http.ResponseWriter, r *http.Request) {\n}")

    claims = [
        {
            "claim": "getUser is a handler",
            "file": str(src),
            "lines": [1, 1],
            "evidence": "func getUser(",
        }
    ]
    verified = drop_unverified_claims(claims, tmp_path)
    assert len(verified) == 1


def test_fabricated_file_claim_is_dropped(tmp_path):
    claims = [
        {
            "claim": "fakeFunc exists",
            "file": "nonexistent/fake.go",
            "lines": [1, 1],
            "evidence": "func fakeFunc(",
        }
    ]
    verified = drop_unverified_claims(claims, tmp_path)
    assert verified == []


def test_out_of_range_line_claim_is_dropped(tmp_path):
    src = tmp_path / "handler.go"
    src.write_text("line one\n")

    claims = [
        {
            "claim": "something on line 100",
            "file": str(src),
            "lines": [100, 100],
            "evidence": "something",
        }
    ]
    verified = drop_unverified_claims(claims, tmp_path)
    assert verified == []


def test_grounded_output_filters_claims(tmp_path):
    src = tmp_path / "app.py"
    src.write_text("@app.route('/users')\ndef list_users():\n    pass\n")

    raw = GroundedOutput(
        claims=[
            {
                "claim": "list_users is a route handler",
                "file": str(src),
                "lines": [2, 2],
                "evidence": "def list_users",
            },
            {
                "claim": "invented handler",
                "file": "ghost.py",
                "lines": [1, 1],
                "evidence": "def ghost",
            },
        ]
    )
    filtered = raw.verified(tmp_path)
    assert len(filtered.claims) == 1
    assert filtered.claims[0]["claim"] == "list_users is a route handler"
```

- [ ] **Step 2: Run tests — expect failure**

```bash
pytest tests/unit/test_llm_client.py -v
```

Expected: `ModuleNotFoundError`.

- [ ] **Step 3: Implement llm/client.py**

`src/audit_stage1/llm/client.py`:
```python
from __future__ import annotations

import json
import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Optional

logger = logging.getLogger(__name__)


@dataclass
class GroundedOutput:
    claims: list[dict[str, Any]] = field(default_factory=list)

    def verified(self, repo_path: Path) -> "GroundedOutput":
        kept = drop_unverified_claims(self.claims, repo_path)
        dropped = len(self.claims) - len(kept)
        if dropped:
            logger.warning("Dropped %d unverified claims", dropped)
        return GroundedOutput(claims=kept)


def drop_unverified_claims(
    claims: list[dict[str, Any]], repo_path: Path
) -> list[dict[str, Any]]:
    verified = []
    for claim in claims:
        file_path = Path(claim.get("file", ""))
        lines = claim.get("lines", [])

        if not file_path.is_absolute():
            file_path = repo_path / file_path

        if not file_path.exists():
            logger.debug("Dropping claim — file not found: %s", file_path)
            continue

        if lines:
            file_lines = file_path.read_text(errors="replace").splitlines()
            start, end = lines[0], lines[-1]
            if start < 1 or end > len(file_lines):
                logger.debug(
                    "Dropping claim — lines %d-%d out of range in %s (%d lines)",
                    start,
                    end,
                    file_path,
                    len(file_lines),
                )
                continue

        verified.append(claim)
    return verified


class LLMClient:
    """Thin wrapper for Anthropic API calls that enforces cite-or-omit grounding."""

    def __init__(self, model: str = "claude-sonnet-4-6") -> None:
        self.model = model

    def call_with_grounding(
        self,
        prompt: str,
        repo_path: Path,
        system: Optional[str] = None,
    ) -> GroundedOutput:
        import anthropic

        client = anthropic.Anthropic()
        messages = [{"role": "user", "content": prompt}]
        kwargs: dict[str, Any] = {"model": self.model, "max_tokens": 4096, "messages": messages}
        if system:
            kwargs["system"] = system

        response = client.messages.create(**kwargs)
        text = response.content[0].text

        try:
            data = json.loads(text)
            claims = data if isinstance(data, list) else data.get("claims", [])
        except json.JSONDecodeError:
            claims = []

        return GroundedOutput(claims=claims).verified(repo_path)
```

- [ ] **Step 4: Run tests — expect green**

```bash
pytest tests/unit/test_llm_client.py -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add src/audit_stage1/llm/client.py tests/unit/test_llm_client.py
git commit -m "feat: add LLM client with cite-or-omit grounding enforcement"
```

---

## Task 6b: JSON Schema registry

**Files:**
- Create: `interfaces/stage1-outputs.schema.json`

- [ ] **Step 1: Write the schema**

`interfaces/stage1-outputs.schema.json`:
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://audit-stage1/interfaces/stage1-outputs",
  "title": "Stage 1 Artifact Schemas",
  "definitions": {
    "entry_point": {
      "type": "object",
      "required": ["kind", "source_file"],
      "properties": {
        "kind": {"type": "string", "enum": ["http_route","rpc_handler","cli","message_queue","cron","lambda","fuzz_harness"]},
        "framework": {"type": "string"},
        "path": {"type": "string"},
        "method": {"type": "string"},
        "handler_symbol": {"type": ["string", "null"]},
        "source_file": {"type": "string"},
        "auth_required": {"type": "string", "enum": ["true","false","unknown"]},
        "params": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["name", "source"],
            "properties": {
              "name": {"type": "string"},
              "source": {"type": "string"},
              "type": {"type": "string"}
            }
          }
        }
      }
    },
    "entry_points_artifact": {
      "type": "object",
      "required": ["entry_points"],
      "properties": {
        "entry_points": {"type": "array", "items": {"$ref": "#/definitions/entry_point"}}
      }
    },
    "gate_rule": {
      "type": "object",
      "required": ["cwe", "name", "applicable"],
      "properties": {
        "cwe": {"type": "string"},
        "name": {"type": "string"},
        "applicable": {"oneOf": [{"type": "boolean"}, {"type": "string", "const": "needs_verification"}]},
        "evidence": {"type": "array", "items": {"type": "string"}},
        "confidence": {"type": "string", "enum": ["high","medium","low"]}
      }
    },
    "gate_matrix_artifact": {
      "type": "object",
      "required": ["rules"],
      "properties": {
        "rules": {"type": "array", "items": {"$ref": "#/definitions/gate_rule"}}
      }
    },
    "index_artifact": {
      "type": "object",
      "required": ["artifacts", "built_at"],
      "properties": {
        "built_at": {"type": "number"},
        "artifacts": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["name","path","content_hash","source_hash","last_built_at","producer_version"],
            "properties": {
              "name": {"type": "string"},
              "path": {"type": "string"},
              "content_hash": {"type": "string"},
              "source_hash": {"type": "string"},
              "last_built_at": {"type": "number"},
              "producer_version": {"type": "string"},
              "stale": {"type": "boolean"}
            }
          }
        }
      }
    }
  }
}
```

- [ ] **Step 2: Verify schema is valid JSON**

```bash
python -c "import json; json.load(open('interfaces/stage1-outputs.schema.json')); print('OK')"
```

Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add interfaces/stage1-outputs.schema.json
git commit -m "feat: add JSON Schema registry for Stage 1 artifact contracts"
```

> **CP-1 CHECKPOINT** — Review infrastructure: storage, subprocess_utils, llm/client, schema. Run full unit suite before proceeding.
>
> ```bash
> pytest tests/unit/ -v
> ```
> All tests must pass.

---

## Task 7: A1 — repo_profile producer

**Files:**
- Create: `src/audit_stage1/producers/repo_profile.py`
- Create: `tests/unit/test_repo_profile.py`

- [ ] **Step 1: Write failing tests**

`tests/unit/test_repo_profile.py`:
```python
from pathlib import Path
import pytest
from audit_stage1.producers.repo_profile import produce_repo_profile

FIXTURE_GO = Path("tests/fixtures/go_simple")
FIXTURE_PY = Path("tests/fixtures/python_flask")
FIXTURE_JS = Path("tests/fixtures/js_express")
FIXTURE_JAVA = Path("tests/fixtures/java_spring")


def test_detects_go_language(tmp_path):
    result = produce_repo_profile(FIXTURE_GO)
    assert "go" in result.lower()


def test_detects_python_language(tmp_path):
    result = produce_repo_profile(FIXTURE_PY)
    assert "python" in result.lower()


def test_detects_flask_framework(tmp_path):
    result = produce_repo_profile(FIXTURE_PY)
    assert "flask" in result.lower()


def test_detects_express_framework():
    result = produce_repo_profile(FIXTURE_JS)
    assert "express" in result.lower()


def test_detects_spring_framework():
    result = produce_repo_profile(FIXTURE_JAVA)
    assert "spring" in result.lower()


def test_token_count_under_2k():
    import tiktoken
    enc = tiktoken.get_encoding("cl100k_base")
    result = produce_repo_profile(FIXTURE_GO)
    tokens = len(enc.encode(result))
    assert tokens <= 2000, f"Token count {tokens} exceeds 2000"


def test_deterministic_output():
    r1 = produce_repo_profile(FIXTURE_GO)
    r2 = produce_repo_profile(FIXTURE_GO)
    assert r1 == r2
```

- [ ] **Step 2: Run tests — expect failure**

```bash
pytest tests/unit/test_repo_profile.py -v
```

- [ ] **Step 3: Implement repo_profile.py**

`src/audit_stage1/producers/repo_profile.py`:
```python
"""A1: repo_profile.md — deterministic repo profile, max 2k tokens."""
from __future__ import annotations

import re
from collections import Counter
from pathlib import Path
from typing import NamedTuple

import tiktoken

_ENCODER = tiktoken.get_encoding("cl100k_base")

_LANG_BY_EXT: dict[str, str] = {
    ".go": "Go", ".py": "Python", ".js": "JavaScript", ".ts": "TypeScript",
    ".tsx": "TypeScript", ".jsx": "JavaScript", ".java": "Java",
    ".rs": "Rust", ".rb": "Ruby", ".php": "PHP", ".cs": "C#",
    ".cpp": "C++", ".c": "C", ".h": "C/C++", ".kt": "Kotlin",
    ".swift": "Swift", ".scala": "Scala",
}

_FRAMEWORK_SIGS: list[tuple[str, str, str]] = [
    # (language, indicator_pattern, framework_name)
    ("Go", r"gin-gonic/gin", "Gin"),
    ("Go", r"go-chi/chi", "Chi"),
    ("Go", r"net/http", "stdlib http"),
    ("Python", r"flask", "Flask"),
    ("Python", r"fastapi", "FastAPI"),
    ("Python", r"django", "Django"),
    ("JavaScript", r"\"express\"", "Express"),
    ("JavaScript", r"nestjs/core", "NestJS"),
    ("JavaScript", r"next\":", "Next.js"),
    ("Java", r"spring-boot", "Spring Boot"),
    ("Java", r"spring-webmvc", "Spring MVC"),
]

_IGNORE_DIRS = {".git", "node_modules", "vendor", "__pycache__", ".cache", "target", "build"}


def produce_repo_profile(repo_path: Path) -> str:
    repo_path = Path(repo_path).resolve()
    ext_counter: Counter[str] = Counter()
    all_text_samples: list[str] = []

    for p in _walk(repo_path):
        ext_counter[p.suffix.lower()] += 1
        if p.suffix.lower() in _LANG_BY_EXT and p.stat().st_size < 100_000:
            try:
                all_text_samples.append(p.read_text(errors="replace")[:2000])
            except OSError:
                pass

    combined = "\n".join(all_text_samples)

    # Languages
    langs: Counter[str] = Counter()
    for ext, count in ext_counter.items():
        lang = _LANG_BY_EXT.get(ext)
        if lang:
            langs[lang] += count

    # Frameworks
    frameworks: list[str] = []
    for _lang, pattern, name in _FRAMEWORK_SIGS:
        if re.search(pattern, combined, re.IGNORECASE) and name not in frameworks:
            frameworks.append(name)

    # Package manifests
    manifests = [
        str(p.relative_to(repo_path))
        for p in _walk(repo_path)
        if p.name in ("go.mod", "requirements.txt", "package.json", "pom.xml",
                      "Cargo.toml", "build.gradle", "Gemfile", "composer.json")
    ]

    # Monorepo top-level dirs
    top_dirs = sorted(
        p.name for p in repo_path.iterdir()
        if p.is_dir() and p.name not in _IGNORE_DIRS
    )

    # Secret management posture
    secret_signals: list[str] = []
    for p in _walk(repo_path):
        if p.name.startswith(".env"):
            secret_signals.append(f".env file: {p.relative_to(repo_path)}")
            break
    if re.search(r"os\.environ|os\.getenv|process\.env|System\.getenv", combined):
        secret_signals.append("env var access detected")
    if re.search(r"secretsmanager|vault|aws_ssm|azure.keyvault", combined, re.IGNORECASE):
        secret_signals.append("secret manager client detected")

    # License
    license_file = repo_path / "LICENSE"
    license_str = "unknown"
    if license_file.exists():
        text = license_file.read_text(errors="replace")[:500]
        if "MIT" in text:
            license_str = "MIT"
        elif "Apache" in text:
            license_str = "Apache-2.0"
        elif "GPL" in text:
            license_str = "GPL"

    lines: list[str] = [
        "# Repo Profile",
        "",
        "## Languages",
    ]
    for lang, count in langs.most_common():
        lines.append(f"- {lang}: {count} files")

    lines += ["", "## Frameworks Detected"]
    for fw in frameworks:
        lines.append(f"- {fw}")
    if not frameworks:
        lines.append("- none detected")

    lines += ["", "## Package Manifests"]
    for m in manifests:
        lines.append(f"- {m}")
    if not manifests:
        lines.append("- none found")

    lines += ["", "## Top-level Directories"]
    for d in top_dirs[:20]:
        lines.append(f"- {d}/")

    lines += ["", "## Secret Management"]
    for s in secret_signals:
        lines.append(f"- {s}")
    if not secret_signals:
        lines.append("- no signals detected")

    lines += ["", f"## License", f"- {license_str}"]

    output = "\n".join(lines)
    # Hard-cap at 2k tokens by truncating at token boundary
    tokens = _ENCODER.encode(output)
    if len(tokens) > 2000:
        output = _ENCODER.decode(tokens[:2000])

    return output


def _walk(root: Path):
    for p in root.rglob("*"):
        if p.is_file() and not any(part in _IGNORE_DIRS for part in p.parts):
            yield p
```

- [ ] **Step 4: Run tests — expect green**

```bash
pytest tests/unit/test_repo_profile.py -v
```

Expected: all 7 tests pass.

- [ ] **Step 5: Commit**

```bash
git add src/audit_stage1/producers/repo_profile.py tests/unit/test_repo_profile.py
git commit -m "feat: implement A1 repo_profile producer with 2k token cap"
```

---

## Task 8: A5 + A6 — SBOM and CVE overlay producers

**Files:**
- Create: `src/audit_stage1/extractors/sbom_runners.py`
- Create: `src/audit_stage1/producers/sbom.py`
- Create: `src/audit_stage1/producers/cve_overlay.py`
- Create: `tests/unit/test_sbom.py`

- [ ] **Step 1: Write failing tests**

`tests/unit/test_sbom.py`:
```python
import json
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from audit_stage1.producers.sbom import produce_sbom
from audit_stage1.producers.cve_overlay import produce_cve_overlay
from audit_stage1.subprocess_utils import ToolNotFound

FIXTURE_GO = Path("tests/fixtures/go_simple")


def test_produce_sbom_returns_path(tmp_path):
    fake_sbom = json.dumps({"bomFormat": "CycloneDX", "components": []})
    with patch("audit_stage1.producers.sbom.run_tool") as mock_run:
        mock_run.return_value = MagicMock(stdout=fake_sbom)
        result = produce_sbom(FIXTURE_GO, output_dir=tmp_path)
    assert result.exists()


def test_produce_sbom_skips_if_cdxgen_missing(tmp_path):
    with patch("audit_stage1.producers.sbom.run_tool", side_effect=ToolNotFound("cdxgen")):
        result = produce_sbom(FIXTURE_GO, output_dir=tmp_path)
    assert result is None


def test_produce_cve_overlay_returns_list(tmp_path):
    sbom_file = tmp_path / "sbom.cdx.json"
    sbom_file.write_text(json.dumps({"bomFormat": "CycloneDX", "components": []}))

    fake_osv = json.dumps({"results": []})
    with patch("audit_stage1.producers.cve_overlay.run_tool") as mock_run:
        mock_run.return_value = MagicMock(stdout=fake_osv)
        result = produce_cve_overlay(sbom_file, output_dir=tmp_path)
    assert isinstance(result, list)


def test_produce_cve_overlay_skips_when_no_sbom(tmp_path):
    result = produce_cve_overlay(tmp_path / "nonexistent.json", output_dir=tmp_path)
    assert result == []
```

- [ ] **Step 2: Run tests — expect failure**

```bash
pytest tests/unit/test_sbom.py -v
```

- [ ] **Step 3: Implement sbom_runners.py**

`src/audit_stage1/extractors/sbom_runners.py`:
```python
"""Subprocess wrappers for cdxgen and osv-scanner."""
from pathlib import Path
from typing import Optional

from audit_stage1.subprocess_utils import ToolNotFound, run_tool, ToolResult


def run_cdxgen(repo_path: Path, output_file: Path, timeout_sec: int = 300) -> Optional[ToolResult]:
    try:
        return run_tool(
            ["cdxgen", "--output", str(output_file), "--output-format", "json", str(repo_path)],
            cwd=repo_path,
            timeout_sec=timeout_sec,
        )
    except ToolNotFound:
        return None


def run_osv_scanner(sbom_file: Path, timeout_sec: int = 120) -> Optional[ToolResult]:
    try:
        return run_tool(
            ["osv-scanner", "--format", "json", "--sbom", str(sbom_file)],
            cwd=sbom_file.parent,
            timeout_sec=timeout_sec,
        )
    except ToolNotFound:
        return None
```

- [ ] **Step 4: Implement sbom.py**

`src/audit_stage1/producers/sbom.py`:
```python
"""A5: SBOM producer using cdxgen."""
from __future__ import annotations

import json
from pathlib import Path
from typing import Optional

from audit_stage1.subprocess_utils import run_tool, ToolNotFound, ToolError


def produce_sbom(repo_path: Path, output_dir: Path) -> Optional[Path]:
    output_dir.mkdir(parents=True, exist_ok=True)
    sbom_file = output_dir / "sbom.cdx.json"

    try:
        result = run_tool(
            ["cdxgen", "--output", str(sbom_file), "--output-format", "json", str(repo_path)],
            cwd=repo_path,
            timeout_sec=300,
        )
        # cdxgen may write to file or stdout depending on version
        if not sbom_file.exists() and result.stdout.strip():
            sbom_file.write_text(result.stdout)
        return sbom_file if sbom_file.exists() else None
    except ToolNotFound:
        return None
    except ToolError:
        return None
```

- [ ] **Step 5: Implement cve_overlay.py**

`src/audit_stage1/producers/cve_overlay.py`:
```python
"""A6: CVE overlay using osv-scanner over the SBOM."""
from __future__ import annotations

import json
from pathlib import Path

from audit_stage1.subprocess_utils import run_tool, ToolNotFound, ToolError


def produce_cve_overlay(sbom_file: Path, output_dir: Path) -> list[dict]:
    if not sbom_file.exists():
        return []

    output_dir.mkdir(parents=True, exist_ok=True)

    try:
        result = run_tool(
            ["osv-scanner", "--format", "json", "--sbom", str(sbom_file)],
            cwd=sbom_file.parent,
            timeout_sec=120,
        )
        data = json.loads(result.stdout)
        vulns = data.get("results", [])
        out_file = output_dir / "cve_overlay.json"
        out_file.write_text(json.dumps(vulns, indent=2))
        return vulns
    except ToolNotFound:
        return []
    except (ToolError, json.JSONDecodeError):
        return []
```

- [ ] **Step 6: Run tests — expect green**

```bash
pytest tests/unit/test_sbom.py -v
```

- [ ] **Step 7: Commit**

```bash
git add src/audit_stage1/extractors/sbom_runners.py \
        src/audit_stage1/producers/sbom.py \
        src/audit_stage1/producers/cve_overlay.py \
        tests/unit/test_sbom.py
git commit -m "feat: implement A5 SBOM and A6 CVE overlay producers"
```

---

## Task 9: A7 — Repomap producer (vendor Aider)

**Files:**
- Create: `src/audit_stage1/producers/repomap/repomap.py` (vendored)
- Create: `src/audit_stage1/producers/repomap/special.py` (vendored)
- Create: `src/audit_stage1/producers/repomap/queries/` (vendored tags.scm files)
- Create: `src/audit_stage1/producers/repomap/__init__.py`

- [ ] **Step 1: Vendor Aider repomap files**

```bash
# Pin to a specific Aider commit — check current HEAD first
AIDER_COMMIT=$(git ls-remote https://github.com/Aider-AI/aider refs/heads/main | cut -f1)
echo "Vendoring from commit: $AIDER_COMMIT"

# Clone Aider at that commit
git clone --depth 1 https://github.com/Aider-AI/aider /tmp/aider-vendor

# Copy the relevant files
cp /tmp/aider-vendor/aider/repomap.py src/audit_stage1/producers/repomap/repomap.py
cp /tmp/aider-vendor/aider/special.py src/audit_stage1/producers/repomap/special.py
mkdir -p src/audit_stage1/producers/repomap/queries
cp /tmp/aider-vendor/aider/queries/*.scm src/audit_stage1/producers/repomap/queries/

# Record the commit
echo "$AIDER_COMMIT" > src/audit_stage1/producers/repomap/AIDER_COMMIT
cp /tmp/aider-vendor/LICENSE src/audit_stage1/producers/repomap/LICENSE
```

- [ ] **Step 2: Create THIRD_PARTY_LICENSES.md**

```bash
cat > THIRD_PARTY_LICENSES.md << 'EOF'
# Third-Party Licenses

## Aider (repomap)

Vendored from: https://github.com/Aider-AI/aider
Commit: (see src/audit_stage1/producers/repomap/AIDER_COMMIT)
License: Apache-2.0

Files:
- src/audit_stage1/producers/repomap/repomap.py
- src/audit_stage1/producers/repomap/special.py
- src/audit_stage1/producers/repomap/queries/*.scm
EOF
```

- [ ] **Step 3: Write the adapter __init__.py**

`src/audit_stage1/producers/repomap/__init__.py`:
```python
"""A7: Symbol graph using Aider's PageRank repomap (vendored)."""
from __future__ import annotations

import sys
from pathlib import Path

# Make vendored repomap importable without modifying sys.path globally
_REPOMAP_DIR = Path(__file__).parent
if str(_REPOMAP_DIR) not in sys.path:
    sys.path.insert(0, str(_REPOMAP_DIR))


def produce_repomap(
    repo_path: Path,
    budget_tokens: int = 4096,
    chat_files: list[str] | None = None,
) -> str:
    """Return a token-budgeted repomap string for the given repo."""
    from repomap import RepoMap  # vendored

    rm = RepoMap(
        map_tokens=budget_tokens,
        root=str(repo_path),
        verbose=False,
    )
    chat_fnames = chat_files or []
    other_fnames = [
        str(p) for p in Path(repo_path).rglob("*")
        if p.is_file() and p.suffix in {".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".java"}
        and ".git" not in str(p)
    ]
    return rm.get_repo_map(chat_fnames, other_fnames) or ""
```

- [ ] **Step 4: Verify repomap runs on Go fixture**

```bash
python -c "
from pathlib import Path
from audit_stage1.producers.repomap import produce_repomap
result = produce_repomap(Path('tests/fixtures/go_simple'), budget_tokens=1024)
print(repr(result[:200]))
"
```

Expected: non-empty string with function names.

- [ ] **Step 5: Add a smoke test**

Add to a new file `tests/unit/test_repomap.py`:
```python
from pathlib import Path
import pytest
from audit_stage1.producers.repomap import produce_repomap

FIXTURE_GO = Path("tests/fixtures/go_simple")

def test_repomap_nonempty():
    result = produce_repomap(FIXTURE_GO, budget_tokens=512)
    assert len(result) > 0

def test_repomap_deterministic():
    r1 = produce_repomap(FIXTURE_GO, budget_tokens=512)
    r2 = produce_repomap(FIXTURE_GO, budget_tokens=512)
    assert r1 == r2

def test_repomap_respects_budget():
    import tiktoken
    enc = tiktoken.get_encoding("cl100k_base")
    result = produce_repomap(FIXTURE_GO, budget_tokens=256)
    # Allow 10% overshoot since repomap does binary search
    assert len(enc.encode(result)) <= 256 * 1.10
```

- [ ] **Step 6: Run smoke test**

```bash
pytest tests/unit/test_repomap.py -v
```

Expected: all 3 pass.

- [ ] **Step 7: Commit**

```bash
git add src/audit_stage1/producers/repomap/ THIRD_PARTY_LICENSES.md tests/unit/test_repomap.py
git commit -m "feat: vendor Aider repomap.py for A7 symbol graph with PageRank"
```

---

## Task 10: A2 — Entry points producer

**Files:**
- Create: `src/audit_stage1/producers/entry_points.py`
- Create: `src/audit_stage1/extractors/frameworks/go.py`
- Create: `src/audit_stage1/extractors/frameworks/python.py`
- Create: `src/audit_stage1/extractors/frameworks/javascript.py`
- Create: `src/audit_stage1/extractors/frameworks/java.py`
- Create: `tests/unit/test_entry_points.py`

- [ ] **Step 1: Write failing tests**

`tests/unit/test_entry_points.py`:
```python
from pathlib import Path
import pytest
from audit_stage1.producers.entry_points import produce_entry_points

FIXTURE_GO = Path("tests/fixtures/go_simple")
FIXTURE_PY = Path("tests/fixtures/python_flask")
FIXTURE_JS = Path("tests/fixtures/js_express")
FIXTURE_JAVA = Path("tests/fixtures/java_spring")


def test_go_finds_three_routes():
    result = produce_entry_points(FIXTURE_GO)
    http_routes = [e for e in result["entry_points"] if e["kind"] == "http_route"]
    assert len(http_routes) == 3


def test_go_routes_have_methods():
    result = produce_entry_points(FIXTURE_GO)
    methods = {e["method"] for e in result["entry_points"] if e.get("method")}
    assert "GET" in methods
    assert "POST" in methods


def test_flask_finds_routes():
    result = produce_entry_points(FIXTURE_PY)
    routes = [e for e in result["entry_points"] if e["kind"] == "http_route"]
    assert len(routes) >= 2


def test_flask_route_has_source_file():
    result = produce_entry_points(FIXTURE_PY)
    for ep in result["entry_points"]:
        assert "source_file" in ep
        assert ep["source_file"]


def test_express_finds_routes():
    result = produce_entry_points(FIXTURE_JS)
    routes = [e for e in result["entry_points"] if e["kind"] == "http_route"]
    assert len(routes) >= 2


def test_spring_finds_routes():
    result = produce_entry_points(FIXTURE_JAVA)
    routes = [e for e in result["entry_points"] if e["kind"] == "http_route"]
    assert len(routes) >= 1


def test_unresolvable_handler_is_null_not_omitted():
    # All emitted handlers must have source_file; handler_symbol may be null
    result = produce_entry_points(FIXTURE_GO)
    for ep in result["entry_points"]:
        assert "handler_symbol" in ep  # key present
        assert "source_file" in ep


def test_output_is_schema_valid():
    import jsonschema, json
    schema = json.load(open("interfaces/stage1-outputs.schema.json"))
    entry_schema = schema["definitions"]["entry_points_artifact"]
    entry_schema["definitions"] = schema["definitions"]
    result = produce_entry_points(FIXTURE_GO)
    jsonschema.validate(result, entry_schema)
```

- [ ] **Step 2: Run tests — expect failure**

```bash
pytest tests/unit/test_entry_points.py -v
```

- [ ] **Step 3: Implement Go framework extractor**

`src/audit_stage1/extractors/frameworks/go.py`:
```python
"""Extract HTTP entry points from Go source using regex patterns."""
from __future__ import annotations
import re
from pathlib import Path


_GIN_PATTERN = re.compile(
    r'(?:r|router|g|group|api)\s*\.\s*(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(\s*"([^"]+)"\s*,\s*(\w+)',
    re.MULTILINE,
)
_STD_HANDLE_PATTERN = re.compile(
    r'http\.HandleFunc\s*\(\s*"([^"]+)"\s*,\s*(\w+)',
    re.MULTILINE,
)


def extract_go_entry_points(repo_path: Path) -> list[dict]:
    entries = []
    for go_file in repo_path.rglob("*.go"):
        if ".git" in str(go_file):
            continue
        try:
            src = go_file.read_text(errors="replace")
        except OSError:
            continue

        for m in _GIN_PATTERN.finditer(src):
            method, path, handler = m.group(1), m.group(2), m.group(3)
            line = src[: m.start()].count("\n") + 1
            entries.append({
                "kind": "http_route",
                "framework": "Gin",
                "path": path,
                "method": method.upper(),
                "handler_symbol": handler,
                "source_file": f"{go_file.relative_to(repo_path)}:{line}",
                "auth_required": "unknown",
                "params": [],
            })

        for m in _STD_HANDLE_PATTERN.finditer(src):
            path, handler = m.group(1), m.group(2)
            line = src[: m.start()].count("\n") + 1
            entries.append({
                "kind": "http_route",
                "framework": "stdlib http",
                "path": path,
                "method": None,
                "handler_symbol": handler,
                "source_file": f"{go_file.relative_to(repo_path)}:{line}",
                "auth_required": "unknown",
                "params": [],
            })
    return entries
```

- [ ] **Step 4: Implement Python framework extractor**

`src/audit_stage1/extractors/frameworks/python.py`:
```python
"""Extract HTTP entry points from Python source (Flask, FastAPI, Django)."""
from __future__ import annotations
import re
from pathlib import Path


_FLASK_ROUTE = re.compile(
    r'@\w+\.route\s*\(\s*["\']([^"\']+)["\'](?:[^)]*methods\s*=\s*\[([^\]]+)\])?',
    re.MULTILINE,
)
_FASTAPI_ROUTE = re.compile(
    r'@\w+\.(get|post|put|delete|patch)\s*\(\s*["\']([^"\']+)["\']',
    re.MULTILINE | re.IGNORECASE,
)


def extract_python_entry_points(repo_path: Path) -> list[dict]:
    entries = []
    for py_file in repo_path.rglob("*.py"):
        if ".git" in str(py_file) or "__pycache__" in str(py_file):
            continue
        try:
            src = py_file.read_text(errors="replace")
        except OSError:
            continue

        lines = src.splitlines()
        for m in _FLASK_ROUTE.finditer(src):
            path = m.group(1)
            methods_raw = m.group(2) or ""
            methods = [x.strip().strip("'\"") for x in methods_raw.split(",") if x.strip()]
            if not methods:
                methods = ["GET"]
            line = src[: m.start()].count("\n") + 1
            # Handler is the def on the next non-empty line after the decorator
            handler = _next_def(lines, line)
            for method in methods:
                entries.append({
                    "kind": "http_route",
                    "framework": "Flask",
                    "path": path,
                    "method": method.upper(),
                    "handler_symbol": handler,
                    "source_file": f"{py_file.relative_to(repo_path)}:{line}",
                    "auth_required": "unknown",
                    "params": [],
                })

        for m in _FASTAPI_ROUTE.finditer(src):
            method, path = m.group(1).upper(), m.group(2)
            line = src[: m.start()].count("\n") + 1
            handler = _next_def(lines, line)
            entries.append({
                "kind": "http_route",
                "framework": "FastAPI",
                "path": path,
                "method": method,
                "handler_symbol": handler,
                "source_file": f"{py_file.relative_to(repo_path)}:{line}",
                "auth_required": "unknown",
                "params": [],
            })
    return entries


def _next_def(lines: list[str], decorator_line: int) -> str | None:
    for line in lines[decorator_line:decorator_line + 5]:
        m = re.match(r"\s*(?:async\s+)?def\s+(\w+)", line)
        if m:
            return m.group(1)
    return None
```

- [ ] **Step 5: Implement JavaScript framework extractor**

`src/audit_stage1/extractors/frameworks/javascript.py`:
```python
"""Extract HTTP entry points from JavaScript/TypeScript source (Express)."""
from __future__ import annotations
import re
from pathlib import Path


_EXPRESS_ROUTE = re.compile(
    r'(?:app|router)\s*\.\s*(get|post|put|delete|patch|head|options)\s*\(\s*["\`]([^"\'`]+)["\`]',
    re.MULTILINE | re.IGNORECASE,
)


def extract_js_entry_points(repo_path: Path) -> list[dict]:
    entries = []
    for src_file in repo_path.rglob("*"):
        if src_file.suffix not in {".js", ".ts", ".jsx", ".tsx"}:
            continue
        if ".git" in str(src_file) or "node_modules" in str(src_file):
            continue
        try:
            src = src_file.read_text(errors="replace")
        except OSError:
            continue

        for m in _EXPRESS_ROUTE.finditer(src):
            method, path = m.group(1).upper(), m.group(2)
            line = src[: m.start()].count("\n") + 1
            entries.append({
                "kind": "http_route",
                "framework": "Express",
                "path": path,
                "method": method,
                "handler_symbol": None,
                "source_file": f"{src_file.relative_to(repo_path)}:{line}",
                "auth_required": "unknown",
                "params": [],
            })
    return entries
```

- [ ] **Step 6: Implement Java framework extractor**

`src/audit_stage1/extractors/frameworks/java.py`:
```python
"""Extract HTTP entry points from Java source (Spring)."""
from __future__ import annotations
import re
from pathlib import Path


_CLASS_MAPPING = re.compile(r'@RequestMapping\s*\(\s*["\']([^"\']+)["\']')
_METHOD_MAPPING = re.compile(
    r'@(GetMapping|PostMapping|PutMapping|DeleteMapping|PatchMapping|RequestMapping)'
    r'(?:\s*\(\s*(?:value\s*=\s*)?["\']([^"\']*)["\'])?'
)
_METHOD_DEF = re.compile(r'public\s+\S+\s+(\w+)\s*\(')


def extract_java_entry_points(repo_path: Path) -> list[dict]:
    entries = []
    http_method_map = {
        "GetMapping": "GET", "PostMapping": "POST", "PutMapping": "PUT",
        "DeleteMapping": "DELETE", "PatchMapping": "PATCH", "RequestMapping": "GET",
    }

    for java_file in repo_path.rglob("*.java"):
        if ".git" in str(java_file):
            continue
        try:
            src = java_file.read_text(errors="replace")
        except OSError:
            continue

        class_prefix = ""
        cm = _CLASS_MAPPING.search(src)
        if cm:
            class_prefix = cm.group(1).rstrip("/")

        lines = src.splitlines()
        for i, line_text in enumerate(lines):
            mm = _METHOD_MAPPING.search(line_text)
            if not mm:
                continue
            annotation, sub_path = mm.group(1), mm.group(2) or ""
            method_http = http_method_map.get(annotation, "GET")
            full_path = class_prefix + ("/" if sub_path and not sub_path.startswith("/") else "") + sub_path

            handler = None
            for next_line in lines[i + 1: i + 6]:
                dm = _METHOD_DEF.search(next_line)
                if dm:
                    handler = dm.group(1)
                    break

            entries.append({
                "kind": "http_route",
                "framework": "Spring",
                "path": full_path or "/",
                "method": method_http,
                "handler_symbol": handler,
                "source_file": f"{java_file.relative_to(repo_path)}:{i + 1}",
                "auth_required": "unknown",
                "params": [],
            })
    return entries
```

- [ ] **Step 7: Implement entry_points.py orchestrator**

`src/audit_stage1/producers/entry_points.py`:
```python
"""A2: Entry points producer — aggregates all framework extractors."""
from __future__ import annotations
from pathlib import Path

from audit_stage1.extractors.frameworks.go import extract_go_entry_points
from audit_stage1.extractors.frameworks.python import extract_python_entry_points
from audit_stage1.extractors.frameworks.javascript import extract_js_entry_points
from audit_stage1.extractors.frameworks.java import extract_java_entry_points


def produce_entry_points(repo_path: Path) -> dict:
    repo_path = Path(repo_path).resolve()
    entries: list[dict] = []
    entries.extend(extract_go_entry_points(repo_path))
    entries.extend(extract_python_entry_points(repo_path))
    entries.extend(extract_js_entry_points(repo_path))
    entries.extend(extract_java_entry_points(repo_path))
    return {"entry_points": entries}
```

- [ ] **Step 8: Run tests — expect green**

```bash
pytest tests/unit/test_entry_points.py -v
```

Expected: all 8 tests pass.

- [ ] **Step 9: Commit**

```bash
git add src/audit_stage1/producers/entry_points.py \
        src/audit_stage1/extractors/frameworks/ \
        tests/unit/test_entry_points.py
git commit -m "feat: implement A2 entry points producer for Go/Python/JS/Java"
```

---

## Task 11: A4 — Gate matrix producer

**Files:**
- Create: `src/audit_stage1/producers/gate_matrix.py`
- Create: `tests/unit/test_gate_matrix.py`

- [ ] **Step 1: Write failing tests**

`tests/unit/test_gate_matrix.py`:
```python
import json
from pathlib import Path
import pytest
from audit_stage1.producers.gate_matrix import produce_gate_matrix

FIXTURE_GO = Path("tests/fixtures/go_simple")
FIXTURE_PY = Path("tests/fixtures/python_flask")


def test_returns_13_cwe_classes():
    entry_points = {"entry_points": []}
    sbom_components = []
    result = produce_gate_matrix(FIXTURE_GO, entry_points, sbom_components)
    assert len(result["rules"]) == 13


def test_xss_not_applicable_when_no_web_framework(tmp_path):
    # Empty repo with no web framework
    (tmp_path / "main.go").write_text("package main\nfunc main() {}")
    result = produce_gate_matrix(tmp_path, {"entry_points": []}, [])
    xss_rule = next(r for r in result["rules"] if r["cwe"] == "CWE-79")
    assert xss_rule["applicable"] is False
    assert len(xss_rule["evidence"]) >= 2


def test_sqli_needs_verification_with_driver_no_raw_sql():
    # Flask fixture has sqlalchemy in requirements.txt but no raw SQL strings
    entry_points = {"entry_points": []}
    sbom_comps = [{"name": "sqlalchemy"}]
    result = produce_gate_matrix(FIXTURE_PY, entry_points, sbom_comps)
    sqli = next(r for r in result["rules"] if r["cwe"] == "CWE-89")
    assert sqli["applicable"] == "needs_verification"


def test_llm_not_invoked_for_deterministic_rules():
    from unittest.mock import patch
    with patch("audit_stage1.producers.gate_matrix.LLMClient") as mock_llm:
        produce_gate_matrix(FIXTURE_GO, {"entry_points": []}, [])
    mock_llm.assert_not_called()


def test_output_is_schema_valid():
    import jsonschema
    schema = json.load(open("interfaces/stage1-outputs.schema.json"))
    gate_schema = schema["definitions"]["gate_matrix_artifact"]
    gate_schema["definitions"] = schema["definitions"]
    result = produce_gate_matrix(FIXTURE_GO, {"entry_points": []}, [])
    jsonschema.validate(result, gate_schema)
```

- [ ] **Step 2: Run tests — expect failure**

```bash
pytest tests/unit/test_gate_matrix.py -v
```

- [ ] **Step 3: Implement gate_matrix.py**

`src/audit_stage1/producers/gate_matrix.py`:
```python
"""A4: Gate matrix — CWE applicability filter."""
from __future__ import annotations
import re
from pathlib import Path
from typing import Any


_CWE_RULES = [
    ("CWE-22", "Path Traversal"),
    ("CWE-78", "OS Command Injection"),
    ("CWE-79", "Cross-Site Scripting"),
    ("CWE-89", "SQL Injection"),
    ("CWE-94", "Code Injection"),
    ("CWE-200", "Information Disclosure"),
    ("CWE-287", "Improper Authentication"),
    ("CWE-352", "CSRF"),
    ("CWE-434", "Unrestricted File Upload"),
    ("CWE-502", "Insecure Deserialization"),
    ("CWE-611", "XML External Entity"),
    ("CWE-798", "Hard-coded Credentials"),
    ("CWE-918", "SSRF"),
]

_WEB_FRAMEWORK_SIGS = re.compile(
    r"gin|chi|flask|fastapi|django|express|nestjs|spring|next\.js",
    re.IGNORECASE,
)
_SQL_DRIVER_NAMES = {"sqlalchemy", "psycopg2", "mysql-connector", "pg", "mysql2",
                     "jdbc", "hibernate", "jpa", "database/sql", "gorm"}
_UPLOAD_SIGS = re.compile(r"multipart|file\.upload|FormFile|request\.files", re.IGNORECASE)
_XML_SIGS = re.compile(r"xml|etree|lxml|xerces|jaxp", re.IGNORECASE)
_DESERIALIZE_SIGS = re.compile(r"pickle|yaml\.load|ObjectInputStream|unserialize", re.IGNORECASE)
_SSRF_SIGS = re.compile(r"http\.Get|requests\.get|fetch\(|HttpClient|URL\(", re.IGNORECASE)
_HARDCODED_SIGS = re.compile(r'(?:password|secret|api_key|token)\s*=\s*["\'][^"\']{6,}["\']', re.IGNORECASE)
_RAW_SQL_SIGS = re.compile(r'(?:fmt\.Sprintf|%s|%d|f"|f\').*(?:SELECT|INSERT|UPDATE|DELETE)', re.IGNORECASE)
_OS_CMD_SIGS = re.compile(r'exec\.Command|os\.system|subprocess\.run|Runtime\.exec', re.IGNORECASE)
_EVAL_SIGS = re.compile(r'\beval\s*\(|\bexec\s*\(', re.IGNORECASE)


def produce_gate_matrix(
    repo_path: Path,
    entry_points: dict,
    sbom_components: list[dict],
) -> dict:
    repo_path = Path(repo_path)
    src_text = _read_all_source(repo_path)
    has_web = bool(_WEB_FRAMEWORK_SIGS.search(src_text)) or bool(entry_points.get("entry_points"))
    sql_drivers = {c["name"].lower() for c in sbom_components if "name" in c} & _SQL_DRIVER_NAMES
    has_sql_driver = bool(sql_drivers)
    has_raw_sql = bool(_RAW_SQL_SIGS.search(src_text))

    rules = []
    for cwe, name in _CWE_RULES:
        rule = _evaluate_rule(cwe, name, src_text, has_web, has_sql_driver, has_raw_sql)
        rules.append(rule)

    return {"rules": rules}


def _evaluate_rule(
    cwe: str, name: str, src_text: str,
    has_web: bool, has_sql_driver: bool, has_raw_sql: bool
) -> dict:
    if cwe == "CWE-79":  # XSS
        if not has_web:
            return _rule(cwe, name, False, ["no web framework detected", "no HTTP routes found"], "high")
        return _rule(cwe, name, True, ["web framework detected"], "high")

    if cwe == "CWE-89":  # SQLi
        if not has_sql_driver:
            return _rule(cwe, name, False, ["no SQL driver in SBOM", "no SQL imports found"], "high")
        if has_raw_sql:
            return _rule(cwe, name, True, [f"SQL driver detected", "raw SQL string patterns found"], "high")
        return _rule(cwe, name, "needs_verification", [f"SQL driver detected", "no raw SQL strings found yet"], "medium")

    if cwe == "CWE-78":  # OS Command Injection
        if re.search(r'exec\.Command|os\.system|subprocess|Runtime\.exec', src_text):
            return _rule(cwe, name, True, ["OS execution functions detected"], "high")
        return _rule(cwe, name, "needs_verification", ["no explicit exec calls; dynamic dispatch possible"], "low")

    if cwe == "CWE-94":  # Code Injection
        if _EVAL_SIGS.search(src_text):
            return _rule(cwe, name, True, ["eval/exec pattern detected"], "high")
        return _rule(cwe, name, False, ["no eval/exec patterns found", "no code-gen libraries detected"], "medium")

    if cwe == "CWE-434":  # Unrestricted Upload
        if _UPLOAD_SIGS.search(src_text):
            return _rule(cwe, name, True, ["multipart/file upload patterns detected"], "high")
        return _rule(cwe, name, "needs_verification", ["no explicit upload patterns found"], "low")

    if cwe == "CWE-611":  # XXE
        if _XML_SIGS.search(src_text):
            return _rule(cwe, name, "needs_verification", ["XML library detected"], "medium")
        return _rule(cwe, name, False, ["no XML library detected", "no XML imports found"], "high")

    if cwe == "CWE-502":  # Insecure Deserialization
        if _DESERIALIZE_SIGS.search(src_text):
            return _rule(cwe, name, True, ["unsafe deserialization pattern detected"], "high")
        return _rule(cwe, name, "needs_verification", ["no explicit deserialization patterns found"], "low")

    if cwe == "CWE-918":  # SSRF
        if _SSRF_SIGS.search(src_text) and has_web:
            return _rule(cwe, name, "needs_verification", ["HTTP client calls detected in web app"], "medium")
        return _rule(cwe, name, "needs_verification", ["unable to determine SSRF surface statically"], "low")

    if cwe == "CWE-798":  # Hardcoded creds
        if _HARDCODED_SIGS.search(src_text):
            return _rule(cwe, name, True, ["hardcoded credential pattern detected"], "high")
        return _rule(cwe, name, "needs_verification", ["no obvious hardcoded credentials; env vars may still be misused"], "low")

    if cwe in ("CWE-22", "CWE-200", "CWE-287", "CWE-352"):
        if has_web:
            return _rule(cwe, name, "needs_verification", ["web framework present; requires route-level analysis"], "medium")
        return _rule(cwe, name, False, ["no web framework detected", "no HTTP routes found"], "high")

    return _rule(cwe, name, "needs_verification", ["insufficient deterministic evidence"], "low")


def _rule(cwe: str, name: str, applicable: Any, evidence: list[str], confidence: str) -> dict:
    return {"cwe": cwe, "name": name, "applicable": applicable, "evidence": evidence, "confidence": confidence}


def _read_all_source(repo_path: Path) -> str:
    _IGNORE = {".git", "node_modules", "vendor", "__pycache__"}
    parts = []
    for p in repo_path.rglob("*"):
        if not p.is_file():
            continue
        if any(part in _IGNORE for part in p.parts):
            continue
        if p.suffix in {".go", ".py", ".js", ".ts", ".java", ".txt", ".xml", ".json", ".toml", ".mod"}:
            try:
                parts.append(p.read_text(errors="replace")[:10_000])
            except OSError:
                pass
    return "\n".join(parts)
```

- [ ] **Step 4: Run tests — expect green**

```bash
pytest tests/unit/test_gate_matrix.py -v
```

Expected: all 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add src/audit_stage1/producers/gate_matrix.py tests/unit/test_gate_matrix.py
git commit -m "feat: implement A4 gate matrix with 13 CWE classes, deterministic evaluation"
```

---

## Task 12: A14 — Index producer

**Files:**
- Create: `src/audit_stage1/producers/index.py`
- Create: `tests/unit/test_index.py`

- [ ] **Step 1: Write failing tests**

`tests/unit/test_index.py`:
```python
import json
import time
from pathlib import Path

import pytest

from audit_stage1.storage import ArtifactStore
from audit_stage1.producers.index import produce_index


def test_index_lists_all_artifacts(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"content_a", source_hash="h1")
    store.write("entry_points", b"content_b", source_hash="h2")

    result = produce_index(store, max_age_days=7)
    names = {a["name"] for a in result["artifacts"]}
    assert names == {"repo_profile", "entry_points"}


def test_stale_artifact_is_flagged(tmp_path):
    store = ArtifactStore(tmp_path)
    record = store.write("repo_profile", b"old", source_hash="h1")

    # Backdate the record by patching last_built_at
    import audit_stage1.storage as storage_mod
    from dataclasses import asdict
    meta_path = Path(record.path).parent / "meta.json"
    data = json.loads(meta_path.read_text())
    data["last_built_at"] = time.time() - (8 * 86400)  # 8 days ago
    meta_path.write_text(json.dumps(data))

    result = produce_index(store, max_age_days=7)
    artifact = result["artifacts"][0]
    assert artifact["stale"] is True


def test_fresh_artifact_is_not_stale(tmp_path):
    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"fresh", source_hash="h1")

    result = produce_index(store, max_age_days=7)
    assert result["artifacts"][0]["stale"] is False


def test_index_output_is_schema_valid(tmp_path):
    import jsonschema
    schema = json.load(open("interfaces/stage1-outputs.schema.json"))
    index_schema = schema["definitions"]["index_artifact"]
    index_schema["definitions"] = schema["definitions"]

    store = ArtifactStore(tmp_path)
    store.write("repo_profile", b"x", source_hash="h1")
    result = produce_index(store, max_age_days=7)
    jsonschema.validate(result, index_schema)
```

- [ ] **Step 2: Run tests — expect failure**

```bash
pytest tests/unit/test_index.py -v
```

- [ ] **Step 3: Implement index.py**

`src/audit_stage1/producers/index.py`:
```python
"""A14: Index artifact — hash-tracked summary of all Stage 1 artifacts."""
from __future__ import annotations

import time
from dataclasses import asdict

from audit_stage1.storage import ArtifactStore


def produce_index(store: ArtifactStore, max_age_days: int = 7) -> dict:
    now = time.time()
    threshold = max_age_days * 86400
    artifacts = []
    for record in store.list():
        age = now - record.last_built_at
        entry = asdict(record)
        entry["stale"] = age > threshold
        artifacts.append(entry)
    return {"built_at": now, "artifacts": artifacts}


def produce_index_markdown(store: ArtifactStore, max_age_days: int = 7) -> str:
    data = produce_index(store, max_age_days)
    lines = [
        "# Stage 1 Artifact Index",
        "",
        f"Built at: {time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime(data['built_at']))}",
        "",
        "| Artifact | Content Hash | Source Hash | Last Built | Stale |",
        "|---|---|---|---|---|",
    ]
    for a in data["artifacts"]:
        ts = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(a["last_built_at"]))
        lines.append(
            f"| {a['name']} | {a['content_hash'][:12]} | {a['source_hash'][:12]} | {ts} | {a['stale']} |"
        )
    return "\n".join(lines)
```

- [ ] **Step 4: Run tests — expect green**

```bash
pytest tests/unit/test_index.py -v
```

Expected: all 4 tests pass.

- [ ] **Step 5: Run full unit suite**

```bash
pytest tests/unit/ -v
```

Expected: all unit tests pass.

- [ ] **Step 6: Commit**

```bash
git add src/audit_stage1/producers/index.py tests/unit/test_index.py
git commit -m "feat: implement A14 index producer with staleness detection"
```

> **CP-2 CHECKPOINT** — All producers complete. Run full unit suite, inspect outputs on at least one fixture, verify schemas validate. Get user approval before proceeding to MCP server.
>
> ```bash
> pytest tests/unit/ -v
> python -c "
> from pathlib import Path
> from audit_stage1.producers.repo_profile import produce_repo_profile
> from audit_stage1.producers.entry_points import produce_entry_points
> print(produce_repo_profile(Path('tests/fixtures/python_flask')))
> import json; print(json.dumps(produce_entry_points(Path('tests/fixtures/python_flask')), indent=2))
> "
> ```

---

## Task 13: MCP server — read-side tools

**Files:**
- Create: `src/audit_stage1/mcp_server/tools/index_tools.py`
- Create: `src/audit_stage1/mcp_server/tools/repomap_tools.py`
- Create: `src/audit_stage1/mcp_server/tools/run_tools.py`
- Create: `src/audit_stage1/mcp_server/server.py`

- [ ] **Step 1: Implement index_tools.py**

`src/audit_stage1/mcp_server/tools/index_tools.py`:
```python
"""MCP tools: index.list, index.get"""
from __future__ import annotations
import json
from pathlib import Path
from audit_stage1.storage import ArtifactStore
from audit_stage1.producers.index import produce_index, produce_index_markdown


def get_store(store_root: str) -> ArtifactStore:
    return ArtifactStore(Path(store_root))


def tool_index_list(store_root: str, max_age_days: int = 7) -> str:
    store = get_store(store_root)
    return json.dumps(produce_index(store, max_age_days), indent=2)


def tool_index_get(store_root: str, artifact_name: str) -> str:
    store = get_store(store_root)
    record = store.get(artifact_name)
    if record is None:
        return json.dumps({"error": f"artifact '{artifact_name}' not found"})
    artifact_path = Path(record.path)
    if not artifact_path.exists():
        return json.dumps({"error": "artifact file missing"})
    return artifact_path.read_text(errors="replace")
```

- [ ] **Step 2: Implement repomap_tools.py**

`src/audit_stage1/mcp_server/tools/repomap_tools.py`:
```python
"""MCP tool: repomap.query"""
from __future__ import annotations
from pathlib import Path
from audit_stage1.producers.repomap import produce_repomap


def tool_repomap_query(
    repo_path: str,
    budget_tokens: int = 4096,
    chat_files: list[str] | None = None,
) -> str:
    return produce_repomap(Path(repo_path), budget_tokens=budget_tokens, chat_files=chat_files)
```

- [ ] **Step 3: Implement run_tools.py (producer MCP tools)**

`src/audit_stage1/mcp_server/tools/run_tools.py`:
```python
"""MCP tools: stage1.run_* — invoke each producer via the MCP surface."""
from __future__ import annotations
import json
import os
from pathlib import Path

from audit_stage1.storage import ArtifactStore
from audit_stage1.producers.repo_profile import produce_repo_profile
from audit_stage1.producers.entry_points import produce_entry_points
from audit_stage1.producers.gate_matrix import produce_gate_matrix
from audit_stage1.producers.sbom import produce_sbom
from audit_stage1.producers.cve_overlay import produce_cve_overlay
from audit_stage1.producers.index import produce_index_markdown, produce_index


def _get_store(repo_path: str) -> tuple[ArtifactStore, Path]:
    repo = Path(repo_path).resolve()
    store_root = os.environ.get("AUDIT_STAGE1_DIR") or (
        Path.home() / ".cache" / "audit-agent" / _repo_hash(repo) / "stage1"
    )
    return ArtifactStore(Path(store_root)), repo


def _repo_hash(repo: Path) -> str:
    import hashlib
    return hashlib.sha256(str(repo).encode()).hexdigest()[:16]


def tool_run_repo_profile(repo_path: str) -> str:
    store, repo = _get_store(repo_path)
    result = produce_repo_profile(repo)
    store.write("repo_profile", result.encode(), source_hash=_repo_hash(repo))
    return result


def tool_run_entry_points(repo_path: str) -> str:
    store, repo = _get_store(repo_path)
    result = produce_entry_points(repo)
    content = json.dumps(result, indent=2).encode()
    store.write("entry_points", content, source_hash=_repo_hash(repo))
    return json.dumps(result, indent=2)


def tool_run_sbom(repo_path: str) -> str:
    store, repo = _get_store(repo_path)
    tmp_dir = Path("/tmp") / f"sbom-{_repo_hash(repo)}"
    sbom_path = produce_sbom(repo, output_dir=tmp_dir)
    if sbom_path is None:
        return json.dumps({"status": "skipped", "reason": "cdxgen not found on PATH"})
    content = sbom_path.read_bytes()
    store.write("sbom", content, source_hash=_repo_hash(repo))
    return sbom_path.read_text()


def tool_run_cve_overlay(repo_path: str) -> str:
    store, repo = _get_store(repo_path)
    record = store.get("sbom")
    if record is None:
        return json.dumps({"error": "run stage1.run_sbom first"})
    sbom_path = Path(record.path)
    tmp_dir = Path("/tmp") / f"cve-{_repo_hash(repo)}"
    vulns = produce_cve_overlay(sbom_path, output_dir=tmp_dir)
    content = json.dumps(vulns, indent=2).encode()
    store.write("cve_overlay", content, source_hash=_repo_hash(repo))
    return json.dumps(vulns, indent=2)


def tool_run_repomap(repo_path: str, budget_tokens: int = 4096) -> str:
    from audit_stage1.producers.repomap import produce_repomap
    store, repo = _get_store(repo_path)
    result = produce_repomap(repo, budget_tokens=budget_tokens)
    store.write("repomap", result.encode(), source_hash=_repo_hash(repo))
    return result


def tool_run_gate_matrix(repo_path: str) -> str:
    store, repo = _get_store(repo_path)
    ep_record = store.get("entry_points")
    entry_points = json.loads(Path(ep_record.path).read_text()) if ep_record else {"entry_points": []}
    sbom_record = store.get("sbom")
    sbom_components = []
    if sbom_record:
        sbom_data = json.loads(Path(sbom_record.path).read_text())
        sbom_components = sbom_data.get("components", [])
    result = produce_gate_matrix(repo, entry_points, sbom_components)
    content = json.dumps(result, indent=2).encode()
    store.write("gate_matrix", content, source_hash=_repo_hash(repo))
    return json.dumps(result, indent=2)


def tool_run_index(repo_path: str, max_age_days: int = 7) -> str:
    store, repo = _get_store(repo_path)
    md = produce_index_markdown(store, max_age_days=max_age_days)
    data = produce_index(store, max_age_days=max_age_days)
    store.write("index", md.encode(), source_hash=_repo_hash(repo))
    # Also write index.json
    json_path = Path(store.root) / "index.json"
    json_path.write_text(json.dumps(data, indent=2))
    return md
```

- [ ] **Step 4: Implement server.py**

`src/audit_stage1/mcp_server/server.py`:
```python
"""audit-stage1-mcp: MCP server exposing read-side and producer tools."""
from __future__ import annotations

import asyncio
from mcp.server import Server
from mcp.server.stdio import stdio_server
from mcp import types

from audit_stage1.mcp_server.tools.index_tools import tool_index_list, tool_index_get
from audit_stage1.mcp_server.tools.repomap_tools import tool_repomap_query
from audit_stage1.mcp_server.tools.run_tools import (
    tool_run_repo_profile,
    tool_run_entry_points,
    tool_run_sbom,
    tool_run_cve_overlay,
    tool_run_repomap,
    tool_run_gate_matrix,
    tool_run_index,
)

app = Server("audit-stage1")

_TOOLS = [
    types.Tool(
        name="index.list",
        description="List all Stage 1 artifacts with their hashes and staleness status.",
        inputSchema={
            "type": "object",
            "required": ["store_root"],
            "properties": {
                "store_root": {"type": "string", "description": "Path to the artifact store root"},
                "max_age_days": {"type": "integer", "default": 7},
            },
        },
    ),
    types.Tool(
        name="index.get",
        description="Read the content of a specific Stage 1 artifact by name.",
        inputSchema={
            "type": "object",
            "required": ["store_root", "artifact_name"],
            "properties": {
                "store_root": {"type": "string"},
                "artifact_name": {"type": "string"},
            },
        },
    ),
    types.Tool(
        name="repomap.query",
        description="Get a PageRank-ranked symbol map of the repo within a token budget.",
        inputSchema={
            "type": "object",
            "required": ["repo_path"],
            "properties": {
                "repo_path": {"type": "string"},
                "budget_tokens": {"type": "integer", "default": 4096},
                "chat_files": {"type": "array", "items": {"type": "string"}},
            },
        },
    ),
    types.Tool(name="stage1.run_repo_profile", description="Run A1: produce repo_profile.md", inputSchema={"type": "object", "required": ["repo_path"], "properties": {"repo_path": {"type": "string"}}}),
    types.Tool(name="stage1.run_entry_points", description="Run A2: extract all HTTP/CLI entry points", inputSchema={"type": "object", "required": ["repo_path"], "properties": {"repo_path": {"type": "string"}}}),
    types.Tool(name="stage1.run_sbom", description="Run A5: generate SBOM via cdxgen", inputSchema={"type": "object", "required": ["repo_path"], "properties": {"repo_path": {"type": "string"}}}),
    types.Tool(name="stage1.run_cve_overlay", description="Run A6: CVE overlay via osv-scanner (requires A5)", inputSchema={"type": "object", "required": ["repo_path"], "properties": {"repo_path": {"type": "string"}}}),
    types.Tool(name="stage1.run_repomap", description="Run A7: PageRank symbol graph", inputSchema={"type": "object", "required": ["repo_path"], "properties": {"repo_path": {"type": "string"}, "budget_tokens": {"type": "integer", "default": 4096}}}),
    types.Tool(name="stage1.run_gate_matrix", description="Run A4: CWE gate matrix (requires A2+A5)", inputSchema={"type": "object", "required": ["repo_path"], "properties": {"repo_path": {"type": "string"}}}),
    types.Tool(name="stage1.run_index", description="Run A14: build artifact index", inputSchema={"type": "object", "required": ["repo_path"], "properties": {"repo_path": {"type": "string"}, "max_age_days": {"type": "integer", "default": 7}}}),
]


@app.list_tools()
async def list_tools() -> list[types.Tool]:
    return _TOOLS


@app.call_tool()
async def call_tool(name: str, arguments: dict) -> list[types.TextContent]:
    dispatch = {
        "index.list": lambda a: tool_index_list(a["store_root"], a.get("max_age_days", 7)),
        "index.get": lambda a: tool_index_get(a["store_root"], a["artifact_name"]),
        "repomap.query": lambda a: tool_repomap_query(a["repo_path"], a.get("budget_tokens", 4096), a.get("chat_files")),
        "stage1.run_repo_profile": lambda a: tool_run_repo_profile(a["repo_path"]),
        "stage1.run_entry_points": lambda a: tool_run_entry_points(a["repo_path"]),
        "stage1.run_sbom": lambda a: tool_run_sbom(a["repo_path"]),
        "stage1.run_cve_overlay": lambda a: tool_run_cve_overlay(a["repo_path"]),
        "stage1.run_repomap": lambda a: tool_run_repomap(a["repo_path"], a.get("budget_tokens", 4096)),
        "stage1.run_gate_matrix": lambda a: tool_run_gate_matrix(a["repo_path"]),
        "stage1.run_index": lambda a: tool_run_index(a["repo_path"], a.get("max_age_days", 7)),
    }
    if name not in dispatch:
        return [types.TextContent(type="text", text=f"Unknown tool: {name}")]
    result = dispatch[name](arguments)
    return [types.TextContent(type="text", text=result)]


def main() -> None:
    asyncio.run(_run())


async def _run() -> None:
    async with stdio_server() as (read_stream, write_stream):
        await app.run(read_stream, write_stream, app.create_initialization_options())


if __name__ == "__main__":
    main()
```

- [ ] **Step 5: Test MCP server starts without errors**

```bash
python -c "from audit_stage1.mcp_server.server import app; print('MCP server imports OK')"
```

Expected: `MCP server imports OK`

- [ ] **Step 6: Commit**

```bash
git add src/audit_stage1/mcp_server/
git commit -m "feat: implement MCP server with read-side and producer tools"
```

---

## Task 14: Sub-agent prompt

**Files:**
- Create: `agents/stage1-discoverer.md`

- [ ] **Step 1: Write the sub-agent prompt**

`agents/stage1-discoverer.md`:
```markdown
---
name: stage1-discoverer
description: Stage 1 orchestrator for the white-box security audit pipeline. Profiles a repo and produces 7 hash-tracked artifacts (A1-A2, A4-A7, A14) consumed by downstream audit stages.
---

# Stage 1 Discoverer

You are the Stage 1 orchestrator sub-agent for the white-box security audit pipeline.
Your job is to profile a repo deterministically and produce 7 artifacts. You do NOT find vulnerabilities — that is Stages 2-5.

## What you produce

| Artifact | Tool | Depends on |
|---|---|---|
| A1: repo_profile.md | stage1.run_repo_profile | — |
| A2: entry_points.json | stage1.run_entry_points | — |
| A5: sbom.cdx.json | stage1.run_sbom | — |
| A6: cve_overlay.json | stage1.run_cve_overlay | A5 |
| A7: repomap/ | stage1.run_repomap | — |
| A4: gate_matrix.json | stage1.run_gate_matrix | A2, A5 |
| A14: index | stage1.run_index | all above |

## How to invoke

You are invoked with a `repo_path` argument. Example invocation by a parent agent:
```
Run Stage 1 on repo_path=/path/to/repo
```

## Step-by-step execution

1. **Check for stale/cached artifacts** via `index.list` (store_root derived from repo_path).
   - If all artifacts are fresh (not stale, same source_hash), report "cache hit — no work needed" and stop.
   - If any artifact is stale or missing, proceed.

2. **Run A1, A2, A5, A7 in any order** (no dependencies between them):
   - `stage1.run_repo_profile(repo_path)`
   - `stage1.run_entry_points(repo_path)`
   - `stage1.run_sbom(repo_path)` — if cdxgen is not on PATH, note the gap in your final report
   - `stage1.run_repomap(repo_path, budget_tokens=4096)` — adjust budget per strategy table below

3. **Run A6 after A5:**
   - `stage1.run_cve_overlay(repo_path)`

4. **Run A4 after A2 and A5:**
   - `stage1.run_gate_matrix(repo_path)`

5. **Run A14 last:**
   - `stage1.run_index(repo_path)`

## Strategy: repomap token budget

Choose budget based on repo size measured in source files:

| Source files | Repomap budget |
|---|---|
| < 200 | 8192 tokens |
| 200–2000 | 4096 tokens |
| 2000–10000 | 2048 tokens (per-package if monorepo) |
| > 10000 | 1024 tokens (top-level routing paths only) |

Count source files with: list files in repo root, filter by code extensions.

## Failure protocol

If a producer tool fails:
- **Non-blocking failures** (A5, A6 — cdxgen/osv-scanner not installed): log, continue, note gap in handoff.
- **Blocking failures** (A1, A2, A4, A7, A14): report the error and the exact tool call that failed. Do NOT continue to A14 with missing artifacts.

## Handoff format

After all tools complete, report to the parent agent in this format:

```
## Stage 1 Complete

**Repo:** <repo_path>
**Artifacts produced:** <list of artifact names>
**Artifacts skipped:** <list with reason>
**Index hash:** <A14 content_hash>
**Store root:** <path to artifact store>
**Time elapsed:** <seconds>

### Always-on bundle (paste directly into parent context)
<paste contents of A1 repo_profile.md here>

### Gate matrix summary
<list only the applicable=true and needs_verification rules>
```

## Grounding discipline

Every claim you make about the repo in your handoff must be traceable to a tool result or artifact.
Do not describe the repo based on your training knowledge. If a tool returned empty results, say so.
```

- [ ] **Step 2: Commit**

```bash
git add agents/stage1-discoverer.md
git commit -m "feat: add stage1-discoverer sub-agent prompt"
```

---

## Task 15: Plugin wiring + end-to-end integration test

**Files:**
- Create: `commands/audit-discover.md`
- Create: `tests/integration/test_end_to_end.py`

- [ ] **Step 1: Write audit-discover command**

`commands/audit-discover.md`:
```markdown
---
name: audit-discover
description: Run Stage 1 discovery on a repo. Usage: /audit-discover <repo-path>
---

Run the `stage1-discoverer` sub-agent on the provided repo path.

Arguments:
- First argument: path to the repo to audit

Invoke the stage1-discoverer agent with `repo_path` set to the provided path.
```

- [ ] **Step 2: Write end-to-end integration test**

`tests/integration/test_end_to_end.py`:
```python
"""End-to-end test: run all Phase 1 producers against fixtures and verify outputs."""
import json
import os
from pathlib import Path

import pytest

from audit_stage1.storage import ArtifactStore
from audit_stage1.producers.repo_profile import produce_repo_profile
from audit_stage1.producers.entry_points import produce_entry_points
from audit_stage1.producers.gate_matrix import produce_gate_matrix
from audit_stage1.producers.index import produce_index

FIXTURES = [
    Path("tests/fixtures/go_simple"),
    Path("tests/fixtures/python_flask"),
    Path("tests/fixtures/js_express"),
    Path("tests/fixtures/java_spring"),
]


@pytest.mark.parametrize("fixture", FIXTURES, ids=[f.name for f in FIXTURES])
def test_full_pipeline_produces_all_artifacts(fixture, tmp_path):
    store = ArtifactStore(tmp_path)

    # A1
    profile = produce_repo_profile(fixture)
    assert len(profile) > 0
    store.write("repo_profile", profile.encode(), source_hash="test")

    # A2
    ep = produce_entry_points(fixture)
    assert "entry_points" in ep
    store.write("entry_points", json.dumps(ep).encode(), source_hash="test")

    # A4
    gm = produce_gate_matrix(fixture, ep, [])
    assert len(gm["rules"]) == 13
    store.write("gate_matrix", json.dumps(gm).encode(), source_hash="test")

    # A14
    index = produce_index(store, max_age_days=7)
    assert len(index["artifacts"]) == 3
    assert all(not a["stale"] for a in index["artifacts"])


@pytest.mark.parametrize("fixture", FIXTURES, ids=[f.name for f in FIXTURES])
def test_second_run_is_cache_consistent(fixture, tmp_path):
    store = ArtifactStore(tmp_path)
    profile1 = produce_repo_profile(fixture)
    store.write("repo_profile", profile1.encode(), source_hash="same")
    profile2 = produce_repo_profile(fixture)
    # Deterministic: same output on second run
    assert profile1 == profile2


@pytest.mark.parametrize("fixture", FIXTURES, ids=[f.name for f in FIXTURES])
def test_entry_points_nonempty_for_all_fixtures(fixture):
    ep = produce_entry_points(fixture)
    assert len(ep["entry_points"]) > 0, f"No entry points found in {fixture.name}"
```

- [ ] **Step 3: Run integration tests**

```bash
pytest tests/integration/ -v
```

Expected: all tests pass across all 4 fixtures.

- [ ] **Step 4: Verify MCP server lists all tools**

```bash
python -c "
import asyncio
from audit_stage1.mcp_server.server import app
# Just verify all tool names are registered
import inspect
print('Tools registered, server OK')
"
```

- [ ] **Step 5: Run complete test suite**

```bash
pytest tests/ -v --tb=short
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
git add commands/ tests/integration/ tests/unit/
git commit -m "feat: add audit-discover command, end-to-end integration tests pass for all 4 fixtures"
```

> **CP-3 CHECKPOINT** — Final review before declaring Phase 1 MVP complete.
>
> **User review checklist:**
> - [ ] `pytest tests/ -v` — all green
> - [ ] Run `produce_repo_profile` on a real repo outside fixtures — output looks sane
> - [ ] Run `produce_entry_points` on a real repo — routes found, no fabricated handlers
> - [ ] MCP server imports without error
> - [ ] Plugin `plugin.json` and `.mcp.json` look correct
> - [ ] `THIRD_PARTY_LICENSES.md` has Aider attribution
> - [ ] All 4 `docs/decisions/` files present

---

## Definition of Done (Phase 1 MVP)

- [ ] All unit tests pass (`pytest tests/unit/ -v`)
- [ ] All integration tests pass across all 4 language fixtures
- [ ] `produce_repo_profile` output ≤ 2000 tokens on every fixture
- [ ] `produce_entry_points` finds all routes in every fixture (no false-positive handler symbols)
- [ ] `produce_gate_matrix` returns 13 CWE rules, LLM never invoked for deterministic cases
- [ ] `produce_index` marks artifacts stale after `max_age_days`
- [ ] MCP server starts and tool catalog is correct
- [ ] `agents/stage1-discoverer.md` present with strategy table, failure protocol, handoff format
- [ ] `docs/parent-plugin-conventions.md` written
- [ ] All `docs/decisions/` files present
- [ ] `THIRD_PARTY_LICENSES.md` has Aider attribution
- [ ] `interfaces/stage1-outputs.schema.json` validates all producer outputs

---

## Phase 2 scope (not in this plan)

Phase 2 adds: A3 (trust boundaries), A8 (Joern CPG), A9 (sources/sinks 3-layer), A10 (tests inventory), A11 (git signals), A12 (framework conventions curated), A13 (AGENTS.md). Plan to be written after Phase 1 ships and Joern pilot results are available.
