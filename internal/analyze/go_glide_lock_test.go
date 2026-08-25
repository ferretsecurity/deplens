package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGlideLockFixturesExtractDependencyReferences(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
		analysis   SourceAnalysis
	}{
		{
			name:       "empty imports and test imports",
			fixtureDir: "glide-lock-empty",
			want:       []DependencyReference{},
			analysis:   SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
		},
		{
			name:       "imports with Git metadata",
			fixtureDir: "glide-lock-imports",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.com/http@0123456789abcdef", Name: "example.com/http", Version: "0123456789abcdef", SourceGroup: "imports", Relationship: RelationshipInconclusive, Scope: ScopeRuntime},
				{PackageType: "golang", Raw: "golang.example/network@fedcba9876543210", Name: "golang.example/network", Version: "fedcba9876543210", SourceGroup: "imports", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.com/network", "vcs": "git"}},
			},
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:       "imports without test imports",
			fixtureDir: "glide-lock-compact",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.com/context@1111111111111111", Name: "example.com/context", Version: "1111111111111111", SourceGroup: "imports", Relationship: RelationshipInconclusive, Scope: ScopeRuntime},
				{PackageType: "golang", Raw: "example.com/session@2222222222222222", Name: "example.com/session", Version: "2222222222222222", SourceGroup: "imports", Relationship: RelationshipInconclusive, Scope: ScopeRuntime},
			},
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "go", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "go-glide-lock" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
