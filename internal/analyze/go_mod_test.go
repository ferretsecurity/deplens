package analyze

import (
	"path/filepath"
	"testing"
)

func TestGoModParserSetsStructuredFields(t *testing.T) {
	matcher, _ := newGoModMatcher(goModMatcherConfig{})
	result, _ := matcher.Match("go.mod", []byte(`module example.com/app

go 1.21

require github.com/BurntSushi/toml v0.3.1
`))
	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result.Dependencies))
	}
	dep := result.Dependencies[0]
	if dep.Raw != "github.com/BurntSushi/toml" {
		t.Errorf("Raw: got %q", dep.Raw)
	}
	if dep.Name != "github.com/BurntSushi/toml" {
		t.Errorf("Name: got %q", dep.Name)
	}
	if dep.Version != "v0.3.1" {
		t.Errorf("Version: got %q", dep.Version)
	}
}

func TestScanExtractsGoModDependenciesFromFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo", "go-service"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, manifest := range result.Manifests {
		if manifest.Path != "go.mod" {
			continue
		}
		if manifest.Type != ManifestType("go-mod") {
			t.Fatalf("expected go.mod manifest type %q, got %q", ManifestType("go-mod"), manifest.Type)
		}
		if manifest.HasDependencies == nil || !*manifest.HasDependencies {
			t.Fatalf("expected go.mod to report extracted dependencies, got %+v", manifest)
		}
		if got := dependencyNames(manifest.Dependencies); len(got) != 1 || got[0] != "github.com/stretchr/testify" {
			t.Fatalf("unexpected go.mod dependencies: %+v", manifest.Dependencies)
		}
		return
	}

	t.Fatalf("expected go.mod manifest in result, got %+v", result.Manifests)
}

func TestScanExtractsGoModDependenciesFromDedicatedFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "go", "mod-with-deps")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %+v", result.Manifests)
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("go-mod") {
		t.Fatalf("expected go.mod manifest type %q, got %q", ManifestType("go-mod"), manifest.Type)
	}
	if manifest.HasDependencies == nil || !*manifest.HasDependencies {
		t.Fatalf("expected go.mod to report extracted dependencies, got %+v", manifest)
	}
	if got := dependencyNames(manifest.Dependencies); len(got) != 2 || got[0] != "github.com/google/uuid" || got[1] != "github.com/spf13/cobra" {
		t.Fatalf("unexpected go.mod dependencies: %+v", manifest.Dependencies)
	}
}

func TestScanMarksGoModWithoutRequireAsEmpty(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "go", "mod-no-require")
	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %+v", result.Manifests)
	}

	manifest := result.Manifests[0]
	if manifest.Type != ManifestType("go-mod") {
		t.Fatalf("expected go.mod manifest type %q, got %q", ManifestType("go-mod"), manifest.Type)
	}
	if manifest.HasDependencies == nil || *manifest.HasDependencies {
		t.Fatalf("expected go.mod without require directives to be empty, got %+v", manifest)
	}
	if len(manifest.Dependencies) != 0 {
		t.Fatalf("expected no extracted dependencies, got %+v", manifest.Dependencies)
	}
}

func TestScanExtractsOnlyDirectGoModRequirements(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "go", "mod-direct-only")
	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %+v", result.Manifests)
	}

	manifest := result.Manifests[0]
	if manifest.HasDependencies == nil || !*manifest.HasDependencies {
		t.Fatalf("expected go.mod to report extracted direct dependencies, got %+v", manifest)
	}
	if got := dependencyNames(manifest.Dependencies); len(got) != 1 || got[0] != "github.com/google/uuid" {
		t.Fatalf("unexpected go.mod dependencies: %+v", manifest.Dependencies)
	}
}
