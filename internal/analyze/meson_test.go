package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestMesonFixturesExtractDependencyReferences(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "compiler library",
			fixtureDir: "meson-find-library",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "m", Name: "m", SourceGroup: "find_library", Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:       "python installation and compiler library",
			fixtureDir: "meson-python-installation",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "python", Name: "python", SourceGroup: "dependency", Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "m", Name: "m", SourceGroup: "find_library", Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:       "dependency-free Python package",
			fixtureDir: "meson-empty",
			want:       []DependencyReference{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "cpp", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "cpp-meson" {
				t.Fatalf("detector = %q, want cpp-meson", source.Detector)
			}
			wantAnalysis := SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}
			if len(tc.want) > 0 {
				wantAnalysis.Presence = PresencePresent
			}
			if source.Analysis != wantAnalysis {
				t.Fatalf("analysis = %+v, want %+v", source.Analysis, wantAnalysis)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
