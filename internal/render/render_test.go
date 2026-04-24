package render

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

func dependencyNames(dependencies []analyze.Dependency) []string {
	names := make([]string, 0, len(dependencies))
	for _, dependency := range dependencies {
		names = append(names, dependency.Name)
	}
	return names
}

func TestHumanIncludesDetectedPaths(t *testing.T) {
	hasDependencies := true
	noDependencies := false
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:            analyze.ManifestType("js"),
				Path:            "web/package.json",
				HasDependencies: &hasDependencies,
			},
			{
				Type:            analyze.ManifestType("python-requirements"),
				Path:            "api/requirements.txt",
				HasDependencies: &noDependencies,
			},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("python-requirements"), analyze.ManifestType("js")}, HumanOptions{ShowEmpty: true})
	expected := strings.Join([]string{
		"Root: /tmp/project",
		"",
		"Found 2 manifests:",
		"- 1 confirmed empty",
		"- 1 with dependencies present, not extracted",
		"",
		"api/requirements.txt [no dependencies]",
		"",
		"web/package.json [dependencies present, not extracted]",
		"",
	}, "\n")
	if output != expected {
		t.Fatalf("unexpected human output:\n%s", output)
	}
}

func TestHumanEmptyState(t *testing.T) {
	output := Human(analyze.ScanResult{Root: "/tmp/project"}, nil, HumanOptions{})
	if !strings.Contains(output, "No manifests found.") {
		t.Fatalf("expected empty state output, got %q", output)
	}
}

func TestHumanUsesPathFirstOrder(t *testing.T) {
	hasDependencies := true
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{Type: analyze.ManifestType("js"), Path: "web/package.json", HasDependencies: &hasDependencies},
			{Type: analyze.ManifestType("python-requirements"), Path: "api/requirements.txt", HasDependencies: &hasDependencies},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("js"), analyze.ManifestType("python-requirements")}, HumanOptions{})
	if strings.Index(output, "api/requirements.txt") > strings.Index(output, "web/package.json") {
		t.Fatalf("expected api/requirements.txt before web/package.json, got %q", output)
	}
}

func TestJSONMatchesExpectedSchema(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{Type: analyze.ManifestType("js-yarn"), Path: "frontend/yarn.lock"},
		},
	}

	output, err := JSON(result)
	if err != nil {
		t.Fatalf("json render failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if payload["root"] != "/tmp/project" {
		t.Fatalf("unexpected root value: %#v", payload["root"])
	}
	manifests, ok := payload["manifests"].([]any)
	if !ok || len(manifests) != 1 {
		t.Fatalf("unexpected manifests payload: %#v", payload["manifests"])
	}
	manifest, ok := manifests[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected manifest payload: %#v", manifests[0])
	}
	if value, ok := manifest["has_dependencies"]; !ok || value != nil {
		t.Fatalf("expected has_dependencies to be present as null, got %#v", manifest["has_dependencies"])
	}
}

func TestJSONDoesNotEscapeHTMLSensitiveCharacters(t *testing.T) {
	hasDependencies := true
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:            analyze.ManifestType("python-requirements"),
				Path:            "requirements.txt",
				HasDependencies: &hasDependencies,
				Dependencies: []analyze.Dependency{
					{Name: "requests>=2.31"},
				},
			},
		},
	}

	output, err := JSON(result)
	if err != nil {
		t.Fatalf("json render failed: %v", err)
	}

	if strings.Contains(string(output), `\u003e`) {
		t.Fatalf("expected JSON output to preserve '>', got %q", output)
	}
	if !strings.Contains(string(output), `"name": "requests>=2.31"`) {
		t.Fatalf("expected JSON output to contain literal dependency string, got %q", output)
	}
}

func TestHumanIncludesDependenciesWhenPresent(t *testing.T) {
	hasDependencies := true
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:            analyze.ManifestType("yaml-pip"),
				Path:            "workflow.yaml",
				HasDependencies: &hasDependencies,
				Dependencies:    []analyze.Dependency{{Name: "requests"}, {Name: "pendulum"}},
			},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("yaml-pip")}, HumanOptions{})
	expected := strings.Join([]string{
		"Root: /tmp/project",
		"",
		"Found 1 manifest:",
		"- 1 with extracted dependencies",
		"",
		"workflow.yaml [2 deps]",
		"  - requests",
		"  - pendulum",
		"",
	}, "\n")
	if output != expected {
		t.Fatalf("unexpected human output:\n%s", output)
	}
}

