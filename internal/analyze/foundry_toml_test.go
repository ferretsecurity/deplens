package analyze

import (
	"path/filepath"
	"testing"
)

func TestFoundryTOMLFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "profile configuration without dependencies",
			fixture:  "profile-configuration",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "legacy remappings without dependencies",
			fixture:  "legacy-remappings",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "Soldeer registry dependencies",
			fixture:  "soldeer-dependencies",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "@openzeppelin-contracts@5.4.0", Name: "@openzeppelin-contracts", VersionConstraint: "5.4.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "flare-periphery@0.1.38", Name: "flare-periphery", VersionConstraint: "0.1.38", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "forge-std@1.10.0", Name: "forge-std", VersionConstraint: "1.10.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "foundry", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "foundry.toml")
			if source.Detector != "foundry-toml" || source.Analysis != test.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
