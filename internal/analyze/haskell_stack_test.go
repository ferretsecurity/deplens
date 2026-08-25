package analyze

import (
	"path/filepath"
	"testing"
)

func TestHaskellStackFixturesExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "Hackage extra dependencies",
			fixture:  "stack-extra-deps",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				haskellStackHackageTestDependency("api-toolkit", "1.4.2"),
				haskellStackHackageTestDependency("json-golden", "0.8.0.1"),
			},
		},
		{
			name:     "Git extra dependency",
			fixture:  "stack-git-extra-dep",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{{
				PackageType:  "generic",
				Raw:          "python-bridge@https://github.com/example/python-bridge.git",
				Name:         "python-bridge",
				SourceGroup:  "extra-deps",
				OriginKind:   OriginGit,
				Relationship: RelationshipDirect,
				Scope:        ScopeRuntime,
				Attributes: map[string]string{
					"source_url":      "https://github.com/example/python-bridge.git",
					"source_ref":      "0123456789abcdef",
					"source_ref_kind": "commit",
				},
			}},
		},
		{
			name:     "resolver only",
			fixture:  "stack-no-extra-deps",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "haskell", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "stack.yaml")
			if source.Detector != "haskell-stack" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func haskellStackHackageTestDependency(name, version string) DependencyReference {
	return DependencyReference{
		PackageType:       "hackage",
		Raw:               name + "@" + version,
		Name:              name,
		VersionConstraint: version,
		SourceGroup:       "extra-deps",
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             ScopeRuntime,
	}
}
