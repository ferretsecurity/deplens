package analyze

import (
	"path/filepath"
	"testing"
)

func TestNixFlakeFixturesExtractDependencyReferences(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "dotted GitHub inputs",
			fixtureDir: "flake-dotted-inputs",
			want: []DependencyReference{
				{PackageType: "github", Raw: "github:NixOS/nixpkgs/nixos-unstable", Name: "NixOS/nixpkgs", VersionConstraint: "nixos-unstable", SourceGroup: "inputs", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/NixOS/nixpkgs", "source_ref": "nixos-unstable"}},
				{PackageType: "github", Raw: "github:numtide/flake-utils", Name: "numtide/flake-utils", SourceGroup: "inputs", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/numtide/flake-utils"}},
			},
		},
		{
			name:       "Git URL input",
			fixtureDir: "flake-git-url-input",
			want: []DependencyReference{
				{PackageType: "github", Raw: "git+https://github.com/NixOS/nixpkgs?shallow=1&ref=nixos-unstable-small", Name: "NixOS/nixpkgs", VersionConstraint: "nixos-unstable-small", SourceGroup: "inputs", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/NixOS/nixpkgs", "source_ref": "nixos-unstable-small"}},
				{PackageType: "github", Raw: "github:numtide/treefmt-nix", Name: "numtide/treefmt-nix", SourceGroup: "inputs", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/numtide/treefmt-nix"}},
			},
		},
		{
			name:       "nested input sets",
			fixtureDir: "flake-nested-inputs",
			want: []DependencyReference{
				{PackageType: "github", Raw: "github:NixOS/nixpkgs/nixos-unstable", Name: "NixOS/nixpkgs", VersionConstraint: "nixos-unstable", SourceGroup: "inputs", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/NixOS/nixpkgs", "source_ref": "nixos-unstable"}},
				{PackageType: "github", Raw: "github:ipetkov/crane", Name: "ipetkov/crane", SourceGroup: "inputs", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/ipetkov/crane"}},
				{PackageType: "github", Raw: "github:numtide/flake-utils", Name: "numtide/flake-utils", SourceGroup: "inputs", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/numtide/flake-utils"}},
				{PackageType: "github", Raw: "github:oxalica/rust-overlay", Name: "oxalica/rust-overlay", SourceGroup: "inputs", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/oxalica/rust-overlay"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "nix", test.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "flake.nix")
			if source.Detector != "nix-flake" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestNixFlakeWithoutExternalInputsReportsCompleteEmptyResult(t *testing.T) {
	parser := nixFlakeParser{}
	result, err := parser.Analyze("flake.nix", []byte(`{ inputs = {}; outputs = { self }: {}; }`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
