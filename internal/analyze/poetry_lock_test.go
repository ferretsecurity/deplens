package analyze

import (
	"slices"
	"testing"
)

func TestPoetryLockParserExtractsRetainedDependencies(t *testing.T) {
	parser, err := newPoetryLockParser(poetryLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPoetryLockParser failed: %v", err)
	}

	result, err := parser.Match("poetry.lock", []byte(`
[[package]]
name = "requests"
version = "2.32.3"
groups = ["main"]
files = []

[[package]]
name = "urllib3"
version = "2.2.2"
groups = ["main"]
files = []

[metadata]
lock-version = "2.1"
python-versions = "^3.11"
content-hash = "basic"
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests==2.32.3", "urllib3==2.2.2"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestPoetryLockParserReturnsConclusiveEmptyForMetadataOnlyFiles(t *testing.T) {
	parser, err := newPoetryLockParser(poetryLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPoetryLockParser failed: %v", err)
	}

	result, err := parser.Match("poetry.lock", []byte(`
[metadata]
lock-version = "2.1"
python-versions = "^3.11"
content-hash = "empty"
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

func TestPoetryLockParserSkipsMalformedAndFilteredEntries(t *testing.T) {
	parser, err := newPoetryLockParser(poetryLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPoetryLockParser failed: %v", err)
	}

	result, err := parser.Match("poetry.lock", []byte(`
[[package]]
name = "requests"
version = "2.32.3"
optional = false
markers = "python_version >= '3.8'"
groups = ["main"]
files = []

[[package]]
name = "broken-no-version"
groups = ["main"]
files = []

[[package]]
name = "internal-lib"
version = "1.4.2"
groups = ["main"]
files = []

[package.source]
type = "git"
url = "https://github.com/example/internal-lib.git"

[[package]]
name = "my-app"
version = "0.1.0"
develop = true
groups = ["main"]
files = []

[package.source]
type = "directory"
url = "."

[metadata]
lock-version = "2.1"
python-versions = "^3.11"
content-hash = "mixed"
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests==2.32.3", "internal-lib"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestPoetryLockParserSetsStructuredFieldsForRegistryDep(t *testing.T) {
	parser, _ := newPoetryLockParser(poetryLockMatcherConfig{})
	result, _ := parser.Match("poetry.lock", []byte(`
[[package]]
name = "requests"
version = "2.32.3"

[metadata]
lock-version = "2.1"
python-versions = "^3.11"
content-hash = "x"
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

func TestPoetryLockParserEmitsGitDependencies(t *testing.T) {
	parser, _ := newPoetryLockParser(poetryLockMatcherConfig{})
	result, _ := parser.Match("poetry.lock", []byte(`
[[package]]
name = "mylib"
version = "0.1.0"

[package.source]
type = "git"
url = "https://github.com/org/mylib.git"
reference = "abc123"

[metadata]
lock-version = "2.1"
python-versions = "^3.11"
content-hash = "x"
`))
	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dep (git), got %d", len(result.Dependencies))
	}
	dep := result.Dependencies[0]
	if dep.Raw != "mylib" {
		t.Errorf("Raw: got %q", dep.Raw)
	}
	if dep.Name != "mylib" {
		t.Errorf("Name: got %q", dep.Name)
	}
	if dep.Version != "" {
		t.Errorf("Version: expected empty, got %q", dep.Version)
	}
	if dep.Source != "git" {
		t.Errorf("Source: got %q", dep.Source)
	}
	if dep.Extras["source_url"] != "https://github.com/org/mylib.git" {
		t.Errorf("Extras[source_url]: got %q", dep.Extras["source_url"])
	}
	if dep.Extras["source_ref"] != "abc123" {
		t.Errorf("Extras[source_ref]: got %q", dep.Extras["source_ref"])
	}
}

func TestPoetryLockParserIncludesNonSelfDirectoryAndDeduplicatesExactEntries(t *testing.T) {
	parser, err := newPoetryLockParser(poetryLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPoetryLockParser failed: %v", err)
	}

	result, err := parser.Match("poetry.lock", []byte(`
[[package]]
name = "shared-lib"
version = "0.4.0"
groups = ["main"]
files = []

[package.source]
type = "directory"
url = "../shared-lib"

[[package]]
name = "requests"
version = "2.32.3"
category = "main"
files = []

[[package]]
name = "requests"
version = "2.32.3"
category = "main"
files = []

[[package]]
name = "urllib3"
version = "2.2.1"
groups = ["main"]
files = []

[[package]]
name = "urllib3"
version = "2.2.2"
groups = ["main"]
files = []

[metadata]
lock-version = "2.1"
python-versions = "^3.11"
content-hash = "dupes"
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"shared-lib==0.4.0", "requests==2.32.3", "urllib3==2.2.1", "urllib3==2.2.2"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestPoetryLockParserReturnsConclusiveEmptyAfterFiltering(t *testing.T) {
	parser, err := newPoetryLockParser(poetryLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPoetryLockParser failed: %v", err)
	}

	result, err := parser.Match("poetry.lock", []byte(`
[[package]]
name = "my-app"
version = "0.1.0"
groups = ["main"]
files = []

[package.source]
type = "directory"
url = "./"

[metadata]
lock-version = "2.1"
python-versions = "^3.11"
content-hash = "filtered-empty"
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

func TestPoetryLockParserReturnsNoMatchForUnstructuredTOML(t *testing.T) {
	parser, err := newPoetryLockParser(poetryLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPoetryLockParser failed: %v", err)
	}

	result, err := parser.Match("poetry.lock", []byte("title = \"not a poetry lock\"\n"))
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

func TestPoetryLockParserRejectsInvalidTOML(t *testing.T) {
	parser, err := newPoetryLockParser(poetryLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newPoetryLockParser failed: %v", err)
	}

	_, err = parser.Match("poetry.lock", []byte("[[package]]\nname = \"requests\"\nversion = "))
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
