# Python uv.lock L3 Detector Design

## Goal

Upgrade the built-in `python-uv` detector from Level 1 to Level 3 by replacing the filename-only rule with a dedicated `uv-lock` extractor.

The detector should:

- identify `uv.lock` files
- determine whether the lockfile contains dependency packages
- extract resolved packages into the existing `deplens` dependency model

## Why A Dedicated Extractor

`uv.lock` is TOML-based, but its primary data shape is a repeated `[[package]]` array-of-table structure with uv-specific source semantics. The current generic TOML rule DSL is designed around:

- scalar queries
- table existence checks
- table extraction

Extending that DSL to express `uv.lock` cleanly would require new uv-oriented features such as array-of-table existence checks, package-specific filtering, and package-to-string formatting. That would make the TOML rule layer more complex without improving reuse elsewhere.

Instead, `python-uv` should use a dedicated parser exposed through a new rule field:

```yaml
- name: python-uv
  filename-regex: '^uv\.lock$'
  uv-lock: {}
```

## Detection Semantics

The detector should match only files selected by the rule, then parse the file as TOML.

### Conclusive Match

The detector is a conclusive match when the file is valid TOML and is structurally recognizable as a `uv.lock` file. At minimum, this means:

- the file parses as TOML
- the top-level document contains `version`
- package entries, when present, appear under `[[package]]`

### Dependency Presence

After filtering ignored package entries, dependency presence is determined from the remaining package list:

- `has_dependencies=true` when at least one package remains
- `has_dependencies=false` when the file is a valid `uv.lock` file but no package remains

Examples of `has_dependencies=false`:

- `version = 1`
- only self-style editable/workspace/virtual entries are present

## Extraction Semantics

Each retained `[[package]]` entry produces one `Dependency`.

### Output Shape

Use the existing dependency model:

```json
{
  "type": "python-uv",
  "path": "backend/uv.lock",
  "dependencies": [
    { "name": "requests==2.31.0" },
    { "name": "urllib3==2.2.1" }
  ],
  "has_dependencies": true
}
```

No uv-specific JSON schema changes are required.

### Dependency Formatting

Each dependency should be emitted as:

- `name==version` when both fields are present

The initial implementation should require both `name` and `version` to emit a dependency entry. Package entries missing either field are ignored.

### Section Usage

Do not emit a `section`. All extracted uv package dependencies should be unsectioned.

## Ignored Package Entries

The detector should ignore package entries that represent the current project or workspace members rather than true dependencies.

Ignore packages when:

- `source.editable` is `.` or an equivalent root-self marker
- `source.workspace` is `true`
- `source.virtual` is present and indicates a self entry

This includes cases such as:

- `source = { editable = "." }`
- `source = { virtual = "." }`
- editable paths to local workspace members
- workspace-linked packages

These entries describe local project materialization, not dependency inputs that should be reported by `deplens`.

## Retained Package Entries

The detector should retain package entries sourced from real dependency inputs, including:

- registry
- git
- url
- path
- editable paths such as `source = { editable = "../packages/foo" }`

Path and editable dependencies should be retained unless they are self-style editable/workspace/virtual entries covered by the ignore rules above.

## Error Handling

- Invalid TOML should return a scan error, consistent with other structured parsers.
- Unknown uv-specific fields should be ignored.
- Future lockfile extensions should not fail parsing unless they break TOML decoding.

## Edge Cases

### Empty Or Placeholder Lockfiles

`version = 1` with no retained packages should produce:

- no `dependencies`
- `has_dependencies=false`

### Editable Self Entry Plus Resolved Dependencies

If the lockfile contains both:

- an editable self package
- retained external packages

the self package is ignored and the remaining packages are extracted normally.

Genuine editable dependencies such as `source = { editable = "../packages/foo" }` remain retained.

### Duplicate Package Entries

If multiple retained entries serialize to the same `name==version`, keep all entries in the initial implementation unless tests show uv routinely duplicates equivalent retained packages in a way that harms output quality. Deduplication can be added later if necessary.

### Missing Name Or Version

If a retained package is missing `name` or `version`, ignore that package entry. The detector should still report the manifest if other retained packages exist.

## Rule And Code Changes

### Rule Layer

Add a new parser field to rule configuration:

- `uv-lock`

Update the built-in rule:

```yaml
- name: python-uv
  filename-regex: '^uv\.lock$'
  uv-lock: {}
```

### Parser Layer

Add a dedicated parser implementation, for example in:

- `internal/analyze/uv_lock.go`

The parser should:

1. parse TOML into a small dedicated Go structure
2. iterate `package` entries
3. filter ignored editable/workspace entries
4. serialize retained entries to `name==version`
5. return `has_dependencies=true` with extracted dependencies when any remain
6. return `has_dependencies=false` when none remain

## Testing

Add both unit and scan-level tests.

### Unit Tests

Cover:

- minimal empty lockfile
- basic package extraction
- editable self entry ignored
- workspace entry ignored
- mixed self plus external packages
- path dependency retained
- invalid TOML error

### Scan Tests

Add fixture-backed scan coverage for the default built-in rule so `python-uv` is exercised through the normal scan path.

### Testdata

Because this changes a default rule, add concrete `testdata` fixtures for:

- empty `uv.lock`
- extracted `uv.lock`
- ignored editable/workspace cases

## README Updates

Update `README.md` to reflect that:

- `python-uv` is no longer Level 1
- `uv.lock` is extracted at Level 3
- editable/workspace self entries are ignored

Include a concrete example showing:

- previous behavior: `uv.lock [matched]`
- new behavior: `uv.lock [N deps]` with extracted packages

## Non-Goals

This change does not:

- define a new JSON schema for lockfile graphs
- preserve per-package transitive dependency edges
- preserve markers or optional-dependency groups
- add generic array-of-table TOML extraction features
- implement Level 4 normalization
