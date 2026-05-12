# Cryptography

Loaded by Phase 3 when the reviewed code performs encryption, hashing, signing, random number generation, or TLS operations.

**CWE:** CWE-327 (Use of Broken/Risky Cryptographic Algorithm), CWE-328 (Use of Weak Hash), CWE-330 (Use of Insufficiently Random Values), CWE-326 (Inadequate Encryption Strength), CWE-295 (Improper Certificate Validation), CWE-759 (Use of One-Way Hash without a Salt), CWE-798 (Use of Hard-coded Credentials)

**OWASP:** A02:2021 — Cryptographic Failures

## The common bug classes

### 1. Weak or broken algorithms

- **MD5, SHA-1** — collisions are practical. Never use for security purposes.
- **DES, 3DES, RC4** — broken, do not use.
- **ECB mode for block ciphers** — leaks plaintext patterns.
- **CBC mode without authentication** — padding oracle attacks.

**Detection:** grep for `md5.`, `sha1.`, `des.`, `rc4.`. Grep for `cipher.NewCBCEncrypter` without an accompanying HMAC.

**Safe defaults in Go:**

- Hashing for integrity: `sha256.Sum256` or `sha512.Sum512`
- Password hashing: `golang.org/x/crypto/bcrypt`, `argon2`, or `scrypt`
- Symmetric encryption: AES-256-GCM via `crypto/aes` + `cipher.NewGCM`
- Asymmetric: `crypto/rsa` (2048+ bits) or `crypto/ecdsa` (P-256+)

### 2. Insecure randomness

```go
// BAD — math/rand is deterministic, predictable from seed
import "math/rand"
token := strconv.Itoa(rand.Int())

// GOOD — crypto/rand is cryptographically secure
import "crypto/rand"
b := make([]byte, 32)
_, _ = rand.Read(b)
token := base64.URLEncoding.EncodeToString(b)
```

**Detection:** grep for `math/rand` being used to generate anything security-sensitive (tokens, session IDs, passwords, salts, nonces, IVs). The import alone is a signal.

**Subtle bug:** `math/rand` is fine for jitter, shuffling non-sensitive data, game RNG. Context matters.

### 3. Hardcoded secrets

```go
// BAD
const apiKey = "sk_live_abc123..."
var jwtSecret = []byte("my-secret-key")
```

**Detection:** grep for patterns like `secret`, `password`, `api_key`, `token` assigned to string literals. Also check for keys in `const` blocks, `var` declarations, and struct defaults.

**Tool overlap:** `gosec` has rule G101 for this. If gosec already flagged it, reference the finding — don't duplicate.

### 4. Missing or incorrect TLS verification

```go
// BAD
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
    },
}

// BAD — accepting any cert via custom VerifyPeerCertificate that returns nil
```

**Detection:** grep for `InsecureSkipVerify: true` and `VerifyPeerCertificate` that doesn't perform actual validation.

**Exception:** test code may legitimately use `InsecureSkipVerify` for test servers with self-signed certs. Confirm the file is a test file (`_test.go`) before demoting the finding.

### 5. Weak password hashing

```go
// BAD — SHA-256 is not a password hash
h := sha256.Sum256([]byte(password))

// GOOD — bcrypt handles salt, iteration count, and format
hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
```

**Detection:** grep for password-like variables being hashed with `sha256`, `sha512`, `md5`, or plain hex encoding.

### 6. Missing authenticated encryption

Encryption without integrity protection is broken in modern threat models.

**Detection:** if you see `aes.NewCipher` followed by any mode other than `cipher.NewGCM` (e.g., `NewCBCEncrypter`, `NewCFBEncrypter`), verify that an HMAC is computed separately over the ciphertext. If not, flag as HIGH.

### 7. IV / nonce reuse

AES-GCM nonce must never be reused with the same key. Reusing it leaks plaintext and breaks authentication.

**Detection:** grep for hardcoded nonces (all-zeros, constants). Check that nonces are generated with `crypto/rand.Read` for each encryption.

## False positive patterns

Do NOT flag:

- `md5` or `sha1` used for non-security purposes (git SHAs, ETags, cache keys, file dedup) — if the context makes it clear the hash isn't protecting anything
- `math/rand` in test code, benchmarks, or fuzz corpus generation
- `InsecureSkipVerify` in test files with self-signed certs
- Hardcoded "secrets" that are clearly placeholder values (`"changeme"`, `"YOUR_KEY_HERE"`) — flag the template/documentation issue, not as a live vulnerability

## Reachability check (for Phase 4)

Crypto bugs are generally HIGH confidence once the pattern is confirmed — the bug is structural, not dependent on reachability. However:

- Hardcoded secrets in test files or example code are LOWER severity than in production code
- Weak algorithms in deprecation-path code (old API versions being removed) are LOWER severity than in new code
- Use of `math/rand` in a jitter calculation is LOWER severity than in token generation

The question for Phase 4 is usually "is this the security-critical use?", not "can attacker input reach this?".
