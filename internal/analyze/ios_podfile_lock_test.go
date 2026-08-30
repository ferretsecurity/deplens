package analyze

import (
	"path/filepath"
	"testing"
)

func TestIOSPodfileLockFixturesExtractResolvedPods(t *testing.T) {
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "resolved pods with nested requirements",
			fixtureDir: "podfile-lock-resolved-pods",
			want: []DependencyReference{
				{PackageType: "cocoapods", Raw: "Alamofire@5.7.1", Name: "Alamofire", Version: "5.7.1", SourceGroup: "PODS", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime},
				{PackageType: "cocoapods", Raw: "Moya@15.0.0", Name: "Moya", Version: "15.0.0", SourceGroup: "PODS", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime},
				{PackageType: "cocoapods", Raw: "Moya/Core@15.0.0", Name: "Moya/Core", Version: "15.0.0", SourceGroup: "PODS", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime},
			},
		},
		{
			name:       "quoted subspec",
			fixtureDir: "podfile-lock-quoted-subspec",
			want: []DependencyReference{
				{PackageType: "cocoapods", Raw: "FirebaseCore@4.0.18", Name: "FirebaseCore", Version: "4.0.18", SourceGroup: "PODS", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime},
				{PackageType: "cocoapods", Raw: "GoogleToolboxForMac/NSData+zlib@2.1.3", Name: "GoogleToolboxForMac/NSData+zlib", Version: "2.1.3", SourceGroup: "PODS", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "ios", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Podfile.lock")
			if source.Detector != "ios-podfile-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestIOSPodfileLockMetadataOnlyIsCompleteAndEmpty(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	result, err := Scan(filepath.Join("..", "..", "testdata", "ios", "podfile-lock-metadata-only"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "Podfile.lock")
	if source.Detector != "ios-podfile-lock" || source.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(source.Dependencies) != 0 {
		t.Fatalf("source = %+v", source)
	}
}
