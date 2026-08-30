package analyze

import (
	"path/filepath"
	"testing"
)

func TestChefMetadataFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "static cookbook list with constraint",
			fixture: "metadata-static-list",
			want: []DependencyReference{
				{Name: "apache2", Raw: "apache2", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "apt", Raw: "apt", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "database", Raw: "database", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "keystone", Raw: "keystone@>= 1.0.20", PackageType: "generic", VersionConstraint: ">= 1.0.20", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "mysql", Raw: "mysql", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "osops-utils", Raw: "osops-utils", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "longer static cookbook list",
			fixture: "metadata-static-list-monitoring",
			want: []DependencyReference{
				{Name: "apache2", Raw: "apache2", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "apt", Raw: "apt", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "database", Raw: "database", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "keepalived", Raw: "keepalived", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "keystone", Raw: "keystone@>= 1.0.20", PackageType: "generic", VersionConstraint: ">= 1.0.20", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "monitoring", Raw: "monitoring", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "mysql", Raw: "mysql", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{Name: "osops-utils", Raw: "osops-utils", PackageType: "generic", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "quoted dependency constraint",
			fixture: "metadata-quoted-constraint",
			want: []DependencyReference{
				{Name: "apt", Raw: "apt@~> 2.3.8", PackageType: "generic", VersionConstraint: "~> 2.3.8", SourceGroup: "default", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
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
			source := sourceForPath(t, result, "metadata.rb")
			if source.Detector != "chef-metadata" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
