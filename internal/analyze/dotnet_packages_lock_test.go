package analyze

import (
	"path/filepath"
	"testing"
)

func TestDotnetPackagesLockSemanticFixtures(t *testing.T) {
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "v1 direct and transitive packages",
			fixtureDir: "packages-lock-v1-direct-transitive",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Example.Direct@1.2.3", Name: "Example.Direct", Version: "1.2.3", VersionConstraint: "[1.2.3, )", SourceGroup: "net8.0", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"content_hash": "direct-hash"}},
				{PackageType: "nuget", Raw: "Example.Transitive@2.0.0", Name: "Example.Transitive", Version: "2.0.0", SourceGroup: "net8.0", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime, Attributes: map[string]string{"content_hash": "transitive-hash"}},
			},
		},
		{
			name:       "v1 runtime identifier and project entries",
			fixtureDir: "packages-lock-v1-rid-project",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Example.Command@10.0.9", Name: "Example.Command", Version: "10.0.9", VersionConstraint: "[10.0.9, )", SourceGroup: "net10.0", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"content_hash": "command-hash"}},
				{PackageType: "nuget", Raw: "Example.Library@5.6.7", Name: "Example.Library", Version: "5.6.7", SourceGroup: "net10.0", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime, Attributes: map[string]string{"content_hash": "library-hash"}},
				{PackageType: "nuget", Raw: "Example.Command@10.0.9", Name: "Example.Command", Version: "10.0.9", VersionConstraint: "[10.0.9, )", SourceGroup: "net10.0/linux-arm64", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"content_hash": "command-hash"}},
				{PackageType: "nuget", Raw: "runtime.linux-arm64.Example.Command@10.0.9", Name: "runtime.linux-arm64.Example.Command", Version: "10.0.9", SourceGroup: "net10.0/linux-arm64", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime, Attributes: map[string]string{"content_hash": "runtime-hash"}},
			},
		},
		{
			name:       "v2 central transitive package",
			fixtureDir: "packages-lock-v2-central-transitive",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Example.Central@3.3.1", Name: "Example.Central", Version: "3.3.1", VersionConstraint: "[3.3.1, )", SourceGroup: ".NETStandard,Version=v2.1", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime, Attributes: map[string]string{"content_hash": "central-hash"}},
				{PackageType: "nuget", Raw: "Example.Direct@4.0.0", Name: "Example.Direct", Version: "4.0.0", VersionConstraint: "[4.0.0, )", SourceGroup: ".NETStandard,Version=v2.1", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"content_hash": "direct-hash"}},
				{PackageType: "nuget", Raw: "Example.Transitive@1.0.0-rc.3", Name: "Example.Transitive", Version: "1.0.0-rc.3", SourceGroup: ".NETStandard,Version=v2.1", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime, Attributes: map[string]string{"content_hash": "transitive-hash"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "dotnet", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "packages.lock.json")
			if source.Detector != "dotnet-packages-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %+v, want %+v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestDotnetPackagesLockEmptyDependenciesAreComplete(t *testing.T) {
	parser, err := newDotnetPackagesLockParser(dotnetPackagesLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newDotnetPackagesLockParser: %v", err)
	}
	result, err := parser.Analyze("packages.lock.json", []byte(`{"version": 1}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
