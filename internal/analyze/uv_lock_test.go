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
	if want := []string{"requests==2.32.3", "local-lib==0.1.0"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
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
