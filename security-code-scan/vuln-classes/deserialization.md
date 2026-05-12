# Deserialization of Untrusted Data

Loaded by Phase 3 when the reviewed code deserializes data (JSON, XML, YAML, gob, protobuf, or custom formats) from untrusted sources.

**CWE:** CWE-502 (Deserialization of Untrusted Data), CWE-20 (Improper Input Validation), CWE-400 (Uncontrolled Resource Consumption), CWE-611 (XXE)

**OWASP:** A08:2021 — Software and Data Integrity Failures

## Go's situation

Go is safer than Java, Python, or .NET for deserialization attacks in one important way: **Go's standard library deserializers do not allow arbitrary code execution by default.** There is no equivalent of Python's pickle RCE or Java's ObjectInputStream gadget chains.

However, Go-specific bugs still exist. Three categories:

### 1. Resource exhaustion (DoS)

Untrusted input with no size limit → memory exhaustion.

```go
// BAD
body, _ := io.ReadAll(r.Body)
json.Unmarshal(body, &obj)

// GOOD
body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))  // 1MB cap
json.Unmarshal(body, &obj)
```

**Detection:** any `Unmarshal` on a body that wasn't read through a `LimitReader`.

Also consider nested-structure attacks:

- **JSON bomb**: deeply nested arrays/objects that cause stack exhaustion during parsing
- **YAML billion laughs**: entity references that blow up during expansion
- **XML bomb**: nested entities or quadratic blowup

**Mitigation:** for JSON, Go's stdlib has internal depth limits (10K levels). For YAML and XML, enforce a max-size limit on the reader AND a timeout on the parse operation.

### 2. Parser differentials

See [`languages/go/references/parser-footguns.md`](../languages/go/references/parser-footguns.md) for the full treatment. Summary:

- JSON duplicate keys: Go takes the last
- Unexported fields can be unmarshaled if they have a JSON tag
- Unknown fields are silently ignored (`DisallowUnknownFields` can be enabled)

### 3. Type confusion at the boundary

```go
type Request struct {
    UserID string `json:"user_id"`
}

// Attacker sends: { "user_id": ["not a string"] }
// Unmarshal will fail — good.

// But if the field is interface{}:
type BadRequest struct {
    Payload interface{} `json:"payload"`
}
// Now the attacker controls the shape, and every downstream code path
// that uses `Payload` must handle any JSON type safely.
```

**Detection:** flag `interface{}` or `any` fields on structs that unmarshal untrusted input. Suggest typed alternatives.

## Safe patterns

### ✅ Strict unmarshaling with size limit and unknown-field rejection

```go
func parseRequest(r *http.Request, dst interface{}) error {
    r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1MB
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    if err := dec.Decode(dst); err != nil {
        return err
    }
    return nil
}
```

This pattern:

- Limits body size at the reader level (rejects >1MB before parsing)
- Rejects unknown fields (prevents schema injection)
- Returns a clear error on malformed input

### ✅ Separate request DTOs from domain models

Never `Unmarshal` directly into a domain object. Have a dedicated request struct per endpoint:

```go
// Request DTO — explicit about what the endpoint accepts
type CreateUserRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

// Domain model — never exposed to Unmarshal
type User struct {
    ID           string
    Email        string
    PasswordHash []byte
    IsAdmin      bool  // never settable from request
    CreatedAt    time.Time
}
```

## False positive patterns

Do NOT flag:

- `json.Unmarshal` on constants, config files from trusted paths, or internal messages signed/authenticated at a higher layer
- Protobuf parsing on mTLS-authenticated gRPC — the input is not attacker-controlled
- `yaml.Unmarshal` on Go-authored configs (YAML is dangerous for untrusted input; it's fine for CI configs the team controls)

## Reachability check (for Phase 4)

For HIGH confidence on untrusted-deserialization:

1. Confirm the data source is attacker-controllable (HTTP body, user upload, external API response)
2. Confirm no input authentication/validation happens before parsing (signed payload, schema validation, etc.)
3. Confirm missing size limits, missing `DisallowUnknownFields`, or `interface{}` fields at the unmarshal boundary
