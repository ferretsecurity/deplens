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
