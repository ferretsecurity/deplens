package analyze

import (
	"path/filepath"
	"testing"
)

func TestChefBerksfileLockFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "legacy graph with Git source",
			fixture: "berksfile-lock-legacy-git",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "base@1.2.0", Name: "base", Version: "1.2.0", VersionConstraint: "~> 1.0", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "remote@2.0.0", Name: "remote", Version: "2.0.0", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/remote.git", "source_ref": "abc123", "source_ref_kind": "revision", "source_tag": "v2.0.0"}},
				{PackageType: "generic", Raw: "shared@3.0.0", Name: "shared", Version: "3.0.0", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime},
			},
		},
		{
			name:    "legacy local source",
			fixture: "berksfile-lock-path",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "local@0.1.0", Name: "local", Version: "0.1.0", SourceGroup: "default", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "spec/fixtures/local", "metadata": "true"}},
			},
		},
		{
			name:    "legacy JSON sources",
			fixture: "berksfile-lock-json",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "apt@2.3.0", Name: "apt", Version: "2.3.0", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "docker@0.20.0", Name: "docker", Version: "0.20.0", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "git://example.test/chef-docker.git", "source_ref": "deadbeef", "source_ref_kind": "ref"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "chef", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Berksfile.lock")
			if source.Detector != "chef-berksfile-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
