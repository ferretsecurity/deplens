package analyze

import (
	"path/filepath"
	"testing"
)

func TestPipfileLockAnalyzeDependencySourceExtractsDefaultAndDevelopDependencies(t *testing.T) {
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

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "Pipfile.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("python-pipfile-lock") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	want := []DependencyReference{
		{Raw: "requests==2.32.3", Name: "requests", Version: "2.32.3", SourceGroup: "default"},
		{Raw: "urllib3==2.2.2", Name: "urllib3", Version: "2.2.2", SourceGroup: "default"},
		{Raw: "pytest==8.3.3", Name: "pytest", Version: "8.3.3", SourceGroup: "develop"},
	}
	if !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestPipfileLockParserSetsStructuredFields(t *testing.T) {
	parser, _ := newPipfileLockParser(pipfileLockMatcherConfig{})
	result, _ := parser.Analyze("Pipfile.lock", []byte(`{
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
	if dep.SourceGroup != "default" {
		t.Errorf("SourceGroup: got %q", dep.SourceGroup)
	}
}

func TestPipfileLockAnalyzeDependencySourceFallsBackToNameWhenVersionIsMissing(t *testing.T) {
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

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "Pipfile.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	want := []DependencyReference{{Raw: "requests", Name: "requests", SourceGroup: "default"}}
	if !equalDependencies(deps, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
}

func TestPipfileLockAnalyzeDependencySourceReturnsConclusiveEmptyForMetadataOnlyLockfile(t *testing.T) {
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

	_, deps, present, _, ok, err := analyzeSourceParts(ruleset, filePath, "Pipfile.lock")
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
}

func TestPipfileLockParserFixtureCoverage(t *testing.T) {
	parser, err := newPipfileLockParser(pipfileLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPipfileLockParser returned error: %v", err)
	}

	testCases := []struct {
		name        string
		fixtureDir  string
		wantDeps    []DependencyReference
		wantPresent *bool
	}{
		{
			name:       "basic sections",
			fixtureDir: "pipfile-lock-basic",
			wantDeps: []DependencyReference{
				{Raw: "requests==2.32.3", Name: "requests", Version: "2.32.3", SourceGroup: "default"},
				{Raw: "urllib3==2.2.2", Name: "urllib3", Version: "2.2.2", SourceGroup: "default"},
				{Raw: "pytest==8.3.3", Name: "pytest", Version: "8.3.3", SourceGroup: "develop"},
			},
			wantPresent: boolPtr(true),
		},
		{
			name:       "missing versions fall back to names",
			fixtureDir: "pipfile-lock-missing-version",
			wantDeps: []DependencyReference{
				{Raw: "requests", Name: "requests", SourceGroup: "default"},
				{Raw: "pytest", Name: "pytest", SourceGroup: "develop"},
			},
			wantPresent: boolPtr(true),
		},
		{
			name:        "reports conclusive empty",
			fixtureDir:  "pipfile-lock-empty",
			wantDeps:    nil,
			wantPresent: boolPtr(false),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := mustReadTestdataFile(t, "python", tc.fixtureDir, "Pipfile.lock")
			result, err := parser.Analyze("Pipfile.lock", content)
			if err != nil {
				t.Fatalf("Match returned error: %v", err)
			}
			if !result.Recognized {
				t.Fatalf("expected parser to match Pipfile.lock")
			}
			if tc.wantPresent == nil {
				if result.Analysis.Presence != "" && result.Analysis.Presence != PresenceUnknown {
					t.Fatalf("expected presence=unknown, got %+v", result.Analysis)
				}
			} else if result.Analysis.Presence != presenceAnalysis(*tc.wantPresent).Presence {
				t.Fatalf("unexpected presence: got %+v want %+v", result.Analysis, tc.wantPresent)
			}
			if !equalDependencies(result.Dependencies, tc.wantDeps) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, tc.wantDeps)
			}
		})
	}
}

func mustLoadPipfileLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: python-pipfile-lock\n      filename-regex: '^Pipfile\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: pipfile-lock\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	return ruleset
}
