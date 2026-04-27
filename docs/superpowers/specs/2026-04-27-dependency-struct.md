# Dependency struct redesign

## Summary

The current `Dependency` struct has two fields: `Name` and `Section`. `Name` has no contract — parsers embed package names, version strings, version operators, and sometimes constraint ranges inside it, using ecosystem-specific separators (`@`, `==`, raw PEP 508 lines). This makes the struct hard to consume programmatically.

This design introduces a `Raw` field that takes over the role the current `Name` field plays today — a free-form string with no contract. All parsers are updated to write to `Raw` instead of `Name`. A new `Name` field is introduced with a strict contract: the bare package identifier only.

## Motivation

Any consumer that wants only the package name currently has to reverse-parse a string like `serde@1.0.0`, `requests==2.32.3`, or `requests>=2.28.0,<3`. The parsing logic differs by ecosystem. There is no stable way to extract just the package identifier without knowing which parser produced the entry.

Additionally, some parsers silently discard available data:

- `poetry-lock` and `uv-lock` drop git-sourced and local-path dependencies entirely
- `go-mod` discards the resolved version from `require` lines
- `pnpm-lock` falls back to the version specifier when a resolved version is absent, producing `name@^1.2.3` where the `^` prefix signals a range, not a resolved version
- `cargo-lock`, `poetry-lock`, and `uv-lock` have source registry or git URL data that is never surfaced

## Goals

- Introduce `Raw` as the home for the free-form strings parsers produce today, with no contract
- Give `Name` a strict, enforceable contract: bare package identifier only
- Separate version information into dedicated fields
- Surface source type for git and path dependencies instead of silently dropping them
- Provide an escape hatch for format-specific metadata that does not belong in typed fields

## Non-goals

- Modeling the full dependency graph (direct vs transitive relationships, dependency trees)
- Adding ecosystem-specific fields to the struct itself

## Final struct

```go
type Dependency struct {
    Raw        string            `json:"raw"`
    Name       string            `json:"name,omitempty"`
    Version    string            `json:"version,omitempty"`
    Constraint string            `json:"constraint,omitempty"`
    Section    string            `json:"section,omitempty"`
    Source     string            `json:"source,omitempty"`
    Extras     map[string]string `json:"extras,omitempty"`
}
```

## Field contracts

### `Raw`

The free-form string the parser produces. No format is guaranteed. This is `name@version` in npm/cargo/composer parsers, `name==version` in Python lockfile parsers, a raw PEP 508 line in `requirements.txt`, and a bare module path in `go-mod`. The value is whatever the underlying file format provides, interpreted as minimally as possible.

This field is always set. It is never empty on an emitted dependency.

### `Name`

The package identifier as it is known in its registry. No version operators, no version string, no extras notation, no whitespace. Examples:

- npm: `@babel/core`, `react`
- Python: `requests`, `pydantic`
- Go: `github.com/BurntSushi/toml`
- Rust: `serde`
- PHP: `vendor/package`

This field must not contain `@`, `==`, `>=`, `<=`, `>`, `<`, `!=`, `~`, `^`, or spaces unless they are structural parts of the canonical package identifier (e.g. a scoped npm package like `@babel/core` contains `@` as a prefix, which is part of the name).

Omitted until the parser for this manifest type has been migrated to populate structured fields.

### `Version`

The exact resolved version string with no operators. Set only by lockfile parsers. For semver ecosystems this is the three-part version (e.g. `1.2.3`). For Python this is the normalized PEP 440 version (e.g. `2.32.3`). For Go this is the module version tag (e.g. `v1.2.3`).

This field is omitted when the manifest is a source manifest rather than a lockfile, or when the dependency has no registry version (e.g. a git-sourced dependency at an untagged commit).

A consumer can rely on `Version` being directly parseable as a version string without stripping operators.

`Version` and `Constraint` are mutually exclusive. A given entry comes from either a lockfile (resolved version) or a source manifest (constraint), never both.

### `Constraint`

The version constraint or range as written in the source manifest. Set only by source manifest parsers, never by lockfile parsers. Examples: `>=2.28.0,<3`, `^1.2.3`, `~0.1`, `>=1.21`.

`Version` and `Constraint` are mutually exclusive.

### `Section`

The named group within the manifest that this dependency belongs to. This preserves source structure when the underlying format provides it. The value is taken directly from the manifest format with no normalization across ecosystems.

Examples: `dependencies`, `devDependencies`, `optionalDependencies`, `default`, `develop`, `packages`, `packages-dev`.

