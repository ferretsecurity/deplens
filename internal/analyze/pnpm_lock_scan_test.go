package analyze

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestScanPNPMLockDocumentStream(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, tc := range []struct {
		name    string
		content string
		want    []DependencyReference
	}{
		{
			name: "each document keeps its importer and package schema",
			content: `lockfileVersion: '9.0'
importers:
  .:
    dependencies:
      react: 18.3.1
dependencies:
  ignored: 1.0.0
packages:
  react@18.3.1: {}
  helper@1.0.0: {}
---
lockfileVersion: '5.4'
dependencies:
  vite: 6.0.0
packages:
  /helper/1.0.0: {}
  /legacy/2.0.0: {}
---
dependencies:
  unversioned: 1.0.0
`,
			want: []DependencyReference{
				{PackageType: "npm", Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies"},
				{PackageType: "npm", Raw: "helper@1.0.0", Name: "helper", Version: "1.0.0"},
				{PackageType: "npm", Raw: "vite@6.0.0", Name: "vite", Version: "6.0.0", SourceGroup: "dependencies"},
				{PackageType: "npm", Raw: "legacy@2.0.0", Name: "legacy", Version: "2.0.0"},
			},
		},
		{
			name: "recognizes a later document",
			content: `unrelated: true
---
lockfileVersion: '9.0'
dependencies:
  react: 18.3.1
`,
			want: []DependencyReference{
				{PackageType: "npm", Raw: "react@18.3.1", Name: "react", Version: "18.3.1", SourceGroup: "dependencies"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mustWriteFile(t, filepath.Join(root, "pnpm-lock.yaml"), tc.content)
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			source := mustFindPNPMLockSource(t, result)
			if source.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) || len(source.Diagnostics) != 0 {
				t.Fatalf("unexpected analysis: %+v", source)
			}
			if !equalDependencies(source.Dependencies, tc.want) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", source.Dependencies, tc.want)
			}
		})
	}
}

func TestScanPNPMLockMalformedLaterDocument(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, tail := range []string{"dependencies: [\n", "dependencies: [unexpected]\n"} {
		t.Run(tail, func(t *testing.T) {
			root := t.TempDir()
			mustWriteFile(t, filepath.Join(root, "pnpm-lock.yaml"), "lockfileVersion: '9.0'\ndependencies:\n  react: 18.3.1\n---\n"+tail)
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatal(err)
			}
			source := mustFindPNPMLockSource(t, result)
			if source.Analysis != (SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionFailed}) || len(source.Dependencies) != 0 {
				t.Fatalf("expected failed extraction without partial dependencies, got %+v", source)
			}
			if len(source.Diagnostics) != 1 || !strings.Contains(source.Diagnostics[0].Message, "parse yaml file") {
				t.Fatalf("expected YAML parse diagnostic, got %+v", source.Diagnostics)
			}
		})
	}
}

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
