# repo_profile Degraded Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Return `{"status": "degraded"}` instead of `{"status": "ok"}` when `harvest_run_repo_profile` writes a failure-reason artifact, so the orchestrating agent can detect and react to partial results rather than treating them as success.

**Architecture:** Single-line change in `harvest_run_repo_profile` plus updated test assertions. The degraded artifact is still written (downstream stages get a file at the expected path), but the MCP tool response status now signals the failure explicitly so the calling agent can decide whether to halt, retry, or continue.

**Tech Stack:** Python 3.12, pytest, fastmcp.

---

### Task 1: Change return status to `"degraded"` in `harvest_run_repo_profile`

**Files:**
- Modify: `src/audit_harvest/mcp_server/tools/run_tools.py:83`

Current line 83:
```python
    return {"status": "ok", "artifact": asdict(record), "degraded": True, "reason": reason}
```

- [ ] **Step 1: Apply the change**

Replace line 83 with:
```python
    return {"status": "degraded", "artifact": asdict(record), "reason": reason}
```

The `"degraded": True` key is now redundant -- the status field carries that signal. Remove it to keep the response shape clean.

- [ ] **Step 2: Verify no other callers expect `"ok"` from the degraded path**

```bash
grep -rn '"status".*"ok"' tests/ src/
```

Expected: no test or source file asserts `result["status"] == "ok"` specifically for the repo_profile degraded path (the integration test is updated in Task 2).

- [ ] **Step 3: Commit**

```bash
git add src/audit_harvest/mcp_server/tools/run_tools.py
git commit -m "fix: return degraded status from harvest_run_repo_profile on failure"
```

---

### Task 2: Update integration test to assert `"degraded"` and verify artifact content

**Files:**
- Modify: `tests/integration/test_mcp_pipeline.py:65-68`

Current Step 3 block in `test_full_mcp_tool_pipeline`:
```python
    # Step 3 — A1 repo profile (always written; degraded if tools or SBOM missing)
    result = harvest_run_repo_profile(repo)
    assert result["status"] == "ok", result
    assert "artifact" in result
```

- [ ] **Step 1: Write the failing test first -- run to confirm it fails**

```bash
cd audit-project-context-builder
uv run --project . pytest tests/integration/test_mcp_pipeline.py -v --basetemp=/tmp/fixture-audit -k "go_simple" 2>&1 | tail -15
```

Expected: FAIL because `result["status"]` is now `"degraded"` but the assertion checks `"ok"`.

- [ ] **Step 2: Update the assertions**

Replace the Step 3 block with:
```python
    # Step 3 — A1 repo profile (always written; degraded if tools or SBOM missing)
    result = harvest_run_repo_profile(repo)
    assert result["status"] in ("ok", "degraded"), result
    assert "artifact" in result
    if result["status"] == "degraded":
        assert "reason" in result
        content = Path(result["artifact"]["path"]).read_text()
        assert "Generation failed:" in content
```

Also add `Path` to the imports at the top of the test file if not already present -- it is already imported via `from pathlib import Path` at line 17 (via `_FIXTURES_DIR = Path(...)`). No import change needed.

- [ ] **Step 3: Run all integration tests**

```bash
cd audit-project-context-builder
uv run --project . pytest tests/integration/test_mcp_pipeline.py -v --basetemp=/tmp/fixture-audit 2>&1 | tail -20
```

Expected: 8 passed.

- [ ] **Step 4: Verify degraded artifact content on disk**

```bash
find /tmp/fixture-audit -path "*/repo_profile/*/artifact" | head -4 | xargs -I{} sh -c 'echo "=== {} ==="; cat "{}"'
```

Expected output for each fixture (when enry is absent):
```
=== /tmp/fixture-audit/.../artifact ===
# Repository Profile

Generation failed: Command not found on PATH: enry
```

- [ ] **Step 5: Commit**

```bash
git add tests/integration/test_mcp_pipeline.py
git commit -m "test: assert degraded status and artifact content for repo_profile failures"
```
