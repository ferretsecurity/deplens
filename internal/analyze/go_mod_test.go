package analyze

import (
	"path/filepath"
	"testing"
)

func TestGoModParserSetsStructuredFields(t *testing.T) {
	matcher, _ := newGoModMatcher(goModMatcherConfig{})
	result, _ := matcher.Analyze("go.mod", []byte(`module example.com/app

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
	if dep.SourceGroup != "" {
		t.Errorf("SourceGroup: expected empty for direct require, got %q", dep.SourceGroup)
	}
}

func TestGoModParserSetsSectionIndirect(t *testing.T) {
	matcher, _ := newGoModMatcher(goModMatcherConfig{})
	result, _ := matcher.Analyze("go.mod", []byte(`module example.com/app

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
	if direct.Name != "github.com/google/uuid" || direct.SourceGroup != "" {
		t.Errorf("direct: got %+v", direct)
	}
	if indirect.Name != "golang.org/x/text" || indirect.SourceGroup != "indirect" {
		t.Errorf("indirect: got %+v", indirect)
	}
}

func TestScanExtractsGoModDependenciesFromFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	result, err := Scan(filepath.Join("..", "..", "testdata", "sample-monorepo", "go-service"), nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	for _, source := range result.Sources {
		if source.Path != "go.mod" {
			continue
		}
		if source.Detector != DetectorID("go-mod") {
			t.Fatalf("expected go.mod dependency source type %q, got %q", DetectorID("go-mod"), source.Detector)
		}
		if source.Analysis.Presence != PresencePresent {
			t.Fatalf("expected go.mod to report extracted dependencies, got %+v", source)
		}
		if got := dependencyNames(source.Dependencies); len(got) != 1 || got[0] != "github.com/stretchr/testify" {
			t.Fatalf("unexpected go.mod dependencies: %+v", source.Dependencies)
		}
		return
	}

	t.Fatalf("expected go.mod dependency source in result, got %+v", result.Sources)
}

func TestScanExtractsGoModDependenciesFromDedicatedFixture(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "go", "mod-with-deps")

	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("go-mod") {
		t.Fatalf("expected go.mod dependency source type %q, got %q", DetectorID("go-mod"), source.Detector)
	}
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected go.mod to report extracted dependencies, got %+v", source)
	}
	if got := dependencyNames(source.Dependencies); len(got) != 2 || got[0] != "github.com/google/uuid" || got[1] != "github.com/spf13/cobra" {
		t.Fatalf("unexpected go.mod dependencies: %+v", source.Dependencies)
	}
}

func TestScanMarksGoModWithoutRequireAsEmpty(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "go", "mod-no-require")
	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Detector != DetectorID("go-mod") {
		t.Fatalf("expected go.mod dependency source type %q, got %q", DetectorID("go-mod"), source.Detector)
	}
	if source.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected go.mod without require directives to be empty, got %+v", source)
	}
	if len(source.Dependencies) != 0 {
		t.Fatalf("expected no extracted dependencies, got %+v", source.Dependencies)
	}
}

func TestScanExtractsAllGoModRequirements(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	root := filepath.Join("..", "..", "testdata", "go", "mod-direct-only")
	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 dependency source, got %+v", result.Sources)
	}

	source := result.Sources[0]
	if source.Analysis.Presence != PresencePresent {
		t.Fatalf("expected go.mod to report extracted dependencies, got %+v", source)
	}
	want := []DependencyReference{
		{PackageType: PackageType("golang"), Raw: "github.com/google/uuid", Name: "github.com/google/uuid", Version: "v1.6.0"},
		{PackageType: PackageType("golang"), Raw: "golang.org/x/text", Name: "golang.org/x/text", Version: "v0.25.0", SourceGroup: "indirect"},
	}
	if !equalDependencies(source.Dependencies, want) {
		t.Fatalf("unexpected go.mod dependencies: got %+v want %+v", source.Dependencies, want)
	}
}
