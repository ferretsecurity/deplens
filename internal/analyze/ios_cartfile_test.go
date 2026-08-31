package analyze

import (
	"path/filepath"
	"testing"
)

func TestIOSCartfileFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "unpinned GitHub repositories",
			fixtureDir: "cartfile-github-unpinned",
			want: []DependencyReference{
				{PackageType: "github", Raw: "example/WidgetKit", Name: "example/WidgetKit", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/example/WidgetKit.git"}},
				{PackageType: "github", Raw: "sample/Navigation", Name: "sample/Navigation", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/sample/Navigation.git"}},
			},
		},
		{
			name:       "constrained GitHub repository",
			fixtureDir: "cartfile-github-constraint",
			want: []DependencyReference{
				{PackageType: "github", Raw: "example/Networking@~> 3.2", Name: "example/Networking", VersionConstraint: "~> 3.2", SourceGroup: "default", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://github.com/example/Networking.git"}},
			},
		},
		{
			name:       "versioned binary URL",
			fixtureDir: "cartfile-binary-constraint",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "https://downloads.example.test/WidgetKit.json@== 1.4.0", Name: "https://downloads.example.test/WidgetKit.json", VersionConstraint: "== 1.4.0", SourceGroup: "default", OriginKind: OriginURL, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://downloads.example.test/WidgetKit.json"}},
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
			source := sourceForPath(t, result, "Cartfile")
			if source.Detector != "ios-cartfile" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestIOSCartfileWithoutReferencesIsComplete(t *testing.T) {
	parser, err := newIOSCartfileParser(iosCartfileMatcherConfig{})
	if err != nil {
		t.Fatalf("newIOSCartfileParser: %v", err)
	}
	result, err := parser.Analyze("Cartfile", []byte("# dependencies are managed elsewhere\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
