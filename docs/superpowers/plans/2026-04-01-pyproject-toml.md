# Pyproject TOML Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add rule-driven TOML parsing to `deplens` and ship built-in `pyproject.toml` dependency detection covering PEP 621, dependency groups, and Poetry tables.

**Architecture:** Introduce a new `toml` parser type in the existing rules engine, implemented as a sibling to the current YAML query parser. Keep scanner behavior unchanged, add a reusable TOML query/evaluation layer in `internal/analyze/toml.go`, and expose `pyproject.toml` support by adding a built-in default rule plus tests and README updates.

**Tech Stack:** Go 1.25, `testing`, existing rule-loading code in `internal/analyze`, TOML decoder library added to `go.mod`

---

## File Structure

- Create: `internal/analyze/toml.go`
- Create: `testdata/toml/pyproject/pyproject.toml`
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `internal/analyze/rules.go`
- Modify: `internal/analyze/terraform.go`
- Modify: `internal/analyze/default_rules.yaml`
- Modify: `internal/analyze/scan_test.go`
- Modify: `README.md`

Responsibilities:

- `internal/analyze/toml.go`: TOML parser config, query compilation, query evaluation, dependency extraction, TOML value serialization.
- `internal/analyze/rules.go`: add `toml` to `ruleConfig`.
- `internal/analyze/terraform.go`: extend `compileManifestParser` to count and construct the TOML parser.
- `internal/analyze/default_rules.yaml`: add the built-in `pyproject.toml` rule.
- `internal/analyze/scan_test.go`: rule-load coverage, default-type ordering, scan behavior for TOML and `pyproject.toml`.
- `testdata/toml/pyproject/pyproject.toml`: reusable end-to-end fixture.
- `README.md`: document the new built-in detector and TOML parser capability.

### Task 1: Add TOML Rule Wiring

**Files:**
- Modify: `internal/analyze/rules.go`
- Modify: `internal/analyze/terraform.go`
- Modify: `internal/analyze/scan_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Write the failing rule-loading tests**

Add these tests near the existing YAML rule-loading tests in `internal/analyze/scan_test.go`:

```go
func TestLoadRulesAcceptsTOMLParser(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project.dependencies[]\n        - project.optional-dependencies.*[]\n"))
	if err != nil {
		t.Fatalf("expected toml parser to load: %v", err)
	}
}

func TestLoadRulesRejectsTOMLParserWithoutQueries(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml: {}\n"))
	if err == nil {
		t.Fatalf("expected missing toml queries error")
	}
}

func TestLoadRulesRejectsMalformedTOMLQuery(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project..dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected malformed toml query error")
	}
}

func TestLoadRulesRejectsTOMLParserWithOtherParserType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n  - name: mixed\n    filename-regex: '^pyproject\\.toml$'\n    yaml:\n      query: workflow.steps[].config.packages.pip[]\n    toml:\n      queries:\n        - project.dependencies[]\n"))
	if err == nil {
		t.Fatalf("expected multiple parser type error")
	}
}
```

- [ ] **Step 2: Run the rule-loading tests to verify they fail**

Run: `go test ./internal/analyze -run 'TestLoadRules(AcceptsTOMLParser|RejectsTOMLParserWithoutQueries|RejectsMalformedTOMLQuery|RejectsTOMLParserWithOtherParserType)' -count=1`

Expected: FAIL with compile errors because `ruleConfig` has no `TOML` field and `newTOMLQueryParser` does not exist yet.

- [ ] **Step 3: Add TOML rule config to the rule model**

Update `internal/analyze/rules.go` so `ruleConfig` includes the TOML parser config:

```go
type ruleConfig struct {
	Name          string                   `yaml:"name"`
	FilenameRegex string                   `yaml:"filename-regex"`
	BannerRegex   string                   `yaml:"banner-regex"`
	Terraform     *terraformMatcherConfig  `yaml:"terraform"`
	TypeScript    *typescriptMatcherConfig `yaml:"typescript"`
	Python        *pythonMatcherConfig     `yaml:"python"`
	YAML          *yamlMatcherConfig       `yaml:"yaml"`
	TOML          *tomlMatcherConfig       `yaml:"toml"`
	HTML          *htmlMatcherConfig       `yaml:"html"`
}
```

- [ ] **Step 4: Extend parser selection to count TOML**

Update `compileManifestParser` in `internal/analyze/terraform.go`:

```go
	if raw.TOML != nil {
		parserCount++
	}
