package analyze

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestConanLockFixturesExtractDependencyReferences(t *testing.T) {
	testCases := []struct {
		name       string
		fixtureDir string
		want       []DependencyReference
	}{
		{
			name:       "v0.5 runtime and build requirements",
			fixtureDir: "lock-v05-runtime-build",
			want: []DependencyReference{
				{PackageType: "conan", Raw: "build-tool/4.5.6#0123456789abcdef", Name: "build-tool", Version: "4.5.6", SourceGroup: "build_requires", OriginKind: OriginRegistry, Scope: ScopeBuild, Attributes: map[string]string{"recipe_revision": "0123456789abcdef"}},
				{PackageType: "conan", Raw: "runtime-lib/1.2.3#abcdef0123456789%1700000000.000", Name: "runtime-lib", Version: "1.2.3", SourceGroup: "requires", OriginKind: OriginRegistry, Scope: ScopeRuntime, Attributes: map[string]string{"recipe_revision": "abcdef0123456789"}},
			},
		},
		{
			name:       "v0.5 Python requirements",
			fixtureDir: "lock-v05-python-requires",
			want: []DependencyReference{
				{PackageType: "conan", Raw: "build-system/3.4.5", Name: "build-system", Version: "3.4.5", SourceGroup: "build_requires", OriginKind: OriginRegistry, Scope: ScopeBuild},
				{PackageType: "conan", Raw: "recipe-helper/0.4.0#1234567890abcdef", Name: "recipe-helper", Version: "0.4.0", SourceGroup: "python_requires", OriginKind: OriginRegistry, Scope: ScopeDevelopment, Attributes: map[string]string{"recipe_revision": "1234567890abcdef"}},
				{PackageType: "conan", Raw: "utility/2.0.0", Name: "utility", Version: "2.0.0", SourceGroup: "requires", OriginKind: OriginRegistry, Scope: ScopeRuntime},
			},
		},
		{
			name:       "v0.4 graph nodes",
			fixtureDir: "lock-v04-graph",
			want: []DependencyReference{
				{PackageType: "conan", Raw: "graphics-lib/1.0.0", Name: "graphics-lib", Version: "1.0.0", SourceGroup: "graph_lock.nodes", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"package_id": "0123456789abcdef"}},
				{PackageType: "conan", Raw: "math-lib/2.3.4", Name: "math-lib", Version: "2.3.4", SourceGroup: "graph_lock.nodes", OriginKind: OriginRegistry, Relationship: RelationshipDirect, Scope: ScopeRuntime, Attributes: map[string]string{"package_id": "fedcba9876543210", "package_revision": "abc123"}},
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "cpp", "conan", tc.fixtureDir), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			if len(result.Sources) != 1 {
				t.Fatalf("sources = %+v, want one", result.Sources)
			}
			source := result.Sources[0]
			if source.Detector != "cpp-conan-lock" {
				t.Fatalf("detector = %q, want cpp-conan-lock", source.Detector)
			}
			if want := (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}); source.Analysis != want {
				t.Fatalf("analysis = %+v, want %+v", source.Analysis, want)
			}
			if !reflect.DeepEqual(source.Dependencies, tc.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestConanLockReturnsConclusiveEmptyForEmptyV05Lock(t *testing.T) {
	parser, err := newConanLockParser(conanLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newConanLockParser: %v", err)
	}

	result, err := parser.Analyze("conan.lock", []byte(`{"version":"0.5","requires":[]}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if !result.Recognized {
		t.Fatal("expected recognized Conan lockfile")
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("analysis = %+v", result.Analysis)
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("dependencies = %+v, want none", result.Dependencies)
	}
}
