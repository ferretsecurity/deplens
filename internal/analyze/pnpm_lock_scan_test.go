package analyze

import (
	"path/filepath"
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
		{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", Section: "dependencies", Extras: map[string]string{"specifier": "^18.3.1"}},
		{Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", Section: "devDependencies", Extras: map[string]string{"specifier": "^20.12.7"}},
	}
	if !equalDependencies(pnpmLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", pnpmLock.Dependencies, want)
	}
}

func TestScanPNPMLockExtractsTopLevelDependenciesForOlderLocks(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "pnpm-lock-v5-top-level"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	pnpmLock := mustFindPNPMLockManifest(t, result)
	if pnpmLock.HasDependencies == nil || !*pnpmLock.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", pnpmLock.HasDependencies)
	}

	want := []Dependency{
		{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", Section: "dependencies"},
		{Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", Section: "devDependencies"},
		{Raw: "fsevents@2.3.3", Name: "fsevents", Version: "2.3.3", Section: "optionalDependencies"},
	}
	if !equalDependencies(pnpmLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", pnpmLock.Dependencies, want)
	}
}

func TestScanPNPMLockWorkspaceExtractsOnlyRootImporterDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "pnpm-lock-workspace-root-only"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	pnpmLock := mustFindPNPMLockManifest(t, result)
	if pnpmLock.HasDependencies == nil || !*pnpmLock.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", pnpmLock.HasDependencies)
	}

	want := []Dependency{
		{Raw: "react@18.3.1", Name: "react", Version: "18.3.1", Section: "dependencies"},
		{Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", Section: "devDependencies"},
	}
	if !equalDependencies(pnpmLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", pnpmLock.Dependencies, want)
	}
}

func TestScanPNPMLockWorkspaceWithoutRootDependenciesIsConclusiveEmpty(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "pnpm-lock-workspace-empty-root"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	pnpmLock := mustFindPNPMLockManifest(t, result)
	if pnpmLock.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", pnpmLock.Dependencies)
	}
	if pnpmLock.HasDependencies == nil || *pnpmLock.HasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", pnpmLock.HasDependencies)
	}
}

func mustFindPNPMLockManifest(t *testing.T, result ScanResult) *ManifestMatch {
	t.Helper()

	for i := range result.Manifests {
		if result.Manifests[i].Type == ManifestType("js-pnpm-lock") && result.Manifests[i].Path == "pnpm-lock.yaml" {
			return &result.Manifests[i]
		}
	}

	t.Fatalf("expected js-pnpm-lock manifest, got %+v", result.Manifests)
	return nil
}
