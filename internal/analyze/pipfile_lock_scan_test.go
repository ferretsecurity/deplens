package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestScanPipfileLockExtractsDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo", "backend"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Manifests) != 5 {
		t.Fatalf("expected 5 manifests, got %d", len(result.Manifests))
	}

	var pipfileLock *ManifestMatch
	for i := range result.Manifests {
		if result.Manifests[i].Type == ManifestType("python-pipfile-lock") {
			pipfileLock = &result.Manifests[i]
			break
		}
	}
	if pipfileLock == nil {
		t.Fatalf("expected python-pipfile-lock manifest, got %+v", result.Manifests)
	}
	if pipfileLock.HasDependencies == nil || !*pipfileLock.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", pipfileLock.HasDependencies)
	}

	want := []Dependency{
		{Name: "requests==2.32.3", Section: "default"},
		{Name: "pytest==8.3.3", Section: "develop"},
	}
	if !slices.Equal(pipfileLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", pipfileLock.Dependencies, want)
	}
}
