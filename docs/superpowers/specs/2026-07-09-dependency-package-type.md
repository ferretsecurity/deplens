# Dependency package type output

## Summary

Add a PURL/VERS-compatible package type to each extracted dependency in JSON output. The field identifies the package ecosystem or package manager using the registered Package URL type token, such as `pypi`, `npm`, `docker`, `golang`, `cargo`, `composer`, or `maven`.

The implementation should use rule-level metadata as the default source of truth. A rule that knows it detects npm dependencies declares `dependency-type: npm`; the scanner copies that value onto each dependency emitted by that rule unless a parser has already set a more specific value.

## Motivation

`deplens` already has a manifest-level `type`, but that field is a detector/rule name such as `python-requirements`, `js-npm-lock`, or `dockerfile`. It is useful for explaining why a file matched, but it is not the package ecosystem type used by PURL or VERS.

Consumers that want to build package URLs, compare version ranges, or join dependency output with vulnerability databases need the ecosystem token associated with each dependency. They should not have to infer it from manifest type names or dependency strings.

## Goals

- Add a dependency-level package type field to JSON output.
- Use PURL/VERS-compatible values, not internal detector names.
- Keep package type assignment close to rule configuration so default and custom rules can participate.
- Avoid parsing or guessing package type from dependency names or raw dependency text.
- Preserve existing human-readable output unless explicitly changed later.

## Non-goals

- Generate full PURLs.
- Generate VERS strings from constraints.
- Normalize package names according to each PURL type.
- Model mixed-ecosystem manifests beyond allowing parser-level overrides.
- Rename the existing manifest-level `type` field.

## Data model

Add a package type field to `analyze.Dependency`:

```go
type PackageType string

type Dependency struct {
    Type       PackageType       `json:"type,omitempty"`
    Raw        string            `json:"raw"`
    Name       string            `json:"name,omitempty"`
    Version    string            `json:"version,omitempty"`
    Constraint string            `json:"constraint,omitempty"`
    Section    string            `json:"section,omitempty"`
    Source     string            `json:"source,omitempty"`
    Extras     map[string]string `json:"extras,omitempty"`
}
```

The JSON field name should be `type` because the dependency object is the package object in this context, and PURL calls this component `type`.

This creates two distinct `type` fields in the JSON tree:

- `manifest.type`: the `deplens` detector/rule type, for example `python-requirements`
- `manifest.dependencies[].type`: the PURL/VERS package type, for example `pypi`

Document this distinction in `README.md` and `docs/glossary.md`.

## Field contract

`Dependency.Type` is the package ecosystem token used by PURL and, where applicable, VERS.

Examples:

| Ecosystem | Value |
|---|---|
| Python / PyPI | `pypi` |
| npm, Yarn, pnpm, package-lock | `npm` |
| Docker images | `docker` |
| Go modules | `golang` |
| Rust crates | `cargo` |
| Composer / Packagist | `composer` |
| Maven / Gradle JVM artifacts | `maven` |
| RubyGems | `gem` |
| NuGet | `nuget` |
| Swift Package Manager | `swift` |
| CocoaPods | `cocoapods` |
| Conan | `conan` |
| vcpkg | `vcpkg` |
| Conda | `conda` |
| Pub | `pub` |
| Hex | `hex` |
| CPAN | `cpan` |
| CRAN | `cran` |
| Hackage | `hackage` |
| OPAM | `opam` |
| LuaRocks | `luarocks` |
| Julia packages | `julia` |
| Generic or unregistered package type | `generic` |

The value is omitted when the rule or parser cannot state the package type confidently.

## Rule configuration

Add an optional `dependency-type` field to rule config:

```yaml
rules:
  - name: python-requirements
    dependency-type: pypi
    filename-regex: '(^|.*[^A-Za-z])requirements([^A-Za-z].*)?\.(txt|in)$'
    py-requirements: {}

  - name: js-npm-lock
    dependency-type: npm
    filename-regex: '^package-lock\.json$'
    package-lock: {}
```

Compile the value into `manifestRule`:

```go
type manifestRule struct {
    Type           ManifestType
    DependencyType PackageType
    FilenameRegexp *regexp.Regexp
    PathGlob       string
    Parser         manifestParser
}
```

## Data flow

1. Rule loading parses and validates `dependency-type`.
2. A matching parser returns dependencies as it does today.
3. `Ruleset.detectManifestFile` applies the rule's `DependencyType` to each returned dependency whose `Type` is empty.
4. Dependencies that already have `Type` keep their parser-supplied value.
5. Selector-only or presence-only matches do not emit dependencies, so their `dependency-type` has no JSON effect until a parser extracts dependencies.

