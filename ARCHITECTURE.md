# Architecture

`deplens` walks a directory tree, identifies dependency manifest files, extracts structured
dependency data from them, and prints the results. It has no network calls and does no
vulnerability scanning - it is a pure static extraction tool.

## Repository layout

```
cmd/deplens/main.go              CLI entry point: flags, config, wiring
internal/analyze/
  scan.go                        Directory walker, core types (Dependency, ManifestMatch, ScanResult)
  rules.go                       Rule model, Ruleset, file detection logic, path-glob matching
  parser_factory.go              Instantiates parsers from YAML rule configs (one parser per rule key)
  default_rules.yaml             Embedded ruleset (~60+ detectors), loaded via go:embed
  pep508.go                      PEP 508 version constraint parser (shared by Python parsers)
  [ecosystem].go                 One file per parser type (go_mod, package_lock, pnpm_lock, ...)
internal/render/render.go        Human-readable and JSON output formatting
docs/glossary.md                 Canonical terminology - read this first
testdata/                        One subdirectory per ecosystem with fixture files for tests
```

## Data flow

```
main()
  parseArgs()        flags + path -> Config
  loadRuleset()      default_rules.yaml (or --rules file) -> Ruleset
  analyze.Scan()     walks tree, matches files, calls parsers -> ScanResult
  render.Human()     or render.JSON()  -> stdout
```

Inside `Scan`:

```
filepath.WalkDir(root)
  for each file:
    Ruleset.DetectManifestFileAtRelativePath(absPath, name, relPath)
      for each rule: match filename-regex and/or path-glob (AND semantics)
      if selector-only rule (nil parser): matched, no extraction
      if parser rule: read file content (lazy, once per file), call parser.Match()
        -> manifestParserResult { Matched, Dependencies, HasDependencies, Warnings }
    collect ManifestMatch
sort by (path, type) -> ScanResult
```

Content is read lazily: a file is only read from disk when the first matching rule that has a
parser is encountered. If multiple rules match the same file, the content is reused.

## Core types

```go
// scan.go
type Dependency struct {
    Raw        string            // free-form string as it appears in the source file
    Name       string            // bare package identifier (no version)
    Version    string            // resolved version for lockfiles
    Constraint string            // version range from source manifests, e.g. ">=1.0,<2.0"
    Section    string            // grouping from source file, e.g. "indirect", "dev", "packages-dev"
    Source     string            // origin type: "git", "registry", "path", "url"
    Extras     map[string]string // format-specific metadata: checksum, specifier, git ref, etc.
}

type ManifestMatch struct {
    Type            ManifestType // rule name, e.g. "go-mod", "js-npm-lock"
    Path            string       // relative path from scan root, forward slashes
    Dependencies    []Dependency
    HasDependencies *bool        // nil=unknown, true=confirmed present, false=confirmed empty
    Warnings        []string     // non-fatal issues: include cycle, unreadable include file, etc.
}

type ScanResult struct {
    Root      string
    Manifests []ManifestMatch
}

// rules.go
type manifestParser interface {
    Match(path string, content []byte) (manifestParserResult, error)
}

type manifestParserResult struct {
    Dependencies    []Dependency
    HasDependencies *bool
    Warnings        []string
    Matched         bool  // false = parser ran but did not recognize the file
}
```

The `HasDependencies` tri-state matters:
- `nil` - detector matched but cannot determine whether deps exist (selector-only, level 1)
- `true` - deps confirmed present (either extracted or presence-checked, level 2/3)
- `false` - conclusively found none (empty lockfile, metadata-only file, etc.)

`Matched: false` from a parser means the file was not recognized; the ruleset continues trying
subsequent rules. This is how a file can match a filename-regex but still be rejected after
content inspection.

## Rules and parser factory

Rules are defined in YAML. Each rule has:
- `name` - becomes the `ManifestType` string in output
- `filename-regex` and/or `path-glob` - file selectors (AND-ed when both present)
- exactly one parser key OR no parser key (selector-only / level 1)

`default_rules.yaml` is embedded in the binary via `//go:embed`. Users can supply an
alternative file with `--rules`; there is no rule merging - one replaces the other entirely.

`parser_factory.go` reads the parser key from a rule config and constructs the right struct.
Every parser key maps to a `new<Parser>` constructor that returns `(manifestParser, error)`.
The factory enforces that at most one parser key is set per rule.

Supported parser keys:

| Key | Parser | Notes |
|---|---|---|
| `go-mod` | `goModMatcher` | direct + indirect sections |
| `package-lock` | `packageLockParser` | npm v1/v2/v3, includes transitive deps |
| `pnpm-lock` | `pnpmLockParser` | v5 and v9+, includes transitive deps |
| `yarn-lock` | `yarnLockParser` | classic (v1) and berry |
| `poetry-lock` | `poetryLockParser` | skips self/directory entries |
| `uv-lock` | `uvLockParser` | skips workspace/editable entries |
| `pipfile-lock` | `pipfileLockParser` | default + develop sections |
| `cargo-lock` | `cargoLockParser` | source stored in Extras |
| `composer-lock` | `composerLockParser` | packages vs packages-dev |
| `py-requirements` | `pyRequirementsMatcher` | recursive -r includes, cycle detection |
| `python` | `pythonMatcher` | tree-sitter AST: setup.py call extraction |
| `typescript` | `typeScriptMatcher` | tree-sitter AST: import statements |
| `html` | `htmlMatcher` | external script src attributes |
| `terraform` | `terraformResourceParser` | HCL resource blocks via hashicorp/hcl |
| `toml` | `tomlQueryParser` | field-path queries from rule config |
| `yaml` | `yamlQueryParser` | field-path queries from rule config |
| `json` | `jsonQueryParser` | field-path queries from rule config |
| `xml` | `xmlQueryParser` | field-path queries from rule config |
| `ini` | `iniQueryParser` | section/key queries from rule config |
| `banner-regex` | `bannerRegexParser` | regex over first 4096 bytes of content |

