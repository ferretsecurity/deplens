# Glossary

## Core Concepts

### Manifest
A manifest is a file or path that `deplens` reports because a detector considers it relevant to dependency scanning. Users can treat it as an item the scanner recognized as worth describing. A manifest may yield extracted dependencies, only a match, or a status that says extraction did not happen.

### Detector
A detector is the logic that decides whether a file or path should be treated as a manifest and, if possible, whether dependencies can be extracted from it. Detectors are what connect file patterns and content patterns to the scanner’s output. A detector may match a file without being able to extract any dependencies.

### Rule
A rule is a detector definition made from selectors and extraction behavior. Rules tell `deplens` what to look for and how to interpret a match. They can be built in or supplied by the user.

### Built-in Rule
A built-in rule is a rule embedded in the binary. Built-in rules define the default built-in ruleset and the default detection behavior. They are part of the tool’s normal behavior rather than user-provided configuration.

### Custom Rule
A custom rule is a rule loaded from a YAML file passed with `--rules`. When `--rules` is set, `deplens` loads that file for the run instead of using the embedded default ruleset. It uses the same rule concepts as a built-in rule.

### Match
A match is the result of a detector recognizing a file or path as relevant. A match does not necessarily mean dependencies were extracted. It only means the detector considered the item in scope.

### Extracted Dependency
An extracted dependency is a dependency name reported by the scanner for a matched manifest. In human-readable output, it appears as a listed dependency, either flat or within a section. In JSON output, it appears as an object in the `dependencies` array.

### Section
A section is a named group of extracted dependencies, such as `project.dependencies` or `extras_require.dev`. It preserves source structure when the underlying format provides that structure. When no section is available, dependencies may be shown without one.

## Detection and Extraction

### Dependency Status
Dependency status is the user-facing label that describes what the scanner could conclude about a manifest’s dependency presence and extraction. It distinguishes confirmed empty results, confirmed presence without extracted names, and cases where dependency presence could not be determined. The label shown depends on what the detector or extractor was able to confirm.

### Dependency Status Unknown
Dependency status unknown means the scanner matched the file but could not determine whether dependencies are present. It is the `has_dependencies: null` state. In summary text, this state is described as `dependency status unknown`; in detailed human-readable output, it is rendered as `[matched]`; in JSON output, it corresponds to `has_dependencies: null`. This is distinct from `no dependencies` and from `Dependencies Present, Not Extracted`.

### No Dependencies
No dependencies means the scanner determined that a matched manifest is empty. This is a conclusive empty result, not just a lack of extraction. In human-readable output, it is the label used for known-empty manifests when they are shown.

### Dependencies Present, Not Extracted
Dependencies present, not extracted is a project and output label for the case where dependency presence is confirmed but the individual dependency names are not listed. It describes the vocabulary used by the renderer and README, not a guaranteed current analyzer state. In practice, it marks a confirmed presence without extraction detail.

### Empty Manifest
An empty manifest is a manifest that was conclusively matched and found to have no dependencies. The phrase describes the file’s content state, not its file type. In human-readable output, empty manifests are the entries that can be shown with the `no dependencies` label.

## Output Semantics

### Human-Readable Output
Human-readable output is the default text output. It starts with summary counts and then prints manifest details in path-first order. The labels in this output are meant to be read directly by a person.

### JSON Output
JSON output is machine-readable output that contains a top-level `root` and a `manifests` array. Each manifest entry includes `type`, `path`, and `has_dependencies`, with `dependencies` optional when present. The structure is intended for tools that want to process scanner results programmatically.

### Path-First Output
Path-first output is an ordering style where manifest details are printed by path rather than grouped by detector or file type. This makes the output easier to scan by repository location. It is the default layout for the detailed human-readable view.

### Root
Root is the directory that `deplens` scanned. In JSON output, it is the top-level `root` field. In human-readable output, it is the scan root path shown on the `Root: ...` line.

### `has_dependencies`
`has_dependencies` is a JSON field on each manifest entry. It is `true` when extraction confirmed at least one dependency, `false` when a detector or extractor conclusively found none, and `null` when dependency presence is unknown. The field summarizes dependency presence without requiring a user to inspect the extracted list.

## Configuration Terms

### `--show-empty`
`--show-empty` is a CLI flag that includes known-empty manifests in the detailed human-readable output. It does not change whether they are counted in the summary. The flag only affects visibility of empty matches in the text view.
