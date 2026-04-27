package analyze

import (
	"slices"
	"testing"
)

func TestUVLockParserExtractsRegistryAndPathDependencies(t *testing.T) {
	parser, err := newUVLockParser(uvLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newUVLockParser failed: %v", err)
	}

	result, err := parser.Match("uv.lock", []byte(`
version = 1

[[package]]
name = "requests"
version = "2.32.3"
source = { registry = "https://pypi.org/simple" }

[[package]]
name = "local-lib"
version = "0.1.0"
source = { path = "../my-lib" }
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests==2.32.3", "local-lib"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestUVLockParserSetsStructuredFields(t *testing.T) {
	parser, _ := newUVLockParser(uvLockMatcherConfig{})
	result, _ := parser.Match("uv.lock", []byte(`
version = 1

[[package]]
name = "requests"
version = "2.32.3"
`))
	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result.Dependencies))
	}
	dep := result.Dependencies[0]
	if dep.Raw != "requests==2.32.3" {
		t.Errorf("Raw: got %q", dep.Raw)
	}
	if dep.Name != "requests" {
		t.Errorf("Name: got %q", dep.Name)
	}
	if dep.Version != "2.32.3" {
		t.Errorf("Version: got %q", dep.Version)
	}
}

func TestUVLockParserEmitsNonSelfPathDependencies(t *testing.T) {
	parser, _ := newUVLockParser(uvLockMatcherConfig{})
	result, _ := parser.Match("uv.lock", []byte(`
version = 1

[[package]]
name = "my-lib"
version = "0.1.0"

[package.source]
path = "../my-lib"
`))
	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 path dep, got %d", len(result.Dependencies))
	}
	dep := result.Dependencies[0]
	if dep.Raw != "my-lib" {
		t.Errorf("Raw: got %q", dep.Raw)
	}
	if dep.Name != "my-lib" {
		t.Errorf("Name: got %q", dep.Name)
	}
	if dep.Source != "path" {
		t.Errorf("Source: got %q", dep.Source)
	}
	if dep.Version != "" {
		t.Errorf("Version: expected empty, got %q", dep.Version)
	}
}

func TestUVLockParserReturnsConclusiveEmptyForVersionOnlyFiles(t *testing.T) {
	parser, err := newUVLockParser(uvLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newUVLockParser failed: %v", err)
	}

	result, err := parser.Match("uv.lock", []byte("version = 1\n"))
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
}

func TestUVLockParserRetainsEditableDependenciesAndIgnoresSelfStyleSources(t *testing.T) {
	parser, err := newUVLockParser(uvLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newUVLockParser failed: %v", err)
	}

	result, err := parser.Match("uv.lock", []byte(`
version = 1

[[package]]
name = "foo"
version = "0.1.0"
source = { editable = "../packages/foo" }

[[package]]
name = "editable-lib"
version = "0.1.0"
source = { editable = "." }

[[package]]
name = "workspace-lib"
version = "0.2.0"
source = { workspace = true }

[[package]]
name = "requests"
version = "2.32.3"
source = { registry = "https://pypi.org/simple" }
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"foo==0.1.0", "requests==2.32.3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestUVLockParserIgnoresSelfStyleEditableEntries(t *testing.T) {
	parser, err := newUVLockParser(uvLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newUVLockParser failed: %v", err)
	}

	result, err := parser.Match("uv.lock", []byte(`
version = 1

[[package]]
name = "editable-lib"
version = "0.1.0"
source = { editable = "." }

[[package]]
name = "editable-dot-slash"
version = "0.1.1"
source = { editable = "./" }

[[package]]
name = "requests"
version = "2.32.3"
source = { registry = "https://pypi.org/simple" }
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests==2.32.3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestUVLockParserReturnsConclusiveEmptyAfterFilteringIgnoredEntries(t *testing.T) {
	parser, err := newUVLockParser(uvLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newUVLockParser failed: %v", err)
	}

	result, err := parser.Match("uv.lock", []byte(`
version = 1

[[package]]
name = "editable-lib"
version = "0.1.0"
source = { editable = "." }

[[package]]
name = "workspace-lib"
version = "0.2.0"
source = { workspace = true }

[[package]]
name = "virtual-lib"
version = "0.3.0"
source = { virtual = "." }
`))
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
}

func TestUVLockParserIgnoresVirtualSelfEntries(t *testing.T) {
	parser, err := newUVLockParser(uvLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newUVLockParser failed: %v", err)
	}

	result, err := parser.Match("uv.lock", []byte(`
version = 1

[[package]]
name = "virtual-lib"
version = "0.1.0"
source = { virtual = "." }

[[package]]
name = "requests"
version = "2.32.3"
source = { registry = "https://pypi.org/simple" }
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests==2.32.3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestUVLockParserReturnsNoMatchWithoutTopLevelVersion(t *testing.T) {
	parser, err := newUVLockParser(uvLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newUVLockParser failed: %v", err)
	}

	result, err := parser.Match("uv.lock", []byte(`
[[package]]
name = "requests"
version = "2.32.3"
source = { registry = "https://pypi.org/simple" }
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if result.Matched {
		t.Fatalf("expected no match, got %+v", result)
	}
	if result.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
	if result.HasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", result.HasDependencies)
	}
}

func TestUVLockParserRejectsInvalidTOML(t *testing.T) {
	parser, err := newUVLockParser(uvLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newUVLockParser failed: %v", err)
	}

	_, err = parser.Match("uv.lock", []byte("version = 1\n[[package]]\nname = \"requests\"\nversion = "))
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
