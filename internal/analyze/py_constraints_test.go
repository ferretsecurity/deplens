package analyze

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPythonConstraintsExtractDependenciesFromFixtures(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	tests := []struct {
		name        string
		fixture     string
		wantRaw     []string
		wantNames   []string
		wantVersion []string
		wantMarker  []string
	}{
		{
			name:        "pinned packages",
			fixture:     "constraints-pinned",
			wantRaw:     []string{"fastapi==0.136.1", "pydantic-settings==2.14.0"},
			wantNames:   []string{"fastapi", "pydantic-settings"},
			wantVersion: []string{"==0.136.1", "==2.14.0"},
			wantMarker:  []string{"", ""},
		},
		{
			name:        "marker guarded constraint",
			fixture:     "constraints-marker",
			wantRaw:     []string{"pytest < 5.0 ; python_version < '3.5'"},
			wantNames:   []string{"pytest"},
			wantVersion: []string{"< 5.0"},
			wantMarker:  []string{"python_version < '3.5'"},
		},
		{
			name:        "index directive and local versions",
			fixture:     "constraints-index-local-version",
			wantRaw:     []string{"torch==2.9.1+cu128", "torchaudio==2.9.1+cu128"},
			wantNames:   []string{"torch", "torchaudio"},
			wantVersion: []string{"==2.9.1+cu128", "==2.9.1+cu128"},
			wantMarker:  []string{"", ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "python", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}

			source := result.Sources[0]
			if source.Detector != "python-constraints" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("unexpected source: %+v", source)
			}
			if got := dependencyNames(source.Dependencies); !slices.Equal(got, tc.wantRaw) {
				t.Fatalf("dependency raw values: got %q want %q", got, tc.wantRaw)
			}
			for index, dependency := range source.Dependencies {
				if dependency.PackageType != "pypi" || dependency.Name != tc.wantNames[index] || dependency.VersionConstraint != tc.wantVersion[index] || dependency.Attributes["marker"] != tc.wantMarker[index] {
					t.Fatalf("dependency %d: got %+v", index, dependency)
				}
			}
		})
	}
}

func TestPythonConstraintsReportsConclusiveEmptyForConfigurationOnlyFile(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	directory := t.TempDir()
	path := filepath.Join(directory, "constraints.txt")
	if err := os.WriteFile(path, []byte("--index-url https://pypi.example.test/simple\n--extra-index-url https://packages.example.test/simple\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	result, err := Scan(directory, nil, ruleset)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected one source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Detector != "python-constraints" || source.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected source: %+v", source)
	}
	if len(source.Dependencies) != 0 {
		t.Fatalf("dependencies: got %+v, want none", source.Dependencies)
	}
}
