package analyze

import (
	"path/filepath"
	"testing"
)

func TestOCamlOpamFixturesExtractDependencies(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		path    string
		want    []DependencyReference
	}{
		{
			name:    "runtime constraints",
			fixture: "opam-runtime-constraints",
			path:    "runtime.opam",
			want: []DependencyReference{
				ocamlOpamTestDependency("minimum-version", ">= 5.1.0", ScopeRuntime),
				ocamlOpamTestDependency("runtime-core", "", ScopeRuntime),
				ocamlOpamTestDependency("version-range", ">= 0.33.0 & < 0.36.0", ScopeRuntime),
			},
		},
		{
			name:    "test and documentation filters",
			fixture: "opam-scoped-dependencies",
			path:    "scoped.opam",
			want: []DependencyReference{
				ocamlOpamTestDependency("doc-generator", "", ScopeDevelopment),
				ocamlOpamTestDependency("runtime-parser", "", ScopeRuntime),
				ocamlOpamTestDependency("test-runner", ">= 1.3.0", ScopeTest),
			},
		},
		{
			name:    "upper and lower bounds",
			fixture: "opam-upper-bound",
			path:    "upper-bound.opam",
			want: []DependencyReference{
				ocamlOpamTestDependency("build-tool", "", ScopeRuntime),
				ocamlOpamTestDependency("compatibility-layer", "< 4.0.0", ScopeRuntime),
				ocamlOpamTestDependency("compiler", ">= 4.2.0", ScopeRuntime),
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
			source := sourceForPath(t, result, test.path)
			if source.Detector != "ocaml-opam" || source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("source = %+v", source)
			}
			if !equalDependencies(source.Dependencies, test.want) {
				t.Fatalf("dependencies = %#v, want %#v", source.Dependencies, test.want)
			}
		})
	}
}

func TestOCamlOpamReportsAbsentForValidManifestWithoutDependencies(t *testing.T) {
	parser, err := newOCamlOpamParser(ocamlOpamParserConfig{})
	if err != nil {
		t.Fatalf("newOCamlOpamParser: %v", err)
	}
	result, err := parser.Analyze("empty.opam", []byte("opam-version: \"2.0\"\nname: \"empty\"\n"))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) || len(result.Dependencies) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func ocamlOpamTestDependency(name, constraint string, scope DependencyScope) DependencyReference {
	dependency := DependencyReference{
		PackageType:  "opam",
		Raw:          name,
		Name:         name,
		SourceGroup:  "depends",
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
