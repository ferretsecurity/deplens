package analyze

import (
	"path/filepath"
	"testing"
)

func TestNixDefaultShellFixturesExtractDependencyReferences(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	tests := []struct {
		name       string
		fixtureDir string
		analysis   SourceAnalysis
		want       []DependencyReference
	}{
		{
			name:       "fetchTarball URL",
			fixtureDir: "default-shell-fetch-tarball",
			analysis:   SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "github", Raw: "https://github.com/example/compat/archive/${rev}.tar.gz", Name: "example/compat", SourceGroup: "fetchTarball", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/example/compat"}},
			},
		},
		{
			name:       "Nix search path",
			fixtureDir: "default-shell-nixpkgs",
			analysis:   SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				{PackageType: "generic", Raw: "<nixpkgs>", Name: "nixpkgs", SourceGroup: "nix-path", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:       "local overlay imports",
			fixtureDir: "default-shell-overlay",
			analysis:   SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:       []DependencyReference{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "nix", test.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "default.nix")
			if source.Detector != "nix-default-shell" || source.Analysis != test.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
