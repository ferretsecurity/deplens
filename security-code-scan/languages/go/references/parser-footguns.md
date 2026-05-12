# Go Parser Footguns

Go's standard library parsers (`encoding/json`, `encoding/xml`, `gopkg.in/yaml.v3`) have default behaviors that surprise developers and have caused real authentication bypasses and privilege escalations.

Loaded by Phase 3 when the reviewed code parses untrusted JSON, XML, or YAML.

## The top 4 footguns

### 1. JSON duplicate keys — Go takes the LAST one

```go
// Attacker sends:
// { "action": "read", "action": "admin_delete" }

var req struct { Action string }
json.Unmarshal(body, &req)
// req.Action == "admin_delete"
```

**Why dangerous:** if another service in your request chain uses a JSON parser that takes the FIRST duplicate key, you have a parser differential. An auth proxy might approve `action=read` while the backend sees `action=admin_delete`. This is the class of bug behind the GitLab SAML auth bypass (2025) and Hashicorp Vault CVE-2020-16250.

**There is no way to disable this behavior in Go's stdlib.** Detect the pattern by grepping for `json.Unmarshal` on request bodies and flag if the struct has no request-signing or normalization step.

**Mitigation:** either normalize the JSON before processing (parse, re-serialize, re-parse), or use a strict parser like [github.com/go-json-experiment/json](https://github.com/go-json-experiment/json) that rejects duplicates by default.

### 2. Unexported fields can still be unmarshaled if a struct tag says so

```go
type User struct {
    Name    string `json:"name"`
    IsAdmin bool   `json:"is_admin"`  // developer thought this was private
}

// Attacker sends: { "name": "eve", "is_admin": true }
// Result: user is admin. The json tag made it public.
```

**Detection:** grep for structs with `json:` tags on security-sensitive fields (`IsAdmin`, `Role`, `Permissions`, `Internal*`). Flag any struct where some fields have tags and others don't — partial tagging is a common mistake.

**Mitigation:** use `json:"-"` on fields that must never be unmarshaled from user input, or use separate request/response DTOs.

### 3. XML external entities (XXE) — Go's encoding/xml is safer than most, but not immune

Go's `encoding/xml` does not resolve external entities by default (unlike Java or PHP), so classic XXE attacks don't apply. However:

- **Billion laughs / quadratic blowup** — still possible with nested entity references
- **XML bomb DoS** — untrusted XML without size limits can exhaust memory
- **Namespace confusion** — between similar-looking tags

**Detection:** grep for `xml.Unmarshal` or `xml.NewDecoder` on untrusted input. Flag if there's no size limit on the reader.

**Mitigation:** wrap the reader with `io.LimitReader(r, maxSize)`. Reject XML over a known threshold.

### 4. YAML tags can invoke Go code (gopkg.in/yaml.v3)

YAML supports custom tags that can trigger type conversion or even custom unmarshaling logic. Untrusted YAML is dangerous in the same way untrusted pickle is dangerous in Python.

**Detection:** grep for `yaml.Unmarshal` on user input (config files from uploads, Kubernetes-style manifests from untrusted sources).

**Mitigation:** use `yaml.UnmarshalStrict` and define strict structs. Never parse untrusted YAML without schema validation. Prefer JSON for data from untrusted sources.

## Cross-reference

- CWE-20 (Improper Input Validation)
- CWE-400 (Uncontrolled Resource Consumption)
- CWE-502 (Deserialization of Untrusted Data) — for YAML with custom tags
- CWE-611 (XXE) — for XML, though Go's stdlib is largely safe

## Source

[Trail of Bits — Unexpected security footguns in Go's parsers (2025)](https://blog.trailofbits.com/2025/06/17/unexpected-security-footguns-in-gos-parsers/)
