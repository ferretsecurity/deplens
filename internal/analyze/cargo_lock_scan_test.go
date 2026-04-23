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
	if len(result.Manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(result.Manifests))
	}

	var cargoLock *ManifestMatch
	for i := range result.Manifests {
		if result.Manifests[i].Type == ManifestType("rust-cargo-lock") {
			cargoLock = &result.Manifests[i]
			break
		}
	}
	if cargoLock == nil {
		t.Fatalf("expected rust-cargo-lock manifest, got %+v", result.Manifests)
	}
	if cargoLock.HasDependencies == nil || !*cargoLock.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", cargoLock.HasDependencies)
	}

	got := dependencyNames(cargoLock.Dependencies)
	want := []string{"serde@1.0.217"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", got, want)
	}
}
