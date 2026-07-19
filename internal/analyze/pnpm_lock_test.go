package analyze

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPNPMLockAnalyzeDependencySourceExtractsImporterDependencies(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      react:
        specifier: ^18.3.1
        version: 18.3.1
    devDependencies:
      '@types/node':
        specifier: ^20.12.7
        version: 20.12.7
    optionalDependencies:
      fsevents:
        specifier: ^2.3.3
        version: 2.3.3
`)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-pnpm-lock") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if want := []DependencyReference{
		{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "^18.3.1"}},
		{Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", SourceGroup: "devDependencies", Attributes: map[string]string{"specifier": "^20.12.7"}},
		{Raw: "fsevents@2.3.3", Name: "fsevents", Version: "2.3.3", SourceGroup: "optionalDependencies", Attributes: map[string]string{"specifier": "^2.3.3"}},
	}; !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestPNPMLockAnalyzeDependencySourceFallsBackToSpecifierWhenVersionIsMissing(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      left-pad:
        specifier: ^1.3.0
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []DependencyReference{{Raw: "left-pad@^1.3.0", Name: "left-pad", SourceGroup: "dependencies", VersionConstraint: "^1.3.0"}}; !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPNPMLockAnalyzeDependencySourceAcceptsScalarImporterDependencies(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '6.0'

importers:
  .:
    dependencies:
      react: 18.3.1
    devDependencies:
      '@types/node': 20.12.7
    optionalDependencies:
      fsevents: 2.3.3
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []DependencyReference{
		{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies"},
		{Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", SourceGroup: "devDependencies"},
		{Raw: "fsevents@2.3.3", Name: "fsevents", Version: "2.3.3", SourceGroup: "optionalDependencies"},
	}; !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPNPMLockAnalyzeDependencySourceExtractsTopLevelDependenciesForOlderLocks(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '5.4'

dependencies:
  react: 18.3.1
devDependencies:
  '@types/node': 20.12.7
optionalDependencies:
  fsevents: 2.3.3
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []DependencyReference{
		{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies"},
		{Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", SourceGroup: "devDependencies"},
		{Raw: "fsevents@2.3.3", Name: "fsevents", Version: "2.3.3", SourceGroup: "optionalDependencies"},
	}; !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPNPMLockAnalyzeDependencySourceMergesPackagesSectionV9(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      react:
        specifier: ^18.3.1
        version: 18.3.1

packages:
  react@18.3.1: {}
  loose-envify@1.4.0: {}
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	want := []DependencyReference{
		{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "^18.3.1"}},
		{Raw: "loose-envify@1.4.0", Name: "loose-envify", Version: "1.4.0"},
	}
	if !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPNPMLockPackageKeyV5Path(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")
	mustWriteFile(t, filePath, `
lockfileVersion: '5.4'

dependencies:
  react: 18.3.1

packages:
  /react/18.3.1: {}
  /loose-envify/1.4.0: {}
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	want := []DependencyReference{
		{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies"},
		{Raw: "loose-envify@1.4.0", Name: "loose-envify", Version: "1.4.0"},
	}
	if !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPNPMLockAnalyzeDependencySourceExtractsOnlyRootImporterDependencies(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      react:
        version: 18.3.1
  packages/api:
    dependencies:
      fastify:
        version: 5.2.1
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []DependencyReference{{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies"}}; !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPNPMLockAnalyzeDependencySourceFallsBackToNameWhenVersionAndSpecifierAreMissing(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      left-pad: {}
`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []DependencyReference{{Raw: "left-pad", Name: "left-pad", SourceGroup: "dependencies"}}; !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPNPMLockAnalyzeDependencySourceReturnsConclusiveEmptyWhenImportersHaveNoDependencies(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '9.0'

importers:
  .: {}
`)

	_, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
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

func TestPNPMLockAnalyzeDependencySourceReturnsNoMatchWithoutLockfileVersion(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
importers:
  .:
    dependencies:
      react:
        version: 18.3.1
`)

	_, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if ok {
		t.Fatalf("expected no match")
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

func TestPNPMLockAnalyzeDependencySourceRejectsMalformedYAML(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, "lockfileVersion: '9.0'\nimporters: [")

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("expected warning, got error: %v", err)
	}
	if !ok || got != DetectorID("js-pnpm-lock") {
		t.Fatalf("expected pnpm lock warning match, got detector=%q ok=%v", got, ok)
	}
	if deps != nil || present != nil {
		t.Fatalf("expected no dependency result, got deps=%+v presence=%+v", deps, present)
	}
	if len(diagnosticMessages) != 1 || !strings.Contains(diagnosticMessages[0], "pnpm-lock.yaml") {
		t.Fatalf("unexpected diagnostics: %+v", diagnosticMessages)
	}
}

func mustLoadPNPMLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: js-pnpm-lock\n      filename-regex: '^pnpm-lock\\.yaml$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: pnpm-lock\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	return ruleset
}
