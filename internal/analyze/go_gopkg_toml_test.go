package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGopkgTOMLFixturesExtractDependencyReferences(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "overrides with source and branch",
			fixtureDir: "gopkg-toml-overrides",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.test/log", Name: "example.test/log", SourceGroup: "override", Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "example.test/forks/log"}},
				{PackageType: "golang", Raw: "example.test/network", Name: "example.test/network", SourceGroup: "override", Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_branch": "master"}},
			},
		},
		{
			name:       "versioned and branch constraints",
			fixtureDir: "gopkg-toml-constraints",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.test/logger@1.0.0", Name: "example.test/logger", VersionConstraint: "1.0.0", VERS: "vers:golang/1.0.0", SourceGroup: "constraint", Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "golang", Raw: "example.test/table", Name: "example.test/table", SourceGroup: "constraint", Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_branch": "master"}},
			},
		},
		{
			name:       "constraints with alternate source",
			fixtureDir: "gopkg-toml-source-constraint",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.test/environment@1.17.0", Name: "example.test/environment", VersionConstraint: "1.17.0", VERS: "vers:golang/1.17.0", SourceGroup: "constraint", Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "golang", Raw: "example.test/queue", Name: "example.test/queue", SourceGroup: "constraint", Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_branch": "fixshutdown", "source_url": "example.test/forks/queue"}},
			},
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
			wantAnalysis := SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}
			if source.Detector != "go-gopkg-toml" || source.Analysis != wantAnalysis {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
