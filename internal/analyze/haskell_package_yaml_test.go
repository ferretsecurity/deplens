package analyze

import (
	"path/filepath"
	"testing"
)

func TestHaskellPackageYAMLFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "top-level and test dependencies",
			fixture: "package-yaml-top-level-and-tests",
			want: []DependencyReference{
				haskellPackageYAMLTestDependency("base", "", "dependencies", ScopeRuntime),
				haskellPackageYAMLTestDependency("custom-set", "", "tests.spec.dependencies", ScopeTest),
				haskellPackageYAMLTestDependency("hspec", "", "tests.spec.dependencies", ScopeTest),
			},
		},
		{
			name:    "top-level executable and test dependencies",
			fixture: "package-yaml-global-and-components",
			want: []DependencyReference{
				haskellPackageYAMLTestDependency("transformers", "", "dependencies", ScopeRuntime),
				haskellPackageYAMLTestDependency("web-haskell", "", "executables.web.dependencies", ScopeRuntime),
				haskellPackageYAMLTestDependency("web-haskell", "", "tests.integration.dependencies", ScopeTest),
			},
		},
		{
			name:    "component-specific dependencies",
			fixture: "package-yaml-component-dependencies",
			want: []DependencyReference{
				haskellPackageYAMLTestDependency("base", "", "executables.app.dependencies", ScopeRuntime),
				haskellPackageYAMLTestDependency("higher-rank", "", "executables.app.dependencies", ScopeRuntime),
				haskellPackageYAMLTestDependency("base", "", "library.dependencies", ScopeRuntime),
				haskellPackageYAMLTestDependency("containers", "", "library.dependencies", ScopeRuntime),
				haskellPackageYAMLTestDependency("base", "", "tests.spec.dependencies", ScopeTest),
				haskellPackageYAMLTestDependency("higher-rank", "", "tests.spec.dependencies", ScopeTest),
				haskellPackageYAMLTestDependency("hspec", "", "tests.spec.dependencies", ScopeTest),
				haskellPackageYAMLTestDependency("template-haskell", "", "tests.spec.dependencies", ScopeTest),
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
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "haskell-package-yaml" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestHaskellPackageYAMLWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newHaskellPackageYAMLParser(haskellPackageYAMLMatcherConfig{})
	if err != nil {
		t.Fatalf("newHaskellPackageYAMLParser: %v", err)
	}
	result, err := parser.Analyze("package.yaml", []byte("name: empty\nversion: 0.1.0\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func haskellPackageYAMLTestDependency(name, constraint, group string, scope DependencyScope) DependencyReference {
	dependency := DependencyReference{
		PackageType:  "hackage",
		Raw:          name,
		Name:         name,
		SourceGroup:  group,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        scope,
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}
