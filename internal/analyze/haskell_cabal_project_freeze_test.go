package analyze

import (
	"path/filepath"
	"testing"
)

func TestHaskellCabalProjectFreezeFixturesExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "index state only",
			fixture:  "cabal-project-freeze-index-state",
			analysis: SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
			want:     []DependencyReference{},
		},
		{
			name:     "any qualified constraints",
			fixture:  "cabal-project-freeze-any-constraints",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				haskellCabalProjectFreezeTestDependency("example-library", "1.2.3"),
				haskellCabalProjectFreezeTestDependency("example-parser", "4.5.6"),
			},
		},
		{
			name:     "plain constraints",
			fixture:  "cabal-project-freeze-constraints",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{
				haskellCabalProjectFreezeTestDependency("Example-Package", "7.8.9"),
				haskellCabalProjectFreezeTestDependency("example-tools", "0.1.2"),
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "haskell", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "cabal.project.freeze")
			if source.Detector != "haskell-cabal-project-freeze" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func haskellCabalProjectFreezeTestDependency(name, version string) DependencyReference {
	return DependencyReference{
		PackageType:  "hackage",
		Raw:          name + "@" + version,
		Name:         name,
		Version:      version,
		SourceGroup:  "constraints",
		OriginKind:   OriginRegistry,
		Relationship: RelationshipInconclusive,
		Scope:        ScopeRuntime,
	}
}
