from __future__ import annotations

import json
import re
from pathlib import Path
from urllib.parse import unquote

import yaml

from audit_harvest.constants import MANIFEST_NAMES
from audit_harvest.storage import ArtifactRecord, ArtifactStore
from audit_harvest.subprocess_utils import run_tool, _resolver, ToolError, ToolNotFound

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


def _detect_secret_manager(sbom: dict) -> dict | None:
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


_BUILD_FILES = [
    ("Makefile", "make"),
    ("justfile", "just"),
    ("Taskfile.yaml", "task"),
    ("Jenkinsfile", "jenkins"),
    (".gitlab-ci.yml", "gitlab-ci"),
]
_MAKEFILE_TARGET_RE = re.compile(r"^([a-zA-Z_][a-zA-Z0-9_-]*):")


def _detect_build_system(repo_path: Path) -> list[dict]:
    results: list[dict] = []
    for filename, tool_name in _BUILD_FILES:
        path = repo_path / filename
        if not path.exists():
            continue
        if tool_name == "make":
            targets: list[str] = []
            for line in path.read_text(errors="replace").splitlines():
                if line.startswith(".PHONY"):
                    continue
                m = _MAKEFILE_TARGET_RE.match(line)
                if m:
                    targets.append(m.group(1))
            results.append({"tool": tool_name, "targets": targets[:20]})
        else:
            results.append({"tool": tool_name})
    workflows_dir = repo_path / ".github" / "workflows"
    if workflows_dir.is_dir():
        results.append({"tool": "github-actions"})
    return results


_MONOREPO_MANIFESTS = {"go.mod", "package.json", "pom.xml", "pyproject.toml", "Cargo.toml"}
_MONOREPO_EXCLUDES = {"node_modules", "vendor", ".git", "testdata", ".venv"}


def _detect_monorepo(repo_path: Path) -> list[dict]:
    results: list[dict] = []
    for manifest_name in _MONOREPO_MANIFESTS:
        for found in repo_path.rglob(manifest_name):
            if found.parent == repo_path:
                continue
            if any(part in _MONOREPO_EXCLUDES for part in found.parts):
                continue
            rel = found.relative_to(repo_path)
            results.append({"path": str(rel), "type": manifest_name})
            if len(results) >= 20:
                return results
    return results


def _rg_files(repo_path: Path, glob: str, pattern: str) -> list[str]:
    """Return relative paths of files matching pattern. Returns [] if rg absent or finds nothing."""
    try:
        result = run_tool(
            [_resolver.resolve("rg"), "--files-with-matches", "--glob", glob, pattern, str(repo_path)],
            cwd=repo_path,
            timeout_sec=30,
        )
        return [
            str(Path(line).relative_to(repo_path))
            for line in result.stdout.splitlines()
            if line.strip()
        ]
    except (ToolNotFound, ToolError):
        return []


def _detect_entry_binaries(repo_path: Path) -> list[dict]:
    results: list[dict] = []

    # Go: cmd/*/main.go and root main.go
    for go_main in repo_path.glob("cmd/*/main.go"):
        results.append({"file": str(go_main.relative_to(repo_path)), "language": "Go"})
    if (repo_path / "main.go").exists():
        results.append({"file": "main.go", "language": "Go"})

    # Python: rg for __main__
    if len(results) < 10:
        for path in _rg_files(repo_path, "*.py", r"if __name__ == ['\"]__main__['\"]"):
            results.append({"file": path, "language": "Python"})
            if len(results) >= 10:
                break

    # JS/TS: package.json main/bin
    pkg_json = repo_path / "package.json"
    if pkg_json.exists() and len(results) < 10:
        try:
            pkg = json.loads(pkg_json.read_text())
            if "main" in pkg:
                results.append({"file": pkg["main"], "language": "JavaScript"})
            if "bin" in pkg and isinstance(pkg["bin"], dict):
                for _name, bin_path in pkg["bin"].items():
                    results.append({"file": bin_path, "language": "JavaScript"})
                    if len(results) >= 10:
                        break
        except (json.JSONDecodeError, OSError):
            pass

    # Java: rg for main method
    if len(results) < 10:
        for path in _rg_files(repo_path, "*.java", "public static void main"):
            results.append({"file": path, "language": "Java"})
            if len(results) >= 10:
                break

    return results[:10]


