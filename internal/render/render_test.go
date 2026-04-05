package render

import (
	"encoding/json"
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
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{Type: analyze.ManifestType("js"), Path: "web/package.json"},
			{Type: analyze.ManifestType("python-requirements"), Path: "api/requirements.txt"},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("python-requirements"), analyze.ManifestType("js")})
	if !strings.Contains(output, "web/package.json") {
		t.Fatalf("expected human output to include package.json path, got %q", output)
	}
	if !strings.Contains(output, "api/requirements.txt") {
		t.Fatalf("expected human output to include requirements path, got %q", output)
	}
}

func TestHumanEmptyState(t *testing.T) {
	output := Human(analyze.ScanResult{Root: "/tmp/project"}, nil)
	if !strings.Contains(output, "No manifests found.") {
		t.Fatalf("expected empty state output, got %q", output)
	}
}

func TestHumanUsesProvidedManifestTypeOrder(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{Type: analyze.ManifestType("js"), Path: "web/package.json"},
			{Type: analyze.ManifestType("python-requirements"), Path: "api/requirements.txt"},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("js"), analyze.ManifestType("python-requirements")})
	if strings.Index(output, "js") > strings.Index(output, "python-requirements") {
		t.Fatalf("expected js section before python-requirements, got %q", output)
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

func TestHumanIncludesDependenciesWhenPresent(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:         analyze.ManifestType("yaml-pip"),
				Path:         "workflow.yaml",
				Dependencies: []analyze.Dependency{{Name: "requests"}, {Name: "pendulum"}},
			},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("yaml-pip")})
	if !strings.Contains(output, "workflow.yaml") {
		t.Fatalf("expected human output to include yaml manifest path, got %q", output)
	}
	if !strings.Contains(output, "requests") || !strings.Contains(output, "pendulum") {
		t.Fatalf("expected human output to include dependencies, got %q", output)
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

func TestHumanIncludesExternalScriptURLs(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{
				Type:         analyze.ManifestType("html-external-scripts"),
				Path:         "templates/index.html",
				Dependencies: []analyze.Dependency{{Name: "https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js"}},
			},
		},
	}

	output := Human(result, []analyze.ManifestType{analyze.ManifestType("html-external-scripts")})
	if !strings.Contains(output, "templates/index.html") || !strings.Contains(output, "https://cdn.jsdelivr.net/npm/dompurify@3.0.8/dist/purify.min.js") {
		t.Fatalf("expected human output to include external script URL, got %q", output)
	}
}
