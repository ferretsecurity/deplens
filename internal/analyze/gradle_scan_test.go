package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestScanGradleVersionlessAndRichVersions(t *testing.T) {
	tests := []struct {
		fixture      string
		path         string
		detector     DetectorID
		extraction   ExtractionState
		dependencies []string
	}{
		{
			fixture:      "gradle-versionless-groovy",
			path:         "build.gradle",
			detector:     "java-gradle",
			extraction:   ExtractionComplete,
			dependencies: []string{"org.junit:junit-bom:5.10.0", "org.junit.jupiter:junit-jupiter"},
		},
		{
			fixture:      "gradle-versionless-kotlin",
			path:         "build.gradle.kts",
			detector:     "java-gradle-kts",
			extraction:   ExtractionComplete,
			dependencies: []string{"org.junit:junit-bom:5.10.0", "org.junit.jupiter:junit-jupiter"},
		},
		{
			fixture:      "gradle-rich-versions-groovy",
			path:         "build.gradle",
			detector:     "java-gradle",
			extraction:   ExtractionPartial,
			dependencies: []string{"org.example:core", "org.example:dynamic"},
		},
		{
			fixture:      "gradle-rich-versions-kotlin",
			path:         "build.gradle.kts",
			detector:     "java-gradle-kts",
			extraction:   ExtractionPartial,
			dependencies: []string{"org.example:core", "org.example:dynamic"},
		},
	}
	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "java", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("unexpected sources: %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Path != test.path || source.Detector != test.detector || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: test.extraction}) {
				t.Fatalf("unexpected source: %+v", source)
			}
			if got := dependencyNames(source.Dependencies); !slices.Equal(got, test.dependencies) {
				t.Fatalf("dependencies: got %v want %v", got, test.dependencies)
			}
			if test.extraction == ExtractionPartial && len(source.Diagnostics) == 0 {
				t.Fatal("expected a diagnostic for unsupported rich versions")
			}
			for _, dependency := range source.Dependencies {
				if dependency.Name == "org.junit:junit-bom" {
					continue
				}
				if dependency.Version != "" || dependency.VersionConstraint != "" || dependency.VERS != "" {
					t.Fatalf("expected unknown version: %+v", dependency)
				}
				if dependency.Name == "org.junit.jupiter:junit-jupiter" && (dependency.SourceGroup != "testImplementation" || dependency.Scope != ScopeTest) {
					t.Fatalf("unexpected configuration or scope: %+v", dependency)
				}
			}
		})
	}
}
