package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPackageLockAnalyzeDependencySourceExtractsV1RootDependencies(t *testing.T) {
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

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-npm-lock") {
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

func TestPackageLockAnalyzeDependencySourceExtractsV3RootDependenciesAndOptionalDependenciesWithDedupe(t *testing.T) {
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

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-npm-lock") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if want := []string{"left-pad@1.3.0", "fsevents@2.3.3"}; !equalStringSets(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestPackageLockV3AnalyzeDependencySourceIncludesTransitiveNodeModules(t *testing.T) {
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
      "dependencies": { "left-pad": "^1.3.0" }
    },
    "node_modules/left-pad": { "version": "1.3.0" },
    "node_modules/loose-envify": { "version": "1.4.0" }
  }
}
`)
	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	want := []string{"left-pad@1.3.0", "loose-envify@1.4.0"}
	if !equalStringSets(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", dependencyNames(deps), want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPackageLockV1WalksNestedDependencies(t *testing.T) {
	ruleset := mustLoadPackageLockRules(t)
	filePath := filepath.Join(t.TempDir(), "package-lock.json")
	mustWriteFile(t, filePath, `{
  "name": "demo",
  "lockfileVersion": 1,
  "dependencies": {
    "left-pad": {
      "version": "1.3.0",
      "dependencies": {
        "loose-envify": { "version": "1.4.0" }
      }
    }
  }
}`)

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"left-pad@1.3.0", "loose-envify@1.4.0"}; !equalStringSets(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", dependencyNames(deps), want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPackageLockAnalyzeDependencySourceFallsBackToNameWhenVersionIsMissing(t *testing.T) {
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

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-npm-lock") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if want := []string{"left-pad"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestPackageLockAnalyzeDependencySourceReturnsConclusiveEmptyWhenSupportedRootMapsAreMissing(t *testing.T) {
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

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("js-npm-lock") {
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

func TestPackageLockAnalyzeDependencySourceRejectsMalformedJSON(t *testing.T) {
	ruleset := mustLoadPackageLockRules(t)
	filePath := filepath.Join(t.TempDir(), "package-lock.json")

	mustWriteFile(t, filePath, `{"lockfileVersion": 3,`)

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "package-lock.json")
	if err != nil {
		t.Fatalf("expected warning, got error: %v", err)
	}
	if !ok || got != DetectorID("js-npm-lock") {
		t.Fatalf("expected package-lock warning match, got detector=%q ok=%v", got, ok)
	}
	if deps != nil || present != nil {
		t.Fatalf("expected no dependency result, got deps=%+v presence=%+v", deps, present)
	}
	if len(diagnosticMessages) != 1 || !strings.Contains(diagnosticMessages[0], "package-lock.json") {
		t.Fatalf("unexpected diagnostics: %+v", diagnosticMessages)
	}
}

func TestPackageLockV2ParserSetsStructuredFieldsWithSections(t *testing.T) {
	parser, _ := newPackageLockParser(packageLockMatcherConfig{})
	result, _ := parser.Analyze("package-lock.json", []byte(`{
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
	find := func(name string) *DependencyReference {
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
	if r.SourceGroup != "dependencies" {
		t.Errorf("react SourceGroup: got %q", r.SourceGroup)
	}
	j := find("jest")
	if j == nil {
		t.Fatal("jest not found")
	}
	if j.SourceGroup != "devDependencies" {
		t.Errorf("jest SourceGroup: got %q", j.SourceGroup)
	}
	f := find("fsevents")
	if f == nil {
		t.Fatal("fsevents not found")
	}
	if f.SourceGroup != "optionalDependencies" {
		t.Errorf("fsevents SourceGroup: got %q", f.SourceGroup)
	}
}

func mustLoadPackageLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: js-npm-lock\n      filename-regex: '^package-lock\\.json$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: package-lock\n"))
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
