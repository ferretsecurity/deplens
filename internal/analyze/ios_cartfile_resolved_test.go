package analyze

import (
	"path/filepath"
	"testing"
)

func TestIOSCartfileResolvedFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "GitHub commit pins",
			fixtureDir: "cartfile-resolved-github-commit",
			want: []DependencyReference{
				{PackageType: "github", Raw: "sample/Tree@0123456789abcdef0123456789abcdef01234567", Name: "sample/Tree", Version: "0123456789abcdef0123456789abcdef01234567", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/sample/Tree.git", "source_ref": "0123456789abcdef0123456789abcdef01234567", "source_ref_kind": "commit"}},
			},
		},
		{
			name:       "GitHub release pins",
			fixtureDir: "cartfile-resolved-github-release",
			want: []DependencyReference{
				{PackageType: "github", Raw: "sample/Networking@4.2.0", Name: "sample/Networking", Version: "4.2.0", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/sample/Networking.git"}},
			},
		},
		{
			name:       "binary and GitHub release pins",
			fixtureDir: "cartfile-resolved-binary-and-github",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "https://downloads.example.test/Telemetry.json@10.21.0", Name: "https://downloads.example.test/Telemetry.json", Version: "10.21.0", SourceGroup: "default", OriginKind: OriginURL, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://downloads.example.test/Telemetry.json"}},
				{PackageType: "github", Raw: "sample/Categories@6.13.0", Name: "sample/Categories", Version: "6.13.0", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipInconclusive, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/sample/Categories.git"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "ios", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Cartfile.resolved")
			if source.Detector != "ios-cartfile-resolved" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestIOSCartfileResolvedWithoutReferencesIsComplete(t *testing.T) {
	parser, err := newIOSCartfileResolvedParser(iosCartfileResolvedMatcherConfig{})
	if err != nil {
		t.Fatalf("newIOSCartfileResolvedParser: %v", err)
	}
	result, err := parser.Analyze("Cartfile.resolved", []byte("# dependencies are managed elsewhere\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
