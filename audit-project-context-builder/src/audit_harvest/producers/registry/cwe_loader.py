"""Loads cwe_rules.yaml into typed dataclasses.
Single source of truth for the A4 CWE rule set.
"""
from __future__ import annotations
from dataclasses import dataclass
from functools import lru_cache
from pathlib import Path
import yaml

_REGISTRY_PATH = Path(__file__).parent / "cwe_rules.yaml"


@dataclass(frozen=True)
class CweRule:
    id: str
    name: str
    sbom_signals: tuple[str, ...]
    rg_pattern: str | None
    rg_file_types: tuple[str, ...] | None
    negative_requires_two: bool
    no_web_means_false: bool


@lru_cache(maxsize=1)
def load_cwe_rules() -> tuple[CweRule, ...]:
    raw = yaml.safe_load(_REGISTRY_PATH.read_text())
    rules = []
    for entry in raw["rules"]:
        rules.append(CweRule(
            id=entry["id"],
            name=entry["name"],
            sbom_signals=tuple(entry.get("sbom_signals") or []),
            rg_pattern=entry.get("rg_pattern"),
            rg_file_types=tuple(entry["rg_file_types"]) if entry.get("rg_file_types") else None,
            negative_requires_two=bool(entry.get("negative_requires_two", False)),
            no_web_means_false=bool(entry.get("no_web_means_false", False)),
        ))
    return tuple(rules)
