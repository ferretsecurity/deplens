"""A4: Gate matrix -- CWE applicability filter."""
from __future__ import annotations
import re
import subprocess
from pathlib import Path
from typing import Any

from audit_harvest.llm.client import LLMClient  # imported for future LLM-ambiguous rules; not called in Phase 1
from audit_harvest.producers.registry.cwe_loader import CweRule, load_cwe_rules

_IGNORE_DIRS = {".git", "node_modules", "vendor", "__pycache__", ".venv"}


def produce_gate_matrix(
    repo_path: Path,
    entry_points: dict,
    sbom_components: list[dict],
) -> dict:
    repo_path = Path(repo_path)
    has_web = bool(entry_points.get("entry_points")) or _sbom_has_purl_matching(
        sbom_components,
        "gin", "chi", "echo", "fiber", "flask", "fastapi", "django",
        "express", "nestjs", "next", "spring-web", "spring-boot",
    )
    rules = load_cwe_rules()
    return {
        "rules": [
            _evaluate_rule(rule, repo_path, sbom_components, has_web)
            for rule in rules
        ]
    }


def _evaluate_rule(
    rule: CweRule,
    repo_path: Path,
    sbom_components: list[dict],
    has_web: bool,
) -> dict:
    if rule.no_web_means_false and not has_web:
        return _rule(rule.id, rule.name, False,
                     ["no web framework detected", "no HTTP routes found"], "high")

    sbom_hit = bool(rule.sbom_signals) and _sbom_has_purl_matching(
        sbom_components, *rule.sbom_signals
    )

    rg_hit, rg_samples = False, []
    if rule.rg_pattern:
        rg_hit, rg_samples = _rg_match(
            repo_path,
            rule.rg_pattern,
            file_types=list(rule.rg_file_types) if rule.rg_file_types else None,
        )

    # KNOWN GAP: CWE-89 requires BOTH SBOM signal AND rg confirmation to conclude
    # applicable=True. SBOM-alone -> needs_verification. The generic "any positive
    # signal = True" logic cannot express this "requires-both-for-positive" contract.
    if rule.id == "CWE-89" and sbom_hit and not rg_hit:
        return _rule(rule.id, rule.name, "needs_verification",
                     ["SQL driver detected", "no raw SQL string patterns found yet"], "medium")

    has_any_positive = sbom_hit or rg_hit

    if has_any_positive:
        evidence: list[str] = []
        if sbom_hit:
            evidence.append(f"SBOM component matches {rule.id} signal")
        if rg_samples:
            evidence += rg_samples[:2]
        elif rg_hit:
            evidence.append("pattern match found")
        confidence = "high" if (sbom_hit and rg_hit) else "medium"
        return _rule(rule.id, rule.name, True, evidence, confidence)

    negative_evidence: list[str] = []
    if rule.sbom_signals:
        negative_evidence.append(f"no {rule.id}-related component in SBOM")
    if rule.rg_pattern:
        negative_evidence.append("no matching pattern found in source")

    if rule.negative_requires_two:
        if len(negative_evidence) >= 2:
            return _rule(rule.id, rule.name, False, negative_evidence, "high")
        return _rule(rule.id, rule.name, "needs_verification",
                     negative_evidence or ["insufficient evidence"], "low")

    if negative_evidence:
        confidence = "medium" if len(negative_evidence) == 1 else "high"
        return _rule(rule.id, rule.name, False, negative_evidence, confidence)
    return _rule(rule.id, rule.name, "needs_verification",
                 ["insufficient deterministic evidence"], "low")


def _sbom_has_purl_matching(components: list[dict], *substrings: str) -> bool:
    for c in components:
        combined = ((c.get("purl") or "") + " " + (c.get("name") or "")).lower()
        if any(s in combined for s in substrings):
            return True
    return False


def _rg_match(
    repo_path: Path,
    pattern: str,
    file_types: list[str] | None = None,
) -> tuple[bool, list[str]]:
    cmd = ["rg", "--no-heading", "-m", "5"]
    if file_types:
        for ft in file_types:
            cmd.extend(["--type", ft])
    cmd.extend([pattern, str(repo_path)])
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
        if result.returncode == 0 and result.stdout.strip():
            lines = [ln.strip() for ln in result.stdout.strip().split("\n") if ln.strip()]
            return True, lines[:3]
        return False, []
    except (FileNotFoundError, OSError):
        import logging
        logging.getLogger(__name__).warning(
            "ripgrep not found or failed for pattern %r — "
            "rg is a Bucket A tool, install it to enable CWE pattern matching",
            pattern,
        )
        return False, []
    except subprocess.TimeoutExpired:
        return False, []


def _rule(cwe: str, name: str, applicable: Any, evidence: list[str], confidence: str) -> dict:
    return {"cwe": cwe, "name": name, "applicable": applicable, "evidence": evidence, "confidence": confidence}
