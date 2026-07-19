package analyze

import (
	"path/filepath"
	"testing"
)

func TestScanYarnLockClassicExtractsDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "yarn-lock-v1-with-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	yarnLock := mustFindYarnLockSource(t, result)
	if yarnLock.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", yarnLock.Analysis)
	}

	want := []DependencyReference{
		{PackageType: PackageType("npm"), Raw: "left-pad@1.3.0", Name: "left-pad", Version: "1.3.0"},
		{PackageType: PackageType("npm"), Raw: "lodash@4.17.21", Name: "lodash", Version: "4.17.21"},
	}
	if !equalDependencies(yarnLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", yarnLock.Dependencies, want)
	}
}

func TestScanYarnLockModernExtractsDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "yarn-lock-modern-with-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	yarnLock := mustFindYarnLockSource(t, result)
	if yarnLock.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", yarnLock.Analysis)
	}

	want := []DependencyReference{
		{PackageType: PackageType("npm"), Raw: "@babel/code-frame@7.27.1", Name: "@babel/code-frame", Version: "7.27.1"},
		{PackageType: PackageType("npm"), Raw: "react@18.3.1", Name: "react", Version: "18.3.1"},
		{PackageType: PackageType("npm"), Raw: "typescript@5.4.5", Name: "typescript", Version: "5.4.5"},
	}
	if !equalDependencies(yarnLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", yarnLock.Dependencies, want)
	}
}

func TestScanYarnLockModernMetadataOnlyIsConclusiveEmpty(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "javascript", "yarn-lock-no-deps"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	yarnLock := mustFindYarnLockSource(t, result)
	if yarnLock.Dependencies != nil {
		t.Fatalf("expected no dependencies, got %+v", yarnLock.Dependencies)
	}
	if yarnLock.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", yarnLock.Analysis)
	}
}

func mustFindYarnLockSource(t *testing.T, result ScanResult) *DependencySourceResult {
	t.Helper()

	for i := range result.Sources {
		if result.Sources[i].Detector == DetectorID("js-yarn") && result.Sources[i].Path == "yarn.lock" {
			return &result.Sources[i]
		}
	}

	t.Fatalf("expected js-yarn dependency source, got %+v", result.Sources)
	return nil
}
