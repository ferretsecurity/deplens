package analyze

import (
	"slices"
	"testing"
)

func TestINIParserRejectsPartialWildcardKey(t *testing.T) {
	_, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{{Section: "options", Key: "install_*"}},
	})
	if err == nil {
		t.Fatalf("expected partial wildcard key to be rejected")
	}
}

func TestINIParserMatchesOnPresenceWithoutDependencies(t *testing.T) {
	parser, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{{Section: "options", Key: "install_requires"}},
	})
	if err != nil {
		t.Fatalf("newINIQueryParser failed: %v", err)
	}

	deps, hasDependencies, ok, err := parser.Match("setup.cfg", []byte("[options]\ninstall_requires = requests>=2.31, urllib3<3\n"))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if len(deps) != 0 {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if hasDependencies == nil || *hasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", hasDependencies)
	}
}

func TestINIParserExtractsMultilineDependencies(t *testing.T) {
	parser, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{{Section: "options", Key: "install_requires"}},
	})
	if err != nil {
		t.Fatalf("newINIQueryParser failed: %v", err)
	}

	content := []byte("[options]\ninstall_requires =\n    requests>=2.31\n    urllib3<3\n")
	deps, hasDependencies, ok, err := parser.Match("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "urllib3<3"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestINIParserWildcardExtrasExtractsAcrossKeys(t *testing.T) {
	parser, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{{Section: "options.extras_require", Key: "*"}},
	})
	if err != nil {
		t.Fatalf("newINIQueryParser failed: %v", err)
	}

	content := []byte("[options.extras_require]\ndev =\n    pytest>=8\n    ruff>=0.4\ndocs =\n    sphinx>=7\n")
	deps, hasDependencies, ok, err := parser.Match("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"pytest>=8", "ruff>=0.4", "sphinx>=7"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestINIParserStripsCommentsAndBlankLines(t *testing.T) {
	parser, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{{Section: "options", Key: "install_requires"}},
	})
	if err != nil {
		t.Fatalf("newINIQueryParser failed: %v", err)
	}

	content := []byte("[options]\ninstall_requires =\n    requests>=2.31  # runtime client\n\n    ; comment only\n    urllib3<3\n")
	deps, hasDependencies, ok, err := parser.Match("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "urllib3<3"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestINIParserSkipsUnsupportedEntriesButKeepsMatch(t *testing.T) {
	parser, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{{Section: "options", Key: "install_requires"}},
	})
	if err != nil {
		t.Fatalf("newINIQueryParser failed: %v", err)
	}

	content := []byte("[options]\ninstall_requires =\n    file: requirements.txt\n    %(base_deps)s\n    requests>=2.31\n")
	deps, hasDependencies, ok, err := parser.Match("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestINIParserReturnsNoMatchWithoutConfiguredKeys(t *testing.T) {
	parser, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{{Section: "options", Key: "install_requires"}},
	})
	if err != nil {
		t.Fatalf("newINIQueryParser failed: %v", err)
	}

	deps, hasDependencies, ok, err := parser.Match("setup.cfg", []byte("[metadata]\nname = demo\n"))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if ok {
		t.Fatalf("expected no match, got deps %+v", deps)
	}
	if hasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", hasDependencies)
	}
}
