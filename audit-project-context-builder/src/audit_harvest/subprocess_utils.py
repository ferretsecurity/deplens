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


class ToolPathResolver:
    """Returns the binary name/path for a tool.

    Maturity Stage 1: returns bare binary name for PATH resolution.
    Maturity Stage 2 (Docker): subclass and override `resolve` to return
    the container-internal path.
    """

    def resolve(self, name: str) -> str:
        return name

    def check(self, name: str) -> dict:
        path = shutil.which(self.resolve(name))
        if path is None:
            return {"status": "missing", "path": None, "version": None}
        try:
            result = subprocess.run(
                [path, "--version"],
                capture_output=True,
                text=True,
                timeout=5,
            )
            version_out = (result.stdout + result.stderr).strip()
            version_line = version_out.splitlines()[0] if version_out else "unknown"
        except Exception:
            version_line = "unknown"
        return {"status": "ok", "path": path, "version": version_line}


_resolver = ToolPathResolver()
