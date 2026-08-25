package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGopkgLockFixturesExtractDependencyReferences(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "versioned and branch projects",
			fixtureDir: "gopkg-lock-versioned-and-branch",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.test/branch@0123456789abcdef", Name: "example.test/branch", Version: "0123456789abcdef", SourceGroup: "projects", Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_branch": "main"}},
				{PackageType: "golang", Raw: "example.test/tagged@fedcba9876543210", Name: "example.test/tagged", Version: "fedcba9876543210", SourceGroup: "projects", Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_tag": "v1.2.3"}},
			},
		},
		{
			name:       "single versioned project",
			fixtureDir: "gopkg-lock-single-project",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.test/sqlmock@1111111111111111", Name: "example.test/sqlmock", Version: "1111111111111111", SourceGroup: "projects", Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_tag": "v1.3.0"}},
			},
		},
		{
			name:       "multiple projects",
			fixtureDir: "gopkg-lock-multiple-projects",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.test/jwt@2222222222222222", Name: "example.test/jwt", Version: "2222222222222222", SourceGroup: "projects", Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_tag": "v3.2.0"}},
				{PackageType: "golang", Raw: "example.test/sse@3333333333333333", Name: "example.test/sse", Version: "3333333333333333", SourceGroup: "projects", Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_branch": "master"}},
				{PackageType: "golang", Raw: "example.test/web@4444444444444444", Name: "example.test/web", Version: "4444444444444444", SourceGroup: "projects", Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_tag": "v1.2"}},
			},
		},
		{
			name:       "no projects",
			fixtureDir: "gopkg-lock-no-deps",
			want:       []DependencyReference{},
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
			if len(tc.want) == 0 {
				wantAnalysis.Presence = PresenceAbsent
			}
			if source.Detector != "go-gopkg-lock" || source.Analysis != wantAnalysis {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
