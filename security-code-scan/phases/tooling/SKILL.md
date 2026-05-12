---
name: security-code-scan-tooling
description: Phase 2 of security-code-scan. Runs deterministic SAST tools and maps findings to CWE.
---

# Phase 2: Tooling

## Tools allowed

Read, Glob, Grep, Bash (within project directory only), Write (`.security-review/tool-results.json` only)

## Goal

Run deterministic security tools and collect structured findings. These prime Phase 3 (Code Review) -- the LLM reasons over tool output rather than starting from scratch.

## Procedure

### 1. Read project profile

Read `.security-review/project-profile.json`. Use `language` to select the appropriate language skill.

### 2. Invoke language skill

For `"language": "go"`: invoke `security-code-scan/languages/go`

The language skill runs the tools and returns raw findings in the normalized format below.

### 3. Normalize output

Each finding must conform to this structure:

```json
{
  "tool": "gosec",
  "ruleId": "G201",
  "cwe": "CWE-89",
  "file": "internal/db/query.go",
  "line": 47,
  "severity": "HIGH",
  "message": "SQL query construction using string concatenation",
  "snippet": "db.Query(\"SELECT * FROM users WHERE id=\" + userID)"
}
```

### 4. Deduplicate

If two tools flag the same file and line, merge into one finding with both tool names listed in `"tool"`.

### 5. Write tool-results.json

```json
{
  "toolsRun": ["govulncheck", "gosec", "staticcheck", "go vet"],
  "toolsSkipped": [],
  "findingCount": 14,
  "findings": [ ... ]
}
```

## Anti-patterns

- Do not filter or triage findings here -- that is Phase 4's job
- Do not run tools outside the project directory
- Do not install missing tools -- report them as skipped with reason

## Output

`.security-review/tool-results.json`
