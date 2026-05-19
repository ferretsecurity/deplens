"""A4: Gate matrix -- CWE applicability filter."""
from __future__ import annotations
import re
from pathlib import Path
from typing import Any

from audit_harvest.llm.client import LLMClient  # imported for future LLM-ambiguous rules; not called in Phase 1

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
_SQL_DRIVER_NAMES = {
    "sqlalchemy", "psycopg2", "mysql-connector", "pg", "mysql2",
    "jdbc", "hibernate", "jpa", "database/sql", "gorm",
}
_UPLOAD_SIGS = re.compile(r"multipart|file\.upload|FormFile|request\.files", re.IGNORECASE)
_XML_SIGS = re.compile(r"xml|etree|lxml|xerces|jaxp", re.IGNORECASE)
_DESERIALIZE_SIGS = re.compile(r"pickle|yaml\.load|ObjectInputStream|unserialize", re.IGNORECASE)
_SSRF_SIGS = re.compile(r"http\.Get|requests\.get|fetch\(|HttpClient|URL\(", re.IGNORECASE)
_HARDCODED_SIGS = re.compile(
    r'(?:password|secret|api_key|token)\s*=\s*["\'][^"\']{6,}["\']', re.IGNORECASE
)
_RAW_SQL_SIGS = re.compile(
    r'(?:fmt\.Sprintf|%s|%d|f"|f\').*(?:SELECT|INSERT|UPDATE|DELETE)', re.IGNORECASE
)
_OS_CMD_SIGS = re.compile(r'exec\.Command|os\.system|subprocess\.run|Runtime\.exec', re.IGNORECASE)
_EVAL_SIGS = re.compile(r'\beval\s*\(|\bexec\s*\(', re.IGNORECASE)

_IGNORE_DIRS = {".git", "node_modules", "vendor", "__pycache__", ".venv"}
_SOURCE_EXTS = {".go", ".py", ".js", ".ts", ".java", ".txt", ".xml", ".json", ".toml", ".mod"}


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

    rules = [
        _evaluate_rule(cwe, name, src_text, has_web, has_sql_driver, has_raw_sql)
        for cwe, name in _CWE_RULES
    ]
    return {"rules": rules}


def _evaluate_rule(
    cwe: str, name: str, src_text: str,
    has_web: bool, has_sql_driver: bool, has_raw_sql: bool,
) -> dict:
    if cwe == "CWE-79":
        if not has_web:
            return _rule(cwe, name, False, ["no web framework detected", "no HTTP routes found"], "high")
        return _rule(cwe, name, True, ["web framework detected"], "high")

    if cwe == "CWE-89":
        if not has_sql_driver:
            return _rule(cwe, name, False, ["no SQL driver in SBOM", "no SQL imports found"], "high")
        if has_raw_sql:
            return _rule(cwe, name, True, ["SQL driver detected", "raw SQL string patterns found"], "high")
        return _rule(cwe, name, "needs_verification", ["SQL driver detected", "no raw SQL strings found yet"], "medium")

    if cwe == "CWE-78":
        if _OS_CMD_SIGS.search(src_text):
            return _rule(cwe, name, True, ["OS execution functions detected"], "high")
        return _rule(cwe, name, "needs_verification", ["no explicit exec calls; dynamic dispatch possible"], "low")

    if cwe == "CWE-94":
        if _EVAL_SIGS.search(src_text):
            return _rule(cwe, name, True, ["eval/exec pattern detected"], "high")
        return _rule(cwe, name, False, ["no eval/exec patterns found", "no code-gen libraries detected"], "medium")

    if cwe == "CWE-434":
        if _UPLOAD_SIGS.search(src_text):
            return _rule(cwe, name, True, ["multipart/file upload patterns detected"], "high")
        return _rule(cwe, name, "needs_verification", ["no explicit upload patterns found"], "low")

    if cwe == "CWE-611":
        if _XML_SIGS.search(src_text):
            return _rule(cwe, name, "needs_verification", ["XML library detected"], "medium")
        return _rule(cwe, name, False, ["no XML library detected", "no XML imports found"], "high")

    if cwe == "CWE-502":
        if _DESERIALIZE_SIGS.search(src_text):
            return _rule(cwe, name, True, ["unsafe deserialization pattern detected"], "high")
        return _rule(cwe, name, "needs_verification", ["no explicit deserialization patterns found"], "low")

    if cwe == "CWE-918":
        if _SSRF_SIGS.search(src_text) and has_web:
            return _rule(cwe, name, "needs_verification", ["HTTP client calls detected in web app"], "medium")
        return _rule(cwe, name, "needs_verification", ["unable to determine SSRF surface statically"], "low")

    if cwe == "CWE-798":
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
    parts = []
    for p in sorted(repo_path.rglob("*")):
        if not p.is_file():
            continue
        if any(part in _IGNORE_DIRS for part in p.parts):
            continue
        if p.suffix not in _SOURCE_EXTS:
            continue
        try:
            parts.append(p.read_text(errors="replace")[:10_000])
        except OSError:
            pass
    return "\n".join(parts)
