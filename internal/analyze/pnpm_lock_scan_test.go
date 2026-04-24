package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestScanPNPMLockExtractsDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo", "frontend"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var pnpmLock *ManifestMatch
	for i := range result.Manifests {
		if result.Manifests[i].Type == ManifestType("js-pnpm-lock") && result.Manifests[i].Path == "pnpm-lock.yaml" {
			pnpmLock = &result.Manifests[i]
			break
		}
	}
	if pnpmLock == nil {
		t.Fatalf("expected js-pnpm-lock manifest, got %+v", result.Manifests)
	}
	if pnpmLock.HasDependencies == nil || !*pnpmLock.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", pnpmLock.HasDependencies)
	}

	want := []Dependency{
		{Name: "react@18.3.1", Section: "dependencies"},
		{Name: "@types/node@20.12.7", Section: "devDependencies"},
	}
	if !slices.Equal(pnpmLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", pnpmLock.Dependencies, want)
	}
}
