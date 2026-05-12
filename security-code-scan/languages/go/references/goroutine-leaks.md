# Goroutine Leaks and Race Conditions

Go's concurrency model is the source of its most subtle security bugs. These are not memory-safety bugs in the C/C++ sense — they are logic bugs that an LLM trained on C patterns will miss.

Loaded by Phase 3 when the reviewed code uses goroutines, channels, or shared mutable state.

## The top footguns

### 1. Goroutine leaks from missing context cancellation

```go
// BAD
func handleRequest(w http.ResponseWriter, r *http.Request) {
    go fetchFromBackend(r.Body)  // no context, no cancellation path
    w.Write([]byte("queued"))
}

// GOOD
func handleRequest(w http.ResponseWriter, r *http.Request) {
    go fetchFromBackend(r.Context(), r.Body)  // tied to request lifecycle
    w.Write([]byte("queued"))
}
```

**Why it matters for security:** leaked goroutines hold resources (file handles, DB connections, memory). An attacker who can trigger handler code repeatedly can create a DoS by exhausting these. Not a memory bug, but a resource-exhaustion vulnerability.

**Detection:** grep for `go ` statements inside HTTP handlers where the goroutine doesn't accept a `context.Context`.

### 2. Race conditions on shared maps

```go
// BAD — concurrent map access panics at runtime or silently corrupts
var cache = map[string]string{}

func get(k string) string { return cache[k] }
func set(k, v string) { cache[k] = v }
```

**Why it matters for security:** logic bugs that only manifest under concurrent access are often exploitable. Authorization decisions computed against a map that's being concurrently updated can produce incorrect results in a window the attacker can hit.

**Detection:** grep for package-level `map` variables that don't have an accompanying `sync.Mutex` or aren't `sync.Map`. Also flag shared slices, counters, and any package-level mutable state.

**Verification:** this is a case where `go test -race` output is ground truth. If the race detector confirms it, it's a real bug. If the race detector doesn't confirm it, the LLM is likely hallucinating — do not report without race-detector confirmation.

### 3. Time-of-check to time-of-use (TOCTOU)

```go
// BAD
if fileIsSafe(path) {
    // attacker swaps the file here
    contents, _ := os.ReadFile(path)
    process(contents)
}

// GOOD
f, err := os.OpenFile(path, os.O_RDONLY, 0)
if err != nil { return err }
defer f.Close()
// check the file handle, not the path — now protected by the open handle
```

**Detection:** grep for patterns where a validation call is followed by a separate operation on the same path or resource. Classic TOCTOU.

### 4. Channel send on closed channel

```go
// BAD — panic
ch := make(chan int)
close(ch)
ch <- 1  // runtime panic, crashes the process

// Worse: in a concurrent service, an attacker can trigger this via a race
// between a graceful shutdown (close) and an in-flight request (send).
```

**Detection:** grep for `close(ch)` calls. Check if the same channel is used for `ch <- ...` anywhere that could race with the close.

### 5. sync.Map isn't a drop-in replacement for map[K]V

`sync.Map` is optimized for specific access patterns (write-once, read-many). Using it as a general-purpose concurrent map can have performance cliffs that create denial-of-service windows.

**Detection:** flag `sync.Map` usage where keys are frequently written after first use. Recommend `map + sync.RWMutex` instead.

## HTTP-specific concurrency bugs

### Missing server timeouts → Slowloris

```go
// BAD
http.ListenAndServe(":8080", handler)

// GOOD
srv := &http.Server{
    Addr:              ":8080",
    Handler:           handler,
    ReadTimeout:       5 * time.Second,
    ReadHeaderTimeout: 2 * time.Second,
    WriteTimeout:      10 * time.Second,
    IdleTimeout:       30 * time.Second,
    MaxHeaderBytes:    1 << 20,
}
srv.ListenAndServe()
```

**Detection:** grep for `http.ListenAndServe` (not `(&http.Server{...}).ListenAndServe`). Every production HTTP server needs explicit timeouts. Without them, an attacker can hold connections open indefinitely (Slowloris) with a handful of connections and exhaust the server.

### Unbounded request body reads

```go
// BAD
body, _ := io.ReadAll(r.Body)

// GOOD
body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))  // 1MB cap
```

**Detection:** grep for `io.ReadAll(r.Body)` or `ioutil.ReadAll(r.Body)` without a `LimitReader` wrapper.

## Cross-reference

- CWE-362 (Race Condition)
- CWE-400 (Uncontrolled Resource Consumption) — goroutine leaks, missing timeouts
- CWE-664 (Improper Control of a Resource Through its Lifetime)
- CWE-667 (Improper Locking)
- CWE-691 (Insufficient Control Flow Management)

## Verification rule

Race-condition and goroutine-leak findings from Phase 3 should be cross-checked against `go test -race ./...` output in Phase 4. If the race detector does not confirm the finding, demote confidence. LLMs frequently over-report these patterns.
