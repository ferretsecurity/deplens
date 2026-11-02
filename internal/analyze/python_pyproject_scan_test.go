package analyze

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestScanPyprojectDynamicFiles(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "app", "pyproject.toml"), `
[build-system]
requires = ["setuptools>=68"]
[project]
name = "example"
dynamic = ["dependencies", "optional-dependencies"]
[tool.setuptools.dynamic]
dependencies = {file = ["../runtime.deps", "extra.deps"]}
[tool.setuptools.dynamic.optional-dependencies]
test = {file = "test.deps"}
docs = {file = ["docs.deps"]}
`)
	mustWriteFile(t, filepath.Join(root, "runtime.deps"), "# runtime\nrequests[socks]>=2.31; python_version >= '3.10'\n\n")
	mustWriteFile(t, filepath.Join(root, "app", "extra.deps"), "urllib3>=2 # transport\n")
	mustWriteFile(t, filepath.Join(root, "app", "test.deps"), "pytest>=8\n")
	mustWriteFile(t, filepath.Join(root, "app", "docs.deps"), "mkdocs>=1.6\n")

	source := scanPythonSource(t, root, "python-pyproject")
	assertPythonAnalysis(t, source, PresencePresent, ExtractionComplete)
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"setuptools>=68", "requests[socks]>=2.31; python_version >= '3.10'", "urllib3>=2", "mkdocs>=1.6", "pytest>=8"}) {
		t.Fatalf("unexpected dependencies: %v", got)
	}
	if source.Dependencies[1].Name != "requests" || source.Dependencies[1].SourceGroup != "project.dependencies" || source.Dependencies[3].SourceGroup != "project.optional-dependencies.docs" || source.Dependencies[4].SourceGroup != "project.optional-dependencies.test" {
		t.Fatalf("lost requirement metadata or source groups: %+v", source.Dependencies)
	}
}

func TestScanPyprojectIncompleteDynamic(t *testing.T) {
	for _, test := range []struct {
		name   string
		config string
		file   string
	}{
		{"unbound", "", "requests>=2\n"},
		{"attribute", `[tool.setuptools.dynamic]
dependencies = {attr = "package.dependencies"}`, "requests>=2\n"},
		{"missing file", `[tool.setuptools.dynamic]
dependencies = {file = "missing.deps"}`, "requests>=2\n"},
		{"invalid file type", `[tool.setuptools.dynamic]
dependencies = {file = 42}`, "requests>=2\n"},
		{"invalid list member", `[tool.setuptools.dynamic]
dependencies = {file = ["runtime.deps", 42]}`, "requests>=2\n"},
		{"empty files", `[tool.setuptools.dynamic]
dependencies = {file = []}`, "requests>=2\n"},
		{"conflicting directives", `[tool.setuptools.dynamic]
dependencies = {file = "runtime.deps", attr = "package.dependencies"}`, "requests>=2\n"},
		{"pip include", `[tool.setuptools.dynamic]
dependencies = {file = "runtime.deps"}`, "requests>=2\n-r outside.deps\n"},
		{"pip index", `[tool.setuptools.dynamic]
dependencies = {file = "runtime.deps"}`, "requests>=2\n--index-url https://example.com\n"},
		{"invalid requirement", `[tool.setuptools.dynamic]
dependencies = {file = "runtime.deps"}`, "requests>=2\nnot a requirement\n"},
		{"invalid marker", `[tool.setuptools.dynamic]
dependencies = {file = "runtime.deps"}`, "requests>=2\npytest; invalid_marker == 'x'\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			mustWriteFile(t, filepath.Join(root, "pyproject.toml"), "[build-system]\nrequires = ['setuptools>=68']\n[project]\ndynamic = ['dependencies']\n"+test.config)
			mustWriteFile(t, filepath.Join(root, "runtime.deps"), test.file)
			source := scanPythonSource(t, root, "python-pyproject")
			assertPythonAnalysis(t, source, PresencePresent, ExtractionPartial)
			if len(source.Diagnostics) == 0 || source.Diagnostics[0].Severity != DiagnosticWarning || !strings.Contains(source.Diagnostics[0].Message, "dependencies") {
				t.Fatalf("missing dynamic diagnostic: %+v", source.Diagnostics)
			}
			if source.Dependencies[0].Raw != "setuptools>=68" {
				t.Fatalf("lost static references: %+v", source.Dependencies)
			}
		})
	}
}

