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
	if want := []string{"left-pad", "lodash"}; !slices.Equal(dependencyNames(deps), want) {
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
	if want := []string{"left-pad", "fsevents"}; !equalStringSets(dependencyNames(deps), want) {
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
