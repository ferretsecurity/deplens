package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPackageLockDetectManifestFileExtractsV1RootDependencies(t *testing.T) {
	ruleset := mustLoadPackageLockRules(t)
	filePath := filepath.Join(t.TempDir(), "package-lock.json")

	mustWriteFile(t, filePath, `
{
  "name": "demo",
  "lockfileVersion": 1,
  "dependencies": {
    "left-pad": {
      "version": "1.3.0"
    },
    "lodash": {
      "version": "4.17.21"
    }
  }
}
`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-npm-lock") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if want := []string{"left-pad@1.3.0", "lodash@4.17.21"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestPackageLockDetectManifestFileExtractsV3RootDependenciesAndOptionalDependenciesWithDedupe(t *testing.T) {
	ruleset := mustLoadPackageLockRules(t)
	filePath := filepath.Join(t.TempDir(), "package-lock.json")

	mustWriteFile(t, filePath, `
{
  "name": "demo",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "demo",
      "version": "1.0.0",
      "dependencies": {
        "left-pad": "^1.3.0"
      },
      "optionalDependencies": {
        "left-pad": "^1.3.0",
        "fsevents": "^2.3.3"
      }
    },
    "node_modules/left-pad": {
      "version": "1.3.0"
    },
    "node_modules/fsevents": {
      "version": "2.3.3"
    }
  }
}
`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-npm-lock") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if want := []string{"left-pad@1.3.0", "fsevents@2.3.3"}; !equalStringSets(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestPackageLockDetectManifestFileFallsBackToNameWhenVersionIsMissing(t *testing.T) {
	ruleset := mustLoadPackageLockRules(t)
	filePath := filepath.Join(t.TempDir(), "package-lock.json")

	mustWriteFile(t, filePath, `
{
  "name": "demo",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "demo",
      "version": "1.0.0",
      "dependencies": {
        "left-pad": "^1.3.0"
      }
    }
  }
}
`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-npm-lock") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if want := []string{"left-pad"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestPackageLockDetectManifestFileReturnsConclusiveEmptyWhenSupportedRootMapsAreMissing(t *testing.T) {
	ruleset := mustLoadPackageLockRules(t)
	filePath := filepath.Join(t.TempDir(), "package-lock.json")

	mustWriteFile(t, filePath, `
{
  "name": "demo",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "demo",
      "version": "1.0.0"
    }
  }
}
`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-npm-lock") {
		t.Fatalf("unexpected manifest type: got %q", got)
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

func TestPackageLockDetectManifestFileRejectsMalformedJSON(t *testing.T) {
	ruleset := mustLoadPackageLockRules(t)
	filePath := filepath.Join(t.TempDir(), "package-lock.json")

	mustWriteFile(t, filePath, `{"lockfileVersion": 3,`)

	_, _, _, _, _, err := ruleset.DetectManifestFile(filePath, "package-lock.json")
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if got := err.Error(); !strings.Contains(got, "package-lock.json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPackageLockV2ParserSetsStructuredFieldsWithSections(t *testing.T) {
	parser, _ := newPackageLockParser(packageLockMatcherConfig{})
	result, _ := parser.Match("package-lock.json", []byte(`{
        "lockfileVersion": 2,
        "packages": {
            "": {
                "dependencies": {"react": "^18"},
                "devDependencies": {"jest": "^29"},
                "optionalDependencies": {"fsevents": "^2"}
            },
            "node_modules/react": {"version": "18.0.0"},
            "node_modules/jest": {"version": "29.0.0"},
            "node_modules/fsevents": {"version": "2.3.3"}
        }
    }`))
	deps := result.Dependencies
	find := func(name string) *Dependency {
		for i := range deps {
			if deps[i].Name == name {
				return &deps[i]
			}
		}
		return nil
	}
	r := find("react")
	if r == nil {
		t.Fatal("react not found")
	}
	if r.Raw != "react@18.0.0" {
		t.Errorf("react Raw: got %q", r.Raw)
	}
	if r.Version != "18.0.0" {
		t.Errorf("react Version: got %q", r.Version)
	}
	if r.Section != "dependencies" {
		t.Errorf("react Section: got %q", r.Section)
	}
	j := find("jest")
	if j == nil {
		t.Fatal("jest not found")
	}
	if j.Section != "devDependencies" {
		t.Errorf("jest Section: got %q", j.Section)
	}
	f := find("fsevents")
	if f == nil {
		t.Fatal("fsevents not found")
	}
	if f.Section != "optionalDependencies" {
		t.Errorf("fsevents Section: got %q", f.Section)
	}
}

func mustLoadPackageLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte(`
rules:
  - name: js-npm-lock
    filename-regex: '^package-lock\.json$'
    package-lock: {}
`))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	return ruleset
}

func equalStringSets(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	gotSet := make(map[string]struct{}, len(got))
	for _, value := range got {
		gotSet[value] = struct{}{}
	}
	for _, value := range want {
		if _, ok := gotSet[value]; !ok {
			return false
		}
		delete(gotSet, value)
	}
	return len(gotSet) == 0
}