func TestHumanGroupsDependenciesBySectionWhenPresent(t *testing.T) {
	hasDependencies := true
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type: analyze.ManifestType("python-pyproject"),
				Path: "backend/pyproject.toml",
				Dependencies: []analyze.Dependency{
					{Name: "requests>=2.31", Section: "project.dependencies"},
					{Name: "pytest>=8", Section: "project.optional-dependencies.dev"},
					{Name: "ruff>=0.4", Section: "project.optional-dependencies.dev"},
				},
				HasDependencies: &hasDependencies,
			},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("python-pyproject")}, HumanOptions{})
	expected := strings.Join([]string{
		"Root: /tmp/project",
		"",
		"Found 1 manifest:",
		"- 1 with extracted dependencies",
		"",
		"backend/pyproject.toml [3 deps]",
		"  project.dependencies:",
		"    - requests>=2.31",
		"  project.optional-dependencies.dev:",
		"    - pytest>=8",
		"    - ruff>=0.4",
		"",
	}, "\n")
	if output != expected {
		t.Fatalf("unexpected human output:\n%s", output)
	}
}

func TestHumanUsesDefaultGroupOnlyForMixedDependencies(t *testing.T) {
	hasDependencies := true
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type: analyze.ManifestType("mixed"),
				Path: "mixed.toml",
				Dependencies: []analyze.Dependency{
					{Name: "build>=1.2"},
					{Name: "pytest>=8", Section: "tool.custom.dev"},
					{Name: "mkdocs>=1.6", Section: "tool.custom.docs"},
				},
				HasDependencies: &hasDependencies,
			},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("mixed")}, HumanOptions{})
	expected := strings.Join([]string{
		"Root: /tmp/project",
		"",
		"Found 1 manifest:",
		"- 1 with extracted dependencies",
		"",
		"mixed.toml [3 deps]",
		"  [default group]",
		"    - build>=1.2",
		"  tool.custom.dev:",
		"    - pytest>=8",
		"  tool.custom.docs:",
		"    - mkdocs>=1.6",
		"",
	}, "\n")
	if output != expected {
		t.Fatalf("unexpected human output:\n%s", output)
	}
}

func TestHumanSummarizesAllDependencyStates(t *testing.T) {
	hasDependencies := true
	noDependencies := false
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type: analyze.ManifestType("python-pyproject"),
				Path: "backend/pyproject.toml",
				Dependencies: []analyze.Dependency{
					{Name: "requests>=2.31", Section: "project.dependencies"},
				},
				HasDependencies: &hasDependencies,
			},
			{
				Type:            analyze.ManifestType("python-conda-environment"),
				Path:            "environment.yml",
				HasDependencies: &noDependencies,
			},
			{
				Type:            analyze.ManifestType("js"),
				Path:            "frontend/package.json",
				HasDependencies: &hasDependencies,
			},
			{
				Type: analyze.ManifestType("yaml"),
				Path: "unknown.yaml",
			},
		},
	}

	output := Human(result, []analyze.ManifestType{
		analyze.ManifestType("python-pyproject"),
		analyze.ManifestType("python-conda-environment"),
		analyze.ManifestType("js"),
		analyze.ManifestType("yaml"),
	}, HumanOptions{ShowEmpty: true})
	for _, expected := range []string{
		"Found 4 manifests:",
		"- 1 with extracted dependencies",
		"- 1 confirmed empty",
		"- 1 with dependencies present, not extracted",
		"- 1 with dependency status unknown",
		"backend/pyproject.toml [1 dep]",
		"environment.yml [no dependencies]",
		"frontend/package.json [dependencies present, not extracted]",
		"unknown.yaml [matched]",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestHumanHidesConfirmedEmptyManifestsByDefault(t *testing.T) {
	hasDependencies := true
	noDependencies := false
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:            analyze.ManifestType("python-conda-environment"),
				Path:            "environment.yml",
				HasDependencies: &noDependencies,
			},
			{
				Type:            analyze.ManifestType("js"),
				Path:            "frontend/package.json",
				HasDependencies: &hasDependencies,
			},
		},
	}

	output := Human(result, []analyze.ManifestType{
		analyze.ManifestType("python-conda-environment"),
		analyze.ManifestType("js"),
	}, HumanOptions{})

	if !strings.Contains(output, "- 1 confirmed empty") {
		t.Fatalf("expected summary to include confirmed empty count, got:\n%s", output)
	}
	if strings.Contains(output, "environment.yml [no dependencies]") {
		t.Fatalf("expected default human output to hide confirmed-empty manifests, got:\n%s", output)
	}
	if !strings.Contains(output, "frontend/package.json [dependencies present, not extracted]") {
		t.Fatalf("expected non-empty manifests to remain visible, got:\n%s", output)
	}
}

