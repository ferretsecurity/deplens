# Architecture

`deplens` is a local Go CLI with three layers:

```text
cmd/deplens
    argument parsing and output selection
        |
internal/analyze
    strict rules -> selection -> analysis -> repository relations -> checks
        |
internal/render
    human and JSON presentation
```

## Repository layout

```text
cmd/deplens/main.go                 CLI entry point
internal/analyze/scan.go            directory walker and public result types
internal/analyze/rules.go           strict rule schema, validation, and dispatch
internal/analyze/parser_factory.go  nested analyzer configuration factory
internal/analyze/default_rules.yaml embedded built-in detectors
internal/analyze/findings.go        project ownership and check evaluators
internal/analyze/generate.go        generation planning, preflight, and writes
internal/analyze/*.go               analyzer implementations
internal/render/render.go            human and JSON renderers
testdata/                            integration fixtures
```

## Scan flow

For each regular file under the scan root:

1. Normalize its root-relative path to `/` separators.
2. Evaluate detectors in configured order.
3. Apply `filename-regex` and/or `path-glob`; both must match when both exist.
4. If the detector has no analyzer, return an identified source with `unknown` presence and `unsupported` extraction.
5. Otherwise, read the file once and call `sourceAnalyzer.Analyze`.
6. Continue to later detectors when an analyzer does not recognize the content.
7. On recognition, apply the rule's default package type, derive VERS where supported, and return the first result.
8. Convert file-read or total analyzer errors to `unknown` + `failed` with an error diagnostic.
9. Sort source results by path, then detector ID.
10. Build ecosystem-specific project and workspace ownership from the immutable source set and normalized paths.
11. Parse evaluator-specific policy inputs into repository facts.
12. Evaluate configured checks in stable check-ID order, then sort check runs and findings by project root and check ID.
13. When the caller requests generation, plan and validate every selected group before writing any output.

Ignored directory names are skipped during traversal. Scanning does not access the network.

## Core result model

```go
type DependencySourceResult struct {
    Detector     DetectorID
    Path         string
    Form         SourceForm
    Roles        []SourceRole
    Analysis     SourceAnalysis
    Dependencies []DependencyReference
    Diagnostics  []Diagnostic
    Groups       []DependencyGroup
    Generate     GenerationFormat
}

type DependencyGroup struct {
    Name         string
    Location     string
    Dependencies []DependencyReference
    State        DependencyGroupState // missing, empty, or ready
}

type SourceAnalysis struct {
    Presence   DependencyPresence
    Extraction ExtractionState
}

type ScanResult struct {
    SchemaVersion int
    Root          string
    Sources       []DependencySourceResult
    CheckRuns     []CheckRun
    Findings      []Finding
    Generation    *GenerationResult
}
```

`Groups` and `Generate` are internal scan-to-generation contracts and are not serialized. A group represents one independently writable unit, such as a Glue job or one selected YAML node. `Location` is stable source context, not a result index. `missing` means the selected group has no dependency declaration, `empty` means it declares no dependencies, and `ready` means every dependency was parsed and can be written. An analyzer reports unreadable or malformed declarations through its normal partial or failed extraction result instead of a group state.

All result collections are initialized as empty slices so JSON emits `[]`, not `null`, for an empty scan.

Dependency references preserve `Raw` and may add normalized fields:

```go
type DependencyReference struct {
    PackageType       PackageType
    Raw               string
    Name              string
    Version           string
    VersionConstraint string
    VERS              string
    SourceGroup       string
    OriginKind        OriginKind
    Relationship      Relationship
    Scope             DependencyScope
    Attributes        map[string]string
}
```

`Version` is the selected version. There is no `ResolvedVersion` field.

## Analyzer contract

```go
type sourceAnalyzer interface {
    Analyze(path string, content []byte) (sourceAnalyzerResult, error)
}

type sourceAnalyzerResult struct {
    Recognized   bool
    Analysis     SourceAnalysis
    Dependencies []DependencyReference
    Diagnostics  []Diagnostic
}
```

`Recognized` distinguishes selector matching from semantic recognition. An unrecognized analyzer result allows a later detector to inspect the same file. A recognized result must use one of the valid analysis pairs defined in the glossary.

Dedicated extractors normally return present/complete or absent/complete. Presence-only analyzers return present/unsupported or absent/unsupported. Recoverable extraction problems with usable references return present/partial plus warning diagnostics. Total failures become unknown/failed plus an error diagnostic.

## Rule schema

```yaml
rules:
  - id: go-mod
    package-type: golang
    form: manifest
    roles: [declaration, constraint, resolution]
    filename-regex: '^go\.mod$'
    analyzer:
      type: go-mod
```

Validation requires:

