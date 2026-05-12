---
name: security-code-scan-go
description: Go language toolchain for security-code-scan Phase 2. Runs govulncheck, gosec, staticcheck, and semgrep, then returns normalized findings.
---

# Go Language Skill

Invoked by Phase 2 (Tooling) when `project-profile.json` reports `"language": "go"`.

## Tools to run

Run in this order. If a tool is not installed, report as skipped -- do not abort.

### Must-run

```bash
govulncheck ./...
```
Known CVEs in dependencies. Reachability-aware -- only flags CVEs in code that's actually called. Uses the official Go vuln database.

```bash
gosec -fmt json ./... 2>/dev/null
```
Security-specific SAST. 50+ rules, CWE-mapped, taint analysis (G101-G601).

```bash
staticcheck ./...
```
Deep static analysis. Catches correctness bugs that become security issues.

```bash
go vet ./...
```
Built-in common mistakes. Low noise.

### Should-run (if config or tool exists)

```bash
# Only if .golangci.yml exists in the project root
golangci-lint run --out-format json

# Only if semgrep is installed
semgrep --config=p/golang --json 2>/dev/null
```

### Skip if not installed (note as skipped)

- `nancy` -- alternative dependency vuln scanner
- `go-critic` -- structural checks
- `go test -race ./...` -- data race detection (only run for concurrent-heavy services; can take minutes)

## Go-specific notes for Phase 3

Load these reference files when the patterns below are detected:

| Reference file | Load when |
|---|---|
| `references/goroutine-leaks.md` | Codebase uses goroutines extensively (grep for `go func`, `go `) |
| `references/parser-footguns.md` | Codebase parses user-supplied JSON/YAML/XML with non-stdlib libraries |

### CWEs that rarely apply to pure Go

Skip unless CGo is present:
- CWE-119, CWE-120, CWE-125, CWE-787 (buffer overflows -- memory safety prevents these in pure Go)

### CWEs elevated in Go

These apply and should be actively checked in Phase 3:
- CWE-362 (Race conditions -- goroutines make these easy to introduce)
- CWE-400 (Resource exhaustion -- missing HTTP timeouts, goroutine leaks)
- CWE-89 (SQL injection -- string concatenation common in Go DB code)
- CWE-22 (Path traversal -- filepath.Join does not sanitize)
- CWE-798 (Hardcoded credentials -- common in Go configs and test files)

## Output format

Return each finding in this normalized structure:

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
