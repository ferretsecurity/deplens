package analyze

import (
	"path/filepath"
	"testing"
)

func TestGleamFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "multiple runtime and test dependencies",
			fixture: "cactus-dependencies",
			want: []DependencyReference{
				gleamTestDependency("runtime_alpha", ">= 1.1.0 and < 2.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_beta", ">= 2.2.0 and < 3.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_delta", ">= 2.1.0 and < 3.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_epsilon", ">= 1.2.0 and < 2.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_eta", ">= 1.3.0 and < 2.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_gamma", ">= 0.4.0 and < 1.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_iota", ">= 1.4.0 and < 2.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_theta", ">= 2.0.2 and < 3.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_zeta", ">= 1.7.0 and < 2.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("test_assertions", ">= 1.1.0 and < 2.0.0", "dev-dependencies", ScopeTest),
				gleamTestDependency("test_preview", ">= 4.1.0-rc1 and < 5.0.0", "dev-dependencies", ScopeTest),
			},
		},
		{
			name:    "single runtime and test dependency",
			fixture: "optimist-dependencies",
			want: []DependencyReference{
				gleamTestDependency("ui_runtime", ">= 0.35.0 and < 2.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("ui_tests", ">= 1.1.0 and < 2.0.0", "dev-dependencies", ScopeTest),
			},
		},
		{
			name:    "runtime dependencies with empty test table",
			fixture: "conversation-dependencies",
			want: []DependencyReference{
				gleamTestDependency("runtime_core", ">= 0.33.0 and < 2.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_http", ">= 3.6.0 and < 5.0.0", "dependencies", ScopeRuntime),
				gleamTestDependency("runtime_js", ">= 0.8.0 and < 2.0.0", "dependencies", ScopeRuntime),
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "gleam", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "gleam.toml")
			wantAnalysis := SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}
			if source.Detector != "gleam" || source.Analysis != wantAnalysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestGleamManifestWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	result, err := Scan(filepath.Join("..", "..", "testdata", "gleam", "project-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "gleam.toml")
	if source.Detector != "gleam" || source.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("source = %+v", source)
	}
	if len(source.Dependencies) != 0 {
		t.Fatalf("dependencies = %#v, want none", source.Dependencies)
	}
}

func gleamTestDependency(name, constraint, sourceGroup string, scope DependencyScope) DependencyReference {
	return DependencyReference{
		PackageType:       "hex",
		Raw:               name + "@" + constraint,
		Name:              name,
		VersionConstraint: constraint,
		SourceGroup:       sourceGroup,
		OriginKind:        OriginRegistry,
		Relationship:      RelationshipDirect,
		Scope:             scope,
	}
}
