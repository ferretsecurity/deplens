---
name: security-code-scan-report
description: Phase 5 of security-code-scan. Generates a confidence-tiered vulnerability report with CWE mappings, CVSS estimates, and fix guidance.
---

# Phase 5: Report

## Tools allowed

Read, Write (`.security-review/security-review-report.md` only)

## Goal

Produce a developer-readable report. Each HIGH finding gets a fix prompt. MEDIUM findings get triage notes. LOW findings are omitted by default.

## Procedure

### 1. Read verified-findings.json

### 2. Write the report using this template

```markdown
# Security Review Report

**Date:** <ISO date>
**Target:** <path reviewed>
**Tools run:** <list from tool-results.json>

---

## Summary

| Tier | Count |
|---|---|
| HIGH | <n> |
| MEDIUM | <n> |
| LOW | <n> (not shown) |

---

## HIGH Findings

### [H1] <CWE-ID>: <Title>

**File:** `<file>:<line>`
**OWASP:** <category>
**CVSS estimate:** <score> (<rationale -- Base score only, estimate>)

**Description:**
<data flow from verified-findings>

**Evidence:**
<code snippet>

**Fix:**
<1-2 sentence fix description>

**Fix prompt:**
Fix the <vuln type> at <file>:<line>. <Specific instruction, e.g.: Replace string concatenation with a parameterized query using db.QueryContext with placeholder arguments.>

---

## MEDIUM Findings

### [M1] <CWE-ID>: <Title>

**File:** `<file>:<line>`
**Note:** <why this is MEDIUM, not HIGH -- what factor is missing>

<description>

---

## Appendix: Tools Run

<tool name>: <version if detectable> -- <finding count>
```

### 3. Deduplicate

If multiple findings share the same root cause (same unsanitized input flowing to multiple sinks), group them under one finding with multiple locations listed.

## Anti-patterns

- Do not include LOW findings unless the user explicitly asked
- Do not invent CVSS scores -- use CVSS Base Score methodology and mark as estimate
- Do not omit the fix prompt for HIGH findings -- it is the most actionable output
- OWASP labels are for communication; CWE is the primary identifier

## Output

`.security-review/security-review-report.md`
