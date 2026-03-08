package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/japroc/deplens/internal/analyze"
)

func TestHumanIncludesDetectedPaths(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{Type: analyze.PackageJSON, Path: "web/package.json"},
			{Type: analyze.RequirementsTXT, Path: "api/requirements.txt"},
		},
	}

	output := Human(result)
	if !strings.Contains(output, "web/package.json") {
		t.Fatalf("expected human output to include package.json path, got %q", output)
	}
	if !strings.Contains(output, "api/requirements.txt") {
		t.Fatalf("expected human output to include requirements path, got %q", output)
	}
}

func TestHumanEmptyState(t *testing.T) {
	output := Human(analyze.ScanResult{Root: "/tmp/project"})
	if !strings.Contains(output, "No manifests found.") {
		t.Fatalf("expected empty state output, got %q", output)
	}
}

func TestJSONMatchesExpectedSchema(t *testing.T) {
	result := analyze.ScanResult{
		Root: "/tmp/project",
		Manifests: []analyze.ManifestMatch{
			{Type: analyze.YarnLock, Path: "frontend/yarn.lock"},
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
}
