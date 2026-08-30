package analyze

import (
	"path/filepath"
	"testing"
)

func TestIOSPodfileFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "version constraint",
			fixtureDir: "podfile-version-constraint",
			want: []DependencyReference{
				{PackageType: "cocoapods", Raw: "DVR@~> 1.0", Name: "DVR", VersionConstraint: "~> 1.0", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:       "source and unpinned subspec",
			fixtureDir: "podfile-shared-subspec",
			want: []DependencyReference{
				{PackageType: "cocoapods", Raw: "AGAsyncTestHelper/Shorthand", Name: "AGAsyncTestHelper/Shorthand", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/CocoaPods/Specs.git"}},
				{PackageType: "cocoapods", Raw: "HappyDNS@~> 1.0.3", Name: "HappyDNS", VersionConstraint: "~> 1.0.3", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/CocoaPods/Specs.git"}},
			},
		},
		{
			name:       "conditional local path",
			fixtureDir: "podfile-conditional-path",
			want: []DependencyReference{
				{PackageType: "cocoapods", Raw: "CCListView", Name: "CCListView", SourceGroup: "default", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "./"}},
				{PackageType: "cocoapods", Raw: "FLKAutoLayout", Name: "FLKAutoLayout", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
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
			source := sourceForPath(t, result, "Podfile")
			if source.Detector != "ios-podfile" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestIOSPodfileWithoutReferencesIsComplete(t *testing.T) {
	parser, err := newIOSPodfileParser(iosPodfileMatcherConfig{})
	if err != nil {
		t.Fatalf("newIOSPodfileParser: %v", err)
	}
	result, err := parser.Analyze("Podfile", []byte("platform :ios, '15.0'\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