```

and add the constructor branch before HTML:

```go
	if raw.TOML != nil {
		return newTOMLQueryParser(*raw.TOML)
	}
```

- [ ] **Step 5: Add the TOML decoder dependency**

Update `go.mod` to add a TOML package with generic decode support:

```go
require (
	github.com/BurntSushi/toml v1.5.0
	github.com/hashicorp/hcl/v2 v2.24.0
	github.com/tree-sitter/go-tree-sitter v0.25.0
	github.com/tree-sitter/tree-sitter-typescript v0.23.2
	github.com/zclconf/go-cty v1.17.0
)
```

Then refresh `go.sum` with:

Run: `go mod tidy`

Expected: PASS and `go.sum` gains checksums for `github.com/BurntSushi/toml`.

- [ ] **Step 6: Run the rule-loading tests again**

Run: `go test ./internal/analyze -run 'TestLoadRules(AcceptsTOMLParser|RejectsTOMLParserWithoutQueries|RejectsMalformedTOMLQuery|RejectsTOMLParserWithOtherParserType)' -count=1`

Expected: FAIL in parser construction because `internal/analyze/toml.go` still does not exist.

- [ ] **Step 7: Commit the rule wiring checkpoint**

```bash
git add go.mod go.sum internal/analyze/rules.go internal/analyze/terraform.go internal/analyze/scan_test.go
git commit -m "test: wire toml parser into rules"
```

### Task 2: Implement TOML Query Parsing And Extraction

**Files:**
- Create: `internal/analyze/toml.go`
- Modify: `internal/analyze/scan_test.go`

- [ ] **Step 1: Write the failing scan tests for TOML extraction**

Add these tests in `internal/analyze/scan_test.go` after the YAML scan tests:

```go
func TestScanMatchesTOMLDependenciesFromCustomRule(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project.dependencies[]\n        - project.optional-dependencies.*[]\n        - dependency-groups.*[]\n        - tool.poetry.dependencies\n        - tool.poetry.group.*.dependencies\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project]
dependencies = ["requests>=2.31"]

[project.optional-dependencies]
dev = ["pytest>=8"]

[dependency-groups]
lint = ["mypy>=1.10"]

[tool.poetry.dependencies]
python = "^3.12"
django = "^5.0"
httpx = { version = "^0.27", extras = ["http2"] }

[tool.poetry.group.test.dependencies]
pytest-cov = "^5.0"
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	want := []string{
		"requests>=2.31",
		"pytest>=8",
		"mypy>=1.10",
		"django = \"^5.0\"",
		"httpx = { version = \"^0.27\", extras = [\"http2\"] }",
		"pytest-cov = \"^5.0\"",
	}
	if !slices.Equal(result.Manifests[0].Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Manifests[0].Dependencies, want)
	}
}

