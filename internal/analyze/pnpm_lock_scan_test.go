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

	var pnpmLock *DependencySourceResult
	for i := range result.Sources {
		if result.Sources[i].Detector == DetectorID("js-pnpm-lock") && result.Sources[i].Path == "pnpm-lock.yaml" {
			pnpmLock = &result.Sources[i]
			break
		}
	}
	if pnpmLock == nil {
		t.Fatalf("expected js-pnpm-lock dependency source, got %+v", result.Sources)
	}
	if pnpmLock.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", pnpmLock.Analysis)
	}

	want := []DependencyReference{
		{PackageType: PackageType("npm"), Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "^18.3.1"}},
		{PackageType: PackageType("npm"), Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", SourceGroup: "devDependencies", Attributes: map[string]string{"specifier": "^20.12.7"}},
	}
	if !equalDependencies(pnpmLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", pnpmLock.Dependencies, want)
	}
}

func TestScanPNPMLockWithTransitiveInPackagesFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "pnpm-lock-with-transitive"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	pnpmLock := mustFindPNPMLockSource(t, result)
	if pnpmLock.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", pnpmLock.Analysis)
	}
	want := []DependencyReference{
		{PackageType: PackageType("npm"), Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "^18.3.1"}},
		{PackageType: PackageType("npm"), Raw: "loose-envify@1.4.0", Name: "loose-envify", Version: "1.4.0"},
	}
	if !equalDependencies(pnpmLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", pnpmLock.Dependencies, want)
	}
}

func TestScanPNPMLockV6PackageKeys(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "pnpm-lock-v6-package-keys"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatal(err)
	}
	source := mustFindPNPMLockSource(t, result)
	if source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(source.Diagnostics) != 0 {
		t.Fatalf("unexpected analysis: %+v", source)
	}
	want := []DependencyReference{
		{PackageType: "npm", Raw: "react@18.2.0", Name: "react", Version: "18.2.0", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "^18.2.0"}},
		{PackageType: "npm", Raw: "@docsearch/react@3.5.2", Name: "@docsearch/react", Version: "3.5.2"},
		{PackageType: "npm", Raw: "loose-envify@1.4.0", Name: "loose-envify", Version: "1.4.0"},
		{PackageType: "npm", Raw: "react-dom@18.2.0", Name: "react-dom", Version: "18.2.0"},
	}
	if !equalDependencies(source.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, want)
	}
}

func TestScanPNPMLockExtractsTopLevelDependenciesForOlderLocks(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "pnpm-lock-v5-top-level"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	pnpmLock := mustFindPNPMLockSource(t, result)
	if pnpmLock.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", pnpmLock.Analysis)
	}

	want := []DependencyReference{
		{PackageType: PackageType("npm"), Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies"},
		{PackageType: PackageType("npm"), Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", SourceGroup: "devDependencies"},
		{PackageType: PackageType("npm"), Raw: "fsevents@2.3.3", Name: "fsevents", Version: "2.3.3", SourceGroup: "optionalDependencies"},
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

	pnpmLock := mustFindPNPMLockSource(t, result)
	if pnpmLock.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", pnpmLock.Analysis)
	}

	want := []DependencyReference{
		{PackageType: PackageType("npm"), Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies"},
		{PackageType: PackageType("npm"), Raw: "@types/node@20.12.7", Name: "@types/node", Version: "20.12.7", SourceGroup: "devDependencies"},
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

	pnpmLock := mustFindPNPMLockSource(t, result)
	if pnpmLock.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", pnpmLock.Dependencies)
	}
	if pnpmLock.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", pnpmLock.Analysis)
	}
}

func mustFindPNPMLockSource(t *testing.T, result ScanResult) *DependencySourceResult {
	t.Helper()

	for i := range result.Sources {
		if result.Sources[i].Detector == DetectorID("js-pnpm-lock") && result.Sources[i].Path == "pnpm-lock.yaml" {
			return &result.Sources[i]
		}
	}

	t.Fatalf("expected js-pnpm-lock dependency source, got %+v", result.Sources)
	return nil
}