func TestJSONIncludesDependenciesWhenPresent(t *testing.T) {
	hasDependencies := true
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:            analyze.ManifestType("yaml-pip"),
				Path:            "workflow.yaml",
				Dependencies:    []analyze.Dependency{{Name: "requests"}, {Name: "pendulum"}},
				HasDependencies: &hasDependencies,
			},
		},
	}

	output, err := JSON(result)
	if err != nil {
		t.Fatalf("json render failed: %v", err)
	}

	var payload struct {
		Manifests []struct {
			Dependencies    []analyze.Dependency `json:"dependencies"`
			HasDependencies *bool                `json:"has_dependencies"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if len(payload.Manifests) != 1 || !slices.Equal(dependencyNames(payload.Manifests[0].Dependencies), []string{"requests", "pendulum"}) {
		t.Fatalf("unexpected dependencies payload: %+v", payload.Manifests)
	}
	if payload.Manifests[0].HasDependencies == nil || !*payload.Manifests[0].HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", payload.Manifests[0].HasDependencies)
	}
}

func TestJSONIncludesDependencySectionsWhenPresent(t *testing.T) {
	hasDependencies := true
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type: analyze.ManifestType("python-pyproject"),
				Path: "pyproject.toml",
				Dependencies: []analyze.Dependency{
					{Name: "requests>=2.31", Section: "project.dependencies"},
				},
				HasDependencies: &hasDependencies,
			},
		},
	}

	output, err := JSON(result)
	if err != nil {
		t.Fatalf("json render failed: %v", err)
	}

	var payload struct {
		Manifests []struct {
			Dependencies []analyze.Dependency `json:"dependencies"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if len(payload.Manifests) != 1 || len(payload.Manifests[0].Dependencies) != 1 {
		t.Fatalf("unexpected dependencies payload: %+v", payload.Manifests)
	}
	if payload.Manifests[0].Dependencies[0].Section != "project.dependencies" {
		t.Fatalf("expected dependency section to be preserved, got %+v", payload.Manifests[0].Dependencies[0])
	}
}

func TestJSONIncludesHasDependenciesFalseWhenKnownEmpty(t *testing.T) {
	hasDependencies := false
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:            analyze.ManifestType("python-conda-environment"),
				Path:            "environment.yml",
				HasDependencies: &hasDependencies,
			},
		},
	}

	output, err := JSON(result)
	if err != nil {
		t.Fatalf("json render failed: %v", err)
	}

	var payload struct {
		Manifests []struct {
			HasDependencies *bool `json:"has_dependencies"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if len(payload.Manifests) != 1 || payload.Manifests[0].HasDependencies == nil || *payload.Manifests[0].HasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", payload.Manifests)
	}
}

func TestHumanIncludesManifestWarnings(t *testing.T) {
	hasDependencies := true
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:            analyze.ManifestType("python-requirements"),
				Path:            "requirements.txt",
				HasDependencies: &hasDependencies,
				Dependencies:    []analyze.Dependency{{Name: "requests>=2.31"}},
				Warnings:        []string{`could not read included requirements file "missing.txt"`},
			},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("python-requirements")}, HumanOptions{})
	if !strings.Contains(output, `warning: could not read included requirements file "missing.txt"`) {
		t.Fatalf("expected human output to include warning, got %q", output)
	}
}

func TestJSONIncludesWarningsWhenPresent(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:            analyze.ManifestType("python-requirements"),
				Path:            "requirements.txt",
				Warnings:        []string{`could not read included requirements file "missing.txt"`},
				HasDependencies: nil,
			},
		},
	}

	output, err := JSON(result)
	if err != nil {
		t.Fatalf("json render failed: %v", err)
	}

	var payload struct {
		Manifests []struct {
			Warnings        []string `json:"warnings"`
			HasDependencies *bool    `json:"has_dependencies"`
		} `json:"manifests"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if len(payload.Manifests) != 1 || !slices.Equal(payload.Manifests[0].Warnings, []string{`could not read included requirements file "missing.txt"`}) {
		t.Fatalf("unexpected warnings payload: %+v", payload.Manifests)
	}
	if payload.Manifests[0].HasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", payload.Manifests[0].HasDependencies)
	}
}

