---
name: security-code-scan-discover
description: Phase 1 of security-code-scan. Profiles the codebase and builds a gate matrix of applicable vulnerability classes.
---

# Phase 1: Discover

## Tools allowed

Read, Glob, Grep, Write (`.security-review/project-profile.json` only)

## Goal

Produce `project-profile.json` with enough information to:
1. Focus later phases on what matters
2. Gate out vulnerability classes that do not apply
3. Give Phase 3 (Code Review) signal about where to look first

## Procedure

### 1. Detect language and framework

```bash
find . -name "go.mod" -not -path "*/vendor/*"                        # Go
find . -name "package.json" -not -path "*/node_modules/*"            # Node/TS
find . -name "requirements.txt" -o -name "pyproject.toml"            # Python
find . -name "Cargo.toml"                                             # Rust
```

Also check imports in source files for framework signals (net/http, gin, echo, django, express, etc.).

### 2. Count files and LOC

```bash
find . -type f -name "*.go" | wc -l   # adjust extension for detected language
```

If LOC > 50,000: set `"largeCodebase": true` -- Phase 3 must prioritize by entry points rather than full scan.

### 3. Find entry points

Entry points are where attacker-controlled data enters the system. Look for:

- HTTP handlers (`http.HandleFunc`, `router.GET`, `@app.route`, `app.use`)
- CLI entrypoints (`main()`, `cmd/` directory, argument parsing)
- Background job processors (message queue consumers, cron handlers)
- Deserialization boundaries (`json.Unmarshal`, `yaml.Unmarshal`, `pickle.loads`)

### 4. Check git signals

```bash
git log --oneline --since="30 days ago" --grep="security\|auth\|cve\|vuln\|fix" 2>/dev/null | head -20
git log --oneline --since="30 days ago" -- "**/*auth*" "**/*crypto*" "**/*token*" 2>/dev/null | head -10
```

Recent security commits and high churn in auth/crypto paths signal elevated review priority.

### 5. Build gate matrix

| Class | Applies if |
|---|---|
| injection | HTTP handlers, DB access, shell execution, or template rendering found |
| authn-authz | Auth middleware, JWT/session handling, or role-check patterns found |
| crypto | Crypto imports, password handling, or secret storage found |
| deserialization | JSON/YAML/XML unmarshal of untrusted input found |

Set `"applicable": true/false` for each.

### 6. Write project-profile.json

```json
{
  "language": "go",
  "framework": "net/http",
  "linesOfCode": 12400,
  "fileCount": 87,
  "entryPoints": [
    "cmd/api/main.go",
    "internal/handlers/user.go",
    "internal/handlers/payment.go"
  ],
  "gitSignals": {
    "recentSecurityCommits": 2,
    "highChurnPaths": ["internal/auth/"]
  },
  "gateMatrix": {
    "injection": true,
    "authn-authz": true,
    "crypto": true,
    "deserialization": false
  },
  "flags": {
    "largeCodebase": false,
    "memoryUnsafe": false
  }
}
```

## Output

`.security-review/project-profile.json`
