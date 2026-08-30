package analyze

import (
	"path/filepath"
	"testing"
)

func TestEmacsCaskFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "package declaration with runtime and development dependencies",
			fixture: "cask-package",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "runtime-board", Name: "runtime-board", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "board-test", Name: "board-test", SourceGroup: "development", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{PackageType: "generic", Raw: "test-support", Name: "test-support", SourceGroup: "development", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
			},
		},
		{
			name:    "package file with multiple development dependencies",
			fixture: "cask-files",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "client-core", Name: "client-core", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "completion-ui", Name: "completion-ui", SourceGroup: "development", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
				{PackageType: "generic", Raw: "syntax-checker", Name: "syntax-checker", SourceGroup: "development", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
			},
		},
		{
			name:    "versioned runtime dependencies",
			fixture: "cask-versioned",
			want: []DependencyReference{
				{PackageType: "generic", Raw: "list-helper@1.9.0", Name: "list-helper", VersionConstraint: "1.9.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "string-helper@2.4.0", Name: "string-helper", VersionConstraint: "2.4.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "generic", Raw: "test-runner", Name: "test-runner", SourceGroup: "development", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "emacs", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Cask")
			if source.Detector != "emacs-cask" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestEmacsCaskWithoutDependenciesIsAbsent(t *testing.T) {
	parser, err := newEmacsCaskParser(emacsCaskMatcherConfig{})
	if err != nil {
		t.Fatalf("newEmacsCaskParser: %v", err)
	}
	result, err := parser.Analyze("Cask", []byte(`(package "empty-board" "0.1.0" "No dependencies.")`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
