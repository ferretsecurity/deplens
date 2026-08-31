package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDenoJSONCFixturesExtractImportMapDependencies(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		analysis   SourceAnalysis
		want       []DependencyReference
	}{
		{
			name:       "comments trailing commas and local import",
			fixtureDir: "imports-with-comments",
			analysis:   SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "@sample/assert@jsr:@sample/assert@^1.2.3", Name: "@sample/assert", VersionConstraint: "^1.2.3", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "jsr"}},
				{PackageType: "generic", Raw: "local@./src/module.ts", Name: "local", SourceGroup: "imports", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"path": "./src/module.ts"}},
				{PackageType: "npm", Raw: "tooling@npm:tooling@^4.5.6", Name: "tooling", VersionConstraint: "^4.5.6", VERS: "vers:npm/>=4.5.6|<5.0.0", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "npm"}},
			},
		},
		{
			name:       "JSR and npm import targets",
			fixtureDir: "imports-without-comments",
			analysis:   SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "@example/cli@jsr:@example/cli@1.2.3-rc.1", Name: "@example/cli", VersionConstraint: "1.2.3-rc.1", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "jsr"}},
				{PackageType: "npm", Raw: "bundler@npm:bundler@2.3.4", Name: "bundler", VersionConstraint: "2.3.4", VERS: "vers:npm/2.3.4", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "npm"}},
				{PackageType: "npm", Raw: "validator@npm:validator@next", Name: "validator", VersionConstraint: "next", VERS: "vers:npm/next.0.0", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "npm"}},
			},
		},
		{
			name:       "no imports",
			fixtureDir: "no-imports",
			analysis:   SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "deno-jsonc", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "deno-jsonc" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestNormalizeJSONCPreservesCommentMarkersInStrings(t *testing.T) {
	content := []byte(`{"imports":{"url":"https://example.test/module.ts","text":"/* not a comment */",},}`)
	got, err := normalizeJSONC(content)
	if err != nil {
		t.Fatalf("normalizeJSONC: %v", err)
	}
	want := `{"imports":{"url":"https://example.test/module.ts","text":"/* not a comment */"}}`
	if string(got) != want {
		t.Fatalf("normalized JSONC = %q, want %q", got, want)
	}
}
