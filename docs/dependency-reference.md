# Dependency reference

A dependency reference is one dependency value extracted from a dependency
source. It preserves a source-facing representation and adds normalized fields
when the analyzer can determine them.

Dependency references are part of the versioned JSON output. This document
describes the dependency object in JSON schema version 1.

## Examples

A direct npm declaration can contain a native constraint and its normalized
[VERS](https://github.com/package-url/vers-spec) value:

```json
{
  "package_type": "npm",
  "raw": "react@^19.0.0",
  "name": "react",
  "version_constraint": "^19.0.0",
  "vers": "vers:npm/>=19.0.0|<20.0.0",
  "source_group": "dependencies",
  "origin_kind": "registry",
  "relationship": "direct",
  "scope": "runtime"
}
```

A resolved Git dependency can instead carry source details in `attributes`:

```json
{
  "package_type": "generic",
  "raw": "example/codec@a1b2c3d4",
  "name": "example/codec",
  "version": "a1b2c3d4",
  "source_group": "dependencies",
  "origin_kind": "git",
  "relationship": "direct",
  "scope": "runtime",
  "attributes": {
    "source_url": "https://example.com/codec.git",
    "source_ref": "a1b2c3d4",
    "source_ref_kind": "revision"
  }
}
```

## Fields

All field values are strings except `attributes`, which is an object whose keys
and values are strings. Only `raw` is always emitted. Every other field is
omitted when the source or analyzer does not provide enough information. An
omitted field means unknown or not applicable; it must not be read as an empty
value or a default classification.

| Field | Meaning | Values |
| --- | --- | --- |
| `package_type` | Package ecosystem or coordinate type. | Usually a Package URL type such as `npm`, `pypi`, or `maven`; open vocabulary. See [Package types](#package-types). |
| `raw` | Source-facing dependency expression retained or assembled by the analyzer for display and traceability. | Free-form. It has no common grammar and consumers should not parse it instead of using the normalized fields. |
| `name` | Package name or coordinate in the ecosystem's native naming convention. | Free-form; for example `react`, `github.com/gin-gonic/gin`, or `org.example:library`. |
| `version` | Selected version or revision, commonly obtained from a lockfile. | Free-form native value. An exact declaration belongs in `version_constraint`, not here. |
| `version_constraint` | Version requirement declared by the source, including exact declaration specifiers. | Free-form ecosystem-native syntax such as `^19.0.0`, `>=2`, or `[1.0,2.0)`. |
| `vers` | Canonical VERS representation derived from `version_constraint`. | A `vers:` URI. See [VERS normalization](#vers-normalization). |
| `source_group` | Logical section or group in the source. | Open, source-specific vocabulary such as `dependencies`, `devDependencies`, or `project.optional-dependencies.dev`. |
| `origin_kind` | How the dependency is obtained. | `registry`, `git`, `path`, `url`, or `workspace`. |
| `relationship` | Relationship of the reference to the project being scanned. | `direct`, `transitive`, or `inconclusive`. |
| `scope` | Purpose or lifecycle scope of the dependency. | `runtime`, `development`, `test`, `build`, or `optional`. |
| `attributes` | Extra source- or ecosystem-specific facts that do not belong in the shared fields. | Open string-to-string object. See [Attributes](#attributes). |

`version` and `version_constraint` may both be present. For example, a lockfile
can record that `1.5.2` was selected for a declaration constrained to `^1.5.0`.

## Package types

`package_type` follows the Package URL type vocabulary where possible. The
built-in rules currently emit these values:

```text
cargo, cocoapods, composer, conan, conda, cpan, cran, docker, gem, generic,
github, golang, hackage, hex, julia, luarocks, maven, npm, nuget, opam, pub,
pypi, swift, vcpkg
```

`generic` means that deplens has a useful normalized reference but no more
specific supported package type fits it. Absence of `package_type` means that
the analyzer could not assign one; it does not mean `generic`.

Custom rules may use other values. Deplens recognizes the following additional
Package URL types for advisory validation, and preserves unknown values with a
warning so future ecosystems are not rejected:

```text
alpm, apk, bazel, bitbucket, bitnami, chrome-extension, deb, huggingface,
mlflow, oci, otp, qpkg, rpm, swid, vscode-extension, yocto
```

Consumers must therefore treat `package_type` as an open string, not a closed
enum.

## VERS normalization

`vers` is derived only from `version_constraint`; it is not derived from
`version`. In schema version 1, deplens attempts normalization for these package
types:

```text
cargo, gem, golang, maven, npm, pypi
```

The field is omitted when the package type is unsupported, the constraint is
empty or templated, or the native constraint cannot be parsed. Consumers can
always fall back to `version_constraint`, which preserves the native syntax.

## Classification fields

### Origin kind

- `registry`: obtained from a package or artifact registry.
- `git`: obtained from a Git repository.
- `path`: obtained from a local filesystem path.
- `url`: obtained directly from a URL outside a more specific origin kind.
- `workspace`: provided by another member of the same workspace.

### Relationship

- `direct`: declared or referenced directly by the project or source.
- `transitive`: introduced through another dependency.
- `inconclusive`: the source contains the reference, but does not provide enough
  information to classify it as direct or transitive.

An absent `relationship` means the analyzer did not classify it. It is distinct
from `inconclusive`, which is an explicit classification based on limited source
information.

### Scope

- `runtime`: needed for normal execution or the default dependency group.
- `development`: used for local development tooling or development-only code.
- `test`: used for tests.
- `build`: needed to build, package, generate, or otherwise produce the project.
- `optional`: enabled only through an optional feature or dependency group.

Scopes normalize source-specific groups. `source_group` retains the original
group information, so both fields may be present. When a format can express
multiple or more detailed scopes, the analyzer chooses the closest shared scope
and may retain details in `source_group` or `attributes`.

## Attributes

`attributes` is the extension point for information that is useful but is not
shared by all ecosystems. There is intentionally no closed list of keys.
Consumers must tolerate unknown keys.

Common cross-ecosystem conventions include:

| Attribute | Meaning |
| --- | --- |
| `source_url` | Registry, repository, or download URL. |
| `source_path` | Path within a repository, archive, or local source. |
| `source_ref` | Branch, tag, revision, or other source reference. |
| `source_ref_kind` | Kind of `source_ref`, such as `branch`, `tag`, `commit`, `revision`, or `ref`. |
| `checksum` | Ecosystem-native checksum. |
| `digest` | Content or image digest, including its algorithm when available. |
| `integrity` | Ecosystem-native integrity value. |
| `declared_name` | Name used at the declaration site when it differs from normalized `name`. |
| `registry` | Registry identifier when `package_type` alone is not specific enough. |

Other keys are analyzer-specific. Examples include Python `extras` and `marker`,
Docker `platform` and `stage`, Gradle `classifier` and `version_ref`, and
ecosystem identifiers such as `uuid`, `channel`, or `package_id`. All attribute
values remain strings even when the source represents a number, Boolean, or
list.

## Stability and consumer guidance

- The enclosing JSON document's `schema_version` governs the dependency object.
- Adding a new value to an open field or a new `attributes` key does not change
  the object shape. Consumers should preserve and tolerate values they do not
  recognize.
- Use normalized fields for comparisons and policy. Use `raw` for display and
  traceability.
- Do not infer missing classifications. An omitted field means that deplens did
  not provide that fact for this reference.
