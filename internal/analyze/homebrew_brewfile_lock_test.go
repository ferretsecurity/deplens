package analyze

import (
	"path/filepath"
	"testing"
)

func TestHomebrewBrewfileLockFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "bottled formulas",
			fixture: "brewfile-lock-bottled-formulas",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "formatter@1.2.3", Name: "formatter", Version: "1.2.3", SourceGroup: "brew", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://registry.example.test/homebrew/core", "bottle_sha256_arm64_test": "formatter-arm64-checksum"}},
				{PackageType: "generic", Raw: "toolkit@0.17.5", Name: "toolkit", Version: "0.17.5", SourceGroup: "brew", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://registry.example.test/homebrew/core", "bottle_sha256_x86_64_test": "toolkit-x86-checksum"}},
			},
		},
		{
			name:    "formulas with unavailable bottles and a pinned tap",
			fixture: "brewfile-lock-brew-and-tap",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "assertion-tools@2.0.0", Name: "assertion-tools", Version: "2.0.0", SourceGroup: "brew", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "file-tools@0.3.0", Name: "file-tools", Version: "0.3.0", SourceGroup: "brew", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "test-runner@1.8.2", Name: "test-runner", Version: "1.8.2", SourceGroup: "brew", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://registry.example.test/homebrew/core", "bottle_sha256_all": "runner-checksum"}},
				{PackageType: "generic", Raw: "example/tools@a967f8f3542021c13afc0c61d9a9f548c5ed05e7", Name: "example/tools", Version: "a967f8f3542021c13afc0c61d9a9f548c5ed05e7", SourceGroup: "tap", OriginKind: OriginGit, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_ref": "a967f8f3542021c13afc0c61d9a9f548c5ed05e7", "source_ref_kind": "revision"}},
			},
		},
		{
			name:    "versioned formula name",
			fixture: "brewfile-lock-versioned-formula",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "runtime@20@20.17.0", Name: "runtime@20", Version: "20.17.0", SourceGroup: "brew", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"source_url": "https://registry.example.test/homebrew/core", "bottle_sha256_arm64_test": "runtime-checksum"}},
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
			source := sourceForPath(t, result, "Brewfile.lock.json")
			if source.Detector != "homebrew-brewfile-lock" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestHomebrewBrewfileLockWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newHomebrewBrewfileLockParser(homebrewBrewfileLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newHomebrewBrewfileLockParser: %v", err)
	}

	result, err := parser.Analyze("Brewfile.lock.json", []byte(`{"entries": {}}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
