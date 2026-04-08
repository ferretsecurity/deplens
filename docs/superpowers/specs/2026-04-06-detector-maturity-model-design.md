# Detector Maturity Model Design

## Goal

Introduce a detector maturity model as an internal and documentation-facing concept for `deplens`, without changing CLI output, JSON output, or detector behavior.

The maturity model should provide a stable way to describe detector capability across both built-in detectors and user-defined rules loaded through `--rules`.

## Non-Goals

- No CLI flags or output changes
- No JSON schema changes
- No detector implementation changes
- No attempt to define a normalized dependency schema yet

## Definitions

The maturity level describes detector capability, not the outcome for an individual matched file.

Per-file outcomes remain represented by existing result fields:

- `has_dependencies` indicates whether the detector could determine dependency presence for a specific file
- `dependencies` contains extracted dependency entries for a specific file when extraction is supported and succeeds

### Level 1

The detector can identify candidate files.

Examples:

- filename-based rules
- path-based rules

### Level 2

The detector can determine whether a matched file contains dependency declarations.

This includes presence checks where the detector can conclusively report that dependencies are present or absent without extracting a structured dependency list.

The maturity model describes detector capability, not necessarily the exact status labels currently emitted for every detector in scan output.

Examples:

- rules that detect a dependency-bearing section
- matchers that can prove a file has no dependencies

### Level 3

The detector can extract dependency data in a detector-specific or source-specific form.

This includes the current extraction behavior in `deplens`, where dependency entries are emitted but are not normalized into a shared representation across detector families.

Examples:

- extracting package names from a `Pipfile`
- extracting dependency strings from `setup.py`
- extracting external script URLs from HTML

### Level 4

The detector can extract dependency data into a normalized format shared across detector families.

No current detector is level 4. This level is reserved for future work because `deplens` does not yet define a normalized dependency schema.

## Scope

The maturity model applies to:

- built-in detectors shipped with `deplens`
- custom rules defined by users in YAML files passed with `--rules`

For built-in detectors, the README should document the current maturity level explicitly.

For custom rules, the README should describe the maturity model as a general framework:

- selector-only rules are typically level 1
- presence-check rules are typically level 2
- extraction rules are typically level 3
- level 4 is not currently available

## Built-In Detector Classification

The initial built-in maturity levels should be documented as follows.

### Level 1

- filename regex match
- path glob match

### Level 2

- yaml
- terraform

### Level 3

- toml
- pipfile
- python call
- ini
- banner regex
- html external scripts
- typescript cdk construct
- python cdk construct

Built-in `yaml` support should be documented as level 2 because the shipped `python-conda-environment` rule is a presence check.

The README should also state separately that custom `yaml` rules can be:

- level 2 when used for presence checks only
- level 3 when used for extraction

## README Changes

Add a new section named `Detector Maturity Model` near `Supported Detectors`.

That section should:

1. Define levels 1 through 4
2. State that maturity is a detector capability model, not a per-file result model
3. Explain that the model applies to both built-in detectors and custom rules
4. State that no detector currently reaches level 4

Update the `Supported Detectors` table to include a `Maturity` column.

The built-in table should describe shipped detector behavior only. For configurable detector families such as `yaml`, document the built-in rule capability in the table and explain custom-rule maturity separately in prose after the table.

Add a short note after the built-in table explaining how user-defined rules fit the same model.

## Rationale

This approach keeps the model stable and easy to reason about:

- detector capability is documented once
- per-file scan outcomes continue to use existing result fields
- users and contributors gain a roadmap for future detector improvements
- future normalization work can be introduced later as level 4 without rewriting the earlier levels

## Open Questions

None for the initial documentation-only change.
