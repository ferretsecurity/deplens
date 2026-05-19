from __future__ import annotations

import hashlib
import logging
import tempfile
from pathlib import Path

from audit_harvest.storage import ArtifactRecord, ArtifactStore
from audit_harvest.subprocess_utils import ToolError, _resolver, run_tool

log = logging.getLogger(__name__)


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


def _is_js_ts_or_java(repo_path: Path) -> bool:
    return (
        (repo_path / "package.json").exists()
        or (repo_path / "pom.xml").exists()
        or (repo_path / "build.gradle").exists()
    )


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

        record = store.write("sbom", content, source_hash=src_hash)

        if _is_js_ts_or_java(repo_path):
            _run_appsec_pass(repo_path, store, Path(tmp), src_hash)

    return record


def _run_appsec_pass(
    repo_path: Path, store: ArtifactStore, tmp: Path, src_hash: str
) -> None:
    appsec_path = tmp / "sbom_appsec.cdx.json"
    try:
        run_tool(
            [
                _resolver.resolve("cdxgen"),
                "--profile", "appsec",
                "--output", str(appsec_path),
                "--output-format", "cyclonedx",
                str(repo_path),
            ],
            cwd=repo_path,
            timeout_sec=300,
        )
        store.write("sbom_appsec", appsec_path.read_bytes(), source_hash=src_hash)
    except (ToolError, OSError, Exception) as exc:
        log.warning("appsec SBOM pass failed (non-fatal): %s", exc)
