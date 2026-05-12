# CWE Top 25 — Most Dangerous Software Weaknesses

Reference file mapping common Go-relevant CWEs to their meaning and detection approach. Loaded by Phase 3 when a finding needs CWE classification.

CWE (Common Weakness Enumeration) is the MITRE-maintained catalog of software weakness types. CWE IDs are stable primary keys — use them in reports, tickets, and cross-referencing.

## Go-relevant entries from the current Top 25

| CWE | Name | Go relevance |
|---|---|---|
| **CWE-79** | Cross-Site Scripting | Relevant only if Go service renders HTML. Use `html/template`, not `text/template`, for HTML output. |
| **CWE-89** | SQL Injection | Direct concern — use parameterized queries (`$1`, `?`). See [`vuln-classes/injection.md`](../vuln-classes/injection.md). |
| **CWE-20** | Improper Input Validation | Broad category — any handler that trusts input shape/values without checking. Common for JSON body parsing. |
| **CWE-22** | Path Traversal | Relevant for file uploads, file serving. Use `filepath.Clean` + prefix check. |
| **CWE-78** | OS Command Injection | Relevant for any `os/exec` usage with user-controlled args. Never use `sh -c`. |
| **CWE-287** | Improper Authentication | Missing auth middleware, JWT bypasses, session fixation. |
| **CWE-125** | Out-of-Bounds Read | Mostly impossible in pure Go (bounds-checked). Relevant for `unsafe` or CGo. |
| **CWE-416** | Use After Free | Impossible in pure Go (GC). Relevant for CGo. |
| **CWE-352** | Cross-Site Request Forgery | Relevant for cookie-auth endpoints — require CSRF tokens or use `SameSite` cookies. |
| **CWE-434** | Unrestricted Upload | File upload handlers without content-type, size, or extension validation. |
| **CWE-862** | Missing Authorization | Authenticated endpoint that doesn't check the user is authorized for the specific resource. |
| **CWE-476** | NULL Pointer Dereference | Go-specific: panic on nil pointer. Usually a crash, occasionally security-relevant if exposed to untrusted input. |
| **CWE-798** | Hardcoded Credentials | Secrets in source code. Detect with `gosec` G101. |
| **CWE-190** | Integer Overflow | Rare in Go due to typed ints, but `int64` arithmetic with user input can overflow. |
| **CWE-400** | Uncontrolled Resource Consumption | Missing HTTP timeouts, unbounded reads, goroutine leaks. |
| **CWE-306** | Missing Authentication for Critical Function | Admin endpoint reachable without auth. |
| **CWE-502** | Deserialization of Untrusted Data | YAML, JSON with `interface{}` fields, etc. See [`vuln-classes/deserialization.md`](../vuln-classes/deserialization.md). |
| **CWE-863** | Incorrect Authorization | Auth check exists but is wrong — uses attacker-controlled ID, has logic error, etc. |
| **CWE-918** | Server-Side Request Forgery | Any handler that fetches URLs from user input without validation. Common in webhook receivers, URL previews. |
| **CWE-362** | Race Condition | See [`languages/go/references/goroutine-leaks.md`](../languages/go/references/goroutine-leaks.md). Only report with race-detector confirmation. |
| **CWE-295** | Improper Certificate Validation | `InsecureSkipVerify: true` or custom `VerifyPeerCertificate` that returns nil. |

## CWEs that rarely apply to pure Go

The following are common in C/C++ but structurally prevented by Go's memory safety model — do NOT report them unless `unsafe` package or CGo is present:

- CWE-119 (Improper Restriction of Operations within the Bounds of a Memory Buffer)
- CWE-120 (Classic Buffer Overflow)
- CWE-125 (Out-of-Bounds Read) — Go panics instead
- CWE-787 (Out-of-Bounds Write) — Go panics instead
- CWE-416 (Use After Free)
- CWE-476 (NULL Pointer Dereference) — Go panics, usually non-exploitable unless the panic itself creates a denial of service

## Format for reporting

Every finding in Phase 5's report should cite a CWE as the primary classification:

```
Finding: SQL injection in user search endpoint
CWE: CWE-89 (SQL Injection)
OWASP: A03:2021 — Injection
```

CWE is the stable key for deduplication, cross-system tracking, and integration with SAST tools. OWASP category is the secondary, audience-friendly label.
