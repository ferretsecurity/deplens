package analyze

import (
	"path/filepath"
	"testing"
)

func TestGemspecFixtures(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		analysis    SourceAnalysis
		want        []DependencyReference
		diagnostics int
	}{
		{
			name:        "legacy dynamic declarations",
			fixture:     "gemspec-yaml-driven/lang.gemspec",
			analysis:    SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported},
			want:        []DependencyReference{},
			diagnostics: 1,
		},
		{
			name:        "YAML-driven declarations",
			fixture:     "gemspec-yaml-loop/example.gemspec",
			analysis:    SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported},
			want:        []DependencyReference{},
			diagnostics: 1,
		},
		{
			name:     "static and dynamic declarations",
			fixture:  "gemspec-mixed/example.gemspec",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial},
			want: []DependencyReference{
				{PackageType: "gem", Raw: "shared-library", Name: "shared-library", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
			diagnostics: 1,
		},
		{
			name:     "file-derived versions",
			fixture:  "gemspec-file-derived-version/example.gemspec",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial},
			want: []DependencyReference{
				{PackageType: "gem", Raw: "framework-core", Name: "framework-core", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "gem", Raw: "request-router@>= 1.8.5", Name: "request-router", VersionConstraint: ">= 1.8.5", VERS: "vers:gem/>=1.8.5", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
			diagnostics: 1,
		},
		{
			name:     "environment-derived versions",
			fixture:  "gemspec-environment-derived-version/example.gemspec",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial},
			want: []DependencyReference{
				{PackageType: "gem", Raw: "http-client", Name: "http-client", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
			diagnostics: 1,
		},
		{
			name:        "metadata only",
			fixture:     "gemspec-metadata-only/flags2env.gemspec",
			analysis:    SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:        []DependencyReference{},
			diagnostics: 0,
		},
		{
			name:     "runtime and development dependencies",
			fixture:  "gemspec-runtime-and-development/solana-pay-kit.gemspec",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "gem", Raw: "rack@~> 3.1", Name: "rack", VersionConstraint: "~> 3.1", VERS: "vers:gem/>=3.1|<4", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "gem", Raw: "rake@~> 13.2", Name: "rake", VersionConstraint: "~> 13.2", VERS: "vers:gem/>=13.2|<14", SourceGroup: "development_dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
			},
			diagnostics: 0,
		},
	}

	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "ruby")
	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := sourceForPath(t, result, test.fixture)
			if source.Detector != "ruby-gemspec" || source.Analysis != test.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
			if len(source.Diagnostics) != test.diagnostics {
				t.Fatalf("diagnostics = %+v, want %d", source.Diagnostics, test.diagnostics)
			}
			if test.diagnostics > 0 && source.Diagnostics[0].Code != incompleteExtractionCode {
				t.Fatalf("incomplete extraction diagnostics = %+v", source.Diagnostics)
			}
		})
	}
}
