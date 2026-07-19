# Architecture

`deplens` is a local Go CLI with three layers:

```text
cmd/deplens
    argument parsing and output selection
        |
internal/analyze
    strict detector rules -> selection -> analysis -> normalized results
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
9. Sort final results by path, then detector ID.

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
}

type SourceAnalysis struct {
    Presence   DependencyPresence
    Extraction ExtractionState
}

type ScanResult struct {
    Root    string
    Sources []DependencySourceResult
}
```

`Sources` is initialized as an empty slice so JSON emits `[]`, not `null`, for an empty scan.

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

## Adding an analyzer

1. Add a configuration type with YAML tags.
2. Implement `sourceAnalyzer` in `internal/analyze/<name>.go`.
3. Add the analyzer type to `compileSourceAnalyzer` in `parser_factory.go`.
4. Add or update a rule in `default_rules.yaml` with explicit form and roles.
5. Add focused analyzer tests and scan integration fixtures.
6. Update README and dependency coverage documentation.

Constructors validate analyzer-specific configuration before scanning starts. Runtime syntax errors identify the selected source and appear as structured diagnostics; they do not abort the entire directory walk.

## Rendering

Human rendering reads `SourceAnalysis` directly. It does not infer state from dependency count. Sources with absent presence are hidden by default and shown with `--show-without-dependencies`.

JSON uses the Go struct tags as its sole unversioned contract. Required source fields are detector, path, form, roles, and analysis. Empty dependencies and diagnostics are omitted. No schema-version selector or compatibility output exists.

## Verification

The main checks are:

```bash
go test ./...
go vet ./...
```

Rule-schema tests verify strict legacy-field rejection, analyzer-field rejection, unique detector IDs, complete metadata for all 185 built-ins, and successful loading of the embedded rules.
