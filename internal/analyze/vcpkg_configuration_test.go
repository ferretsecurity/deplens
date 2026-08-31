package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestVcpkgConfigurationFixturesExtractRegistryPackages(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "artifact registry without package assignments",
			fixtureDir: "vcpkg-config-artifact-registry",
			want:       []DependencyReference{},
		},
		{
			name:       "package registry assignments",
			fixtureDir: "vcpkg-config-package-registry",
			want: []DependencyReference{
				{PackageType: "vcpkg", Raw: "example-codec", Name: "example-codec", SourceGroup: "registries.0.packages", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "vcpkg", Raw: "example-ui", Name: "example-ui", SourceGroup: "registries.0.packages", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:       "overlay configuration without package assignments",
			fixtureDir: "vcpkg-config-overlays",
			want:       []DependencyReference{},
		},
	}

	ruleset := mustLoadDefaultRules(t)
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
			if source.Detector != "cpp-vcpkg-config" {
				t.Fatalf("detector = %q, want cpp-vcpkg-config", source.Detector)
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
