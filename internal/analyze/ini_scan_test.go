package analyze

import (
	"path/filepath"
	"testing"
)

func TestScanSetupCfgRequirementSyntax(t *testing.T) {
	tests := []struct {
		fixture string
		want    []DependencyReference
	}{
		{
			fixture: "setup-cfg-inline-requires",
			want: []DependencyReference{
				{Name: "setuptools_scm", VersionConstraint: ">=8"},
				{Name: "wheel"},
				{Name: "requests", VersionConstraint: ">=2.31,<3"},
				{Name: "urllib3", VersionConstraint: "<3"},
				{Name: "pytest", VersionConstraint: ">=8"},
				{Name: "ruff", VersionConstraint: ">=0.4"},
				{Name: "sphinx", VersionConstraint: ">=7"},
			},
		},
		{
			fixture: "setup-cfg-pep508-requires",
			want: []DependencyReference{
				{Name: "wheel"},
				{Name: "setuptools_scm", VersionConstraint: ">=8"},
				{Name: "requests", VersionConstraint: ">=2.31,<3"},
				{Name: "urllib3", VersionConstraint: "<3"},
				{Name: "pytest", VersionConstraint: ">=8"},
				{Name: "ruff", VersionConstraint: ">=0.4"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "python", tt.fixture), nil, mustLoadDefaultRules(t))
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "python-setup-cfg" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("unexpected source: %+v", source)
			}
			if len(source.Dependencies) != len(tt.want) {
				t.Fatalf("dependencies: got %+v, want %+v", source.Dependencies, tt.want)
			}
			for i, want := range tt.want {
				got := source.Dependencies[i]
				if got.Name != want.Name || got.VersionConstraint != want.VersionConstraint || got.PackageType != "pypi" {
					t.Errorf("dependency %d: got %+v, want %+v", i, got, want)
				}
			}
		})
	}
}
