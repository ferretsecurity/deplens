package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDenoJSONFixturesExtractImportMapDependencies(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		analysis   SourceAnalysis
		want       []DependencyReference
	}{
		{
			name:       "URL and npm imports",
			fixtureDir: "import-map-url-and-npm",
			analysis:   SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "fresh@https://deno.land/x/fresh@1.7.3/mod.ts", Name: "fresh", SourceGroup: "imports", OriginKind: OriginURL, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://deno.land/x/fresh@1.7.3/mod.ts"}},
				{PackageType: "npm", Raw: "tailwindcss@npm:tailwindcss@3.3.5", Name: "tailwindcss", VersionConstraint: "3.3.5", VERS: "vers:npm/3.3.5", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "npm"}},
				{PackageType: "npm", Raw: "tailwindcss/@npm:tailwindcss@3.3.5/", Name: "tailwindcss", VersionConstraint: "3.3.5", VERS: "vers:npm/3.3.5", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"declared_name": "tailwindcss/", "registry": "npm"}},
			},
		},
		{
			name:       "no import map",
			fixtureDir: "import-map-no-dependencies",
			analysis:   SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:       nil,
		},
		{
			name:       "JSR imports",
			fixtureDir: "import-map-jsr",
			analysis:   SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "@std/assert@jsr:@std/assert@1.0.11", Name: "@std/assert", VersionConstraint: "1.0.11", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "jsr"}},
				{PackageType: "generic", Raw: "@std/bytes@jsr:@std/bytes@1.0.5", Name: "@std/bytes", VersionConstraint: "1.0.5", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "jsr"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "deno", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "deno-json" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
