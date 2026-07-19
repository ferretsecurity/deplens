package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestYarnLockClassicParserSetsStructuredFields(t *testing.T) {
	parser, _ := newYarnLockParser(yarnLockMatcherConfig{})
	result, _ := parser.Analyze("yarn.lock", []byte(`# yarn lockfile v1

react@^18.0.0:
  version "18.0.0"
`))
	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result.Dependencies))
	}
	dep := result.Dependencies[0]
	if dep.Raw != "react@18.0.0" {
		t.Errorf("Raw: got %q", dep.Raw)
	}
	if dep.Name != "react" {
		t.Errorf("Name: got %q", dep.Name)
	}
	if dep.Version != "18.0.0" {
		t.Errorf("Version: got %q", dep.Version)
	}
}

func TestYarnLockAnalyzeDependencySourceExtractsClassicEntries(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

left-pad@^1.3.0:
	version "1.3.0"

lodash@^4.17.0, lodash@~4.17.21:
	version "4.17.21"
`)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-yarn") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if want := []string{"left-pad@1.3.0", "lodash@4.17.21"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestYarnLockAnalyzeDependencySourceDeduplicatesClassicSelectors(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

left-pad@^1.3.0, left-pad@~1.3.0:
	version "1.3.0"

left-pad@1.3.0:
	version "1.3.0"
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"left-pad@1.3.0"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestYarnLockAnalyzeDependencySourceClassicFallsBackToNameWhenVersionMissing(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

left-pad@^1.3.0:
	resolved "https://registry.yarnpkg.com/left-pad/-/left-pad-1.3.0.tgz"
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"left-pad"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestYarnLockAnalyzeDependencySourceClassicEmitsMixedVersionedAndVersionMissingEntries(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

left-pad@^1.3.0:
	version "1.3.0"

lodash@^4.17.0:
	resolved "https://registry.yarnpkg.com/lodash/-/lodash-4.17.21.tgz"
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"left-pad@1.3.0", "lodash"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestYarnLockAnalyzeDependencySourceExtractsClassicScopedGroupedSelectors(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

"@babel/code-frame@^7.0.0", "@babel/code-frame@^7.27.1":
	version "7.27.1"
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"@babel/code-frame@7.27.1"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestYarnLockAnalyzeDependencySourceClassicHeaderOnlyIsConclusiveEmpty(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, "# yarn lockfile v1\n")

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-yarn") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present == nil || *present {
		t.Fatalf("expected presence=absent, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestYarnLockAnalyzeDependencySourceExtractsModernEntries(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10

"react@npm:^18.3.1":
  version: "18.3.1"
  resolution: "react@npm:18.3.1"

"typescript@patch:typescript@npm%3A5.4.5#~builtin<compat/typescript>":
  version: "5.4.5"
  resolution: "typescript@patch:typescript@npm%3A5.4.5#~builtin<compat/typescript>"

"@babel/code-frame@npm:^7.27.1":
  version: "7.27.1"
  resolution: "@babel/code-frame@npm:7.27.1"
`)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-yarn") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if want := []string{"@babel/code-frame@7.27.1", "react@18.3.1", "typescript@5.4.5"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestYarnLockAnalyzeDependencySourceModernFallsBackToNameWhenVersionMissing(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10

"string-width@npm:^4.2.3":
  resolution: "string-width@npm:4.2.3"
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"string-width"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestYarnLockAnalyzeDependencySourceModernGroupedSelectorsFallBackToNameWithoutResolution(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10

" left-pad@npm:^1.3.0, left-pad@npm:~1.3.0 ":
  version: "1.3.0"

" @babel/core@npm:^7.0.0, @babel/core@npm:^7.27.0 ":
  version: "7.27.0"
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"@babel/core@7.27.0", "left-pad@1.3.0"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestYarnLockAnalyzeDependencySourceModernMatchesWithLeadingCommentPreamble(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# This file is generated by running "yarn install"
# Manual changes might be lost
__metadata:
  version: 8
  cacheKey: 10

"react@npm:^18.3.1":
  version: "18.3.1"
  resolution: "react@npm:18.3.1"
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"react@18.3.1"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestYarnLockAnalyzeDependencySourceModernMetadataOnlyIsConclusiveEmpty(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10
`)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-yarn") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present == nil || *present {
		t.Fatalf("expected presence=absent, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestYarnLockAnalyzeDependencySourceRejectsMalformedModernYAML(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
"react@npm:^18.3.1"
  version: "18.3.1"
`)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("expected warning, got error: %v", err)
	}
	if !ok || got != DetectorID("js-yarn") {
		t.Fatalf("expected yarn warning match, got detector=%q ok=%v", got, ok)
	}
	if deps != nil || present != nil {
		t.Fatalf("expected no dependency result, got deps=%+v presence=%+v", deps, present)
	}
	if len(diagnosticMessages) != 1 || !strings.Contains(diagnosticMessages[0], "yarn.lock") {
		t.Fatalf("expected warning to mention yarn.lock, got %+v", diagnosticMessages)
	}
}

func TestYarnLockAnalyzeDependencySourceRejectsStructurallyInvalidModernEntry(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10

"react@npm:^18.3.1": oops
`)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("expected warning, got error: %v", err)
	}
	if !ok || got != DetectorID("js-yarn") {
		t.Fatalf("expected yarn warning match, got detector=%q ok=%v", got, ok)
	}
	if deps != nil || present != nil {
		t.Fatalf("expected no dependency result, got deps=%+v presence=%+v", deps, present)
	}
	if len(diagnosticMessages) != 1 || !strings.Contains(diagnosticMessages[0], "yarn.lock") {
		t.Fatalf("expected warning to mention yarn.lock, got %+v", diagnosticMessages)
	}
}

func TestYarnLockAnalyzeDependencySourceReturnsNoMatchForUnrecognizedContent(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `not a yarn lockfile
just some text
`)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if ok {
		t.Fatalf("expected no match, got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present != nil {
		t.Fatalf("expected unknown presence, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestYarnLockAnalyzeDependencySourceClassicHeaderMatchesWithUTF8BOM(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, "\ufeff# yarn lockfile v1\n")

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-yarn") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present == nil || *present {
		t.Fatalf("expected presence=absent, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestYarnLockAnalyzeDependencySourceClassicHeaderMatchesWithGeneratedPreamble(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, "# THIS IS AN AUTOGENERATED FILE. DO NOT EDIT THIS FILE DIRECTLY.\n# yarn lockfile v1\n")

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-yarn") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present == nil || *present {
		t.Fatalf("expected presence=absent, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func mustLoadYarnLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: js-yarn\n      filename-regex: '^yarn\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: yarn-lock\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	return ruleset
}
