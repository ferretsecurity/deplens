package analyze

import (
	"path/filepath"
	"testing"
)

func TestCrystalShardLockFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "v1 GitHub version and commit pins",
			fixture: "shard-lock-v1-github",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "jwt@f0bf2608", Name: "jwt", Version: "f0bf2608", SourceGroup: "shards", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "github:example/jwt", "source_ref": "f0bf2608", "source_ref_kind": "commit"}},
				{PackageType: "generic", Raw: "kilt@0.3.3", Name: "kilt", Version: "0.3.3", SourceGroup: "shards", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "github:example/kilt"}},
			},
		},
		{
			name:    "v1 path and GitHub pins",
			fixture: "shard-lock-v1-path",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "glew@0.1.0", Name: "glew", Version: "0.1.0", SourceGroup: "shards", OriginKind: OriginPath, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"path": "../glew"}},
				{PackageType: "generic", Raw: "lib_glew@50773a1c", Name: "lib_glew", Version: "50773a1c", SourceGroup: "shards", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "github:example/lib_glew", "source_ref": "50773a1c", "source_ref_kind": "commit"}},
			},
		},
		{
			name:    "v2 Git version pins",
			fixture: "shard-lock-v2-git",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "ameba@1.7.0-dev+git.commit.fe12a756", Name: "ameba", Version: "1.7.0-dev+git.commit.fe12a756", SourceGroup: "shards", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/example/ameba.git"}},
				{PackageType: "generic", Raw: "backtracer@1.2.4", Name: "backtracer", Version: "1.2.4", SourceGroup: "shards", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/example/backtracer.cr.git"}},
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
			source := sourceForPath(t, result, "shard.lock")
			if source.Detector != "crystal-shard-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestCrystalShardLockNoDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newCrystalShardLockParser(crystalShardLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newCrystalShardLockParser: %v", err)
	}
	result, err := parser.Analyze("shard.lock", []byte("version: 2.0\nshards: {}\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
