package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestScanGradleConstraintsPreserveRelationships(t *testing.T) {
	type expectedReference struct {
		raw          string
		group        string
		relationship Relationship
		scope        DependencyScope
	}
	tests := []struct {
		fixture  string
		path     string
		detector DetectorID
		want     []expectedReference
	}{
		{
			fixture:  "gradle-constraints",
			path:     "build.gradle",
			detector: "java-gradle",
			want: []expectedReference{
				{"org.example:core:1.2", "constraints.implementation", RelationshipInconclusive, ScopeRuntime},
				{"org.example:runtime:2.0", "constraints.runtimeOnly", RelationshipInconclusive, ScopeRuntime},
				{"org.example:test-platform:3.0", "constraints.testImplementation", RelationshipInconclusive, ScopeTest},
			},
		},
		{
			fixture:  "gradle-constraints-kts",
			path:     "build.gradle.kts",
			detector: "java-gradle-kts",
			want: []expectedReference{
				{"org.example:core:1.2", "implementation", RelationshipDirect, ScopeRuntime},
				{"org.example:core:1.2", "constraints.implementation", RelationshipInconclusive, ScopeRuntime},
				{"org.example:platform:2.0", "constraints.api", RelationshipInconclusive, ScopeRuntime},
				{"org.example:compile:3.0", "constraints.compileOnly", RelationshipInconclusive, ScopeBuild},
				{"org.example:test-kit:4.0", "testImplementation", RelationshipDirect, ScopeTest},
			},
		},
	}
	ruleset := mustLoadDefaultRules(t)
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "java", tt.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("expected one source, got %+v", result.Sources)
			}
			source := result.Sources[0]
			if source.Path != tt.path || source.Detector != tt.detector ||
				source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("unexpected source: %+v", source)
			}
			if len(source.Dependencies) != len(tt.want) {
				t.Fatalf("expected %d dependencies, got %+v", len(tt.want), source.Dependencies)
			}
			for _, want := range tt.want {
				if !slices.ContainsFunc(source.Dependencies, func(d DependencyReference) bool {
					return d.Raw == want.raw && d.SourceGroup == want.group && d.Relationship == want.relationship &&
						d.Scope == want.scope && d.PackageType == "maven" && d.VERS != ""
				}) {
					t.Errorf("missing reference %+v in %+v", want, source.Dependencies)
				}
			}
		})
	}
}

func TestScanGradleVersionCatalogDoesNotNormalizeRichVersions(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "java", "gradle-version-catalog-rich"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected one catalog, got %+v", result.Sources)
	}
	source := result.Sources[0]
	if source.Detector != "java-gradle-version-catalog" || source.Analysis.Extraction != ExtractionPartial {
		t.Fatalf("expected partial version catalog, got %+v", source)
	}
	if len(source.Dependencies) != 9 || len(source.Diagnostics) != 8 {
		t.Fatalf("expected all aliases and rich-version warnings, got %+v", source)
	}
	for _, dependency := range source.Dependencies {
		if dependency.PackageType != "maven" {
			t.Errorf("unexpected package type: %+v", dependency)
		}
		if dependency.Attributes["alias"] == "simple" {
			if dependency.VersionConstraint != "[1.5]" || dependency.VERS != "vers:maven/1.5" {
				t.Errorf("simple versions should still normalize: %+v", dependency)
			}
			continue
		}
		if dependency.Version != "" || dependency.VersionConstraint != "" || dependency.VERS != "" {
			t.Errorf("rich version acquired a shared version or VERS: %+v", dependency)
		}
	}
}
