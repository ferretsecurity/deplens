package analyze

import (
	"path/filepath"
	"testing"
)

func TestPipfileLockDetectManifestFileExtractsDefaultAndDevelopDependencies(t *testing.T) {
	ruleset := mustLoadPipfileLockRules(t)
	filePath := filepath.Join(t.TempDir(), "Pipfile.lock")

	mustWriteFile(t, filePath, `{
  "_meta": {
    "hash": {
      "sha256": "deadbeef"
    },
    "pipfile-spec": 6
  },
  "default": {
    "requests": {
      "version": "==2.32.3"
    },
    "urllib3": {
      "version": "==2.2.2"
    }
  },
  "develop": {
    "pytest": {
      "version": "==8.3.3"
    }
  }
}`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "Pipfile.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("python-pipfile-lock") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	want := []Dependency{
		{Raw: "requests==2.32.3", Name: "requests", Version: "2.32.3", Section: "default"},
		{Raw: "urllib3==2.2.2", Name: "urllib3", Version: "2.2.2", Section: "default"},
		{Raw: "pytest==8.3.3", Name: "pytest", Version: "8.3.3", Section: "develop"},
	}
	if !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestPipfileLockParserSetsStructuredFields(t *testing.T) {
	parser, _ := newPipfileLockParser(pipfileLockMatcherConfig{})
	result, _ := parser.Match("Pipfile.lock", []byte(`{
        "_meta": {},
        "default": {"requests": {"version": "==2.32.3"}},
        "develop": {}
    }`))
	dep := result.Dependencies[0]
	if dep.Raw != "requests==2.32.3" {
		t.Errorf("Raw: got %q", dep.Raw)
	}
	if dep.Name != "requests" {
		t.Errorf("Name: got %q", dep.Name)
	}
	if dep.Version != "2.32.3" {
		t.Errorf("Version: got %q", dep.Version)
	}
	if dep.Section != "default" {
		t.Errorf("Section: got %q", dep.Section)
	}
}

func TestPipfileLockDetectManifestFileFallsBackToNameWhenVersionIsMissing(t *testing.T) {
	ruleset := mustLoadPipfileLockRules(t)
	filePath := filepath.Join(t.TempDir(), "Pipfile.lock")

	mustWriteFile(t, filePath, `{
  "_meta": {
    "hash": {
      "sha256": "deadbeef"
    },
    "pipfile-spec": 6
  },
  "default": {
    "requests": {}
  }
}`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "Pipfile.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	want := []Dependency{{Raw: "requests", Name: "requests", Section: "default"}}
	if !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestPipfileLockDetectManifestFileReturnsConclusiveEmptyForMetadataOnlyLockfile(t *testing.T) {
	ruleset := mustLoadPipfileLockRules(t)
	filePath := filepath.Join(t.TempDir(), "Pipfile.lock")

	mustWriteFile(t, filePath, `{
  "_meta": {
    "hash": {
      "sha256": "deadbeef"
    },
    "pipfile-spec": 6
  },
  "default": {},
  "develop": {}
}`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "Pipfile.lock")
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
}

func TestPipfileLockParserFixtureCoverage(t *testing.T) {
	parser, err := newPipfileLockParser(pipfileLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPipfileLockParser returned error: %v", err)
	}

	testCases := []struct {
		name       string
		fixtureDir string
		wantDeps   []Dependency
		wantHas    *bool
	}{
		{
			name:       "basic sections",
			fixtureDir: "pipfile-lock-basic",
			wantDeps: []Dependency{
				{Raw: "requests==2.32.3", Name: "requests", Version: "2.32.3", Section: "default"},
				{Raw: "urllib3==2.2.2", Name: "urllib3", Version: "2.2.2", Section: "default"},
				{Raw: "pytest==8.3.3", Name: "pytest", Version: "8.3.3", Section: "develop"},
			},
			wantHas: boolPtr(true),
		},
		{
			name:       "missing versions fall back to names",
			fixtureDir: "pipfile-lock-missing-version",
			wantDeps: []Dependency{
				{Raw: "requests", Name: "requests", Section: "default"},
				{Raw: "pytest", Name: "pytest", Section: "develop"},
			},
			wantHas: boolPtr(true),
		},
		{
			name:       "reports conclusive empty",
			fixtureDir: "pipfile-lock-empty",
			wantDeps:   nil,
			wantHas:    boolPtr(false),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := mustReadTestdataFile(t, "python", tc.fixtureDir, "Pipfile.lock")
			result, err := parser.Match("Pipfile.lock", content)
			if err != nil {
				t.Fatalf("Match returned error: %v", err)
			}
			if !result.Matched {
				t.Fatalf("expected parser to match Pipfile.lock")
			}
			if tc.wantHas == nil {
				if result.HasDependencies != nil {
					t.Fatalf("expected has_dependencies=nil, got %+v", result.HasDependencies)
				}
			} else if result.HasDependencies == nil || *result.HasDependencies != *tc.wantHas {
				t.Fatalf("unexpected has_dependencies: got %+v want %+v", result.HasDependencies, tc.wantHas)
			}
			if !equalDependencies(result.Dependencies, tc.wantDeps) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, tc.wantDeps)
			}
		})
	}
}

func mustLoadPipfileLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte(`
rules:
  - name: python-pipfile-lock
    filename-regex: '^Pipfile\.lock$'
    pipfile-lock: {}
`))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	return ruleset
}
