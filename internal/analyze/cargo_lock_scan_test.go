package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestScanCargoLockExtractsDependencies(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo", "rust-app"), nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Sources) != 2 {
		t.Fatalf("expected 2 dependency sources, got %d", len(result.Sources))
	}

	var cargoLock *DependencySourceResult
	for i := range result.Sources {
		if result.Sources[i].Detector == DetectorID("rust-cargo-lock") {
			cargoLock = &result.Sources[i]
			break
		}
	}
	if cargoLock == nil {
		t.Fatalf("expected rust-cargo-lock dependency source, got %+v", result.Sources)
	}
	if cargoLock.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", cargoLock.Analysis)
	}

	got := dependencyNames(cargoLock.Dependencies)
	want := []string{"serde@1.0.217"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", got, want)
	}
}
