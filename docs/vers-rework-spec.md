# VERS normalization rework

## Objective

Replace the local conversion of `deps.dev/util/semver` range-set text with direct VERS serialization from `github.com/git-pkgs/vers`. The resulting `Dependency.VERS` field remains an optional, derived JSON field named `vers`.

This rework makes one library responsible for parsing native constraints and serializing canonical VERS URIs. `deplens` remains responsible only for deciding when normalization applies and mapping dependency types to parsing schemes.

## Compatibility and public behavior

- Raise the repository's minimum Go version from `1.25.0` to `1.25.6`.
- Replace `deps.dev/util/semver` with `github.com/git-pkgs/vers v0.3.0` in `go.mod` and `go.sum`.
- Preserve the existing `Dependency` API:

  ```go
  VERS string `json:"vers,omitempty"`
  ```

- Preserve `Dependency.Constraint` exactly as extracted from the manifest.
- Set `Dependency.VERS` only when all of the following are true:
  1. `Constraint` is non-empty.
  2. `Type` is a supported ecosystem type.
  3. The native constraint parses successfully.
- Do not derive VERS from `Version`; lockfile and `go.mod` resolved versions remain VERS-free unless they also carry a constraint.
- On unknown type, blank constraint, or parsing failure, leave `VERS` empty without a scan warning or detector failure.

## Implementation

Create or retain a small `internal/analyze/vers.go` adapter with this boundary:

```go
func applyDependencyVERS(dependencies []Dependency)
func dependencyVERS(packageType PackageType, constraint string) string
func versSchemeForPackageType(packageType PackageType) (nativeScheme, outputScheme string, ok bool)
```

For a supported dependency, normalization must use the VERS library directly:

```go
parsed, err := vers.ParseNative(constraint, nativeScheme)
if err != nil {
    return ""
}
return vers.ToVersString(parsed, outputScheme)
```

`applyDependencyVERS` must continue to run in `Ruleset.detectManifestFile` immediately after `applyDependencyType`. No detector-specific VERS logic is permitted.

### Type mapping

| Dependency type | Native parser scheme | Emitted VERS scheme |
| --- | --- | --- |
| `pypi` | `pypi` | `pypi` |
| `golang` | `golang` | `golang` |
| `npm` | `npm` | `npm` |
| `maven` | `maven` | `maven` |
| `cargo` | `cargo` | `cargo` |
| `gem` | `gem` | `gem` |

`composer` is intentionally unsupported for VERS normalization. Composer manifests and lockfiles retain their existing detection and extraction behavior; dependencies of type `composer` simply omit `VERS`.

## Removal

Remove all use of `deps.dev/util/semver`, including:

- the dependency entry and checksums;
- `semver.System` mapping;
- `formatVERS`;
- finite/infinite bound conversion helpers;
- tests that exercise the former range-set text format.

## Tests and acceptance criteria

- Retain a table-driven corpus of exactly 30 native constraints spanning PyPI, Go, npm, Maven, RubyGems, and Cargo.
- For each case, assert the exact emitted VERS string, not only its scheme prefix.
- For representative constraints in every ecosystem, parse the emitted VERS URI with `vers.Parse` and verify an included and an excluded version with `Range.Contains`.
- Include explicit cases for caret, tilde/pessimistic, wildcard, exact, bounded, exclusion, and union/range syntax where the ecosystem supports them.
- Verify invalid, blank, and unsupported constraints omit `VERS` while retaining `Constraint`.
- Verify a resolved `Version` without `Constraint` omits `VERS`.
- Keep scan-level coverage showing typed Python dependencies receive VERS and JSON golden output includes it.
- Run:

  ```bash
  go test ./...
  git diff --check
  ```

## Documentation

Keep the README JSON-output description explicit that `vers` is derived, optional, and omitted for unsupported or invalid constraints and version-only dependencies. Include a concrete example such as:

```json
{
  "type": "pypi",
  "raw": "requests>=2.31",
  "name": "requests",
  "constraint": ">=2.31",
  "vers": "vers:pypi/>=2.31"
}
```

Use the exact string emitted by the pinned library version in the final documentation and golden tests.