func TestScanPyprojectDynamicOnly(t *testing.T) {
	for _, field := range []string{"dependencies", "optional-dependencies"} {
		t.Run(field, func(t *testing.T) {
			root := t.TempDir()
			mustWriteFile(t, filepath.Join(root, "pyproject.toml"), "[project]\nname = 'example'\ndynamic = ['"+field+"']\n")
			source := scanPythonSource(t, root, "python-pyproject")
			assertPythonAnalysis(t, source, PresenceUnknown, ExtractionFailed)
			if len(source.Diagnostics) == 0 || !strings.Contains(source.Diagnostics[0].Message, field) {
				t.Fatalf("missing unresolved field: %+v", source)
			}
		})
	}
}

func TestScanPyprojectOptionalDynamicPartial(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), `
[project]
dynamic = ["optional-dependencies"]
dependencies = ["requests>=2"]
[tool.setuptools.dynamic.optional-dependencies]
test = {file = "test.deps"}
docs = {attr = "package.docs"}
`)
	mustWriteFile(t, filepath.Join(root, "test.deps"), "pytest>=8\n")
	source := scanPythonSource(t, root, "python-pyproject")
	assertPythonAnalysis(t, source, PresencePresent, ExtractionPartial)
	if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests>=2", "pytest>=8"}) {
		t.Fatalf("lost usable declarations: %v", got)
	}
	if !strings.Contains(source.Diagnostics[0].Message, "docs") {
		t.Fatalf("missing extra name: %+v", source.Diagnostics)
	}
}

func TestScanPyprojectDynamicFileContainment(t *testing.T) {
	for _, kind := range []string{"traversal", "absolute", "symlink file", "symlink directory", "directory"} {
		t.Run(kind, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "scan")
			outside := filepath.Join(parent, "outside.deps")
			mustWriteFile(t, outside, "outside-secret-package>=1\n")
			file := "../outside.deps"
			switch kind {
			case "absolute":
				file = outside
			case "symlink file":
				file = "linked.deps"
				if err := os.MkdirAll(root, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(root, file)); err != nil {
					t.Fatal(err)
				}
			case "symlink directory":
				file = "linked/outside.deps"
				if err := os.MkdirAll(root, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(parent, filepath.Join(root, "linked")); err != nil {
					t.Fatal(err)
				}
			case "directory":
				file = "dir.deps"
				if err := os.MkdirAll(filepath.Join(root, file), 0755); err != nil {
					t.Fatal(err)
				}
			}
			mustWriteFile(t, filepath.Join(root, "pyproject.toml"), "[project]\ndependencies = ['requests>=2']\ndynamic = ['optional-dependencies']\n[tool.setuptools.dynamic.optional-dependencies]\ntest = {file = '"+file+"'}\n")
			source := scanPythonSource(t, root, "python-pyproject")
			assertPythonAnalysis(t, source, PresencePresent, ExtractionPartial)
			if got := dependencyNames(source.Dependencies); !slices.Equal(got, []string{"requests>=2"}) {
				t.Fatalf("read an excluded file: %v", got)
			}
		})
	}
}

func scanPythonSource(t *testing.T, root string, detector DetectorID) DependencySourceResult {
	t.Helper()
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range result.Sources {
		if source.Detector == detector {
			return source
		}
	}
	t.Fatalf("missing detector %q: %+v", detector, result.Sources)
	return DependencySourceResult{}
}

func assertPythonAnalysis(t *testing.T, source DependencySourceResult, presence DependencyPresence, extraction ExtractionState) {
	t.Helper()
	want := SourceAnalysis{Presence: presence, Extraction: extraction}
	if source.Analysis != want {
		t.Fatalf("analysis = %+v, want %+v; source: %+v", source.Analysis, want, source)
	}
}
