package analyze

import (
	"path/filepath"
	"testing"
)

func TestOCamlEsyFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    []DependencyReference
	}{
		{
			name:    "runtime dependencies",
			fixture: "esy-runtime-dependencies",
			want: []DependencyReference{
				ocamlEsyTestDependency("@opam/merlin", "*", "dependencies", ScopeRuntime),
				ocamlEsyTestDependency("bs-platform", "*", "dependencies", ScopeRuntime),
			},
		},
		{
			name:    "override dependencies",
			fixture: "esy-override-dependencies",
			want: []DependencyReference{
				ocamlEsyTestDependency("@opam/conf-libev", "*", "override.dependencies", ScopeRuntime),
				ocamlEsyTestDependency("ocaml", "~4.9.0", "override.dependencies", ScopeRuntime),
			},
		},
		{
			name:    "runtime and development dependencies",
			fixture: "esy-dev-dependencies",
			want: []DependencyReference{
				ocamlEsyTestDependency("@opam/dune", ">= 2.0", "dependencies", ScopeRuntime),
				ocamlEsyTestDependency("ocaml", ">= 4.8.0", "dependencies", ScopeRuntime),
				ocamlEsyTestDependency("@opam/ocamlformat", "*", "devDependencies", ScopeDevelopment),
				ocamlEsyTestDependency("ocaml", "~4.13.1", "devDependencies", ScopeDevelopment),
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
			source := sourceForPath(t, result, "esy.json")
			if source.Detector != "ocaml-esy" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestOCamlEsyReportsAbsentWhenDependencyGroupsAreEmpty(t *testing.T) {
	parser, err := newOCamlEsyParser(ocamlEsyParserConfig{})
	if err != nil {
		t.Fatalf("newOCamlEsyParser: %v", err)
	}
	result, err := parser.Analyze("esy.json", []byte(`{"name":"empty","dependencies":{},"override":{"dependencies":{}}}`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func ocamlEsyTestDependency(name, constraint, group string, scope DependencyScope) DependencyReference {
	return DependencyReference{
		Raw:               name + "@" + constraint,
		Name:              name,
		VersionConstraint: constraint,
		SourceGroup:       group,
		Relationship:      RelationshipDirect,
		Scope:             scope,
	}
}