- a non-empty, unique `id`;
- a recognized `form`;
- at least one unique, recognized role;
- at least one selector;
- valid regular expression and glob syntax;
- one supported analyzer type when `analyzer` is present;
- only fields known to the selected analyzer.

The YAML decoder uses strict known-field checking. Legacy `name`, `dependency-type`, and top-level analyzer keys are rejected. There is no compatibility adapter in production.

Checks are compiled from the same strict document:

```yaml
checks:
  - id: javascript-npm-lockfile-missing
    summary: npm project has dependencies but no npm lockfile
    severity: medium
    evaluator:
      type: npm-lockfile-missing
    remediation: Run `npm install` and commit the generated lockfile.
```

Nine dependency-policy evaluator types have empty configurations. The CODEOWNERS evaluator additionally accepts `platform: auto|github|gitlab`. Manager evidence, dependency gating, workspace ownership, application-role requirements, conflicting JavaScript lockfile families, local Go replacements, ownership matching, and ambiguity handling are evaluator invariants implemented in Go. JavaScript package publishability does not affect missing-lockfile eligibility. Ambiguous inputs produce skipped or failed check runs as appropriate; parsing failures produce failed runs; neither produces a policy finding.

## Repository relationships and checks

Missing-file and ownership checks run after traversal because a file-local analyzer cannot observe repository-wide policy. JavaScript package workspaces, pnpm workspaces, uv workspaces, and Cargo workspaces attach member manifests to explicit owners. A lockfile only satisfies the compatible owning project; directory ancestry by itself is insufficient. Generic source recognition remains separate from policy input collection: when the uv evaluator is configured, `pyproject.toml` content is retained from the scanner's single read and parsed into uv facts even if dependency queries did not recognize it as a dependency source. When dependency sources exist, regular-file CODEOWNERS candidates are read directly from their standard root-relative locations so directory-ignore settings affect source discovery without disabling repository ownership policy.

The evaluator layer remains offline and does not invoke package managers. A finding subject contains only its normalized `project_root`; concrete manifest anchors live in `locations`. Fingerprints use a dedicated fingerprint-format version plus the check ID, project root, and stable evidence. They are independent of the JSON output schema version, human wording, severity, and source location movement.

## Adding an analyzer

1. Add a configuration type with YAML tags.
2. Implement `sourceAnalyzer` in `internal/analyze/<name>.go`.
3. Add the analyzer type to `compileSourceAnalyzer` in `parser_factory.go`.
4. Add or update a rule in `default_rules.yaml` with explicit form and roles.
5. Add focused analyzer tests and scan integration fixtures.
6. Update README and dependency coverage documentation.

Constructors validate analyzer-specific configuration before scanning starts. Runtime syntax errors identify the selected source and appear as structured diagnostics; they do not abort the entire directory walk.

## Generation

Generation is an optional phase after scanning and checks. A detector opts in with `generate: python-requirements`. The CLI currently accepts only the `python-requirements` format. It ignores sources without that opt-in and turns each `ready` group into one requirements file. `missing` and `empty` groups remain successful structured outcomes without paths.

The generator performs a full preflight before its first write. It rejects partial or failed extraction, malformed Python requirements, destination collisions, paths outside the scan root, existing destinations unless overwrite is enabled, and non-regular overwrite targets. This means a validation error cannot leave files from earlier plans behind.

Without overwrite, files are created with exclusive create semantics. With overwrite, each planned regular file is written to a temporary file in the destination directory and renamed over the old file. Writes are atomic per file, not across the whole generation set. The generator does not remove stale output and does not follow destination symlinks.

## Rendering

Human rendering reads `SourceAnalysis` directly. It does not infer state from dependency count. Sources with absent presence are hidden by default and shown with `--show-without-dependencies`.

JSON schema version 1 includes sources, check runs, findings, and an optional `generation` object. Required source fields are detector, path, form, roles, and analysis. Empty dependencies and diagnostics are omitted. `generation.format` identifies the requested format, `paths` contains root-relative written paths, and `outcomes` records every selected group with its source, name, location, status, and optional path. Both generation collections are empty arrays when no groups produce output. Human output renders findings after dependency sources and prints a generation report only when generation was requested. Findings and generation skips do not alter the CLI's successful exit status.

## Verification

The main checks are:

```bash
go test ./...
go vet ./...
```

Rule-schema tests verify strict legacy-field rejection, analyzer/evaluator-field rejection, unique IDs, complete metadata for all 185 detectors and ten checks, and successful loading of the embedded rules. Finding tests cover positive, clean, dependency-free, conflicting-lockfile, CODEOWNERS coverage, ambiguous, library, workspace, nested-project, and fingerprint-stability cases.
