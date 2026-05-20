from __future__ import annotations

import hashlib
import json
import warnings
from pathlib import Path

import tiktoken

from audit_harvest.storage import ArtifactRecord, ArtifactStore

_enc = tiktoken.get_encoding("cl100k_base")


EXCLUDE_DIRS = {"vendor", "node_modules", ".git", "testdata", "__pycache__", ".venv"}

# Source file extensions used for hashing staleness
_SOURCE_EXTS = {
    ".py", ".go", ".js", ".mjs", ".ts", ".tsx", ".java",
    ".rb", ".rs", ".c", ".cc", ".cpp", ".cs", ".h",
}


def _source_hash(repo_path: Path) -> str:
    parts = []
    for p in sorted(repo_path.rglob("*")):
        if not p.is_file():
            continue
        if any(part in EXCLUDE_DIRS for part in p.parts):
            continue
        if p.suffix not in _SOURCE_EXTS:
            continue
        stat = p.stat()
        parts.append(f"{p}:{stat.st_mtime}:{stat.st_size}")
    return hashlib.sha256("\n".join(parts).encode()).hexdigest()


def produce_repomap(
    repo_path: Path,
    store: ArtifactStore,
    budget_tokens: int = 8000,
) -> ArtifactRecord:
    src_hash = _source_hash(repo_path)
    if store.is_fresh("repomap", src_hash):
        return store.get("repomap")  # type: ignore[return-value]

    all_files = [
        str(p) for p in sorted(repo_path.rglob("*"))
        if p.is_file()
        and not any(part in EXCLUDE_DIRS for part in p.parts)
    ]

    from audit_harvest.producers.repomap.vendor.repomap import RepoMap
    error_msg = None
    try:
        rm = RepoMap(map_tokens=budget_tokens, root=str(repo_path), main_model=None)
        repo_map_text = rm.get_repo_map(chat_files=[], other_files=all_files)
    except Exception as exc:
        repo_map_text = None
        error_msg = str(exc)
        warnings.warn(f"RepoMap failed: {exc}")

    meta: dict = {
        "repo_path": str(repo_path),
        "budget_tokens": budget_tokens,
        "actual_tokens": len(_enc.encode(repo_map_text)) if repo_map_text else 0,
    }
    if error_msg is not None:
        meta["error"] = error_msg
    output = {
        "meta": meta,
        "repo_map": repo_map_text or "",
    }
    return store.write("repomap", json.dumps(output).encode(), source_hash=src_hash)
