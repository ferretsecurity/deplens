# Dependency struct redesign

## Summary

The current `Dependency` struct has two fields: `Name` and `Section`. `Name` has no contract — parsers embed package names, version strings, version operators, and sometimes constraint ranges inside it, using ecosystem-specific separators. This makes the struct hard to consume programmatically.

This design introduces structured fields alongside the existing `Name` field, which is preserved intact for backward compatibility under the JSON key `"name"`. A new typed field for the bare package identifier is introduced under the JSON key `"package"`. All new fields are additive and opt-in per parser.

## Motivation

Consumers of the JSON output who want only the package name currently need to reverse-parse a string like `serde@1.0.0`, `requests==2.32.3`, or `requests>=2.28.0,<3` — and the parsing logic differs by ecosystem. There is no stable way to extract just the package identifier without knowing which parser produced the entry.

Additionally, some parsers silently discard available data:

- `poetry-lock` and `uv-lock` drop git-sourced and local-path dependencies entirely
- `go-mod` discards the resolved version from `require` lines
- `pnpm-lock` falls back to the version specifier when a resolved version is absent, producing `name@^1.2.3` where the `^` prefix signals a range, not a resolved version
- `cargo-lock`, `poetry-lock`, and `uv-lock` have source registry or git URL data that is never surfaced

## Goals

- Add structured fields that individual parsers can populate incrementally
- Preserve the existing `"name"` JSON key and its current behavior to avoid breaking existing consumers
- Give each new field a clear, enforceable contract
- Allow parsers to be migrated one at a time with no coordination required
- Never require a breaking JSON schema change for the transition

## Non-goals

- Changing any existing `"name"` values or JSON output structure before parsers explicitly opt in
- Modeling the full dependency graph (direct vs transitive relationships, dependency trees)
- Adding ecosystem-specific fields to the struct itself

## Final struct

```go
type Dependency struct {
    Raw        string            `json:"name"`                  // see contract below
    Name       string            `json:"package,omitempty"`     // see contract below
    Version    string            `json:"version,omitempty"`     // see contract below
    Constraint string            `json:"constraint,omitempty"`  // see contract below
    Section    string            `json:"section,omitempty"`
    Source     string            `json:"source,omitempty"`
    Extras     map[string]string `json:"extras,omitempty"`
}
```

## Field contracts

### `Raw` (`json:"name"`)

The free-form string the parser chose to emit. No format is guaranteed. Today this is `name@version` in npm/cargo/composer parsers, `name==version` in Python lockfile parsers, a raw PEP 508 line in `requirements.txt`, and a bare module path in `go-mod`. This field will keep its current value for every parser until that parser explicitly migrates to the new fields.

Consumers that need to display a dependency without structured parsing should read this field. Consumers that need to process the package name or version programmatically should prefer `Name` and `Version` once those are populated.

This field is always set. It is never empty on an emitted dependency.

### `Name` (`json:"package"`)

The package identifier as it is known in its registry. No version operators, no version string, no extras notation. For npm this is the package name including any scope prefix (e.g. `@babel/core`). For Python this is the distribution name (e.g. `requests`). For Go this is the module path (e.g. `github.com/BurntSushi/toml`). For Rust this is the crate name.

This field is omitted until the parser for this manifest type has been migrated. Consumers should check `package != ""` before using it.

This field must not contain `@`, `==`, `>=`, `<=`, `>`, `<`, `!=`, `~`, `^`, or spaces unless they are part of the canonical package identifier itself (e.g. a Go module path never contains those characters; a Python distribution name never contains them either).

### `Version` (`json:"version"`)

The exact resolved version string with no operators. Set only by lockfile parsers. For semver ecosystems this is the three-part version (e.g. `1.2.3`). For Python this is the normalized PEP 440 version (e.g. `2.32.3`). For Go this is the module version tag (e.g. `v1.2.3`).

This field is omitted when the manifest is a source manifest rather than a lockfile, when the parser has not yet been migrated, or when the dependency has no registry version (e.g. a git-sourced dependency at an untagged commit).

A consumer can rely on `Version` being directly parseable as a version string without stripping operators.

### `Constraint` (`json:"constraint"`)

The version constraint or range as written in the source manifest. Set only by source manifest parsers, never by lockfile parsers. Examples: `>=2.28.0,<3`, `^1.2.3`, `~0.1`, `>=1.21`.

This field and `Version` are mutually exclusive: a given dependency entry will have one or the other, not both, because an entry comes from either a lockfile (resolved version) or a source manifest (constraint).

### `Section` (`json:"section"`)

The named group within the manifest that this dependency belongs to. This field preserves source structure when the underlying format provides it. Examples: `dependencies`, `devDependencies`, `optionalDependencies`, `default`, `develop`, `packages`, `packages-dev`.

The value is taken directly from the manifest format with no normalization across ecosystems. If two formats use different labels for the same semantic concept (e.g. npm `devDependencies` and Pipfile `develop`), those labels will differ in output.

This field is omitted when the manifest format does not provide grouping, or when the parser has not been migrated to populate it for this manifest type.

### `Source` (`json:"source"`)

