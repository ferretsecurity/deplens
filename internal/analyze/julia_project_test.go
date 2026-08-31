package analyze

import (
	"path/filepath"
	"testing"
)

func TestJuliaProjectFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "generated project dependencies",
			fixture: "project-generated-deps",
			want: []DependencyReference{
				juliaProjectTestDependency("CompilerBridge", "11111111-1111-1111-1111-111111111111", "", "deps", ScopeRuntime),
				juliaProjectTestDependency("MessageLayer", "22222222-2222-2222-2222-222222222222", "", "deps", ScopeRuntime),
				juliaProjectTestDependency("SocketBindings", "33333333-3333-3333-3333-333333333333", "", "deps", ScopeRuntime),
			},
		},
		{
			name:    "dependency-only project",
			fixture: "project-deps-only",
			want: []DependencyReference{
				juliaProjectTestDependency("FieldSolver", "44444444-4444-4444-4444-444444444444", "", "deps", ScopeRuntime),
				juliaProjectTestDependency("PlotCanvas", "55555555-5555-5555-5555-555555555555", "", "deps", ScopeRuntime),
			},
		},
		{
			name:    "compatibility and test extras",
			fixture: "project-compat-and-extras",
			want: []DependencyReference{
				juliaProjectTestDependency("GraphEngine", "66666666-6666-6666-6666-666666666666", "1.3, 1.4", "deps", ScopeRuntime),
				juliaProjectTestDependency("MathCore", "77777777-7777-7777-7777-777777777777", "", "deps", ScopeRuntime),
				juliaProjectTestDependency("FixtureData", "88888888-8888-8888-8888-888888888888", "2.4 - 2.7", "extras", ScopeTest),
				juliaProjectTestDependency("TestHarness", "99999999-9999-9999-9999-999999999999", "", "extras", ScopeTest),
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "julia", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "Project.toml")
			if source.Detector != "julia-project" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestJuliaProjectWithoutDependenciesIsCompleteAndAbsent(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	result, err := Scan(filepath.Join("..", "..", "testdata", "julia", "project-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	source := sourceForPath(t, result, "Project.toml")
	if source.Detector != "julia-project" || source.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("source = %+v", source)
	}
	if len(source.Dependencies) != 0 {
		t.Fatalf("dependencies = %#v, want none", source.Dependencies)
	}
}

func juliaProjectTestDependency(name, uuid, constraint, sourceGroup string, scope DependencyScope) DependencyReference {
	dependency := DependencyReference{
		PackageType:  "julia",
		Raw:          name,
		Name:         name,
		SourceGroup:  sourceGroup,
		OriginKind:   OriginRegistry,
		Relationship: RelationshipDirect,
		Scope:        scope,
		Attributes:   map[string]string{"uuid": uuid},
	}
	if constraint != "" {
		dependency.Raw += "@" + constraint
		dependency.VersionConstraint = constraint
	}
	return dependency
}
