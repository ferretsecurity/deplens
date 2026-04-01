# Pyproject TOML Support Design

## Summary

Add TOML manifest parsing support to `deplens` using the existing rule-driven parser model. The first built-in target is `pyproject.toml`, with dependency extraction covering:

- `[project].dependencies`
- `[project.optional-dependencies]`
- `[dependency-groups]`
- `[tool.poetry.dependencies]`
- `[tool.poetry.group.*.dependencies]`

The design keeps scanner flow unchanged. TOML becomes a new parser type configured in rules, parallel to the existing `yaml` parser. `pyproject.toml` support is enabled through a built-in default rule rather than hardcoded scanner logic.

## Goals

- Add first-class support for TOML-backed dependency manifests.
- Keep the implementation rule-driven and aligned with the existing YAML parser pattern.
- Support common `pyproject.toml` dependency schemas used by PEP 621, dependency groups, and Poetry.
- Preserve declared dependency strings exactly where the source already stores strings.
- For TOML dependency tables, emit compact raw `name = value` strings instead of reconstructing normalized requirement strings.

## Non-Goals

- Do not redesign the scanner or add `pyproject.toml` special cases to `Scan`.
- Do not build a universal structured-data query engine shared by YAML and TOML.
- Do not normalize Poetry dependency tables into standardized requirement syntax.
- Do not attempt full semantic interpretation of all Poetry fields.

## Architecture

Add a new rule parser type, `toml`, alongside `yaml`, `html`, `terraform`, `typescript`, and `python`.

The existing flow remains intact:

1. Rules are loaded from YAML.
2. Each rule configures exactly one parser type.
3. `DetectManifestFile` reads a candidate file once and passes the content to the configured parser.
4. The parser decides whether the file matches and returns extracted dependencies.

The TOML parser is responsible for:

- Parsing TOML into generic Go values.
- Compiling configured TOML queries at rule-load time.
- Evaluating those queries against parsed TOML data.
- Returning dependency strings when a query resolves to supported values.

The scanner does not gain any `pyproject.toml` branching. Built-in `pyproject.toml` support is provided through a default rule in `internal/analyze/default_rules.yaml`.

## Rule Shape

The rule shape is intentionally parallel to YAML support:

```yaml
rules:
  - name: python-pyproject
    filename-regex: '^pyproject\.toml$'
    toml:
      queries:
        - project.dependencies[]
        - project.optional-dependencies.*[]
        - dependency-groups.*[]
        - tool.poetry.dependencies
        - tool.poetry.group.*.dependencies
```

This shape supports built-in defaults and custom TOML rules for other files later.

## Query Language

The first TOML query language is intentionally small.

Supported syntax:

- Dotted paths for nested tables, for example `project.dependencies`
- `[]` to expand arrays, for example `project.dependencies[]`
- `*` to iterate over all keys in a table, for example `project.optional-dependencies.*[]`
- Table-node extraction for dependency maps, for example `tool.poetry.dependencies`

Unsupported in v1:

- Arbitrary bracket expressions beyond trailing `[]`
- Index-based array access
- Recursive descent
- Filters or predicates
- Sharing one generalized query engine with YAML

## Extraction Rules

The parser evaluates queries independently and appends all extracted dependency strings in query order.

### Arrays of strings

If a query resolves to an array, emit each non-empty string item exactly as declared.

Examples:

- `project.dependencies[]`
- `project.optional-dependencies.*[]`
- `dependency-groups.*[]`

### Direct strings

If a query resolves directly to a non-empty string, emit that string exactly as declared.

### Dependency tables

If a query resolves to a TOML table representing dependency entries, emit each dependency as a compact raw TOML-like assignment string:

```text
name = <serialized TOML value>
```

Examples:

- `django = "^5.0"`
- `httpx = { extras = ["http2"], version = "^0.27" }`
- `private-lib = { branch = "main", git = "https://github.com/acme/private-lib.git" }`

This avoids lossy reconstruction while keeping output close to source.

### Poetry Python Constraint

When processing Poetry dependency tables, skip the `python` entry by default because it represents an interpreter constraint rather than a package dependency.

## Example Output

Given this `pyproject.toml`:

```toml
[project]
dependencies = [
  "requests>=2.31",
  "fastapi[all]>=0.110; python_version >= '3.10'",
]

[project.optional-dependencies]
dev = [
  "pytest>=8",
  "ruff==0.4.8",
]

[dependency-groups]
lint = [
  "mypy>=1.10",
]

[tool.poetry.dependencies]
python = "^3.12"
django = "^5.0"
httpx = { version = "^0.27", extras = ["http2"] }
private-lib = { git = "https://github.com/acme/private-lib.git", branch = "main" }

[tool.poetry.group.test.dependencies]
pytest-cov = "^5.0"
factory-boy = { version = "^3.3", markers = "python_version >= '3.11'" }
```

The extracted dependencies would be:

```json
[
  "requests>=2.31",
  "fastapi[all]>=0.110; python_version >= '3.10'",
  "pytest>=8",
  "ruff==0.4.8",
  "mypy>=1.10",
  "django = \"^5.0\"",
  "httpx = { extras = [\"http2\"], version = \"^0.27\" }",
  "private-lib = { branch = \"main\", git = \"https://github.com/acme/private-lib.git\" }",
  "factory-boy = { markers = \"python_version >= '3.11'\", version = \"^3.3\" }",
  "pytest-cov = \"^5.0\""
]
```

The Poetry `python = "^3.12"` entry is intentionally excluded.

## Validation And Error Handling

Rule-load validation should fail fast for invalid TOML parser configuration.

Validation requirements:

- `toml.queries` must be present and contain at least one entry.
- Each query must be non-empty.
- Query segments must not contain empty path components.
- `[]` is only valid as a trailing array-expansion suffix on a segment.
- `*` is only valid as a complete segment.
- Unsupported syntax should return a clear rule-load error.

Scan-time behavior:

- TOML parse failures should return errors in the style `parse toml file "<path>": ...`.
- Missing query paths should not be errors; they should behave as non-matches.
- Non-string array elements, inline tables inside expanded arrays, and unsupported values inside resolved nodes should be ignored.
- If a rule yields no usable dependency strings after evaluation, it should return no match.

## Testing

Add tests covering:

- Valid TOML rules load successfully.
- Invalid TOML queries are rejected with clear errors.
- TOML remains mutually exclusive with other parser types on a rule.
- Built-in supported manifest types include the new `pyproject.toml` detector when enabled.
- Scanning a `pyproject.toml` fixture extracts dependencies from:
  - `project.dependencies[]`
  - `project.optional-dependencies.*[]`
  - `dependency-groups.*[]`
  - `tool.poetry.dependencies`
  - `tool.poetry.group.*.dependencies`
- Poetry dependency tables serialize as stable compact `name = value` strings.
- The Poetry `python` entry is skipped.
- Non-string or unsupported values do not produce false-positive matches.
- README detector documentation is updated to reflect TOML support.

## Rollout

Ship `pyproject.toml` support as a built-in default rule immediately. This keeps the user experience consistent with other first-class manifest types and avoids requiring custom rule files for a common Python manifest.

The TOML parser itself remains reusable for future custom rules and additional built-in TOML detectors.
