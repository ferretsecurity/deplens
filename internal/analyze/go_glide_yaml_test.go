package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestGlideYAMLFixturesExtractDependencyReferences(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
		analysis   SourceAnalysis
	}{
		{
			name:       "runtime imports with repository metadata",
			fixtureDir: "glide-yaml-runtime-metadata",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.test/router@0123abcd", Name: "example.test/router", VersionConstraint: "0123abcd", VERS: "vers:golang/0123abcd", SourceGroup: "import", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://code.example.test/platform/router", "vcs": "git"}},
				{PackageType: "golang", Raw: "example.test/setting", Name: "example.test/setting", SourceGroup: "import", Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:       "version constraints and subpackages",
			fixtureDir: "glide-yaml-version-constraints",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.test/cloud@~2.4.0", Name: "example.test/cloud", VersionConstraint: "~2.4.0", VERS: "vers:golang/~2.4.0", SourceGroup: "import", Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "golang", Raw: "example.test/plain", Name: "example.test/plain", SourceGroup: "import", Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:       "runtime and plural test imports",
			fixtureDir: "glide-yaml-test-imports",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "example.test/command@v1.3.0", Name: "example.test/command", VersionConstraint: "v1.3.0", VERS: "vers:golang/v1.3.0", SourceGroup: "import", Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "golang", Raw: "example.test/assert", Name: "example.test/assert", SourceGroup: "testImports", Relationship: RelationshipDirect, Scope: ScopeTest},
			},
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:       "singular test import",
			fixtureDir: "glide-yaml-with-deps",
			want: []DependencyReference{
				{PackageType: "golang", Raw: "github.com/gin-gonic/gin", Name: "github.com/gin-gonic/gin", SourceGroup: "import", Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "golang", Raw: "github.com/stretchr/testify", Name: "github.com/stretchr/testify", SourceGroup: "testImport", Relationship: RelationshipDirect, Scope: ScopeTest},
			},
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:       "manifest without imports",
			fixtureDir: "glide-yaml-no-deps",
			want:       []DependencyReference{},
			analysis:   SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
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
			if source.Detector != "go-glide-yaml" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
