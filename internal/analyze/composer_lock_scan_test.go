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
	if len(result.Manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(result.Manifests))
	}

	var composerLock *ManifestMatch
	for i := range result.Manifests {
		if result.Manifests[i].Type == ManifestType("php-composer-lock") {
			composerLock = &result.Manifests[i]
			break
		}
	}
	if composerLock == nil {
		t.Fatalf("expected php-composer-lock manifest, got %+v", result.Manifests)
	}
	if composerLock.HasDependencies == nil || !*composerLock.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", composerLock.HasDependencies)
	}

	got := dependencyNames(composerLock.Dependencies)
	want := []string{"monolog/monolog@3.6.0"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", got, want)
	}
}
