package analyze

import (
	"maps"
	"path/filepath"
	"slices"
	"testing"
)

func TestPyRequirementsParserSetsStructuredFields(t *testing.T) {
	parser, _ := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	result, _ := parser.Analyze("requirements.txt", []byte("requests>=2.28.0,<3\nflask==3.0.0\npytest\nfastapi[all]>=0.110; python_version >= '3.10'\n"))
	cases := []struct {
		raw, wantName, wantConstraint string
		wantExtras                    map[string]string
	}{
		{"requests>=2.28.0,<3", "requests", ">=2.28.0,<3", nil},
		{"flask==3.0.0", "flask", "==3.0.0", nil},
		{"pytest", "pytest", "", nil},
		{"fastapi[all]>=0.110; python_version >= '3.10'", "fastapi", ">=0.110", map[string]string{"extras": "all", "marker": "python_version >= '3.10'"}},
	}
	for i, tc := range cases {
		if i >= len(result.Dependencies) {
			t.Fatalf("missing dependency %d", i)
		}
		dep := result.Dependencies[i]
		if dep.Raw != tc.raw {
			t.Errorf("[%d] Raw: got %q want %q", i, dep.Raw, tc.raw)
		}
		if dep.Name != tc.wantName {
			t.Errorf("[%d] Name: got %q want %q", i, dep.Name, tc.wantName)
		}
		if dep.VersionConstraint != tc.wantConstraint {
			t.Errorf("[%d] VersionConstraint: got %q want %q", i, dep.VersionConstraint, tc.wantConstraint)
		}
		if !maps.Equal(dep.Attributes, tc.wantExtras) {
			t.Errorf("[%d] Attributes: got %#v want %#v", i, dep.Attributes, tc.wantExtras)
		}
	}
}

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

	result, err := parser.Analyze("requirements.txt", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{
		"requests>=2.31",
		`uvicorn[standard]>=0.30 ; python_version >= "3.11"`,
	}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
	}
}

func TestPyRequirementsParserJoinsContinuationLines(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	content := []byte("very-long-package-name>=1.0,\\\n  <2.0\n")
	result, err := parser.Analyze("requirements.txt", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{"very-long-package-name>=1.0, <2.0"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
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

	result, err := parser.Analyze("requirements.txt", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if result.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
	if result.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Analysis)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diagnostics)
	}
}

func TestPyRequirementsParserResolvesNestedIncludes(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-recursive", "requirements.txt")
	content := []byte("-r base.txt\npendulum>=3\n--requirements extras/dev.txt\n")

	result, err := parser.Analyze(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "urllib3<3", "pendulum>=3", "pytest>=8"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diagnostics)
	}
}

func TestPyRequirementsParserPreservesDuplicatesAcrossIncludes(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-duplicates", "requirements.txt")
	content := []byte("-r base.txt\nrequests>=2.31\n-r extras.txt\n")

	result, err := parser.Analyze(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "requests>=2.31", "urllib3<3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", result.Diagnostics)
	}
}

func TestPyRequirementsParserWarnsAndKeepsPartialDependenciesForMissingInclude(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-missing-include", "requirements.txt")
	content := []byte("-r missing.txt\nrequests>=2.31\n")

	result, err := parser.Analyze(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one warning, got %+v", result.Diagnostics)
	}
}

func TestPyRequirementsParserWarnsAndReturnsUnknownForUnresolvedIncludesWithoutDependencies(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-missing-include-only", "requirements.txt")
	content := []byte("-r missing.txt\n")

	result, err := parser.Analyze(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if result.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
	if result.Analysis.Presence != PresenceUnknown {
		t.Fatalf("expected unknown presence, got %+v", result.Analysis)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one warning, got %+v", result.Diagnostics)
	}
}

func TestPyRequirementsParserWarnsOnIncludeCycles(t *testing.T) {
	parser, err := newPyRequirementsMatcher(pyRequirementsMatcherConfig{})
	if err != nil {
		t.Fatalf("newPyRequirementsMatcher failed: %v", err)
	}

	root := filepath.Join("..", "..", "testdata", "python", "requirements-cycle", "requirements.txt")
	content := []byte("-r base.txt\n")

	result, err := parser.Analyze(root, content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
	}
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected one warning, got %+v", result.Diagnostics)
	}
}
