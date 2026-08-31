package analyze

import (
	"path/filepath"
	"testing"
)

func TestElixirMixFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "registry dependencies",
			fixture: "mix-registry-dependencies",
			want: []DependencyReference{
				{PackageType: "hex", Raw: "benchee@~> 1.2", Name: "benchee", VersionConstraint: "~> 1.2", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{PackageType: "hex", Raw: "credo@~> 1.7", Name: "credo", VersionConstraint: "~> 1.7", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeTest},
				{PackageType: "hex", Raw: "dialyxir@~> 1.4", Name: "dialyxir", VersionConstraint: "~> 1.4", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{PackageType: "hex", Raw: "ex_doc@~> 0.34", Name: "ex_doc", VersionConstraint: "~> 0.34", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{PackageType: "hex", Raw: "hammer@~> 7.0", Name: "hammer", VersionConstraint: "~> 7.0", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "hex", Raw: "redix@~> 1.5", Name: "redix", VersionConstraint: "~> 1.5", SourceGroup: "deps", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "empty function call",
			fixture: "mix-empty-function-call",
			want:    []DependencyReference{},
		},
		{
			name:    "empty atom reference",
			fixture: "mix-empty-atom-reference",
			want:    []DependencyReference{},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "elixir", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "mix.exs")
			wantAnalysis := SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}
			if len(test.want) == 0 {
				wantAnalysis.Presence = PresenceAbsent
			}
			if source.Detector != "elixir-mix" || source.Analysis != wantAnalysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestElixirMixRequiresMixProject(t *testing.T) {
	parser, err := newElixirMixParser(elixirMixMatcherConfig{})
	if err != nil {
		t.Fatalf("newElixirMixParser: %v", err)
	}
	result, err := parser.Analyze("mix.exs", []byte("defp deps do\n  [{:not_a_project, \"1.0\"}]\nend\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Recognized {
		t.Fatalf("result = %+v", result)
	}
}
