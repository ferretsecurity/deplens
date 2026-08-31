package analyze

import (
	"path/filepath"
	"testing"
)

func TestZigBuildZONFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "Git dependency",
			fixture:  "build-zon-git",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{{
				PackageType: "generic", Raw: "parser@git+https://example.test/parser#abc123", Name: "parser", SourceGroup: "dependencies",
				OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime,
				Attributes: map[string]string{"source_url": "https://example.test/parser", "source_ref": "abc123", "source_ref_kind": "revision"},
			}},
		},
		{
			name:     "empty dependencies",
			fixture:  "build-zon-empty",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "local path dependency",
			fixture:  "build-zon-path",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{{
				PackageType: "generic", Raw: "generator@tools/generator", Name: "generator", SourceGroup: "dependencies",
				OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime,
				Attributes: map[string]string{"path": "tools/generator"},
			}},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "zig", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "build.zig.zon")
			if source.Detector != "zig-build-zon" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
