package analyze

import (
	"path/filepath"
	"testing"
)

func TestJSONNetBundlerFixturesExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "unnamed Git dependencies",
			fixture:  "bundler-unnamed-git-pair",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "alpha@https://example.test/alpha.git", Name: "alpha", VersionConstraint: "main", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/alpha.git", "source_path": "generated", "source_ref": "main", "source_ref_kind": "version"}},
				{PackageType: "generic", Raw: "beta@https://example.test/beta.git", Name: "beta", VersionConstraint: "stable", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/beta.git", "source_path": "library", "source_ref": "stable", "source_ref_kind": "version"}},
			},
		},
		{
			name:     "named Git dependency",
			fixture:  "bundler-named-git",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "utility@https://example.test/libraries.git", Name: "utility", VersionConstraint: "main", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/libraries.git", "source_path": "utility", "source_ref": "main", "source_ref_kind": "version"}},
			},
		},
		{
			name:     "unnamed local and Git dependencies",
			fixture:  "bundler-local-and-git",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "local-library@../local-library", Name: "local-library", SourceGroup: "dependencies", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"path": "../local-library"}},
				{PackageType: "generic", Raw: "metrics@https://example.test/metrics.git", Name: "metrics", VersionConstraint: "main", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/metrics.git", "source_path": "generated", "source_ref": "main", "source_ref_kind": "version"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "jsonnet", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "jsonnetfile.json")
			if source.Detector != "jsonnet-bundler" || source.Analysis != test.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
