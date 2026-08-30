package analyze

import (
	"path/filepath"
	"testing"
)

func TestIOSPodspecFixtures(t *testing.T) {
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "static metadata with source version reference",
			fixtureDir: "podspec-source-version",
			want:       []DependencyReference{},
		},
		{
			name:       "static direct dependencies",
			fixtureDir: "podspec-direct-dependencies",
			want: []DependencyReference{
				{PackageType: "cocoapods", Raw: "Engine", Name: "Engine", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "cocoapods", Raw: "UI-Core", Name: "UI-Core", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "cocoapods", Raw: "UI-Cxx", Name: "UI-Cxx", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:       "static metadata with multiple Swift versions",
			fixtureDir: "podspec-swift-versions",
			want:       []DependencyReference{},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "ios", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Fixture.podspec")
			wantAnalysis := SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}
			if len(tc.want) > 0 {
				wantAnalysis.Presence = PresencePresent
			}
			if source.Detector != "ios-podspec" || source.Analysis != wantAnalysis {
				t.Fatalf("source = %+v, want analysis %+v", source, wantAnalysis)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
