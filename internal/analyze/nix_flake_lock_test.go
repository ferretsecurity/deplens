package analyze

import (
	"path/filepath"
	"testing"
)

func TestNixFlakeLockFixturesExtractResolvedGitHubInputs(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	tests := []struct {
		fixture string
		want    []DependencyReference
	}{
		{
			fixture: "flake-lock-root-and-transitive",
			want: []DependencyReference{
				nixFlakeLockTestDependency("acme/app", "1111111111111111111111111111111111111111", RelationshipDirect),
				nixFlakeLockTestDependency("acme/base", "2222222222222222222222222222222222222222", RelationshipDirect),
				nixFlakeLockTestDependency("acme/systems", "3333333333333333333333333333333333333333", RelationshipTransitive),
			},
		},
		{
			fixture: "flake-lock-input-follow",
			want: []DependencyReference{
				nixFlakeLockTestDependency("acme/base", "4444444444444444444444444444444444444444", RelationshipDirect),
				nixFlakeLockTestDependency("acme/library", "5555555555555555555555555555555555555555", RelationshipTransitive),
				nixFlakeLockTestDependency("acme/parts", "6666666666666666666666666666666666666666", RelationshipDirect),
			},
		},
		{
			fixture: "flake-lock-aliased-nodes",
			want: []DependencyReference{
				nixFlakeLockTestDependency("acme/base", "7777777777777777777777777777777777777777", RelationshipDirect),
				nixFlakeLockTestDependency("acme/base", "8888888888888888888888888888888888888888", RelationshipTransitive),
				nixFlakeLockTestDependency("acme/client", "9999999999999999999999999999999999999999", RelationshipDirect),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "nix", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "flake.lock")
			if source.Detector != "nix-flake-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestNixFlakeLockWithoutLockedInputsReportsCompleteEmptyResult(t *testing.T) {
	parser := nixFlakeLockParser{}
	result, err := parser.Analyze("flake.lock", []byte(`{"nodes":{"root":{"inputs":{}}},"root":"root","version":7}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func nixFlakeLockTestDependency(name, revision string, relationship Relationship) DependencyReference {
	return DependencyReference{
		PackageType:  "github",
		Raw:          name + "@" + revision,
		Name:         name,
		Version:      revision,
		SourceGroup:  "nodes",
		OriginKind:   OriginGit,
		Relationship: relationship,
		Scope:        ScopeRuntime,
		Attributes: map[string]string{
			"source_url":      "https://github.com/" + name,
			"source_ref":      revision,
			"source_ref_kind": "commit",
		},
	}
}
