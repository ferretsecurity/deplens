package analyze

import (
	"path/filepath"
	"testing"
)

func TestScanRRenvLockExtractsCorpusPatterns(t *testing.T) {
	testCases := []struct {
		name string
		path string
		want []DependencyReference
	}{
		{
			name: "repository package without requirements",
			path: filepath.Join("..", "..", "testdata", "r", "renv-lock-r-4-1"),
			want: []DependencyReference{
				{PackageType: "cran", Raw: "plainpkg@1.2.3", Name: "plainpkg", Version: "1.2.3"},
			},
		},
		{
			name: "repository packages with requirements",
			path: filepath.Join("..", "..", "testdata", "r", "renv-lock-r-4-4"),
			want: []DependencyReference{
				{PackageType: "cran", Raw: "graphpkg@2.0.0", Name: "graphpkg", Version: "2.0.0"},
				{PackageType: "cran", Raw: "helperpkg@1.1.0", Name: "helperpkg", Version: "1.1.0"},
			},
		},
		{
			name: "repository spatial package graph",
			path: filepath.Join("..", "..", "testdata", "r", "renv-lock-r-4-3"),
			want: []DependencyReference{
				{PackageType: "cran", Raw: "spatialpkg@3.0.0", Name: "spatialpkg", Version: "3.0.0"},
				{PackageType: "cran", Raw: "utilitypkg@0.4.0", Name: "utilitypkg", Version: "0.4.0"},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(tc.path, nil, ruleset)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one dependency source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != DetectorID("r-renv-lock") {
				t.Fatalf("unexpected detector: %q", source.Detector)
			}
			if source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("unexpected analysis: %+v", source.Analysis)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, tc.want)
			}
		})
	}
}
