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
	if dep.Section != "" {
		t.Errorf("Section: expected empty for direct require, got %q", dep.Section)
	}
}

func TestGoModParserSetsSectionIndirect(t *testing.T) {
	matcher, _ := newGoModMatcher(goModMatcherConfig{})
	result, _ := matcher.Match("go.mod", []byte(`module example.com/app

go 1.25

require (
	github.com/google/uuid v1.6.0
	golang.org/x/text v0.25.0 // indirect
)
`))
	if len(result.Dependencies) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(result.Dependencies))
	}
	direct, indirect := result.Dependencies[0], result.Dependencies[1]
	if direct.Name != "github.com/google/uuid" || direct.Section != "" {
		t.Errorf("direct: got %+v", direct)
	}
	if indirect.Name != "golang.org/x/text" || indirect.Section != "indirect" {
		t.Errorf("indirect: got %+v", indirect)
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

func TestScanExtractsAllGoModRequirements(t *testing.T) {
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
		t.Fatalf("expected go.mod to report extracted dependencies, got %+v", manifest)
	}
	want := []Dependency{
		{Raw: "github.com/google/uuid", Name: "github.com/google/uuid", Version: "v1.6.0"},
		{Raw: "golang.org/x/text", Name: "golang.org/x/text", Version: "v0.25.0", Section: "indirect"},
	}
	if !equalDependencies(manifest.Dependencies, want) {
		t.Fatalf("unexpected go.mod dependencies: got %+v want %+v", manifest.Dependencies, want)
	}
}