The origin type of the dependency. One of:

| Value | Meaning |
|---|---|
| `registry` | Resolved from a package registry (npm, PyPI, crates.io, etc.) |
| `git` | Resolved from a git repository |
| `path` | Resolved from a local filesystem path |
| `url` | Resolved from an arbitrary URL |

This field is omitted when the source is unknown or when the parser has not been migrated to populate it.

When `Source` is `git` or `url`, the specific URL or ref may be present in `Extras` under keys `source_url` and `source_ref`.

### `Extras` (`json:"extras"`)

A string-to-string map of format-specific metadata that does not fit into the typed fields above. Values are always strings; non-string source data (booleans, integers) is stringified.

Consumers should not branch on values in `Extras` for core logic. If a consumer needs to branch on a value (e.g. `if extras["source_type"] == "git"`), that value belongs in a typed field instead. `Extras` is for pass-through metadata: checksums, git refs, content hashes, package type labels, and similar.

Known keys by ecosystem:

| Key | Set by | Meaning |
|---|---|---|
| `checksum` | `cargo-lock` | SHA256 checksum of the crate source archive |
| `source_url` | `poetry-lock`, `uv-lock`, `cargo-lock` | URL of the git repository or direct URL source |
| `source_ref` | `poetry-lock`, `cargo-lock` | Git commit hash or ref for the resolved source |
| `specifier` | `pnpm-lock` | The version specifier from `package.json` (e.g. `^1.2.3`) when the resolved version is also available separately in `Version` |
| `package_type` | `composer-lock` | Composer package type (e.g. `library`, `metapackage`, `composer-plugin`) |

## Migration approach

Every parser continues writing to `Raw` exactly as it does today. No existing behavior changes until a parser explicitly opts in to the new fields.

Parsers are migrated one at a time. A migrated parser populates `Name` and `Version` (or `Constraint`) in addition to `Raw`. During the migration window, `Raw` and the structured fields may be redundant (e.g. `Raw: "serde@1.0.0"`, `Name: "serde"`, `Version: "1.0.0"`). This redundancy is intentional: it lets consumers switch to the new fields at their own pace without a coordinated cutover.

The JSON key `"name"` is never renamed. The old value format in `"name"` is never removed until a future breaking major version, if at all.

### Migration priority order

Priority is based on how many consumers benefit from structured data and how much information is currently lost.

**High priority — lockfile parsers that embed version in name:**

| Parser | `Raw` today | `Name` after | `Version` after |
|---|---|---|---|
| `cargo-lock` | `serde@1.0.0` | `serde` | `1.0.0` |
| `poetry-lock` | `requests==2.32.3` | `requests` | `2.32.3` |
| `uv-lock` | `requests==2.32.3` | `requests` | `2.32.3` |
| `pipfile-lock` | `requests==2.32.3` | `requests` | `2.32.3` |
| `composer-lock` | `vendor/pkg@1.0.0` | `vendor/pkg` | `1.0.0` |
| `package-lock` | `react@18.0.0` | `react` | `18.0.0` |
| `yarn-lock` | `react@18.0.0` | `react` | `18.0.0` |
| `pnpm-lock` | `react@18.0.0` | `react` | `18.0.0` |

**Medium priority — source manifest parsers and currently-missing data:**

| Parser | Change |
|---|---|
| `go-mod` | Add `Version` from `req.Mod.Version` (e.g. `v1.2.3`); `Name` stays the module path |
| `py-requirements` | Split PEP 508 line: `Name` = distribution name, `Constraint` = specifier |
| `poetry-lock` (git deps) | Stop silently dropping git entries; emit with `Source: "git"`, no `Version`, `Extras.source_url` set |
| `uv-lock` (path deps) | Stop silently dropping non-self path entries; emit with `Source: "path"` |
| `package-lock` v2/v3 | Populate `Section` for `devDependencies` and `optionalDependencies` |
| `cargo-lock` | Populate `Extras.checksum` and `Source` / `Extras.source_url` from the `source` field |

**Lower priority — enrichment:**

| Parser | Change |
|---|---|
| `pnpm-lock` | Populate `Extras.specifier` when specifier and version are both available |
| `composer-lock` | Populate `Extras.package_type` |
| `toml` / `pyproject.toml` | Populate `Constraint` for PEP 621 and Poetry source manifests |

## Render behavior

The human-readable renderer currently displays `dependency.Raw` (the `Name` field today) directly. After migration, the renderer should prefer `Name + Version` when both are available, falling back to `Raw` for unmigrated parsers. This keeps the human output stable across the migration window.

Example: a migrated cargo entry with `Name: "serde"` and `Version: "1.0.0"` renders as `serde@1.0.0`, identical to today. An unmigrated entry renders `Raw` as before.

## Success criteria

This design is complete when:

- All lockfile parsers populate `Name` and `Version` separately
- No lockfile parser embeds version operators or separators in `Raw`
- Source manifest parsers populate `Name` and `Constraint` separately
- Git and path dependencies are reported rather than silently dropped, with `Source` set
- The JSON `"name"` key value is unchanged for every parser until that parser is migrated
- No existing test breaks as a result of adding new fields
