package analyze

import (
	"path/filepath"
	"testing"
)

func TestHaskellStackLockFixturesExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		analysis SourceAnalysis
		want     []DependencyReference
	}{
		{
			name:     "URL package",
			fixture:  "stack-lock-url-package",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{{
				PackageType:  "generic",
				Raw:          "wire-format@1.2.3",
				Name:         "wire-format",
				Version:      "1.2.3",
				SourceGroup:  "packages",
				OriginKind:   OriginURL,
				Relationship: RelationshipInconclusive,
				Scope:        ScopeRuntime,
				Attributes:   map[string]string{"source_url": "https://example.test/wire-format.tar.gz"},
			}},
		},
		{
			name:     "Hackage package",
			fixture:  "stack-lock-hackage-package",
			analysis: SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
			want: []DependencyReference{{
				PackageType:  "hackage",
				Raw:          "terminal-ui@0.2.16",
				Name:         "terminal-ui",
				Version:      "0.2.16",
				SourceGroup:  "packages",
				OriginKind:   OriginRegistry,
				Relationship: RelationshipInconclusive,
				Scope:        ScopeRuntime,
			}},
		},
		{
			name:     "no packages",
			fixture:  "stack-lock-no-packages",
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
			source := sourceForPath(t, result, "stack.yaml.lock")
			if source.Detector != "haskell-stack-lock" || source.Analysis != tc.analysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}