func TestScanDoesNotMatchTOMLWhenQueryResolvesToNoUsableValues(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project.dependencies[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project]
dependencies = [123, true]
`)

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 0 {
		t.Fatalf("expected no manifests, got %+v", result.Manifests)
	}
}

func TestScanReturnsTOMLParseErrors(t *testing.T) {
	ruleset, err := loadRules("test.yaml", []byte("rules:\n  - name: python-pyproject\n    filename-regex: '^pyproject\\.toml$'\n    toml:\n      queries:\n        - project.dependencies[]\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}

	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project
dependencies = ["requests>=2.31"]
`)

	_, err = Scan(root, nil, ruleset)
	if err == nil {
		t.Fatalf("expected scan to fail")
	}
	if !strings.Contains(err.Error(), "parse toml file") {
		t.Fatalf("expected toml parse error, got %v", err)
	}
}
```

- [ ] **Step 2: Run the new TOML scan tests to verify they fail**

Run: `go test ./internal/analyze -run 'TestScan(MatchesTOMLDependenciesFromCustomRule|DoesNotMatchTOMLWhenQueryResolvesToNoUsableValues|ReturnsTOMLParseErrors)' -count=1`

Expected: FAIL because `newTOMLQueryParser` and TOML extraction logic do not exist.

- [ ] **Step 3: Create the TOML parser implementation**

Create `internal/analyze/toml.go` with the parser types, query compiler, evaluator, and serializer:

```go
package analyze

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type tomlMatcherConfig struct {
	Queries []string `yaml:"queries"`
}

type tomlSegment struct {
	key    string
	expand bool
	wild   bool
}

type tomlQuery struct {
	segments []tomlSegment
}

type tomlQueryParser struct {
	queries []tomlQuery
}

func newTOMLQueryParser(raw tomlMatcherConfig) (manifestParser, error) {
	if len(raw.Queries) == 0 {
		return nil, fmt.Errorf("toml.queries: must contain at least one entry")
	}

	queries := make([]tomlQuery, 0, len(raw.Queries))
	for queryIdx, rawQuery := range raw.Queries {
		query, err := compileTOMLQuery(rawQuery)
		if err != nil {
			return nil, fmt.Errorf("toml.queries[%d]: %w", queryIdx, err)
		}
		queries = append(queries, query)
	}

	return tomlQueryParser{queries: queries}, nil
}

func (p tomlQueryParser) Match(path string, content []byte) ([]string, bool, error) {
	var root map[string]any
	if err := toml.Unmarshal(content, &root); err != nil {
		return nil, false, fmt.Errorf("parse toml file %q: %w", path, err)
	}

	dependencies := make([]string, 0)
	for _, query := range p.queries {
		nodes := evalTOMLQuery([]any{root}, query)
		dependencies = append(dependencies, extractTOMLDependencies(nodes)...)
	}
	if len(dependencies) == 0 {
		return nil, false, nil
	}
	return dependencies, true, nil
}
```

- [ ] **Step 4: Implement query compilation and traversal**

Add these functions in `internal/analyze/toml.go`:

```go
func compileTOMLQuery(raw string) (tomlQuery, error) {
	if raw == "" {
		return tomlQuery{}, fmt.Errorf("required")
	}

	parts := strings.Split(raw, ".")
	segments := make([]tomlSegment, 0, len(parts))
	for idx, part := range parts {
		if part == "" {
			return tomlQuery{}, fmt.Errorf("invalid empty segment at position %d", idx)
		}

		segment := tomlSegment{key: part}
		if part == "*" {
			segment = tomlSegment{wild: true}
		} else if strings.HasSuffix(part, "[]") {
			key := strings.TrimSuffix(part, "[]")
			if key == "" || key == "*" {
				return tomlQuery{}, fmt.Errorf("invalid segment %q", part)
			}
			segment = tomlSegment{key: key, expand: true}
		}

		if !segment.wild && (strings.Contains(segment.key, "[") || strings.Contains(segment.key, "]") || strings.Contains(segment.key, "*")) {
			return tomlQuery{}, fmt.Errorf("invalid segment %q", part)
		}
		segments = append(segments, segment)
	}

	return tomlQuery{segments: segments}, nil
}

