package analyze

import (
	"path/filepath"
	"testing"
)

func TestChefPolicyfileFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "multiple local cookbooks",
			fixture: "policyfile-local-paths",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "core", Name: "core", SourceGroup: "default", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "."}},
				{PackageType: "generic", Raw: "support", Name: "support", SourceGroup: "default", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "test/cookbooks/support"}},
			},
		},
		{
			name:    "Git cookbooks with constraints",
			fixture: "policyfile-local-and-git",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "helpers@>= 1.2.0", Name: "helpers", VersionConstraint: ">= 1.2.0", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://example.test/helpers.git", "source_ref": "v1.2.3", "source_ref_kind": "tag"}},
				{PackageType: "generic", Raw: "local", Name: "local", SourceGroup: "default", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "."}},
			},
		},
		{
			name:    "single custom local cookbook",
			fixture: "policyfile-single-local-path",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "sample", Name: "sample", SourceGroup: "default", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "spec/cookbooks/sample"}},
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
			source := sourceForPath(t, result, "Policyfile.rb")
			if source.Detector != "chef-policyfile" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
