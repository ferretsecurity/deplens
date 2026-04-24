package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPNPMLockDetectManifestFileExtractsImporterDependencies(t *testing.T) {
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

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-pnpm-lock") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if want := []Dependency{
		{Name: "react@18.3.1", Section: "dependencies"},
		{Name: "@types/node@20.12.7", Section: "devDependencies"},
		{Name: "fsevents@2.3.3", Section: "optionalDependencies"},
	}; !slices.Equal(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestPNPMLockDetectManifestFileFallsBackToSpecifierWhenVersionIsMissing(t *testing.T) {
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

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []Dependency{{Name: "left-pad@^1.3.0", Section: "dependencies"}}; !slices.Equal(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestPNPMLockDetectManifestFileAcceptsScalarImporterDependencies(t *testing.T) {
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

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []Dependency{
		{Name: "react@18.3.1", Section: "dependencies"},
		{Name: "@types/node@20.12.7", Section: "devDependencies"},
		{Name: "fsevents@2.3.3", Section: "optionalDependencies"},
	}; !slices.Equal(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestPNPMLockDetectManifestFileFallsBackToNameWhenVersionAndSpecifierAreMissing(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '9.0'

importers:
  .:
    dependencies:
      left-pad: {}
`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []Dependency{{Name: "left-pad", Section: "dependencies"}}; !slices.Equal(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestPNPMLockDetectManifestFileReturnsConclusiveEmptyWhenImportersHaveNoDependencies(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
lockfileVersion: '9.0'

importers:
  .: {}
`)

	_, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if hasDependencies == nil || *hasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestPNPMLockDetectManifestFileReturnsNoMatchWithoutLockfileVersion(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, `
importers:
  .:
    dependencies:
      react:
        version: 18.3.1
`)

	_, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "pnpm-lock.yaml")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if ok {
		t.Fatalf("expected no match")
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if hasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestPNPMLockDetectManifestFileRejectsMalformedYAML(t *testing.T) {
	ruleset := mustLoadPNPMLockRules(t)
	filePath := filepath.Join(t.TempDir(), "pnpm-lock.yaml")

	mustWriteFile(t, filePath, "lockfileVersion: '9.0'\nimporters: [")

	_, _, _, _, _, err := ruleset.DetectManifestFile(filePath, "pnpm-lock.yaml")
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if got := err.Error(); !strings.Contains(got, "pnpm-lock.yaml") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustLoadPNPMLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte(`
rules:
  - name: js-pnpm-lock
    filename-regex: '^pnpm-lock\.yaml$'
    pnpm-lock: {}
`))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	return ruleset
}
