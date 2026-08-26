package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestImportMapFixturesExtractDependencies(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "npm and HTTPS targets",
			fixtureDir: "importmap-npm-and-url",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "file-api@https://modules.example.test/node/fs.js", Name: "file-api", SourceGroup: "imports", OriginKind: OriginURL, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://modules.example.test/node/fs.js"}},
				{PackageType: "npm", Raw: "net-client@npm:request-client", Name: "request-client", SourceGroup: "imports", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"declared_name": "net-client", "registry": "npm"}},
			},
		},
		{
			name:       "CDN aliases and prefixes",
			fixtureDir: "importmap-cdn-aliases",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "compat@https://cdn.example.test/compat-kit@0.1.x/entry.js", Name: "compat", SourceGroup: "imports", OriginKind: OriginURL, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://cdn.example.test/compat-kit@0.1.x/entry.js"}},
				{PackageType: "generic", Raw: "compat/@https://cdn.example.test/compat-kit@0.1.x/", Name: "compat/", SourceGroup: "imports", OriginKind: OriginURL, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://cdn.example.test/compat-kit@0.1.x/"}},
			},
		},
		{
			name:       "site-local paths",
			fixtureDir: "importmap-local-paths",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "audio@/assets/audio.js", Name: "audio", SourceGroup: "imports", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"path": "/assets/audio.js"}},
				{PackageType: "generic", Raw: "map@/assets/map.js?v1", Name: "map", SourceGroup: "imports", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"path": "/assets/map.js?v1"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "js", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "js-importmap" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestImportMapParserReportsConclusiveEmpty(t *testing.T) {
	parser, err := newImportMapParser(importMapParserConfig{})
	if err != nil {
		t.Fatalf("newImportMapParser: %v", err)
	}

	result, err := parser.Analyze("importmap.json", []byte(`{"imports": {}}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("analysis = %+v", result.Analysis)
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("dependencies = %+v, want none", result.Dependencies)
	}
}
