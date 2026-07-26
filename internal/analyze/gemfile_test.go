package analyze

import (
	"strings"
	"testing"
)

func TestGemfileParserExtractsGroupsConstraintsAndOrigins(t *testing.T) {
	parser, _ := newGemfileParser(gemfileMatcherConfig{})
	result, err := parser.Analyze("Gemfile", []byte(`
source "https://rubygems.org"
gem "rails", "~> 7.1"
group :development, :test do
  gem "rspec", "~> 3.13"
end
gem "private-gem", git: "https://github.com/acme/private-gem.git"
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Dependencies) != 3 || result.Analysis.Extraction != ExtractionComplete {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, dependency := range result.Dependencies {
		switch dependency.Name {
		case "rails":
			if dependency.VersionConstraint != "~> 7.1" || dependency.OriginKind != OriginRegistry {
				t.Fatalf("unexpected Rails dependency: %+v", dependency)
			}
		case "rspec":
			if dependency.SourceGroup != "development,test" || dependency.Scope != ScopeTest {
				t.Fatalf("unexpected RSpec dependency: %+v", dependency)
			}
		case "private-gem":
			if dependency.OriginKind != OriginGit {
				t.Fatalf("unexpected git dependency: %+v", dependency)
			}
		}
	}
}

func TestGemfileParserReportsDynamicDeclarations(t *testing.T) {
	parser, _ := newGemfileParser(gemfileMatcherConfig{})
	result, err := parser.Analyze("Gemfile", []byte(`
gem "rails", "~> 7.1"
gem dynamic_name, dynamic_version
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis.Extraction != ExtractionPartial || len(result.Dependencies) != 1 || len(result.Diagnostics) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGemfileParserExtractsMultilineCallsAndGroupOptions(t *testing.T) {
	parser, _ := newGemfileParser(gemfileMatcherConfig{})
	result, err := parser.Analyze("Gemfile", []byte(`
gem(
  "rack",
  ">= 3.0",
  "< 4.0",
  group: [:development, :test],
)
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Dependencies) != 1 {
		t.Fatalf("unexpected dependencies: %+v", result.Dependencies)
	}
	dependency := result.Dependencies[0]
	if dependency.VersionConstraint != ">= 3.0, < 4.0" || dependency.SourceGroup != "development,test" {
		t.Fatalf("unexpected dependency: %+v", dependency)
	}
}

func TestGemfileParserKeepsGroupAcrossNestedNonGroupBlocks(t *testing.T) {
	parser, _ := newGemfileParser(gemfileMatcherConfig{})
	result, err := parser.Analyze("Gemfile", []byte(`
group :development, :test do
  source "https://gems.example.test" do
    gem "inside"
  end
  gem "still-grouped"
end
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Dependencies) != 2 {
		t.Fatalf("unexpected dependencies: %+v", result.Dependencies)
	}
	for _, dependency := range result.Dependencies {
		if dependency.SourceGroup != "development,test" || dependency.Scope != ScopeTest {
			t.Fatalf("unexpected dependency: %+v", dependency)
		}
	}
}

func TestGemfileParserRecognizesDependencyFreeGemfile(t *testing.T) {
	parser, _ := newGemfileParser(gemfileMatcherConfig{})
	result, err := parser.Analyze("Gemfile", []byte(`source "https://rubygems.org"`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if result.Analysis != (SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete}) {
		t.Fatalf("unexpected analysis: %+v", result.Analysis)
	}
}

func TestGemfileLockParserRelatesDirectAndTransitiveSpecs(t *testing.T) {
	parser, _ := newGemfileLockParser(gemfileLockMatcherConfig{})
	result, err := parser.Analyze("Gemfile.lock", []byte(`
GEM
  remote: https://rubygems.org/
  specs:
    rack (3.0.11)
    rails (7.1.4)
      rack (>= 2.2.4)

DEPENDENCIES
  rails (~> 7.1)
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Dependencies) != 2 {
		t.Fatalf("unexpected dependencies: %+v", result.Dependencies)
	}
	for _, dependency := range result.Dependencies {
		if dependency.Name == "rails" && (dependency.Relationship != RelationshipDirect || dependency.VersionConstraint != "~> 7.1") {
			t.Fatalf("unexpected Rails dependency: %+v", dependency)
		}
		if dependency.Name == "rack" && dependency.Relationship != RelationshipTransitive {
			t.Fatalf("unexpected Rack dependency: %+v", dependency)
		}
	}
}

func TestGemfileLockParserPreservesGitAndPathOrigins(t *testing.T) {
	parser, _ := newGemfileLockParser(gemfileLockMatcherConfig{})
	result, err := parser.Analyze("Gemfile.lock", []byte(`
GIT
  remote: https://github.com/acme/tool.git
  revision: abc123
  specs:
    tool (1.2.0)

PATH
  remote: ../local-gem
  specs:
    local-gem (0.1.0)

DEPENDENCIES
  local-gem!
  tool!
`))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Dependencies) != 2 {
		t.Fatalf("unexpected dependencies: %+v", result.Dependencies)
	}
	for _, dependency := range result.Dependencies {
		if dependency.Name == "tool" && (dependency.OriginKind != OriginGit || dependency.Attributes["source_ref"] != "abc123") {
			t.Fatalf("unexpected git origin: %+v", dependency)
		}
		if dependency.Name == "local-gem" && (dependency.OriginKind != OriginPath || dependency.Attributes["source_path"] != "../local-gem") {
			t.Fatalf("unexpected path origin: %+v", dependency)
		}
	}
}

func TestGemfileLockParserRejectsUnrecognizedContent(t *testing.T) {
	parser, _ := newGemfileLockParser(gemfileLockMatcherConfig{})
	_, err := parser.Analyze("Gemfile.lock", []byte("not a bundler lockfile\n"))
	if err == nil || !strings.Contains(err.Error(), "no recognized") {
		t.Fatalf("expected recognition error, got %v", err)
	}
}
