from __future__ import annotations

import json
from pathlib import Path
from typing import Optional
from urllib.parse import unquote

import yaml

from audit_harvest.storage import ArtifactRecord, ArtifactStore
from audit_harvest.subprocess_utils import run_tool, _resolver


MANIFEST_NAMES = [
    "go.mod", "go.sum", "package.json", "package-lock.json",
    "yarn.lock", "pnpm-lock.yaml", "requirements.txt", "setup.py",
    "setup.cfg", "pyproject.toml", "Pipfile", "pom.xml",
    "build.gradle", "build.gradle.kts", "settings.gradle",
    "Cargo.toml", "Cargo.lock", "Gemfile", "Gemfile.lock",
    "composer.json", "go.work",
]

_REGISTRY_DIR = Path(__file__).parent / "registry"


def _load_registry(name: str) -> dict:
    path = _REGISTRY_DIR / name
    with open(path) as f:
        raw = yaml.safe_load(f)
    return {k.lower(): v for k, v in raw.items()}


def _parse_purl(purl: str) -> tuple[str, str]:
    """Return (purl_type, registry_key) from a purl string.

    Examples:
      pkg:golang/github.com/gin-gonic/gin@v1.9.1 -> ("golang", "golang/github.com/gin-gonic/gin")
      pkg:npm/%40nestjs/core@10.0.0              -> ("npm", "npm/@nestjs/core")
      pkg:pypi/flask@3.0.0                       -> ("pypi", "pypi/flask")
      pkg:maven/org.springframework.boot/spring-boot-starter-web@3.2.0
                                                  -> ("maven", "maven/org.springframework.boot/spring-boot-starter-web")
    """
    rest = purl[4:] if purl.startswith("pkg:") else purl
    type_sep = rest.index("/")
    purl_type = rest[:type_sep]
    remainder = rest[type_sep + 1:]
    at_pos = remainder.rfind("@")
    if at_pos != -1:
        remainder = remainder[:at_pos]
    remainder = unquote(remainder)
    key = f"{purl_type}/{remainder}".lower()
    return purl_type, key


def _detect_frameworks(sbom: dict) -> list[dict]:
    registry = _load_registry("frameworks.yaml")
    detected = []
    for component in sbom.get("components", []):
        purl = component.get("purl", "")
        if not purl:
            continue
        try:
            _, key = _parse_purl(purl)
        except (ValueError, IndexError):
            continue
        if key in registry:
            entry = registry[key]
            detected.append({
                "label": entry["label"],
                "category": entry["category"],
                "version": component.get("version", ""),
                "source": "sbom",
            })
    return detected


def _detect_secret_manager(sbom: dict) -> Optional[dict]:
    registry = _load_registry("secret_managers.yaml")
    for component in sbom.get("components", []):
        purl = component.get("purl", "")
        if not purl:
            continue
        try:
            _, key = _parse_purl(purl)
        except (ValueError, IndexError):
            continue
        if key in registry:
            entry = registry[key]
            return {"manager": entry["manager"], "source": "sbom"}
    return None


def _find_manifests(repo_path: Path) -> list[str]:
    found = []
    for name in MANIFEST_NAMES:
        if (repo_path / name).exists():
            found.append(name)
    return found


def _build_markdown(
    languages: dict,
    scc_data: list,
    frameworks: list[dict],
    manifests: list[str],
    services: list,
    secret_posture: Optional[dict],
) -> str:
    lines = ["# Repository Profile\n"]

    lines.append("## Languages\n")
    total_code = sum(e.get("Code", 0) for e in scc_data)
    for lang, files in languages.items():
        if lang == "Markdown" or lang.startswith("."):
            continue
        lang_loc = next((e.get("Code", 0) for e in scc_data if e.get("Name") == lang), 0)
        pct = f"{lang_loc / total_code * 100:.0f}%" if total_code else "n/a"
        lines.append(f"- **{lang}**: {lang_loc} lines ({pct}), {len(files)} file(s)")
    lines.append("")

    lines.append("## Frameworks\n")
    if frameworks:
        for fw in frameworks:
            version = f" {fw['version']}" if fw.get("version") else ""
            lines.append(f"- **{fw['label']}**{version} ({fw['category']}, source: {fw['source']})")
    else:
        lines.append("- None detected")
    lines.append("")

    lines.append("## Package Manifests\n")
    if manifests:
        for m in manifests:
            lines.append(f"- `{m}`")
    else:
        lines.append("- None found")
    lines.append("")

    if services:
        lines.append("## Infrastructure\n")
        for svc in services:
            name = svc.get("name", "unknown")
            lines.append(f"- {name}")
        lines.append("")

    if secret_posture:
        lines.append("## Secret Management\n")
        lines.append(f"- **{secret_posture['manager']}** (source: {secret_posture['source']})")
        lines.append("")

    return "\n".join(lines)


def produce_repo_profile(
    repo_path: Path,
    store: ArtifactStore,
    sbom_path: Path,
) -> ArtifactRecord:
    if not sbom_path.exists():
        raise ValueError(f"A5 SBOM not found at {sbom_path}. Run harvest_run_sbom first.")

    sbom = json.loads(sbom_path.read_text())

    enry_result = run_tool(
        [_resolver.resolve("enry"), "--json", "--breakdown", str(repo_path)],
        cwd=repo_path,
        timeout_sec=60,
    )
    languages = json.loads(enry_result.stdout)

    scc_result = run_tool(
        [_resolver.resolve("scc"), "--format", "json", str(repo_path)],
        cwd=repo_path,
        timeout_sec=60,
    )
    scc_data = json.loads(scc_result.stdout)

    frameworks = _detect_frameworks(sbom)
    secret_posture = _detect_secret_manager(sbom)
    manifests = _find_manifests(repo_path)
    services = sbom.get("services", [])

    markdown = _build_markdown(languages, scc_data, frameworks, manifests, services, secret_posture)

    src_hash = sbom_path.stat().st_mtime_ns.to_bytes(8, "big").hex()

    record = store.write("repo_profile", markdown.encode(), source_hash=src_hash)

    store.write(
        "repo_profile_scc",
        json.dumps(scc_data).encode(),
        source_hash=src_hash,
    )
    store.write(
        "repo_profile_frameworks",
        json.dumps(frameworks).encode(),
        source_hash=src_hash,
    )
    store.write(
        "repo_profile_languages",
        json.dumps(languages).encode(),
        source_hash=src_hash,
    )

    return record