func evalTOMLQuery(current []any, query tomlQuery) []any {
	for _, segment := range query.segments {
		next := make([]any, 0)
		for _, node := range current {
			mapped, ok := node.(map[string]any)
			if !ok {
				continue
			}

			switch {
			case segment.wild:
				keys := make([]string, 0, len(mapped))
				for key := range mapped {
					keys = append(keys, key)
				}
				slices.Sort(keys)
				for _, key := range keys {
					next = append(next, mapped[key])
				}
			case segment.expand:
				value, ok := mapped[segment.key]
				if !ok {
					continue
				}
				items, ok := value.([]any)
				if !ok {
					continue
				}
				next = append(next, items...)
			default:
				value, ok := mapped[segment.key]
				if !ok {
					continue
				}
				next = append(next, value)
			}
		}
		current = next
		if len(current) == 0 {
			return nil
		}
	}

	return current
}
```

- [ ] **Step 5: Implement dependency extraction and stable TOML serialization**

Add these functions in `internal/analyze/toml.go`:

```go
func extractTOMLDependencies(nodes []any) []string {
	dependencies := make([]string, 0, len(nodes))
	for _, node := range nodes {
		switch value := node.(type) {
		case string:
			if value != "" {
				dependencies = append(dependencies, value)
			}
		case map[string]any:
			keys := make([]string, 0, len(value))
			for key := range value {
				if key == "python" {
					continue
				}
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for _, key := range keys {
				serialized, ok := serializeTOMLValue(value[key])
				if !ok {
					continue
				}
				dependencies = append(dependencies, fmt.Sprintf("%s = %s", key, serialized))
			}
		}
	}
	return dependencies
}

func serializeTOMLValue(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return fmt.Sprintf("%q", typed), true
	case bool:
		if typed {
			return "true", true
		}
		return "false", true
	case int64, float64:
		return fmt.Sprintf("%v", typed), true
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			serialized, ok := serializeTOMLValue(item)
			if !ok {
				return "", false
			}
			parts = append(parts, serialized)
		}
		return "[" + strings.Join(parts, ", ") + "]", true
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		var buf bytes.Buffer
		buf.WriteString("{ ")
		for idx, key := range keys {
			if idx > 0 {
				buf.WriteString(", ")
			}
			serialized, ok := serializeTOMLValue(typed[key])
			if !ok {
				return "", false
			}
			buf.WriteString(key)
			buf.WriteString(" = ")
			buf.WriteString(serialized)
		}
		buf.WriteString(" }")
		return buf.String(), true
	default:
		return "", false
	}
}
```

- [ ] **Step 6: Run the focused TOML tests**

Run: `go test ./internal/analyze -run 'Test(LoadRulesAcceptsTOMLParser|LoadRulesRejectsTOMLParserWithoutQueries|LoadRulesRejectsMalformedTOMLQuery|LoadRulesRejectsTOMLParserWithOtherParserType|ScanMatchesTOMLDependenciesFromCustomRule|ScanDoesNotMatchTOMLWhenQueryResolvesToNoUsableValues|ScanReturnsTOMLParseErrors)' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit the parser implementation**

```bash
git add internal/analyze/toml.go internal/analyze/scan_test.go
git commit -m "feat: add rule-driven toml parser"
```

### Task 3: Ship The Built-In Pyproject Detector

**Files:**
- Modify: `internal/analyze/default_rules.yaml`
- Modify: `internal/analyze/scan_test.go`
- Create: `testdata/toml/pyproject/pyproject.toml`

- [ ] **Step 1: Write the failing built-in detector tests**

Update `TestDetectManifestMatchesSupportedFiles` in `internal/analyze/scan_test.go` to include:

```go
		{name: "pyproject.toml", want: ManifestType("python-pyproject")},
```

Update `TestLoadDefaultRulesProvidesSupportedTypeOrder` to include:

```go
		ManifestType("python-pyproject"),
```

Add this end-to-end fixture test:

