# Authentication & Authorization

Loaded by Phase 3 when the reviewed code handles login, sessions, tokens, permissions, or access control decisions.

**CWE:** CWE-287 (Improper Authentication), CWE-285 (Improper Authorization), CWE-639 (Authorization Bypass Through User-Controlled Key — IDOR), CWE-306 (Missing Authentication for Critical Function), CWE-862 (Missing Authorization), CWE-863 (Incorrect Authorization), CWE-384 (Session Fixation), CWE-613 (Insufficient Session Expiration)

**OWASP:** A01:2021 — Broken Access Control, A07:2021 — Identification and Authentication Failures

## The four classes of authn/authz bugs

### 1. Missing authentication (CWE-306)

An endpoint is registered without any auth middleware.

**Detection:** for each HTTP route, trace which middleware runs before the handler. Flag any route where no authentication middleware is in the chain.

**Common in Go:** mixing auth'd and un-auth'd routes in the same router group — easy to accidentally drop a route outside the `.Use(authMiddleware)` group.

### 2. Missing authorization (CWE-862/863)

Authenticated, but no check of *what this user is allowed to do*.

**Detection:** the endpoint reads a resource by ID. Does the code verify the authenticated user owns/can access that ID? If not, IDOR.

**Red flag pattern:**

```go
func getOrder(w http.ResponseWriter, r *http.Request) {
    orderID := r.URL.Query().Get("id")
    order := db.FindOrder(orderID)  // no ownership check
    json.NewEncoder(w).Encode(order)
}
```

### 3. Session management bugs

- **Session fixation** (CWE-384): session ID not regenerated after login
- **Insufficient expiration** (CWE-613): long-lived sessions with no timeout
- **Missing secure/HttpOnly flags** on session cookies
- **Session ID in URL** (leaks via Referer, logs)

**Detection:** grep for `http.SetCookie` and `http.Cookie{...}`. Verify `Secure: true`, `HttpOnly: true`, `SameSite: http.SameSiteLaxMode` (or Strict), and an explicit `MaxAge` or `Expires`.

### 4. Token validation bugs

For JWT, OAuth, API keys:

- **JWT `alg: none` accepted** — library misuse
- **Signature not verified** — token decoded but not validated
- **Expiration not checked**
- **Audience/issuer not checked** (JWT replay across services)
- **HMAC secret weak or hardcoded**

**Detection:** grep for `jwt.Parse`, `jwt.ParseWithClaims`. Verify the key function enforces algorithm (`token.Method.Alg()` check), and the token's `Valid` field is checked.

## Common patterns in Go HTTP services

### ✅ Auth middleware pattern

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractToken(r)
        claims, err := verifyToken(token)
        if err != nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        ctx := context.WithValue(r.Context(), userCtxKey, claims.UserID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### ✅ Authorization check inside the handler

```go
func getOrder(w http.ResponseWriter, r *http.Request) {
    callerID := r.Context().Value(userCtxKey).(string)
    orderID := chi.URLParam(r, "id")

    order, err := db.FindOrder(r.Context(), orderID)
    if err != nil { /* ... */ }

    if order.OwnerID != callerID {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    json.NewEncoder(w).Encode(order)
}
```

## False positive patterns

Do NOT flag:

- Health-check endpoints (`/healthz`, `/ready`) — these intentionally don't auth
- Internal service-to-service endpoints protected by mTLS or network policy (verify the deployment context before reporting)
- Public documentation endpoints

Do NOT flag session-management issues if the service uses a well-known framework that handles it automatically (e.g., `gin-contrib/sessions` with defaults) — but DO verify the framework's defaults are secure for the current version.

## Reachability check (for Phase 4)

For HIGH confidence on an IDOR-style bug:

1. Confirm the endpoint is reachable from an authenticated but non-privileged user (not just from localhost or admin)
2. Confirm the resource identifier is user-controlled (URL path param, query string, or body)
3. Confirm no ownership check exists on the path from handler entry to database read

For HIGH confidence on missing-auth:

1. Confirm the route is actually exposed in the production router (not behind a disabled handler)
2. Confirm the middleware chain for that route does not contain any auth middleware
3. Confirm the handler reads or modifies sensitive data
