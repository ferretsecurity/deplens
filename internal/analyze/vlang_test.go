package analyze

import (
	"path/filepath"
	"testing"
)

func TestVLangFixtures(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "runtime dependency",
			fixture:  "runtime-dependency",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{{
				PackageType:       "generic",
				Raw:               "vsl@v0.2.0-beta.1",
				Name:              "vsl",
				VersionConstraint: "v0.2.0-beta.1",
				SourceGroup:       "dependencies",
				OriginKind:        OriginRegistry,
				Relationship:      RelationshipDirect,
				Scope:             ScopeRuntime,
			}},
		},
		{
			name:     "empty dependencies with target configuration",
			fixture:  "empty-target-configuration",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "empty dependencies with project metadata",
			fixture:  "empty-project-metadata",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "vlang", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "v.mod")
			if source.Detector != "vlang" || source.Analysis != test.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}