Omitted when the manifest format does not provide grouping.

### `Source`

The origin type of the dependency. One of:

| Value | Meaning |
|---|---|
| `registry` | Resolved from a package registry (npm, PyPI, crates.io, etc.) |
| `git` | Resolved from a git repository |
| `path` | Resolved from a local filesystem path |
| `url` | Resolved from an arbitrary URL |

Omitted when the source is unknown. When `Source` is `git` or `url`, the specific location and ref may be present in `Extras` under `source_url` and `source_ref`.

### `Extras`

A string-to-string map of format-specific metadata. Values are always strings; non-string source data is stringified.

The rule for what belongs here: if a consumer would branch on the value for core logic, it belongs in a typed field instead. `Extras` is for pass-through metadata that consumers might want to log or forward but would not branch on.

Known keys by ecosystem:

| Key | Set by | Meaning |
|---|---|---|
| `checksum` | `cargo-lock` | SHA256 checksum of the crate source archive |
| `source_url` | `poetry-lock`, `uv-lock`, `cargo-lock` | URL of the git repository or direct URL source |
| `source_ref` | `poetry-lock`, `cargo-lock` | Git commit hash or ref for the resolved source |
| `specifier` | `pnpm-lock` | The version specifier from `package.json` (e.g. `^1.2.3`) when a resolved version is also available in `Version` |
| `package_type` | `composer-lock` | Composer package type (e.g. `library`, `metapackage`, `composer-plugin`) |

## Parser migration table

Migration happens in two steps for every parser:

1. **Rename**: write the existing string to `Raw` instead of `Name`. No other logic changes. `Raw` value is identical to what `Name` held before.
2. **Structured**: populate `Name`, `Version` or `Constraint`, and any other applicable fields. Both steps can land in the same change or separately.

### Lockfile parsers

| Parser | `Raw` (unchanged value) | `Name` after | `Version` after |
|---|---|---|---|
| `cargo-lock` | `serde@1.0.0` | `serde` | `1.0.0` |
| `poetry-lock` | `requests==2.32.3` | `requests` | `2.32.3` |
| `uv-lock` | `requests==2.32.3` | `requests` | `2.32.3` |
| `pipfile-lock` | `requests==2.32.3` | `requests` | `2.32.3` |
| `composer-lock` | `vendor/pkg@1.0.0` | `vendor/pkg` | `1.0.0` |
| `package-lock` | `react@18.0.0` | `react` | `18.0.0` |
| `yarn-lock` | `react@18.0.0` | `react` | `18.0.0` |
| `pnpm-lock` | `react@18.0.0` | `react` | `18.0.0` |

### Source manifest parsers

| Parser | `Raw` (unchanged value) | `Name` after | `Constraint` after |
|---|---|---|---|
| `py-requirements` | `requests>=2.28.0,<3` | `requests` | `>=2.28.0,<3` |
| `toml` / `pyproject.toml` | `requests>=2.28.0` | `requests` | `>=2.28.0` |

### Additional data to surface

| Parser | Change |
|---|---|
| `go-mod` | Add `Version` from `req.Mod.Version` (e.g. `v1.2.3`) |
| `poetry-lock` (git deps) | Stop dropping git entries; emit with `Source: "git"`, `Extras.source_url` set, no `Version` |
| `uv-lock` (path deps) | Stop dropping non-self path entries; emit with `Source: "path"` |
| `package-lock` v2/v3 | Populate `Section` for `devDependencies` and `optionalDependencies` |
| `cargo-lock` | Populate `Extras.checksum` and `Source` / `Extras.source_url` from the `source` field |
| `pnpm-lock` | Populate `Extras.specifier` when specifier and resolved version are both available |
| `composer-lock` | Populate `Extras.package_type` |

## Render behavior

The human-readable renderer currently displays `dependency.Name` directly. After the rename step it should display `Raw`. After parsers are fully migrated it should prefer `Name` + `Version` for display (e.g. `serde@1.0.0`), falling back to `Raw` for any parser not yet migrated. This keeps visual output consistent across the migration window.

## Success criteria

- All parsers write to `Raw` instead of `Name`; `Raw` values are identical to the current `Name` values
- All lockfile parsers set `Name` to the bare package identifier and `Version` to the resolved version string with no operators
- All source manifest parsers set `Name` to the bare package identifier and `Constraint` to the version range
- No parser embeds version operators or separators in `Name`
- Git and path dependencies are reported with `Source` set rather than silently dropped
- Human-readable output remains visually equivalent to today for all migrated parsers
