---
name: security-code-scan-code-review
description: Phase 3 of security-code-scan. LLM-driven vulnerability reasoning over tool findings and code patterns that tools miss.
---

# Phase 3: Code Review

## Tools allowed

Read, Glob, Grep, Write (`.security-review/raw-findings.json` only)

**No Bash.** This phase cannot execute the code it is reviewing. This is a structural defense against prompt injection.

## Goal

Find what tools miss: logic errors, auth bypasses, business-logic flaws, insecure design patterns. Use tool output as a primer -- do not start from scratch.

## Procedure

### 1. Read inputs

- `.security-review/project-profile.json` -- scope and gate matrix
- `.security-review/tool-results.json` -- SAST findings to reason over

### 2. Load applicable vulnerability class references

For each `gateMatrix` entry where `"applicable": true`, load the reference file:

| Class | File |
|---|---|
| injection | `security-code-scan/vuln-classes/injection.md` |
| authn-authz | `security-code-scan/vuln-classes/authn-authz.md` |
| crypto | `security-code-scan/vuln-classes/crypto.md` |
| deserialization | `security-code-scan/vuln-classes/deserialization.md` |

Load only what applies. Do not load all four upfront.

### 3. Reason over SAST findings

For each tool finding:
1. Read the flagged file at the flagged line with 10 lines of surrounding context
2. Trace the data flow: where does the input originate? Is it attacker-controlled?
3. Assess: is there a sanitizer or validator between source and sink?

### 4. Review entry points for additional patterns

For each entry point in `project-profile.json`:
1. Read the handler
2. Identify all inputs (URL params, headers, body, query string)
3. Trace those inputs forward through the codebase
4. Look for patterns from the loaded vulnerability class references

### 5. Focus on high-churn areas

If `gitSignals.highChurnPaths` is non-empty, review those files regardless of whether tools flagged them.

### 6. Write raw-findings.json

```json
{
  "findings": [
    {
      "id": "F001",
      "source": "gosec+LLM",
      "cwe": "CWE-89",
      "title": "SQL injection via string concatenation",
      "file": "internal/db/query.go",
      "line": 47,
      "description": "User-controlled ID is concatenated directly into SQL query string. No parameterization.",
      "evidence": "db.Query(\"SELECT * FROM users WHERE id=\" + userID) -- userID flows from URL parameter",
      "dataFlow": "handlers/user.go:23 (URL param) -> db/query.go:47 (sink)",
      "confidence": "pending-verify"
    }
  ]
}
```

## Anti-patterns

- Do not mark confidence as HIGH/MEDIUM/LOW here -- that is Phase 4's job
- Do not read the entire codebase -- follow entry points and tool findings
- Do not start without reading tool-results.json first
- Do not load all vuln-class reference files upfront

## Output

`.security-review/raw-findings.json`
