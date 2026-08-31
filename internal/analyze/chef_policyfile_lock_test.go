package analyze

import (
	"path/filepath"
	"testing"
)

func TestChefPolicyfileLockFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "local checkout and registry cookbook",
			fixture: "policyfile-lock-local-checkout-and-registry",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "build-essential@6.0.6", Name: "build-essential", Version: "6.0.6", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipTransitive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://supermarket.example.test/build-essential/6.0.6/download"}},
				{PackageType: "generic", Raw: "vault@2.4.0", Name: "vault", Version: "2.4.0", SourceGroup: "default", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": ".", "source_ref": "a6409c", "source_ref_kind": "revision"}},
			},
		},
		{
			name:    "Supermarket cookbooks",
			fixture: "policyfile-lock-supermarket",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "sudo@3.5.0", Name: "sudo", Version: "3.5.0", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://supermarket.example.test/sudo/3.5.0/download"}},
				{PackageType: "generic", Raw: "users@4.0.0", Name: "users", Version: "4.0.0", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://supermarket.example.test/users/4.0.0/download"}},
			},
		},
		{
			name:    "local cookbook paths",
			fixture: "policyfile-lock-local-paths",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "base@0.1.0", Name: "base", Version: "0.1.0", SourceGroup: "default", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "../cookbooks/base"}},
				{PackageType: "generic", Raw: "docker_host@0.1.0", Name: "docker_host", Version: "0.1.0", SourceGroup: "default", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "../cookbooks/docker_host"}},
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
			source := sourceForPath(t, result, "Policyfile.lock.json")
			if source.Detector != "chef-policyfile-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
