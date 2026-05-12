# security-code-scan

A 5-phase Claude Agent Skill for whole-codebase security review.

## What it does

1. **Discover** -- profiles the codebase: language, framework, entry points, LOC, git signals. Builds a gate matrix of applicable vulnerability classes.
2. **Tooling** -- runs deterministic SAST tools (govulncheck, gosec, staticcheck, semgrep). Maps findings to CWE.
3. **Code Review** -- LLM-driven reasoning over tool output and code patterns tools miss (logic errors, auth bypasses, business-logic flaws).
4. **Verify** -- reachability check + LLM disproval + SAST cross-reference. Assigns HIGH/MEDIUM/LOW confidence.
5. **Report** -- produces `security-review-report.md` with CWE, CVSS estimates, and fix guidance. HIGH findings get fix prompts.

## How to invoke

In any project directory:

```
Use the security-code-scan skill to review this codebase
```

To limit scope:

```
Use security-code-scan to review only the auth package
```

## Output

`.security-review/security-review-report.md` -- the final report.

Intermediate artifacts (for debugging): `.security-review/project-profile.json`, `tool-results.json`, `raw-findings.json`, `verified-findings.json`.

## Languages supported

| Language | SAST Tools | Status |
|---|---|---|
| Go | govulncheck, gosec, staticcheck, go vet, semgrep | Supported |
| Python | bandit, semgrep, pip-audit | Planned |
| TypeScript/JS | eslint-security, semgrep, npm audit | Planned |

## Limitations

- No formal FPR/recall benchmark against a ground-truth dataset yet
- No revision loop (Phase 3 cannot re-examine based on Phase 4 feedback)
- CVSS scores are estimates -- not authoritative
- Cannot detect business-logic flaws that require domain knowledge of the application

## Research basis

See [ARCHITECTURE.md](../ARCHITECTURE.md) and `ai-security-review-research/` for the design decisions and evidence behind this skill.
