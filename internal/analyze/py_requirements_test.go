package analyze

import (
	"slices"
	"testing"
)

func TestPyRequirementsParserExtractsStaticDependencyLines(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	content := []byte(`
# base runtime deps
requests>=2.31

uvicorn[standard]>=0.30 ; python_version >= "3.11"
`)

	deps, hasDependencies, ok, err := parser.Match("requirements.txt", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{
		"requests>=2.31",
		`uvicorn[standard]>=0.30 ; python_version >= "3.11"`,
	}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestPyRequirementsParserJoinsContinuationLines(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	content := []byte("very-long-package-name>=1.0,\\\n  <2.0\n")
	deps, hasDependencies, ok, err := parser.Match("requirements.txt", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"very-long-package-name>=1.0, <2.0"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestPyRequirementsParserIgnoresDirectivesAndReturnsConclusiveEmpty(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	content := []byte(`
# generated
-r common.txt
--constraint constraints.txt
--index-url https://pypi.example.com/simple
`)

	deps, hasDependencies, ok, err := parser.Match("requirements.txt", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if hasDependencies == nil || *hasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", hasDependencies)
	}
}
