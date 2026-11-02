package analyze

import (
	"maps"
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

	result, err := parser.Analyze("setup.cfg", []byte("[options]\ninstall_requires = requests>=2.31, urllib3<3\n"))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
	if result.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Analysis)
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
	result, err := parser.Analyze("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "urllib3<3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
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
	result, err := parser.Analyze("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{"pytest>=8", "ruff>=0.4", "sphinx>=7"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
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
	result, err := parser.Analyze("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31", "urllib3<3"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
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
	result, err := parser.Analyze("setup.cfg", content)
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected match")
	}
	if want := []string{"requests>=2.31"}; !slices.Equal(dependencyNames(result.Dependencies), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
	}
}

func TestINIParserReturnsNoMatchWithoutConfiguredKeys(t *testing.T) {
	parser, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{{Section: "options", Key: "install_requires"}},
	})
	if err != nil {
		t.Fatalf("newINIQueryParser failed: %v", err)
	}

	result, err := parser.Analyze("setup.cfg", []byte("[metadata]\nname = demo\n"))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if result.Recognized {
		t.Fatalf("expected no match, got deps %+v", result.Dependencies)
	}
	if result.Analysis.Presence != "" && result.Analysis.Presence != PresenceUnknown {
		t.Fatalf("expected unknown presence, got %+v", result.Analysis)
	}
}

func TestINIParserExtractsSetupCfgRequirementSyntax(t *testing.T) {
	parser, err := newINIQueryParser(iniMatcherConfig{
		Queries: []iniQueryConfig{
			{Section: "options", Key: "install_requires"},
			{Section: "options", Key: "setup_requires"},
			{Section: "options.extras_require", Key: "*"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		content string
		want    []DependencyReference
	}{
		{
			name:    "inline install setup and extras",
			content: "[options]\ninstall_requires = requests>=2.31,<3\nsetup_requires = setuptools_scm>=8; wheel\n[options.extras_require]\ndev = pytest>=8; ruff>=0.4\ndocs = sphinx[docs]>=7\n",
			want: []DependencyReference{
				{Raw: "requests>=2.31,<3", Name: "requests", VersionConstraint: ">=2.31,<3"},
				{Raw: "setuptools_scm>=8", Name: "setuptools_scm", VersionConstraint: ">=8"},
				{Raw: "wheel", Name: "wheel"},
				{Raw: "pytest>=8", Name: "pytest", VersionConstraint: ">=8"},
				{Raw: "ruff>=0.4", Name: "ruff", VersionConstraint: ">=0.4"},
				{Raw: "sphinx[docs]>=7", Name: "sphinx", VersionConstraint: ">=7", Attributes: map[string]string{"extras": "docs"}},
			},
		},
		{
			name:    "multiline markers extras comments and constraints",
			content: "[options]\ninstall_requires =\n    requests[security, socks]>=2.31,<3 ; python_version < '3.13'  # runtime client\n\n# comment between values\n    ; comment only\n    urllib3<3; python_version >= '3.9'\n[options.extras_require]\ndev =\n    pytest>=8 ; sys_platform != 'win32'\n",
			want: []DependencyReference{
				{Raw: "requests[security, socks]>=2.31,<3 ; python_version < '3.13'", Name: "requests", VersionConstraint: ">=2.31,<3", Attributes: map[string]string{"extras": "security,socks", "marker": "python_version < '3.13'"}},
				{Raw: "urllib3<3; python_version >= '3.9'", Name: "urllib3", VersionConstraint: "<3", Attributes: map[string]string{"marker": "python_version >= '3.9'"}},
				{Raw: "pytest>=8 ; sys_platform != 'win32'", Name: "pytest", VersionConstraint: ">=8", Attributes: map[string]string{"marker": "sys_platform != 'win32'"}},
			},
		},
		{
			name:    "inline initial value with continuations",
			content: "[options]\ninstall_requires = requests>=2.31 ; python_version < '3.13'\n    urllib3<3\nsetup_requires = wheel\n    setuptools_scm>=8\n[options.extras_require]\ndev = pytest[testing]>=8\n    ruff>=0.4 ; python_version >= '3.9'\n",
			want: []DependencyReference{
				{Raw: "requests>=2.31 ; python_version < '3.13'", Name: "requests", VersionConstraint: ">=2.31", Attributes: map[string]string{"marker": "python_version < '3.13'"}},
				{Raw: "urllib3<3", Name: "urllib3", VersionConstraint: "<3"},
				{Raw: "wheel", Name: "wheel"},
				{Raw: "setuptools_scm>=8", Name: "setuptools_scm", VersionConstraint: ">=8"},
				{Raw: "pytest[testing]>=8", Name: "pytest", VersionConstraint: ">=8", Attributes: map[string]string{"extras": "testing"}},
				{Raw: "ruff>=0.4 ; python_version >= '3.9'", Name: "ruff", VersionConstraint: ">=0.4", Attributes: map[string]string{"marker": "python_version >= '3.9'"}},
			},
		},
		{
			name:    "unsupported multiline references remain raw",
			content: "[options]\ninstall_requires =\n    example @ https://example.org/example.whl\n    https://example.org/archive.zip\n",
			want: []DependencyReference{
				{Raw: "example @ https://example.org/example.whl"},
				{Raw: "https://example.org/archive.zip"},
			},
		},
		{
			name:    "inline comment semicolons do not add requirements",
			content: "[options]\ninstall_requires = requests # comment; fakepkg\n",
			want:    []DependencyReference{{Raw: "requests", Name: "requests"}},
		},
		{
			name:    "inline file directive is not a requirement list",
			content: "[options]\ninstall_requires = file: deps.txt; requests\nsetup_requires = wheel\n",
			want:    []DependencyReference{{Raw: "wheel", Name: "wheel"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.Analyze("setup.cfg", []byte(tt.content))
			if err != nil {
				t.Fatal(err)
			}
			if !result.Recognized || result.Analysis != (SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete}) {
				t.Fatalf("unexpected analysis: %+v", result)
			}
			if len(result.Dependencies) != len(tt.want) {
				t.Fatalf("dependencies: got %+v, want %+v", result.Dependencies, tt.want)
			}
			for i, want := range tt.want {
				got := result.Dependencies[i]
				if got.Raw != want.Raw || got.Name != want.Name || got.VersionConstraint != want.VersionConstraint || !maps.Equal(got.Attributes, want.Attributes) {
					t.Errorf("dependency %d: got %+v, want %+v", i, got, want)
				}
			}
		})
	}
}
