package analyze

import (
	"path/filepath"
	"testing"
)

func TestGitSubmodulesFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "multiple submodules",
			fixture: "submodules-multiple",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "vendor/codec@https://git.example.test/platform/codec.git", Name: "vendor/codec", SourceGroup: "submodules", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "vendor/codec", "source_url": "https://git.example.test/platform/codec.git"}},
				{PackageType: "generic", Raw: "vendor/widgets@https://github.com/example/widgets", Name: "vendor/widgets", SourceGroup: "submodules", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "vendor/widgets", "source_url": "https://github.com/example/widgets"}},
			},
		},
		{
			name:    "branch tracking submodule",
			fixture: "submodules-branch",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "tools/telemetry@ssh://git@example.test/platform/telemetry.git", Name: "tools/telemetry", SourceGroup: "submodules", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "tools/telemetry", "source_url": "ssh://git@example.test/platform/telemetry.git", "source_ref": "stable", "source_ref_kind": "branch"}},
			},
		},
		{
			name:    "single submodule",
			fixture: "submodules-single",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "Networking@https://github.com/example/Networking.git", Name: "Networking", SourceGroup: "submodules", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_path": "Networking", "source_url": "https://github.com/example/Networking.git"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "git", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, ".gitmodules")
			if source.Detector != "git-submodules" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestGitSubmodulesParserRecognizesSubmoduleWithoutURL(t *testing.T) {
	parser, err := newGitSubmodulesParser(gitSubmodulesMatcherConfig{})
	if err != nil {
		t.Fatalf("newGitSubmodulesParser: %v", err)
	}

	result, err := parser.Analyze(".gitmodules", []byte("[submodule \"local\"]\n\tpath = local\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
