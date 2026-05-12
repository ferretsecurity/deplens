---
name: security-code-scan
description: Run a 5-phase security review of the current codebase. Produces a confidence-tiered vulnerability report with CWE mappings and fix guidance.
---

# security-code-scan

## When to invoke

- User asks to "scan", "review for security", "audit", "find vulnerabilities", or "run security checks" on a codebase
- User provides a path and wants security analysis
- Pre-commit or pre-deploy security gate

## Tools allowed

Read, Glob, Grep (dispatch only -- no file modification)

## Procedure

### 1. Confirm scope

Before starting, read the current directory structure (1 level deep) and confirm with the user:

```
I'll run a 5-phase security scan of <path>. This will:
- Profile scope and detect languages (~1 min)
- Run SAST tools (govulncheck, gosec, semgrep) (~2-5 min depending on codebase size)
- LLM-reason over findings
- Verify reachability and assign confidence tiers
- Write report to .security-review/security-review-report.md

Proceed? [y/n]
```

Skip confirmation if user explicitly said "scan without asking" or similar.

### 2. Create output directory

```bash
mkdir -p .security-review
```

### 3. Dispatch phases in sequence

Invoke each phase skill in order, passing the output artifact path as context:

1. Invoke `security-code-scan/phases/discover` -- writes `.security-review/project-profile.json`
2. Invoke `security-code-scan/phases/tooling` -- reads profile, writes `.security-review/tool-results.json`
3. Invoke `security-code-scan/phases/code-review` -- reads tool results, writes `.security-review/raw-findings.json`
4. Invoke `security-code-scan/phases/verify` -- reads raw findings, writes `.security-review/verified-findings.json`
5. Invoke `security-code-scan/phases/report` -- reads verified findings, writes `.security-review/security-review-report.md`

### 4. Report completion

```
Security scan complete. Report at .security-review/security-review-report.md

HIGH findings: <n>
MEDIUM findings: <n>
LOW findings: <n> (not included in report by default)
```

## Anti-patterns

- Do not attempt all phases in one context window -- dispatch each as a subagent
- Do not skip Phase 4 (Verify) -- this is where FPR reduction happens
- Do not modify source files -- this skill is read-only except for `.security-review/`