```go
func TestScanMatchesPyprojectDependenciesFromFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "toml", "pyproject"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(result.Manifests))
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("python-pyproject") || manifest.Path != "pyproject.toml" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}

	want := []string{
		"requests>=2.31",
		"fastapi[all]>=0.110; python_version >= '3.10'",
		"pytest>=8",
		"ruff==0.4.8",
		"mypy>=1.10",
		"django = \"^5.0\"",
		"httpx = { version = \"^0.27\", extras = [\"http2\"] }",
		"private-lib = { git = \"https://github.com/acme/private-lib.git\", branch = \"main\" }",
		"pytest-cov = \"^5.0\"",
		"factory-boy = { version = \"^3.3\", markers = \"python_version >= '3.11'\" }",
	}
	if !slices.Equal(manifest.Dependencies, want) {
		t.Fatalf("unexpected dependencies: %+v", manifest.Dependencies)
	}
}
```

- [ ] **Step 2: Add the fixture file**

Create `testdata/toml/pyproject/pyproject.toml` with:

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

- [ ] **Step 3: Run the built-in detector tests to verify they fail**

Run: `go test ./internal/analyze -run 'Test(DetectManifestMatchesSupportedFiles|LoadDefaultRulesProvidesSupportedTypeOrder|ScanMatchesPyprojectDependenciesFromFixture)' -count=1`

Expected: FAIL because the default rules file does not yet contain `python-pyproject`.

- [ ] **Step 4: Add the built-in `pyproject.toml` rule**

Append this rule to `internal/analyze/default_rules.yaml` after `python-uv`:

```yaml
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

- [ ] **Step 5: Run the built-in detector tests again**

Run: `go test ./internal/analyze -run 'Test(DetectManifestMatchesSupportedFiles|LoadDefaultRulesProvidesSupportedTypeOrder|ScanMatchesPyprojectDependenciesFromFixture)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the built-in detector**

```bash
git add internal/analyze/default_rules.yaml internal/analyze/scan_test.go testdata/toml/pyproject/pyproject.toml
git commit -m "feat: detect dependencies in pyproject toml"
```

### Task 4: Update Documentation And Run Full Verification

**Files:**
- Modify: `README.md`
- Modify: `internal/analyze/scan_test.go`

- [ ] **Step 1: Write the failing documentation expectation**

Add this assertion to `TestLoadDefaultRulesProvidesSupportedTypeOrder` if not already covered by the previous task:

```go
		ManifestType("python-pyproject"),
```

This keeps the test suite enforcing that the built-in detector remains documented in the supported-type list returned by the code.

- [ ] **Step 2: Update the README detector table**

Add a TOML row to the built-in detector table in `README.md`:

```md
| toml | TOML files matched by rules. Built-in support includes `pyproject.toml` with queries over `project.dependencies`, `project.optional-dependencies`, `dependency-groups`, `tool.poetry.dependencies`, and `tool.poetry.group.*.dependencies` | Yes |
```

Also update the example manifest list to include `python-pyproject` when relevant:

```text
python-pyproject
- pyproject.toml
```

- [ ] **Step 3: Run the package tests**

Run: `go test ./internal/analyze ./internal/render -count=1`

Expected: PASS.

- [ ] **Step 4: Run the full repository test suite**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 5: Review the final diff**

Run: `git diff --stat HEAD~4..HEAD`

Expected: A diff touching `go.mod`, `go.sum`, TOML parser code, default rules, tests, fixture data, and `README.md` with no unrelated files.

- [ ] **Step 6: Commit the documentation polish**

```bash
git add README.md
git commit -m "docs: document pyproject toml support"
```

## Self-Review

Spec coverage check:

- TOML parser added as a sibling parser type: covered by Task 1 and Task 2.
- Query language with dotted paths, `[]`, and `*`: covered by Task 2.
- Raw `name = value` serialization for dependency tables: covered by Task 2.
- Skip Poetry `python`: covered by Task 2 and Task 3 tests.
- Built-in `pyproject.toml` rule: covered by Task 3.
- README and supported-manifest updates: covered by Task 4.

Placeholder scan:

- No `TODO`, `TBD`, or deferred “handle later” instructions remain.
- Each task names exact files, commands, and code to add.

Type consistency:

- The plan uses `tomlMatcherConfig`, `newTOMLQueryParser`, `tomlQueryParser`, and `ManifestType("python-pyproject")` consistently across tasks.
