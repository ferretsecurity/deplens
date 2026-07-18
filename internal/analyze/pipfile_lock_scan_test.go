package analyze

import (
	"path/filepath"
	"testing"
)

func TestScanPipfileLockExtractsDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo", "backend"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Sources) != 5 {
		t.Fatalf("expected 5 dependency sources, got %d", len(result.Sources))
	}

	var pipfileLock *DependencySourceResult
	for i := range result.Sources {
		if result.Sources[i].Detector == DetectorID("python-pipfile-lock") {
			pipfileLock = &result.Sources[i]
			break
		}
	}
	if pipfileLock == nil {
		t.Fatalf("expected python-pipfile-lock dependency source, got %+v", result.Sources)
	}
	if pipfileLock.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", pipfileLock.Analysis)
	}

	want := []DependencyReference{
		{PackageType: PackageType("pypi"), Raw: "requests==2.32.3", Name: "requests", Version: "2.32.3", SourceGroup: "default"},
		{PackageType: PackageType("pypi"), Raw: "pytest==8.3.3", Name: "pytest", Version: "8.3.3", SourceGroup: "develop"},
	}
	if !equalDependencies(pipfileLock.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", pipfileLock.Dependencies, want)
	}
}
