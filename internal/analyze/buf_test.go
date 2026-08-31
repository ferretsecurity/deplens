package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestBufManifestFixturesExtractDependencyReferences(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "v1 remote dependency",
			fixtureDir: "manifest-v1-deps",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "buf.build/acme/weather", Name: "buf.build/acme/weather", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:       "v2 local module",
			fixtureDir: "manifest-v2-module",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "proto", Name: "proto", SourceGroup: "modules", OriginKind: OriginWorkspace, Relationship: RelationshipInconclusive},
			},
		},
		{
			name:       "v2 remote dependency and local module",
			fixtureDir: "manifest-v2-deps-modules",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "buf.build/acme/weather", Name: "buf.build/acme/weather", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "internal/proto", Name: "internal/proto", SourceGroup: "modules", OriginKind: OriginWorkspace, Relationship: RelationshipInconclusive},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "buf", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "buf" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