_SECRET_PATTERNS = [
    (r"""(?i)(password|passwd|secret|api_key|apikey|token|private_key)\s*[:=]\s*["'][^"']{8,}""",
     "credential-pattern"),
    (r"AKIA[0-9A-Z]{16}", "aws-key"),
    (r"-----BEGIN (RSA|EC|OPENSSH) PRIVATE KEY-----", "private-key"),
]
_SECRET_EXCLUDES = ["!.git", "!vendor", "!node_modules", "!*.lock", "!testdata"]


def _scan_secrets(repo_path: Path) -> list[dict]:
    cmd = ["rg", "--json"]
    for pattern, _ in _SECRET_PATTERNS:
        cmd += ["-e", pattern]
    for exc in _SECRET_EXCLUDES:
        cmd += ["--glob", exc]

    try:
        result = run_tool(cmd, cwd=repo_path, timeout_sec=60)
        output = result.stdout
    except ToolNotFound:
        return []
    except Exception:
        # rg exits non-zero when no matches found; handle gracefully
        return []

    hits: list[dict] = []
    for raw_line in output.splitlines():
        if not raw_line.strip():
            continue
        try:
            obj = json.loads(raw_line)
        except json.JSONDecodeError:
            continue
        if obj.get("type") != "match":
            continue
        data = obj.get("data", {})
        file_path = data.get("path", {}).get("text", "")
        line_num = data.get("line_number", 0)
        matched_text = data.get("lines", {}).get("text", "")
        label = "credential-pattern"
        if re.search(r"AKIA[0-9A-Z]{16}", matched_text):
            label = "aws-key"
        elif re.search(r"-----BEGIN (RSA|EC|OPENSSH) PRIVATE KEY-----", matched_text):
            label = "private-key"
        hits.append({"file": file_path, "line": line_num, "pattern": label})
        if len(hits) >= 10:
            break

    return hits


def _build_markdown(
    languages: dict,
    scc_data: list,
    frameworks: list[dict],
    manifests: list[str],
    services: list,
    secret_posture: dict | None,
    build_system: list[dict] | None = None,
    monorepo: list[dict] | None = None,
    entry_binaries: list[dict] | None = None,
    secret_leaks: list[dict] | None = None,
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

    if build_system:
        lines.append("## Build / Test\n")
        for entry in build_system:
            tool = entry["tool"]
            if "targets" in entry:
                targets_str = ", ".join(entry["targets"])
                lines.append(f"- **{tool}**: targets: {targets_str}")
            else:
                lines.append(f"- **{tool}**")
        lines.append("")

    if monorepo:
        lines.append("## Monorepo Layout\n")
        for mod in monorepo:
            lines.append(f"- `{mod['path']}` ({mod['type']})")
        lines.append("")

    if entry_binaries:
        lines.append("## Entry Binaries\n")
        for eb in entry_binaries:
            lines.append(f"- `{eb['file']}` ({eb['language']})")
        lines.append("")

    lines.append("## Potential Secret Leaks\n")
    if secret_leaks:
        for hit in secret_leaks:
            lines.append(f"- `{hit['file']}:{hit['line']}` ({hit['pattern']})")
    else:
        lines.append("- None detected")
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
    build_system = _detect_build_system(repo_path)
    monorepo = _detect_monorepo(repo_path)
    entry_binaries = _detect_entry_binaries(repo_path)
    secret_leaks = _scan_secrets(repo_path)

    markdown = _build_markdown(
        languages, scc_data, frameworks, manifests, services, secret_posture,
        build_system=build_system,
        monorepo=monorepo,
        entry_binaries=entry_binaries,
        secret_leaks=secret_leaks,
    )

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
