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
	dependencies []DependencyReference
}

func (p dependencyTypeTestParser) Analyze(string, []byte) (sourceAnalyzerResult, error) {
	return sourceAnalyzerResult{
		Dependencies: p.dependencies,
		Analysis:     completeAnalysis(p.dependencies),
		Recognized:   true,
	}, nil
}

func TestDependencyTypeFromRuleIsCopiedToDependencies(t *testing.T) {
	ruleset := Ruleset{
		detectors: []detector{
			{
				ID:             DetectorID("custom-python"),
				PackageType:    PackageType("pypi"),
				Form:           FormRequirements,
				Roles:          []SourceRole{RoleDeclaration},
				FilenameRegexp: regexp.MustCompile(`^requirements\.txt$`),
				Analyzer:       dependencyTypeTestParser{dependencies: []DependencyReference{{Raw: "requests>=2.31"}}},
			},
		},
	}
	path := filepath.Join(t.TempDir(), "requirements.txt")
	mustWriteFile(t, path, "requests>=2.31\n")

	_, dependencies, _, _, ok, err := analyzeSourceParts(ruleset, path, "requirements.txt")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok || len(dependencies) != 1 {
		t.Fatalf("expected one dependency match, ok=%v deps=%+v", ok, dependencies)
	}
	if dependencies[0].PackageType != PackageType("pypi") {
		t.Fatalf("expected dependency type pypi, got %+v", dependencies[0])
	}
}

func TestDependencyTypeFromParserIsNotOverwrittenByRule(t *testing.T) {
	ruleset := Ruleset{
		detectors: []detector{
			{
				ID:             DetectorID("custom-mixed"),
				PackageType:    PackageType("generic"),
				Form:           FormOther,
				Roles:          []SourceRole{RoleInventory},
				FilenameRegexp: regexp.MustCompile(`^deps\.txt$`),
				Analyzer: dependencyTypeTestParser{dependencies: []DependencyReference{
					{PackageType: PackageType("npm"), Raw: "react@18.2.0"},
				}},
			},
		},
	}
	path := filepath.Join(t.TempDir(), "deps.txt")
	mustWriteFile(t, path, "react@18.2.0\n")

	_, dependencies, _, _, ok, err := analyzeSourceParts(ruleset, path, "deps.txt")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok || len(dependencies) != 1 {
		t.Fatalf("expected one dependency match, ok=%v deps=%+v", ok, dependencies)
	}
	if dependencies[0].PackageType != PackageType("npm") {
		t.Fatalf("expected parser-supplied dependency type npm, got %+v", dependencies[0])
	}
}

func TestLoadRulesAcceptsKnownDependencyType(t *testing.T) {
	_, err := loadRules("test.yaml", []byte("rules:\n    - id: python-requirements\n      package-type: pypi\n      form: requirements\n      roles: [declaration, constraint]\n      filename-regex: '^requirements\\.txt$'\n      analyzer:\n        type: py-requirements\n"))
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

	_, err := loadRules("test.yaml", []byte("rules:\n    - id: future-ecosystem\n      package-type: futurepkg\n      form: other\n      roles: [inventory]\n      filename-regex: '^deps\\.txt$'\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	got := logOutput.String()
	if !strings.Contains(got, `unknown package-type "futurepkg"`) || !strings.Contains(got, `rule "future-ecosystem"`) {
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

	pythonDeps := dependenciesForSource(t, result, "requirements.txt")
	for _, dependency := range pythonDeps {
		if dependency.PackageType != PackageType("pypi") {
			t.Fatalf("expected Python dependency type pypi, got %+v", dependency)
		}
	}

	npmDeps := dependenciesForSource(t, result, "package-lock.json")
	for _, dependency := range npmDeps {
		if dependency.PackageType != PackageType("npm") {
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

func dependenciesForSource(t *testing.T, result ScanResult, path string) []DependencyReference {
	t.Helper()
	for _, source := range result.Sources {
		if source.Path == path {
			if len(source.Dependencies) == 0 {
				t.Fatalf("dependency source %s had no dependencies", path)
			}
			return source.Dependencies
		}
	}
	t.Fatalf("dependency source %s not found in %+v", path, result.Sources)
	return nil
}
