package analyze

import (
	"path/filepath"
	"testing"
)

func TestChefBerksfileFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "Supermarket cookbooks",
			fixture: "berksfile-registry",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "apache2", Name: "apache2", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://supermarket.chef.io"}},
				{PackageType: "generic", Raw: "apt", Name: "apt", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://supermarket.chef.io"}},
			},
		},
		{
			name:    "test group with local cookbook",
			fixture: "berksfile-test-path",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "influxdb-test", Name: "influxdb-test", SourceGroup: "test", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeTest, Attributes: map[string]string{"source_path": "test/fixtures/cookbooks/influxdb-test"}},
				{PackageType: "generic", Raw: "netstat", Name: "netstat", SourceGroup: "test", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest, Attributes: map[string]string{"source_url": "https://supermarket.chef.io"}},
			},
		},
		{
			name:    "GitHub cookbook",
			fixture: "berksfile-github",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "confluence", Name: "confluence", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/esciara/chef-confluence.git"}},
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
			source := sourceForPath(t, result, "Berksfile")
			if source.Detector != "chef-berksfile" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
