package analyze

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type dependencyTypeTestParser struct {
	dependencies []Dependency
}

func (p dependencyTypeTestParser) Match(string, []byte) (manifestParserResult, error) {
	return manifestParserResult{
		Dependencies:    p.dependencies,
		HasDependencies: boolPtr(len(p.dependencies) > 0),
		Matched:         true,
	}, nil
}

func TestDependencyTypeFromRuleIsCopiedToDependencies(t *testing.T) {
	ruleset := Ruleset{
		rules: []manifestRule{
			{
				Type:           ManifestType("custom-python"),
				DependencyType: PackageType("pypi"),
				FilenameRegexp: regexp.MustCompile(`^requirements\.txt$`),
				Parser:         dependencyTypeTestParser{dependencies: []Dependency{{Raw: "requests>=2.31"}}},
			},
		},
	}
	path := filepath.Join(t.TempDir(), "requirements.txt")
	mustWriteFile(t, path, "requests>=2.31\n")

	_, dependencies, _, _, ok, err := ruleset.DetectManifestFile(path, "requirements.txt")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok || len(dependencies) != 1 {
		t.Fatalf("expected one dependency match, ok=%v deps=%+v", ok, dependencies)
	}
	if dependencies[0].Type != PackageType("pypi") {
		t.Fatalf("expected dependency type pypi, got %+v", dependencies[0])
	}
}

func TestDependencyTypeFromParserIsNotOverwrittenByRule(t *testing.T) {
	ruleset := Ruleset{
		rules: []manifestRule{
			{
				Type:           ManifestType("custom-mixed"),
				DependencyType: PackageType("generic"),
				FilenameRegexp: regexp.MustCompile(`^deps\.txt$`),
				Parser: dependencyTypeTestParser{dependencies: []Dependency{
					{Type: PackageType("npm"), Raw: "react@18.2.0"},
				}},
			},
		},
	}
	path := filepath.Join(t.TempDir(), "deps.txt")
	mustWriteFile(t, path, "react@18.2.0\n")

	_, dependencies, _, _, ok, err := ruleset.DetectManifestFile(path, "deps.txt")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok || len(dependencies) != 1 {
		t.Fatalf("expected one dependency match, ok=%v deps=%+v", ok, dependencies)
	}
	if dependencies[0].Type != PackageType("npm") {
		t.Fatalf("expected parser-supplied dependency type npm, got %+v", dependencies[0])
	}
}

func TestLoadRulesAcceptsKnownDependencyType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte(`
rules:
  - name: python-requirements
    dependency-type: pypi
    filename-regex: '^requirements\.txt$'
    py-requirements: {}
`))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
}

func TestLoadRulesWarnsButAcceptsUnknownDependencyType(t *testing.T) {
	var logOutput bytes.Buffer
	oldOutput := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&logOutput)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(oldOutput)
		log.SetFlags(oldFlags)
	})

	_, err := loadRules("test.yaml", []byte(`
rules:
  - name: future-ecosystem
    dependency-type: futurepkg
    filename-regex: '^deps\.txt$'
`))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	got := logOutput.String()
	if !strings.Contains(got, `unknown dependency-type "futurepkg"`) || !strings.Contains(got, `rule "future-ecosystem"`) {
		t.Fatalf("expected warning to mention rule and unknown type, got %q", got)
	}
}

func TestScanAddsPURLCompatibleDependencyTypesFromDefaultRules(t *testing.T) {
	root := t.TempDir()
	mustCopyFile(t, filepath.Join("..", "..", "testdata", "python", "requirements-static", "requirements.txt"), filepath.Join(root, "requirements.txt"))
	mustCopyFile(t, filepath.Join("..", "..", "testdata", "javascript", "package-lock-v3-with-deps", "package-lock.json"), filepath.Join(root, "package-lock.json"))

	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	pythonDeps := dependenciesForManifest(t, result, "requirements.txt")
	for _, dependency := range pythonDeps {
		if dependency.Type != PackageType("pypi") {
			t.Fatalf("expected Python dependency type pypi, got %+v", dependency)
		}
	}

	npmDeps := dependenciesForManifest(t, result, "package-lock.json")
	for _, dependency := range npmDeps {
		if dependency.Type != PackageType("npm") {
			t.Fatalf("expected npm dependency type npm, got %+v", dependency)
		}
	}
}

func mustCopyFile(t *testing.T, src string, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture %q: %v", src, err)
	}
	mustWriteFile(t, dst, string(data))
}

func dependenciesForManifest(t *testing.T, result ScanResult, path string) []Dependency {
	t.Helper()
	for _, manifest := range result.Manifests {
		if manifest.Path == path {
			if len(manifest.Dependencies) == 0 {
				t.Fatalf("manifest %s had no dependencies", path)
			}
			return manifest.Dependencies
		}
	}
	t.Fatalf("manifest %s not found in %+v", path, result.Manifests)
	return nil
}
