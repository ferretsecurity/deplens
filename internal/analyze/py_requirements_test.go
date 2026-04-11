package analyze

import (
	"path/filepath"
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

	result, err := parser.Match("requirements.txt", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{
		"requests>=2.31",
		`uvicorn[standard]>=0.30 ; python_version >= "3.11"`,
	}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestPyRequirementsParserJoinsContinuationLines(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	content := []byte("very-long-package-name>=1.0,\\\n  <2.0\n")
	result, err := parser.Match("requirements.txt", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"very-long-package-name>=1.0, <2.0"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestPyRequirementsParserIgnoresDirectivesAndReturnsConclusiveEmpty(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	content := []byte(`
# generated
--constraint constraints.txt
--index-url https://pypi.example.com/simple
`)

	result, err := parser.Match("requirements.txt", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if result.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
	if result.HasDependencies == nil || *result.HasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", result.HasDependencies)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", result.Warnings)
	}
}

func TestPyRequirementsParserResolvesNestedIncludes(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-recursive", "requirements.txt")
	content := []byte("-r base.txt\npendulum>=3\n--requirements extras/dev.txt\n")

	result, err := parser.Match(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "urllib3<3", "pendulum>=3", "pytest>=8"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", result.Warnings)
	}
}

func TestPyRequirementsParserPreservesDuplicatesAcrossIncludes(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-duplicates", "requirements.txt")
	content := []byte("-r base.txt\nrequests>=2.31\n-r extras.txt\n")

	result, err := parser.Match(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "requests>=2.31", "urllib3<3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %+v", result.Warnings)
	}
}

func TestPyRequirementsParserWarnsAndKeepsPartialDependenciesForMissingInclude(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-missing-include", "requirements.txt")
	content := []byte("-r missing.txt\nrequests>=2.31\n")

	result, err := parser.Match(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected one warning, got %+v", result.Warnings)
	}
}

func TestPyRequirementsParserWarnsAndReturnsUnknownForUnresolvedIncludesWithoutDependencies(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-missing-include-only", "requirements.txt")
	content := []byte("-r missing.txt\n")

	result, err := parser.Match(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if result.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
	if result.HasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", result.HasDependencies)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected one warning, got %+v", result.Warnings)
	}
}

func TestPyRequirementsParserWarnsOnIncludeCycles(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-cycle", "requirements.txt")
	content := []byte("-r base.txt\n")

	result, err := parser.Match(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected one warning, got %+v", result.Warnings)
	}
}
