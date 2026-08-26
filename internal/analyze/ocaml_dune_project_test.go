package analyze

import (
	"path/filepath"
	"testing"
)

func TestOCamlDuneProjectFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "runtime and test constraints",
			fixture: "dune-project-petrol",
			want: []DependencyReference{
				duneProjectTestDependency("runtime-lib", ">= 1.0", "package.api.depends", ScopeRuntime),
				duneProjectTestDependency("test-driver", ">= 2.0", "package.api.depends", ScopeTest),
			},
		},
		{
			name:    "project without package dependencies",
			fixture: "dune-project-jscoq",
			want:    []DependencyReference{},
		},
		{
			name:    "multiple package dependency groups",
			fixture: "dune-project-nightmare",
			want: []DependencyReference{
				duneProjectTestDependency("core", "= :version", "package.addon.depends", ScopeRuntime),
				duneProjectTestDependency("web", ">= 1.0", "package.addon.depends", ScopeRuntime),
				duneProjectTestDependency("helper", "", "package.core.depends", ScopeRuntime),
				duneProjectTestDependency("ocaml", ">= 5.0", "package.core.depends", ScopeRuntime),
				duneProjectTestDependency("test-helper", "= :version", "package.core.depends", ScopeTest),
			},
		},
	}

	ruleset := mustLoadDefaultRules(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "ocaml", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan: %v", err)
			}
			source := sourceForPath(t, result, "dune-project")
			wantAnalysis := SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}
			if len(test.want) == 0 {
				wantAnalysis.Presence = PresenceAbsent
			}
			if source.Detector != "ocaml-dune-project" || source.Analysis != wantAnalysis {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func duneProjectTestDependency(name, constraint, group string, scope DependencyScope) DependencyReference {
	dependency := DependencyReference{
		PackageType:  "opam",
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
