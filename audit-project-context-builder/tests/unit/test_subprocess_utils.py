import json
from pathlib import Path
from unittest.mock import patch

import pytest

from audit_harvest.subprocess_utils import (
    ToolError,
    ToolNotFound,
    ToolPathResolver,
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
    # Malicious argument with shell metacharacters must not expand
    result = run_tool(
        ["echo", "safe; rm -rf /"],
        cwd=tmp_path,
        timeout_sec=5,
    )
    assert "safe; rm -rf /" in result.stdout


def test_wall_time_measured(tmp_path):
    result = run_tool(["echo", "timed"], cwd=tmp_path, timeout_sec=5)
    assert result.wall_time_sec >= 0


def test_resolver_returns_binary_name():
    resolver = ToolPathResolver()
    assert resolver.resolve("cdxgen") == "cdxgen"
    assert resolver.resolve("osv-scanner") == "osv-scanner"


def test_resolver_check_present_tool():
    resolver = ToolPathResolver()
    result = resolver.check("git")
    assert result["status"] == "ok"
    assert result["path"] is not None


def test_resolver_check_missing_tool():
    resolver = ToolPathResolver()
    result = resolver.check("definitely-not-a-real-tool-xyz")
    assert result["status"] == "missing"
    assert result["path"] is None
