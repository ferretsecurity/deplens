from __future__ import annotations

import json
import logging
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Optional

logger = logging.getLogger(__name__)


@dataclass
class GroundedOutput:
    claims: list[dict[str, Any]] = field(default_factory=list)

    def verified(self, repo_path: Path, failure_log: Optional[Path] = None) -> GroundedOutput:
        kept = drop_unverified_claims(self.claims, repo_path, failure_log=failure_log)
        dropped = len(self.claims) - len(kept)
        if dropped:
            logger.warning("Dropped %d unverified LLM claims", dropped)
        return GroundedOutput(claims=kept)


def drop_unverified_claims(
    claims: list[dict[str, Any]],
    repo_path: Path,
    failure_log: Optional[Path] = None,
) -> list[dict[str, Any]]:
    verified: list[dict[str, Any]] = []
    dropped: list[dict[str, Any]] = []

    for claim in claims:
        file_path = Path(claim.get("file", ""))
        lines = claim.get("lines", [])

        if not file_path.is_absolute():
            file_path = repo_path / file_path

        if not file_path.exists():
            dropped.append({**claim, "_reason": "file_not_found"})
            continue

        if lines:
            file_lines = file_path.read_text(errors="replace").splitlines()
            start, end = lines[0], lines[-1]
            if start < 1 or end > len(file_lines):
                dropped.append({**claim, "_reason": "lines_out_of_range"})
                continue

        verified.append(claim)

    if dropped:
        if failure_log is not None:
            failure_log.parent.mkdir(parents=True, exist_ok=True)
            with open(failure_log, "a") as f:
                for entry in dropped:
                    f.write(json.dumps(entry) + "\n")
        else:
            for entry in dropped:
                logger.debug("Dropped unverified claim: %s", entry)

    return verified


class LLMClient:
    """Thin wrapper for Anthropic API calls that enforces cite-or-omit grounding.

    Every call returns a GroundedOutput whose claims have been verified against
    the actual file system. Unverified claims are dropped and logged.
    """

    def __init__(self, model: str = "claude-sonnet-4-6") -> None:
        self.model = model

    def call_with_grounding(
        self,
        prompt: str,
        repo_path: Path,
        system: Optional[str] = None,
        failure_log: Optional[Path] = None,
    ) -> GroundedOutput:
        import anthropic

        client = anthropic.Anthropic()
        messages = [{"role": "user", "content": prompt}]
        kwargs: dict[str, Any] = {
            "model": self.model,
            "max_tokens": 4096,
            "messages": messages,
        }
        if system:
            kwargs["system"] = system

        response = client.messages.create(**kwargs)
        text = response.content[0].text

        try:
            data = json.loads(text)
            claims = data if isinstance(data, list) else data.get("claims", [])
        except json.JSONDecodeError:
            claims = []

        return GroundedOutput(claims=claims).verified(repo_path, failure_log=failure_log)