This flow allows most rules to be configured declaratively while preserving an escape hatch for mixed or special parsers.

## Validation

Validation should reject invalid configured values at rule-load time. A bad `dependency-type` in a default or custom rules file should fail fast with a clear error.

Accepted values should be an internal allowlist based on registered PURL types, with `generic` included. The initial allowlist should include at least the PURL types relevant to existing `deplens` rules:

```text
cargo, cocoapods, composer, conan, conda, cpan, cran, docker, gem,
generic, golang, hackage, hex, julia, luarocks, maven, npm, nuget,
opam, pub, pypi, swift, vcpkg
```

If a rule covers a manifest format whose dependencies are not clearly a package registry ecosystem, leave `dependency-type` unset rather than using a nearby language name. Examples include workspace/configuration files that indicate dependency presence but do not extract package records.

## Default rule mapping policy

Add `dependency-type` only to rules whose extracted dependencies have a clear package ecosystem. Do not try to annotate every detected manifest in the first implementation.

Initial mappings should cover the extracted-dependency rules first:

| Rule/parser family | Dependency type |
|---|---|
| `py-requirements`, `pipfile-lock`, `poetry-lock`, `uv-lock`, Python TOML/INI/setup extractors | `pypi` |
| `package-lock`, `npm-shrinkwrap`, `yarn-lock`, `pnpm-lock`, `package.json` extraction rules if present | `npm` |
| `go-mod` | `golang` |
| `cargo-lock` and Cargo manifest extraction | `cargo` |
| `composer-lock` | `composer` |
| Docker image extraction | `docker` |

Presence-only rules may also carry `dependency-type` when the rule clearly belongs to one ecosystem, but that is not required for the first user-visible change because no dependency objects are emitted.

## Parser override policy

Most parsers should not set `Dependency.Type`; they should rely on rule metadata.

Parsers may set `Dependency.Type` only when a single manifest can emit different package ecosystems or when a specific dependency source changes the correct PURL type. If such a case appears, parser tests must cover both the default rule-level assignment and the override.

## JSON behavior

Before:

```json
{
  "type": "python-requirements",
  "path": "requirements.txt",
  "dependencies": [
    {
      "raw": "requests>=2.31",
      "name": "requests",
      "constraint": ">=2.31"
    }
  ],
  "has_dependencies": true
}
```

After:

```json
{
  "type": "python-requirements",
  "path": "requirements.txt",
  "dependencies": [
    {
      "type": "pypi",
      "raw": "requests>=2.31",
      "name": "requests",
      "constraint": ">=2.31"
    }
  ],
  "has_dependencies": true
}
```

For npm:

```json
{
  "type": "js-npm-lock",
  "path": "package-lock.json",
  "dependencies": [
    {
      "type": "npm",
      "raw": "react@18.2.0",
      "name": "react",
      "version": "18.2.0"
    }
  ],
  "has_dependencies": true
}
```

## Human-readable output

Do not change human-readable output in this feature. The package type is intended for machine-readable consumers first.

If human output later needs this information, it should be a separate design because adding type prefixes to every dependency line changes the main CLI display substantially.

## Documentation

Update `README.md` JSON output documentation to list the new optional dependency field:

- `type`: PURL/VERS package type, such as `pypi`, `npm`, or `docker`

Update `docs/glossary.md` with a short entry that distinguishes:

- manifest `type`: the `deplens` detector/rule type
- dependency `type`: the PURL/VERS package type

Because this changes user-visible JSON behavior, any PR for the implementation should include a concrete before/after JSON example.

## Testing

Add unit tests for:

- Rule loading accepts known PURL-compatible `dependency-type` values.
- Rule loading rejects unknown values with a clear error.
- Rule-level `dependency-type` is copied onto extracted dependencies.
- A parser-supplied dependency type is not overwritten by the rule default.
- JSON golden output includes dependency `type` for at least one Python and one npm fixture.
- Human-readable golden output is unchanged except for unrelated ordering or existing fixture churn.

## Success criteria

- Extracted dependencies from configured rules include `dependencies[].type` in JSON.
- Values use PURL/VERS-compatible tokens such as `pypi`, `npm`, `docker`, and `golang`.
- Invalid configured package types fail during ruleset loading.
- Existing parsers do not need broad rewrites to participate.
- Existing human-readable CLI output stays stable.
