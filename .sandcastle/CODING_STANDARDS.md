# Coding Standards

## Style

- Format changed Go files with `gofmt`.
- Prefer small, explicit interfaces and clear error messages.
- Wrap errors with enough context to identify the failed operation.
- Preserve existing public types and output formats unless the issue requires a change.

## Testing

- Add focused unit tests and scan integration tests for changed detector behavior.
- Put detector fixtures under the appropriate `testdata/<ecosystem>/` directory.
- Run `go test ./...` and `go vet ./...` before committing.

## Architecture

- Follow `ARCHITECTURE.md` for data flow, parser patterns, and core types.
- Prefer declarative rules in `internal/analyze/default_rules.yaml` for structured formats.
- Use a custom `sourceAnalyzer` only when extraction cannot be expressed as a rule.
- Update `README.md` when adding a detector or changing user-visible detector behavior.
