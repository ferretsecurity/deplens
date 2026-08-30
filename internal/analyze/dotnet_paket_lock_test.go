package analyze

import (
	"path/filepath"
	"testing"
)

func TestDotnetPaketLockSemanticFixtures(t *testing.T) {
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "legacy NuGet groups and GitHub module",
			fixtureDir: "paket-lock-legacy-github",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Acme.Build@4.5.6", Name: "Acme.Build", Version: "4.5.6", SourceGroup: "Build", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeBuild, Attributes: map[string]string{"source_url": "https://packages.example.test/v2"}},
				{PackageType: "generic", Raw: "acme/build-tools/modules/Tool.fsx@a1b2c3d4", Name: "acme/build-tools/modules/Tool.fsx", Version: "a1b2c3d4", SourceGroup: "Build", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeBuild, Attributes: map[string]string{"source_url": "https://github.com/acme/build-tools.git", "source_path": "modules/Tool.fsx", "source_ref": "a1b2c3d4", "source_ref_kind": "commit"}},
				{PackageType: "nuget", Raw: "Acme.Test@7.8.9", Name: "Acme.Test", Version: "7.8.9", SourceGroup: "Test", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeTest, Attributes: map[string]string{"source_url": "https://packages.example.test/v2"}},
				{PackageType: "nuget", Raw: "Acme.Core@1.2.3", Name: "Acme.Core", Version: "1.2.3", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/v2"}},
			},
		},
		{
			name:       "modern multiple groups",
			fixtureDir: "paket-lock-multi-group",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Acme.Runtime@2.0.0", Name: "Acme.Runtime", Version: "2.0.0", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/v3/index.json"}},
				{PackageType: "nuget", Raw: "Acme.Automation@3.1.4", Name: "Acme.Automation", Version: "3.1.4", SourceGroup: "fake", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/v3/index.json"}},
				{PackageType: "nuget", Raw: "Acme.Tests@5.0.0", Name: "Acme.Tests", Version: "5.0.0", SourceGroup: "test", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeTest, Attributes: map[string]string{"source_url": "https://packages.example.test/v3/index.json"}},
			},
		},
		{
			name:       "single default group",
			fixtureDir: "paket-lock-simple",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Acme.Template@6.2.0", Name: "Acme.Template", Version: "6.2.0", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/v3/index.json"}},
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
			source := sourceForPath(t, result, "paket.lock")
			if source.Detector != "dotnet-paket-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestDotnetPaketLockWithoutReferencesIsComplete(t *testing.T) {
	parser, err := newDotnetPaketLockParser(dotnetPaketLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newDotnetPaketLockParser: %v", err)
	}
	result, err := parser.Analyze("paket.lock", []byte("STORAGE: NONE\nNUGET\n  remote: https://packages.example.test/v3/index.json\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
