---
name: security-code-scan-verify
description: Phase 4 of security-code-scan. Reachability check, LLM disproval, and SAST cross-reference. Assigns HIGH/MEDIUM/LOW confidence tiers.
---

# Phase 4: Verify

## Tools allowed

Read, Glob, Grep, Bash (scoped -- for call graph tracing if needed), Write (`.security-review/verified-findings.json` only)

## Goal

Reduce false positives before reporting. Three-step process per finding:

1. **Reachability** -- can untrusted input actually reach this code path?
2. **Disproval** -- try to invalidate the finding with evidence of a sanitizer or framework control
3. **SAST cross-reference** -- does at least one deterministic tool agree?

## Confidence tiers

| Tier | Meaning |
|---|---|
| HIGH | Reachable, not disprovable, SAST-confirmed or strong LLM evidence |
| MEDIUM | Plausible -- one factor missing (reachability unclear, partial mitigation present) |
| LOW | Not reachable, or credible mitigation found |

## Procedure

For each finding in `raw-findings.json`:

### Step 1: Reachability check

Trace from the sink backward:
- Is there a code path from an attacker-controlled input source to this sink?
- Does the path require authentication? (Authenticated endpoints are lower priority)
- Is this code dead code?

If no reachable path from untrusted input: demote to LOW.

### Step 2: Disproval

Try to invalidate the finding:
- Is user input validated before reaching the sink?
- Does the framework handle this automatically? (ORM parameterizes, template engine escapes)
- Is there middleware that sanitizes this input type?
- Is the dangerous function called with a constant, not user input?

If a credible countermeasure is found: demote to MEDIUM (not LOW -- countermeasures can be bypassed or removed).

### Step 3: SAST cross-reference

Does any tool in `tool-results.json` flag the same file and line for the same or related CWE?

- Tool agrees: confirm or upgrade current tier
- Tool does not flag: note as "LLM-only" in the finding (slight confidence reduction)

### Step 4: Write verified-findings.json

```json
{
  "summary": {
    "total": 12,
    "high": 3,
    "medium": 5,
    "low": 4
  },
  "findings": [
    {
      "id": "F001",
      "confidence": "HIGH",
      "reachable": true,
      "disprovable": false,
      "sastConfirmed": true,
      "cwe": "CWE-89",
      "title": "SQL injection via string concatenation",
      "file": "internal/db/query.go",
      "line": 47,
      "dataFlow": "handlers/user.go:23 (URL param) -> db/query.go:47 (sink)",
      "verificationNotes": "No parameterization found. gosec agrees (G201). Reachable via unauthenticated /user endpoint."
    }
  ]
}
```

## Anti-patterns

- Do not skip the reachability check -- it is the biggest single lever for FPR reduction
- Do not use SAST agreement as the primary signal -- reachability matters more
- Do not mark every tool finding as HIGH -- tools have known FPR; verify each one

## Output

`.security-review/verified-findings.json`
