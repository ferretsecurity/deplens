package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestPythonPyprojectAnalyzeDynamicFiles(t *testing.T) {
	for _, test := range []struct {
		name       string
		file       string
		content    string
		want       []string
		presence   DependencyPresence
		extraction ExtractionState
	}{
		{"local", "runtime.deps", "requests>=2\n", []string{"requests>=2"}, PresencePresent, ExtractionComplete},
		{"empty", "runtime.deps", "# no dependencies\n", nil, PresenceAbsent, ExtractionComplete},
		{"missing", "missing.deps", "requests>=2\n", nil, PresenceUnknown, ExtractionFailed},
		{"outside manifest directory", "../outside.deps", "requests>=2\n", nil, PresenceUnknown, ExtractionFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			manifest := filepath.Join(root, "app", "pyproject.toml")
			mustWriteFile(t, filepath.Join(root, "app", "runtime.deps"), test.content)
			mustWriteFile(t, filepath.Join(root, "outside.deps"), "outside-secret-package>=1\n")
			parser, err := newPythonPyprojectParser(pythonPyprojectParserConfig{})
			if err != nil {
				t.Fatal(err)
			}
			result, err := parser.Analyze(manifest, []byte("[project]\ndynamic = ['dependencies']\n[tool.setuptools.dynamic]\ndependencies = {file = '"+test.file+"'}\n"))
			if err != nil {
				t.Fatal(err)
			}
			if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: test.presence, Extraction: test.extraction}) {
				t.Fatalf("unexpected analysis: %+v", result)
			}
			if got := dependencyNames(result.Dependencies); !slices.Equal(got, test.want) {
				t.Fatalf("dependencies = %v, want %v", got, test.want)
			}
			if err := validateSourceAnalyzerResult(result); err != nil {
				t.Fatalf("invalid result: %v", err)
			}
		})
	}
}

func TestPythonPyprojectIgnoresUnrelatedDynamicMetadata(t *testing.T) {
	parser, err := newPythonPyprojectParser(pythonPyprojectParserConfig{})
	if err != nil {
		t.Fatal(err)
	}
	result, err := parser.Analyze("pyproject.toml", []byte("[project]\nname = 'example'\ndynamic = ['version']\n"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Recognized {
		t.Fatalf("unrelated metadata became a dependency declaration: %+v", result)
	}
}

func TestPythonPyprojectMalformedDynamic(t *testing.T) {
	for _, declaration := range []string{
		"[project]\ndynamic = 'dependencies'\n",
		"[project]\ndynamic = [42]\n",
		"[project]\ndynamic = ['dependencies']\ndependencies = ['requests>=2']\n[tool.setuptools.dynamic]\ndependencies = {file = 'runtime.deps'}\n",
	} {
		t.Run(declaration, func(t *testing.T) {
			root := t.TempDir()
			mustWriteFile(t, filepath.Join(root, "runtime.deps"), "pytest>=8\n")
			parser, err := newPythonPyprojectParser(pythonPyprojectParserConfig{})
			if err != nil {
				t.Fatal(err)
			}
			result, err := parser.Analyze(filepath.Join(root, "pyproject.toml"), []byte("[build-system]\nrequires = ['setuptools>=68']\n"+declaration))
			if err != nil {
				t.Fatal(err)
			}
			if !result.Recognized || result.Analysis.Extraction != ExtractionPartial || len(result.Diagnostics) == 0 || result.Dependencies[0].Raw != "setuptools>=68" {
				t.Fatalf("malformed dynamic declaration reported complete or lost static references: %+v", result)
			}
		})
	}
}
