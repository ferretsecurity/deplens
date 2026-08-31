package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestBufLockFixturesExtractDependencyReferences(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
		analysis   SourceAnalysis
	}{
		{
			name:       "v1 dependencies",
			fixtureDir: "lock-v1-deps",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "buf.build/acme/weather@a1b2c3", Name: "buf.build/acme/weather", Version: "a1b2c3", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:       "v2 dependencies",
			fixtureDir: "lock-v2-deps",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "buf.build/acme/validate@d4e5f6", Name: "buf.build/acme/validate", Version: "d4e5f6", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"digest": "b5:012345"}},
			},
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:       "v1 empty dependencies",
			fixtureDir: "lock-v1-empty",
			want:       []DependencyReference{},
			analysis:   SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
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
			if source.Detector != "buf-lock" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
