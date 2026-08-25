package analyze

import (
	"path/filepath"
	"testing"
)

func TestDartPubspecFixturesExtractDependencyReferences(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "Flutter SDK and registry dependencies",
			fixture: "pubspec-flutter-sdk-and-registry",
			want: []DependencyReference{
				{PackageType: "pub", Raw: "flutter@sdk:flutter", Name: "flutter", SourceGroup: "dependencies", Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"sdk": "flutter"}},
				{PackageType: "pub", Raw: "route_helper@^1.2.3", Name: "route_helper", VersionConstraint: "^1.2.3", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "pub", Raw: "flutter_test@sdk:flutter", Name: "flutter_test", SourceGroup: "dev_dependencies", Relationship: RelationshipDirect, Scope: ScopeDevelopment, Attributes: map[string]string{"sdk": "flutter"}},
				{PackageType: "pub", Raw: "lint_rules@^2.0.0", Name: "lint_rules", VersionConstraint: "^2.0.0", SourceGroup: "dev_dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
			},
		},
		{
			name:    "registry dependency groups",
			fixture: "pubspec-registry-groups",
			want: []DependencyReference{
				{PackageType: "pub", Raw: "runtime_alpha@^1.0.0", Name: "runtime_alpha", VersionConstraint: "^1.0.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "pub", Raw: "runtime_beta@^2.0.0", Name: "runtime_beta", VersionConstraint: "^2.0.0", SourceGroup: "dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime},
				{PackageType: "pub", Raw: "test_helper@^3.0.0", Name: "test_helper", VersionConstraint: "^3.0.0", SourceGroup: "dev_dependencies", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeDevelopment},
			},
		},
		{
			name:    "Flutter SDK and local path dependencies",
			fixture: "pubspec-flutter-sdk-and-path",
			want: []DependencyReference{
				{PackageType: "pub", Raw: "flutter@sdk:flutter", Name: "flutter", SourceGroup: "dependencies", Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"sdk": "flutter"}},
				{PackageType: "pub", Raw: "local_plugin@../plugin", Name: "local_plugin", SourceGroup: "dependencies", OriginKind: OriginPath, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"path": "../plugin"}},
				{PackageType: "pub", Raw: "flutter_test@sdk:flutter", Name: "flutter_test", SourceGroup: "dev_dependencies", Relationship: RelationshipDirect, Scope: ScopeDevelopment, Attributes: map[string]string{"sdk": "flutter"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "dart", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "pubspec.yaml")
			if source.Detector != "dart-pubspec" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestDartPubspecWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newDartPubspecParser(dartPubspecMatcherConfig{})
	if err != nil {
		t.Fatalf("newDartPubspecParser: %v", err)
	}
	result, err := parser.Analyze("pubspec.yaml", []byte("name: app\nenvironment:\n  sdk: ^3.4.0\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}
