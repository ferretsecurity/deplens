package analyze

import (
	"path/filepath"
	"testing"
)

func TestSwiftPackageFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "named URL dependency",
			fixture:  "package-manifest-with-dependency",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{{
				PackageType:       "swift",
				Raw:               "SwiftTreeSitter@0.9.0",
				Name:              "SwiftTreeSitter",
				VersionConstraint: "0.9.0",
				SourceGroup:       "dependencies",
				OriginKind:        OriginGit,
				Relationship:      RelationshipDirect,
				Scope:             ScopeRuntime,
				Attributes:        map[string]string{"source_url": "https://github.com/tree-sitter/swift-tree-sitter"},
			}},
		},
		{
			name:     "explicit empty dependencies",
			fixture:  "package-manifest-empty-dependencies",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "binary target without dependencies",
			fixture:  "package-manifest-without-dependencies",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "swift", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Package.swift")
			if source.Detector != "swift-package" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
