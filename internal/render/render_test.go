package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ferretsecurity/deplens/internal/analyze"
)

func TestHumanRendersDependencySourceStatesInPathOrder(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Sources: []analyze.DependencySourceResult{
			{
				Detector: "js",
				Path:     "web/package.json",
				Form:     analyze.FormManifest,
				Roles:    []analyze.SourceRole{analyze.RoleDeclaration},
				Analysis: analyze.SourceAnalysis{Presence: analyze.PresencePresent, Extraction: analyze.ExtractionUnsupported},
			},
			{
				Detector: "python-requirements",
				Path:     "api/requirements.txt",
				Form:     analyze.FormRequirements,
				Roles:    []analyze.SourceRole{analyze.RoleDeclaration, analyze.RoleConstraint},
				Analysis: analyze.SourceAnalysis{Presence: analyze.PresencePresent, Extraction: analyze.ExtractionComplete},
				Dependencies: []analyze.DependencyReference{
					{Raw: "requests>=2.31", Name: "requests", VersionConstraint: ">=2.31"},
				},
			},
			{
				Detector: "broken-lock",
				Path:     "broken.lock",
				Form:     analyze.FormLockfile,
				Roles:    []analyze.SourceRole{analyze.RoleResolution},
				Analysis: analyze.SourceAnalysis{Presence: analyze.PresenceUnknown, Extraction: analyze.ExtractionFailed},
				Diagnostics: []analyze.Diagnostic{
					{Severity: analyze.DiagnosticError, Code: "source-analysis-failed", Message: "invalid syntax"},
				},
			},
		},
	}

	output := Human(result, HumanOptions{})
	for _, expected := range []string{
		"Found 3 dependency sources:",
		"api/requirements.txt [requirements · 1 dependency]",
		"  - requests>=2.31",
		"broken.lock [lockfile · analysis failed]",
		"  error [source-analysis-failed]: invalid syntax",
		"web/package.json [manifest · references present, not extracted]",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
	if strings.Index(output, "api/requirements.txt") > strings.Index(output, "web/package.json") {
		t.Fatalf("expected path-first ordering, got:\n%s", output)
	}
}

func TestHumanEmptyState(t *testing.T) {
	output := Human(analyze.ScanResult{Root: "/tmp/project"}, HumanOptions{})
	if output != "Root: /tmp/project\nNo dependency sources found.\n" {
		t.Fatalf("unexpected empty state: %q", output)
	}
}

func TestHumanFiltersAbsentSourcesByDefault(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Sources: []analyze.DependencySourceResult{{
			Detector: "empty", Path: "empty.toml", Form: analyze.FormManifest,
			Roles:    []analyze.SourceRole{analyze.RoleDeclaration},
			Analysis: analyze.SourceAnalysis{Presence: analyze.PresenceAbsent, Extraction: analyze.ExtractionComplete},
		}},
	}
	if output := Human(result, HumanOptions{}); strings.Contains(output, "empty.toml [") {
		t.Fatalf("expected absent source to be hidden, got:\n%s", output)
	}
	output := Human(result, HumanOptions{ShowWithoutDependencies: true})
	if !strings.Contains(output, "empty.toml [manifest · no dependency references]") {
		t.Fatalf("expected absent source to be shown, got:\n%s", output)
	}
}

func TestHumanGroupsDependenciesBySourceGroup(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Sources: []analyze.DependencySourceResult{{
			Detector: "python-pyproject", Path: "pyproject.toml", Form: analyze.FormManifest,
			Roles:    []analyze.SourceRole{analyze.RoleDeclaration},
			Analysis: analyze.SourceAnalysis{Presence: analyze.PresencePresent, Extraction: analyze.ExtractionComplete},
			Dependencies: []analyze.DependencyReference{
				{Raw: "build>=1.2"},
				{Raw: "pytest>=8", SourceGroup: "project.optional-dependencies.dev"},
			},
		}},
	}
	output := Human(result, HumanOptions{})
	for _, expected := range []string{"  [default group]", "    - build>=1.2", "  project.optional-dependencies.dev:", "    - pytest>=8"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got:\n%s", expected, output)
		}
	}
}

func TestJSONUsesDependencySourceSchema(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Sources: []analyze.DependencySourceResult{{
			Detector: "js-yarn", Path: "yarn.lock", Form: analyze.FormLockfile,
			Roles:    []analyze.SourceRole{analyze.RoleResolution, analyze.RoleIntegrity},
			Analysis: analyze.SourceAnalysis{Presence: analyze.PresencePresent, Extraction: analyze.ExtractionComplete},
			Dependencies: []analyze.DependencyReference{{
				PackageType: "npm", Raw: "react@18.3.1", Name: "react", Version: "18.3.1",
				VersionConstraint: "^18", SourceGroup: "dependencies", OriginKind: "registry",
				Attributes: map[string]string{"specifier": "^18"},
			}},
		}},
	}
	output, err := JSON(result)
	if err != nil {
		t.Fatalf("JSON failed: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	sources, ok := payload["sources"].([]any)
	if !ok || len(sources) != 1 {
		t.Fatalf("unexpected sources: %#v", payload["sources"])
	}
	source := sources[0].(map[string]any)
	for _, key := range []string{"detector", "path", "form", "roles", "analysis", "dependencies"} {
		if _, exists := source[key]; !exists {
			t.Fatalf("missing source key %q in %#v", key, source)
		}
	}
	dependency := source["dependencies"].([]any)[0].(map[string]any)
	for _, key := range []string{"package_type", "raw", "name", "version", "version_constraint", "source_group", "origin_kind", "attributes"} {
		if _, exists := dependency[key]; !exists {
			t.Fatalf("missing dependency key %q in %#v", key, dependency)
		}
	}
	for _, removed := range []string{"manifests", "schema_version", "resolved_version"} {
		if _, exists := payload[removed]; exists {
			t.Fatalf("removed key %q is present in %#v", removed, payload)
		}
	}
	for _, removed := range []string{"type", "has_dependencies", "warnings"} {
		if _, exists := source[removed]; exists {
			t.Fatalf("removed source key %q is present in %#v", removed, source)
		}
	}
	for _, removed := range []string{"type", "constraint", "section", "source", "extras", "resolved_version"} {
		if _, exists := dependency[removed]; exists {
			t.Fatalf("removed dependency key %q is present in %#v", removed, dependency)
		}
	}
}

func TestJSONUsesEmptyArrayForNoSources(t *testing.T) {
	output, err := JSON(analyze.ScanResult{Root: "/tmp/project"})
	if err != nil {
		t.Fatalf("JSON failed: %v", err)
	}
	if !strings.Contains(string(output), `"sources": []`) {
		t.Fatalf("expected empty sources array, got %s", output)
	}
}

func TestJSONDoesNotEscapeHTMLSensitiveCharacters(t *testing.T) {
	result := analyze.ScanResult{Root: "/tmp/project", Sources: []analyze.DependencySourceResult{{
		Detector: "requirements", Path: "requirements.txt", Form: analyze.FormRequirements,
		Roles:        []analyze.SourceRole{analyze.RoleDeclaration},
		Analysis:     analyze.SourceAnalysis{Presence: analyze.PresencePresent, Extraction: analyze.ExtractionComplete},
		Dependencies: []analyze.DependencyReference{{Raw: "requests>=2.31"}},
	}}}
	output, err := JSON(result)
	if err != nil {
		t.Fatalf("JSON failed: %v", err)
	}
	if strings.Contains(string(output), `\u003e`) || !strings.Contains(string(output), `"raw": "requests>=2.31"`) {
		t.Fatalf("unexpected escaping: %s", output)
	}
}
