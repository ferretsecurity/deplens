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

func TestScanPNPMLockNormalizesEcosystemSyntax(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "pnpm-lock-ecosystem-syntax"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	pnpmLock := mustFindPNPMLockSource(t, result)
	if pnpmLock.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected analysis: %+v", pnpmLock.Analysis)
	}
	want := []DependencyReference{
		{PackageType: "npm", Raw: "alias@react@18.2.0", Name: "react", Version: "18.2.0", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "npm:react@^18", "declared_name": "alias"}},
		{PackageType: "npm", Raw: "local@link:packages/local", Name: "local", SourceGroup: "dependencies", OriginKind: OriginPath, Attributes: map[string]string{"protocol": "link", "path": "packages/local"}},
		{PackageType: "npm", Raw: "peer@1.0.0(typescript@5.2.2)", Name: "peer", Version: "1.0.0", SourceGroup: "dependencies", Attributes: map[string]string{"specifier": "^1.0.0", "peer_context": "(typescript@5.2.2)"}},
		{PackageType: "npm", Raw: "scoped-alias@@scope/real@2.3.4(react@18.2.0)", Name: "@scope/real", Version: "2.3.4", SourceGroup: "dependencies", Attributes: map[string]string{"declared_name": "scoped-alias", "peer_context": "(react@18.2.0)"}},
		{PackageType: "npm", Raw: "workspace-local@link:packages/workspace-local", Name: "workspace-local", SourceGroup: "dependencies", OriginKind: OriginWorkspace, Attributes: map[string]string{"specifier": "workspace:*", "protocol": "link", "path": "packages/workspace-local"}},
		{PackageType: "npm", Raw: "alias-again@react@18.2.0", Name: "react", Version: "18.2.0", SourceGroup: "devDependencies", Attributes: map[string]string{"declared_name": "alias-again"}},
		{PackageType: "npm", Raw: "peer@1.0.0(typescript@5.3.3)", Name: "peer", Version: "1.0.0", SourceGroup: "devDependencies", Attributes: map[string]string{"peer_context": "(typescript@5.3.3)"}},
		{PackageType: "npm", Raw: "local-file@file:../local.tgz", Name: "local-file", SourceGroup: "optionalDependencies", OriginKind: OriginPath, Attributes: map[string]string{"specifier": "file:../local.tgz", "protocol": "file", "path": "../local.tgz"}},
		{PackageType: "npm", Raw: "helper@1.2.3", Name: "helper", Version: "1.2.3"},
	}
	if !equalDependencies(pnpmLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", pnpmLock.Dependencies, want)
	}
	if len(pnpmLock.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %+v", pnpmLock.Diagnostics)
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
