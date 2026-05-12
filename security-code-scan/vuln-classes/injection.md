# Injection

Loaded by Phase 3 when the reviewed code constructs queries, shell commands, OS calls, or other interpreted strings from user input.

**CWE:** CWE-89 (SQL Injection), CWE-78 (OS Command Injection), CWE-91 (XPath Injection), CWE-943 (NoSQL Injection), CWE-917 (Expression Language Injection), CWE-94 (Code Injection)

**OWASP:** A03:2021 — Injection

## Core pattern

An injection vulnerability exists when untrusted input is concatenated or interpolated into a string that is then parsed or executed by a downstream interpreter (SQL engine, shell, template engine, etc.).

## Detection heuristics

Grep for these sink patterns in reachable code:

| Sink type | Patterns to find |
|---|---|
| **SQL** | `db.Query(...)`, `db.Exec(...)`, `db.QueryRow(...)` with `fmt.Sprintf`, `+`, or `strings.Builder` building the query |
| **Shell** | `exec.Command(sh, "-c", userInput)`, `exec.CommandContext` with user-controlled arg 1 |
| **Template** | `text/template` with user-controlled template source (not data — source) |
| **Path** | `filepath.Join(userDir, ...)` without a subsequent `filepath.Clean` + prefix check |
| **LDAP** | `ldap.Search` with user input in filter string |
| **NoSQL** | MongoDB `bson.M{...}` with user input directly as a filter |

## Safe alternatives (Go)

| Sink type | Safe pattern |
|---|---|
| SQL | `db.Query("SELECT * FROM t WHERE c = $1", userInput)` — parameterized |
| Shell | Never pass user input as a shell arg. Use `exec.Command(binary, arg1, arg2, ...)` with a static binary and typed args. Never use `sh -c`. |
| Template | Separate template source (static) from data (user input). Use `html/template` for HTML output — it escapes automatically. |
| Path | `filepath.Clean`, then verify the result is still under the allowed root prefix |
| LDAP | Use a properly-escaped filter builder, or library like `go-ldap` with bound parameters |
| NoSQL | Use typed filter structs, validate types, never accept raw `bson.M` from user input |

## Examples

See [`languages/go/references/examples/sqli-bad.go`](../languages/go/references/examples/sqli-bad.go) and [`sqli-good.go`](../languages/go/references/examples/sqli-good.go).

## False positive patterns to watch for

Do NOT flag the following as injection:

- `fmt.Sprintf` being used to build log messages (not queries)
- Parameterized queries with `$1`, `?`, or named params — these are safe
- `exec.Command("git", "log", "--oneline")` with static args — no injection sink
- SQL queries built from constants only — no user input path
- Queries built inside a function where all callers pass typed, non-string parameters — verify via call-graph before reporting

## Reachability check (for Phase 4)

A pattern-match on a suspicious sink is NOT the same as a vulnerability. For HIGH confidence, confirm:

1. The source is under attacker control (HTTP body, query param, uploaded file, external API)
2. The path from source to sink does not go through proper validation or parameterization
3. The sink is actually reachable in the production build (not behind a disabled feature flag or in dead code)

If any of the three is missing or uncertain, demote to MEDIUM or LOW.
