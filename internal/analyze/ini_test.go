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

	result, err := parser.Match("setup.cfg", []byte("[options]\ninstall_requires = requests>=2.31, urllib3<3\n"))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
	if result.HasDependencies == nil || *result.HasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", result.HasDependencies)
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
	result, err := parser.Match("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "urllib3<3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
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
	result, err := parser.Match("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"pytest>=8", "ruff>=0.4", "sphinx>=7"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
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
	result, err := parser.Match("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "urllib3<3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
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
	result, err := parser.Match("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}
}

func TestINIParserReturnsNoMatchWithoutConfiguredKeys(t *testing.T) {
	parser, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{{Section: "options", Key: "install_requires"}},
	})
	if err != nil {
		t.Fatalf("newINIQueryParser failed: %v", err)
	}

	result, err := parser.Match("setup.cfg", []byte("[metadata]\nname = demo\n"))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if result.Matched {
		t.Fatalf("expected no match, got deps %+v", result.Dependencies)
	}
	if result.HasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", result.HasDependencies)
	}
}
