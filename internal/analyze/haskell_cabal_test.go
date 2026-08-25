package analyze

import (
	"path/filepath"
	"testing"
)

func TestHaskellCabalFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "library conditional executable and test dependencies",
			fixture: "lambda-circus",
			want: []DependencyReference{
				haskellCabalTestDependency("LambdaCircus", "", "executable.lambda-circus.build-depends", ScopeRuntime),
				haskellCabalTestDependency("base", "", "executable.lambda-circus.build-depends", ScopeRuntime),
				haskellCabalTestDependency("aeson", "", "library.build-depends", ScopeRuntime),
				haskellCabalTestDependency("base", "", "library.build-depends", ScopeRuntime),
				haskellCabalTestDependency("unix", "", "library.build-depends", ScopeRuntime),
				haskellCabalTestDependency("base", "", "test-suite.spec.build-depends", ScopeTest),
				haskellCabalTestDependency("hspec", "", "test-suite.spec.build-depends", ScopeTest),
			},
		},
		{
			name:    "version constraints across component kinds",
			fixture: "orchid",
			want: []DependencyReference{
				haskellCabalTestDependency("Orchid", "", "benchmark.orchid-bench.build-depends", ScopeBuild),
				haskellCabalTestDependency("base", ">= 4.8", "benchmark.orchid-bench.build-depends", ScopeBuild),
				haskellCabalTestDependency("Orchid", "", "executable.orchid.build-depends", ScopeRuntime),
				haskellCabalTestDependency("base", ">= 4.8", "executable.orchid.build-depends", ScopeRuntime),
				haskellCabalTestDependency("optparse-applicative", "", "executable.orchid.build-depends", ScopeRuntime),
				haskellCabalTestDependency("base", ">= 4.8", "library.build-depends", ScopeRuntime),
				haskellCabalTestDependency("containers", "", "library.build-depends", ScopeRuntime),
				haskellCabalTestDependency("Orchid", "", "test-suite.orchid-test.build-depends", ScopeTest),
				haskellCabalTestDependency("base", ">= 4.8", "test-suite.orchid-test.build-depends", ScopeTest),
				haskellCabalTestDependency("hspec", "", "test-suite.orchid-test.build-depends", ScopeTest),
			},
		},
		{
			name:    "value continuation and compound constraints",
			fixture: "parallel-arrows",
			want: []DependencyReference{
				haskellCabalTestDependency("Parallel-Arrows-Definition", "==0.1.1.0", "library.build-depends", ScopeRuntime),
				haskellCabalTestDependency("base", ">=4.7 && <5.0", "library.build-depends", ScopeRuntime),
				haskellCabalTestDependency("monad-par", "", "library.build-depends", ScopeRuntime),
				haskellCabalTestDependency("Parallel-Arrows-BaseSpec", "==0.1.1.0", "test-suite.spec.build-depends", ScopeTest),
				haskellCabalTestDependency("hspec", "==2.*", "test-suite.spec.build-depends", ScopeTest),
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "haskell", "cabal", tc.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, map[string]string{
				"lambda-circus":   "LambdaCircus.cabal",
				"orchid":          "Orchid.cabal",
				"parallel-arrows": "Parallel-Arrows-ParMonad.cabal",
			}[tc.fixture])
			if source.Detector != "haskell-cabal" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestHaskellCabalManifestWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	parser, err := newHaskellCabalParser(haskellCabalMatcherConfig{})
	if err != nil {
		t.Fatalf("newHaskellCabalParser: %v", err)
	}
	result, err := parser.Analyze("empty.cabal", []byte("name: empty\nversion: 0.1.0.0\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func haskellCabalTestDependency(name, constraint, group string, scope DependencyScope) DependencyReference {
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