func TestHumanIncludesExternalScriptURLs(t *testing.T) {
	hasDependencies := true
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:            analyze.ManifestType("html-external-scripts"),
				Path:            "templates/index.html",
				HasDependencies: &hasDependencies,
				Dependencies:    []analyze.Dependency{{Name: "https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js"}},
			},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("html-external-scripts")}, HumanOptions{})
	if !strings.Contains(output, "templates/index.html") || !strings.Contains(output, "https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js") {
		t.Fatalf("expected human output to include external script URL, got %q", output)
	}
}

func TestHumanSummaryPluralization(t *testing.T) {
	hasDependencies := true
	output := Human(analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type: analyze.ManifestType("python-pyproject"),
				Path: "pyproject.toml",
				Dependencies: []analyze.Dependency{
					{Name: "requests>=2.31"},
				},
				HasDependencies: &hasDependencies,
			},
		},
	}, []analyze.ManifestType{analyze.ManifestType("python-pyproject")}, HumanOptions{})

	if !strings.Contains(output, fmt.Sprintf("Found %d manifest:", 1)) {
		t.Fatalf("expected singular manifest count, got %q", output)
	}
	if !strings.Contains(output, "[1 dep]") {
		t.Fatalf("expected singular dependency count, got %q", output)
	}
}

func TestSampleMonorepoHumanGoldenOutput(t *testing.T) {
	ruleset := mustLoadRenderDefaultRules(t)
	result, err := analyze.Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), renderDefaultIgnoreDirs(), ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	result.Root = renderSampleMonorepoRoot

	output := Human(result, ruleset.SupportedManifestTypes(), HumanOptions{})
	want := mustReadRenderGolden(t, "sample_monorepo_human.golden")
	if output != want {
		t.Fatalf("unexpected human output:\n%s", output)
	}
}

func TestSampleMonorepoJSONGoldenOutput(t *testing.T) {
	ruleset := mustLoadRenderDefaultRules(t)
	result, err := analyze.Scan(filepath.Join("..", "..", "testdata", "sample-monorepo"), renderDefaultIgnoreDirs(), ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	result.Root = renderSampleMonorepoRoot

	output, err := JSON(result)
	if err != nil {
		t.Fatalf("JSON failed: %v", err)
	}
	want := mustReadRenderGolden(t, "sample_monorepo_json.golden")
	if string(output) != want {
		t.Fatalf("unexpected JSON output:\n%s", output)
	}
}

func mustLoadRenderDefaultRules(t *testing.T) analyze.Ruleset {
	t.Helper()

	ruleset, err := analyze.LoadDefaultRules()
	if err != nil {
		t.Fatalf("LoadDefaultRules failed: %v", err)
	}
	return ruleset
}

func mustReadRenderGolden(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s failed: %v", name, err)
	}
	return string(data)
}

const renderSampleMonorepoRoot = "/path/to/sample-monorepo"

func renderDefaultIgnoreDirs() []string {
	// Mirror CLI defaults so the sample-monorepo golden covers user-facing output.
	return []string{
		".git",
		"node_modules",
		".venv",
		"venv",
		"vendor",
		".tox",
		".mypy_cache",
		".pytest_cache",
	}
}
