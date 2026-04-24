package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestScanYarnLockClassicExtractsDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "yarn-lock-v1-with-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	yarnLock := mustFindYarnLockManifest(t, result)
	if yarnLock.HasDependencies == nil || !*yarnLock.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", yarnLock.HasDependencies)
	}

	want := []Dependency{
		{Name: "left-pad@1.3.0"},
		{Name: "lodash@4.17.21"},
	}
	if !slices.Equal(yarnLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", yarnLock.Dependencies, want)
	}
}

func TestScanYarnLockModernExtractsDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "yarn-lock-modern-with-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	yarnLock := mustFindYarnLockManifest(t, result)
	if yarnLock.HasDependencies == nil || !*yarnLock.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", yarnLock.HasDependencies)
	}

	want := []Dependency{
		{Name: "@babel/code-frame@7.27.1"},
		{Name: "react@18.3.1"},
		{Name: "typescript@5.4.5"},
	}
	if !slices.Equal(yarnLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", yarnLock.Dependencies, want)
	}
}

func TestScanYarnLockModernMetadataOnlyIsConclusiveEmpty(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "yarn-lock-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	yarnLock := mustFindYarnLockManifest(t, result)
	if yarnLock.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", yarnLock.Dependencies)
	}
	if yarnLock.HasDependencies == nil || *yarnLock.HasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", yarnLock.HasDependencies)
	}
}

func mustFindYarnLockManifest(t *testing.T, result ScanResult) *ManifestMatch {
	t.Helper()

	for i := range result.Manifests {
		if result.Manifests[i].Type == ManifestType("js-yarn") && result.Manifests[i].Path == "yarn.lock" {
			return &result.Manifests[i]
		}
	}

	t.Fatalf("expected js-yarn manifest, got %+v", result.Manifests)
	return nil
}
