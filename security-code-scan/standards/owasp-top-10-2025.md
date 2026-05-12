# OWASP Top 10 — 2025

Reference file for mapping findings to the OWASP Top 10 categories. Loaded by Phase 5 (report) and occasionally Phase 3 when a finding needs an OWASP category.

OWASP Top 10 categories are audience-friendly labels but **too coarse to be used as primary keys**. Always emit CWE first (stable, fine-grained), OWASP second (recognizable to management and developers).

## Categories

### A01:2021 — Broken Access Control

Missing or incorrect authorization checks. Most common web vulnerability.

**Primary CWEs:** CWE-285, CWE-639 (IDOR), CWE-862 (Missing Authorization), CWE-863 (Incorrect Authorization)

Maps to: [`vuln-classes/authn-authz.md`](../vuln-classes/authn-authz.md)

### A02:2021 — Cryptographic Failures

Weak algorithms, hardcoded secrets, insecure randomness, missing encryption.

**Primary CWEs:** CWE-327, CWE-328, CWE-330, CWE-326, CWE-295, CWE-798

Maps to: [`vuln-classes/crypto.md`](../vuln-classes/crypto.md)

### A03:2021 — Injection

SQL injection, command injection, LDAP, NoSQL, XPath, template injection.

**Primary CWEs:** CWE-89, CWE-78, CWE-91, CWE-943, CWE-917, CWE-94

Maps to: [`vuln-classes/injection.md`](../vuln-classes/injection.md)

### A04:2021 — Insecure Design

Architectural flaws that cannot be fixed by a local patch. Race conditions, missing rate limiting, business-logic bypasses.

**Primary CWEs:** CWE-209 (information exposure), CWE-256 (plaintext storage), CWE-501 (trust boundary violation)

### A05:2021 — Security Misconfiguration

Default credentials, exposed admin interfaces, verbose error messages, missing security headers, overly permissive CORS.

**Primary CWEs:** CWE-16, CWE-2, CWE-260, CWE-942

### A06:2021 — Vulnerable and Outdated Components

Dependencies with known CVEs. Output of govulncheck, npm audit, pip-audit feeds here.

**Primary CWE:** CWE-1104, CWE-1035

### A07:2021 — Identification and Authentication Failures

Session fixation, weak password storage, missing MFA, insecure session expiration.

**Primary CWEs:** CWE-287, CWE-384, CWE-613, CWE-521, CWE-319

Maps to: [`vuln-classes/authn-authz.md`](../vuln-classes/authn-authz.md)

### A08:2021 — Software and Data Integrity Failures

Insecure deserialization, unsigned updates, CI/CD compromise.

**Primary CWEs:** CWE-502, CWE-829, CWE-494

Maps to: [`vuln-classes/deserialization.md`](../vuln-classes/deserialization.md)

### A09:2021 — Security Logging and Monitoring Failures

Missing or insufficient logging, logs containing sensitive data, missing alerting on security events.

**Primary CWEs:** CWE-778, CWE-532, CWE-117

### A10:2021 — Server-Side Request Forgery (SSRF)

Handlers that fetch URLs provided by untrusted input without restricting the target.

**Primary CWE:** CWE-918

## Usage in reports

When writing findings in Phase 5, include the OWASP category *after* the CWE:

```
CWE: CWE-89 (SQL Injection)
OWASP: A03:2021 — Injection
```

This lets developers search by category (what they know) and lets security tooling deduplicate by CWE (what's stable).

## Note on the 2025 revision

As of April 2026, OWASP has not yet published a 2025 revision of the Top 10 — the 2021 list remains current. If a 2025 revision is released during the lifetime of this framework, update this file and the report template accordingly.
