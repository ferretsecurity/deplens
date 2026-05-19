from __future__ import annotations

import hashlib
import json
import tempfile
from pathlib import Path

from audit_harvest.storage import ArtifactRecord, ArtifactStore
from audit_harvest.subprocess_utils import _resolver, run_tool


def _source_hash(repo_path: Path) -> str:
    manifest_files = [
        "go.mod", "go.sum",
        "package.json", "package-lock.json",
        "requirements.txt",
        "pom.xml", "build.gradle",
        "Cargo.toml",
    ]
    h = hashlib.sha256()
    for name in manifest_files:
        p = repo_path / name
        if p.exists():
            h.update(p.read_bytes())
    return h.hexdigest()[:16]


def produce_sbom(repo_path: Path, store: ArtifactStore) -> ArtifactRecord:
    src_hash = _source_hash(repo_path)
    if store.is_fresh("sbom", src_hash):
        return store.get("sbom")

    with tempfile.TemporaryDirectory() as tmp:
        out_path = Path(tmp) / "sbom.cdx.json"
        run_tool(
            [
                _resolver.resolve("cdxgen"),
                "--output", str(out_path),
                "--output-format", "cyclonedx",
                str(repo_path),
            ],
            cwd=repo_path,
            timeout_sec=300,
        )
        content = out_path.read_bytes()

    return store.write("sbom", content, source_hash=src_hash)