## Two parser categories

**Dedicated parsers** (`go_mod.go`, `package_lock.go`, `pnpm_lock.go`, etc.) contain
format-specific Go logic. They produce full `Dependency` structs with all fields populated
where the format provides them.

**Query parsers** (`toml.go`, `yaml.go`, `json.go`, `xml.go`, `ini.go`) are generic
data-structure traversal driven entirely by field-path declarations in the YAML rule. Adding
a new detector for these formats requires only a new rule entry - no Go code.

Query parsers support three modes (mutually exclusive per rule):
- `query` / `queries` - extract string values at path as dependencies
- `exists` - match if path exists (level 1.5, no extraction)
- `exists-any` - match if any of the given paths has a non-empty value (level 2)

## Common patterns across dedicated parsers

**Deduplication with a `seen` map.** Key is usually `name@version` or the raw dep string.
This is critical for lockfiles that list the same package multiple times.

**Sections.** The `Section` field on `Dependency` preserves groupings from the source file.
Parsers populate it with values like `"indirect"`, `"dev"`, `"packages-dev"`, etc.

**Warnings are non-fatal.** Returned alongside results and shown inline. Used for cases like
unreadable `-r` include files or circular include chains in requirements files.

**Early return on unrecognized content.** If content doesn't match expectations (e.g. wrong
lockfile version, missing required header), return `manifestParserResult{}` with `Matched: false`.
This signals that the ruleset should continue trying other rules rather than reporting the file.

**Error vs. unmatched.** Return an error only for unexpected parse failures (malformed JSON/YAML).
Return `Matched: false` when the file looks structurally valid but doesn't apply (e.g. wrong
lockfile format version).

**`boolPtr(v bool) *bool`** is a helper in `rules.go` used everywhere to produce the
`HasDependencies` pointer without a named variable.

**Sorted output.** Parsers that iterate over maps sort keys before appending to produce
deterministic output. Use `slices.Sort` then iterate.

## Output

**Human (default):**
```
Root: /path/to/root

Found 3 manifest(s):
- 2 with extracted dependencies
- 1 confirmed empty

go.mod [4 deps]
  - golang.org/x/text@v0.14.0
  [indirect]:
    - golang.org/x/sys@v0.17.0
```

Status labels: `[N deps]` / `[no dependencies]` / `[dependencies present, not extracted]` / `[matched]`

Empty manifests (`has_dependencies: false`) are hidden by default; `--show-empty` includes them.

Dependency display priority in `render.go`: `name@version` > `name+constraint` > `name` > `raw`.

**JSON (`--json`):** Full `ScanResult` struct, pretty-printed. `has_dependencies` is a JSON
tri-state: `true`, `false`, or `null`. Dependencies appear as objects with `raw` always
present and other fields omitted when empty.

## Testing conventions

Tests live next to the code in `internal/analyze/` and use the standard `testing` package.

**Two test file patterns per parser:**
1. `<name>_test.go` - unit tests for the parser directly, using inline fixture data
2. `<name>_scan_test.go` - integration tests that call `Scan()` against `testdata/` directories

`scan_test.go` contains shared helpers: `mustLoadDefaultRules`, `dependencyNames`,
`equalDependencies`, plus a large table-driven `TestDetectSelectorOnlyManifestMatchesSupportedFiles`
covering all level-1 detectors.

Table-driven tests are preferred. Arrange-Act-Assert layout. Test names describe the scenario.

`testdata/` mirrors the ecosystem taxonomy. Each subdirectory holds one or more fixture files
that the scan tests reference. Fixture files are real (or representative) manifest files -
not generated mocks.

## Detector maturity model

Defined in `README.md`, referenced throughout:

- Level 1: identifies candidate files (selector-only, `nil` parser, `has_dependencies: null`)
- Level 2: determines whether deps exist (`has_dependencies: true/false`, no list)
- Level 3: extracts dependency list into `dependencies` array
- Level 4: reserved (normalized cross-ecosystem schema, not yet defined)

## Adding a new detector

1. If the target format is TOML/YAML/JSON/XML/INI, add a rule to `default_rules.yaml` only.
2. If it needs custom extraction logic:
   a. Create `internal/analyze/<name>.go` implementing `manifestParser`.
   b. Add config structs (`<name>MatcherConfig`, the parser struct, a `new<Name>` constructor).
   c. Register in `parser_factory.go` (add a nil-check and constructor call; increment the
      `parserCount` guard at the top).
   d. Add the config field to `ruleConfig` in `rules.go`.
   e. Add a rule to `default_rules.yaml`.
3. Add fixture files under `testdata/<ecosystem>/`.
4. Add unit tests in `<name>_test.go` and scan tests in `<name>_scan_test.go`.
5. Update `README.md` with detector info (required per AGENTS.md).
6. For user-visible behavior changes, include before/after output in the PR description.

## Key Go dependencies

| Package | Used for |
|---|---|
| `golang.org/x/mod/modfile` | go.mod parsing |
| `gopkg.in/yaml.v3` | YAML parsing |
| `github.com/BurntSushi/toml` | TOML parsing |
| `github.com/hashicorp/hcl/v2` | HCL/Terraform parsing |
| `github.com/tree-sitter/go-tree-sitter` | AST parsing (Python, TypeScript) |
| `github.com/tree-sitter/tree-sitter-typescript` | TypeScript grammar for tree-sitter |
| `gopkg.in/ini.v1` | INI parsing |

No external network dependencies at runtime. The binary is self-contained with all rules embedded.
