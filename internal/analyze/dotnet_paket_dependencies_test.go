package analyze

import (
	"path/filepath"
	"testing"
)

func TestDotnetPaketDependenciesSemanticFixtures(t *testing.T) {
	tests := []struct {
		name            string
		fixtureDir      string
		want            []DependencyReference
		wantAnalysis    SourceAnalysis
		wantDiagnostics int
	}{
		{
			name:       "registry dependencies and groups",
			fixtureDir: "paket-dependencies-registry-groups",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Acme.Build.Tool", Name: "Acme.Build.Tool", SourceGroup: "Build", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild, Attributes: map[string]string{"source_url": "https://packages.example.test/v3/index.json"}},
				{PackageType: "generic", Raw: "example/assets@https://github.com/example/assets.git/icons/build.svg", Name: "example/assets", SourceGroup: "Build", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeBuild, Attributes: map[string]string{"source_url": "https://github.com/example/assets.git", "source_path": "icons/build.svg"}},
				{PackageType: "nuget", Raw: "Acme.Formatter@= 2.0.0", Name: "Acme.Formatter", VersionConstraint: "= 2.0.0", SourceGroup: "Formatting", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{PackageType: "nuget", Raw: "Acme.Runtime@>= 1.2.0", Name: "Acme.Runtime", VersionConstraint: ">= 1.2.0", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/v3/index.json"}},
			},
			wantAnalysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:       "CLI, Git, and GitHub dependencies",
			fixtureDir: "paket-dependencies-mixed-sources",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Acme.Build@~> 4", Name: "Acme.Build", VersionConstraint: "~> 4", SourceGroup: "Build", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild},
				{PackageType: "nuget", Raw: "Acme.Cli", Name: "Acme.Cli", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild, Attributes: map[string]string{"source_url": "https://packages.example.test/v2"}},
				{PackageType: "nuget", Raw: "Acme.Library", Name: "Acme.Library", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/v2"}},
				{PackageType: "generic", Raw: "example/editor-support@https://github.com/example/editor-support.git/src/Editor.fs", Name: "example/editor-support", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/example/editor-support.git", "source_path": "src/Editor.fs"}},
				{PackageType: "generic", Raw: "plugin@https://git.example.test/acme/plugin.git", Name: "plugin", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://git.example.test/acme/plugin.git", "source_ref": "main", "source_ref_kind": "ref"}},
			},
			wantAnalysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:       "NuGet dependencies with build group",
			fixtureDir: "paket-dependencies-nuget-build",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Acme.Build.Target", Name: "Acme.Build.Target", SourceGroup: "Build", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeBuild, Attributes: map[string]string{"source_url": "https://packages.example.test/v3/index.json"}},
				{PackageType: "nuget", Raw: "Acme.Data", Name: "Acme.Data", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/v3/index.json"}},
				{PackageType: "nuget", Raw: "Acme.Sql@= 8.0.31", Name: "Acme.Sql", VersionConstraint: "= 8.0.31", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://packages.example.test/v3/index.json"}},
			},
			wantAnalysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		},
		{
			name:            "unsupported HTTP dependency",
			fixtureDir:      "paket-dependencies-unsupported-remote",
			want:            []DependencyReference{},
			wantAnalysis:    SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported},
			wantDiagnostics: 1,
		},
		{
			name:       "partial extraction with an unsupported Gist dependency",
			fixtureDir: "paket-dependencies-partial-remote",
			want: []DependencyReference{
				{PackageType: "nuget", Raw: "Contoso.Runtime", Name: "Contoso.Runtime", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
			wantAnalysis:    SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial},
			wantDiagnostics: 1,
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "dotnet", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "paket.dependencies")
			if source.Detector != "dotnet-paket-dependencies" || source.Analysis != tc.wantAnalysis {
				t.Fatalf("source = %+v", source)
			}
			if len(source.Diagnostics) != tc.wantDiagnostics {
				t.Fatalf("diagnostics = %+v, want %d diagnostics", source.Diagnostics, tc.wantDiagnostics)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %+v, want %+v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestDotnetPaketDependenciesWithoutReferencesIsComplete(t *testing.T) {
	parser, err := newDotnetPaketDependenciesParser(dotnetPaketDependenciesMatcherConfig{})
	if err != nil {
		t.Fatalf("newDotnetPaketDependenciesParser: %v", err)
	}
	result, err := parser.Analyze("paket.dependencies", []byte("source https://packages.example.test/v3/index.json\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
