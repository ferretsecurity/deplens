package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestScanComposerLockExtractsDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo", "php-app"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 dependency sources, got %d", len(result.Sources))
	}

	var composerLock *DependencySourceResult
	for i := range result.Sources {
		if result.Sources[i].Detector == DetectorID("php-composer-lock") {
			composerLock = &result.Sources[i]
			break
		}
	}
	if composerLock == nil {
		t.Fatalf("expected php-composer-lock dependency source, got %+v", result.Sources)
	}
	if composerLock.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", composerLock.Analysis)
	}

	got := dependencyNames(composerLock.Dependencies)
	want := []string{"monolog/monolog@3.6.0"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", got, want)
	}
}
