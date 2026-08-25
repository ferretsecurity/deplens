package analyze

import (
	"path/filepath"
	"testing"
)

func TestCrystalShardFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "GitHub and path dependencies",
			fixture: "shard-github-and-path",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "assets@../assets", Name: "assets", SourceGroup: "dependencies", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"path": "../assets"}},
				{PackageType: "generic", Raw: "web@github:example/web", Name: "web", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "github:example/web"}},
			},
		},
		{
			name:    "runtime and development dependencies",
			fixture: "shard-runtime-and-development",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "database@1.2.3", Name: "database", VersionConstraint: "1.2.3", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "github:example/database"}},
				{PackageType: "generic", Raw: "environment@2.0.0", Name: "environment", VersionConstraint: "2.0.0", SourceGroup: "development_dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeDevelopment, Attributes: map[string]string{"source_url": "github:example/environment"}},
			},
		},
		{
			name:    "version and commit dependency sources",
			fixture: "shard-github-commit",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "markup@github:example/markup", Name: "markup", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "github:example/markup", "source_ref": "0123456789abcdef", "source_ref_kind": "commit"}},
				{PackageType: "generic", Raw: "repl@0.4.0", Name: "repl", VersionConstraint: "0.4.0", SourceGroup: "dependencies", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "github:example/repl"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "crystal", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "shard.yml")
			if source.Detector != "crystal-shard" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestCrystalShardNoDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newCrystalShardParser(crystalShardMatcherConfig{})
	if err != nil {
		t.Fatalf("newCrystalShardParser: %v", err)
	}
	result, err := parser.Analyze("shard.yml", []byte("name: demo\nversion: 0.1.0\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
