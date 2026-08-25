package analyze

import (
	"path/filepath"
	"testing"
)

func TestHomebrewBrewfileFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "commented formulas",
			fixture: "brewfile-commented-formulas",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "cmake", Name: "cmake", SourceGroup: "brew", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "single quoted formulas and casks",
			fixture: "brewfile-single-quoted",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "clang-format", Name: "clang-format", SourceGroup: "brew", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "temurin11", Name: "temurin11", SourceGroup: "cask", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
			},
		},
		{
			name:    "formulas casks and App Store applications",
			fixture: "brewfile-mas",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "bash", Name: "bash", SourceGroup: "brew", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "ghostty", Name: "ghostty", SourceGroup: "cask", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "Bitwarden", Name: "Bitwarden", SourceGroup: "mas", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"app_store_id": "1352778147"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "homebrew", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Brewfile")
			if source.Detector != "homebrew-brewfile" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestHomebrewBrewfileWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newHomebrewBrewfileParser(homebrewBrewfileMatcherConfig{})
	if err != nil {
		t.Fatalf("newHomebrewBrewfileParser: %v", err)
	}

	result, err := parser.Analyze("Brewfile", []byte("tap \"homebrew/cask-versions\"\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
