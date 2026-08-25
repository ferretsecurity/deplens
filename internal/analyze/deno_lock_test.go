package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestDenoLockFixturesExtractResolvedDependencies(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "remote modules",
			fixtureDir: "remote-modules",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "https://deno.land/std@0.181.0/testing/asserts.ts", SourceGroup: "remote", OriginKind: OriginURL, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://deno.land/std@0.181.0/testing/asserts.ts"}},
				{PackageType: "generic", Raw: "https://esm.sh/url-safe-base64@1.3.0", SourceGroup: "remote", OriginKind: OriginURL, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://esm.sh/url-safe-base64@1.3.0"}},
			},
		},
		{
			name:       "nested JSR packages",
			fixtureDir: "nested-jsr-packages",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "@std/assert@1.0.13", Name: "@std/assert", Version: "1.0.13", SourceGroup: "jsr", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "jsr"}},
				{PackageType: "generic", Raw: "@std/internal@1.0.7", Name: "@std/internal", Version: "1.0.7", SourceGroup: "jsr", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "jsr"}},
			},
		},
		{
			name:       "top-level JSR and npm packages",
			fixtureDir: "top-level-jsr-and-npm-packages",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "@deno/dnt@0.42.3", Name: "@deno/dnt", Version: "0.42.3", SourceGroup: "jsr", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "jsr"}},
				{PackageType: "npm", Raw: "@noble/post-quantum@0.5.4", Name: "@noble/post-quantum", Version: "0.5.4", SourceGroup: "npm", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "npm"}},
				{PackageType: "npm", Raw: "mlkem-wasm@0.0.7", Name: "mlkem-wasm", Version: "0.0.7", SourceGroup: "npm", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"registry": "npm"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "deno-lock", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "deno-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestDenoLockParserReturnsConclusiveEmptyForVersionOnlyFile(t *testing.T) {
	parser, err := newDenoLockParser(denoLockParserConfig{})
	if err != nil {
		t.Fatalf("newDenoLockParser: %v", err)
	}

	result, err := parser.Analyze("deno.lock", []byte(`{"version":"4"}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
