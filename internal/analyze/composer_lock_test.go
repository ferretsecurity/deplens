package analyze

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestComposerLockParserExtractsPackageVersions(t *testing.T) {
	parser, err := newComposerLockParser(composerLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newComposerLockParser returned error: %v", err)
	}

	result, err := parser.Match("composer.lock", []byte(`{
  "packages": [
    {
      "name": "monolog/monolog",
      "version": "3.6.0"
    },
    {
      "name": "psr/log",
      "version": "3.0.0"
    }
  ]
}`))
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected parser to match composer.lock")
	}
	if result.HasDependencies == nil || !*result.HasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", result.HasDependencies)
	}

	got := dependencyNames(result.Dependencies)
	want := []string{"monolog/monolog@3.6.0", "psr/log@3.0.0"}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", got, want)
	}
}

func TestComposerLockParserReportsConclusiveEmpty(t *testing.T) {
	parser, err := newComposerLockParser(composerLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newComposerLockParser returned error: %v", err)
	}

	result, err := parser.Match("composer.lock", []byte(`{"packages":[]}`))
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected parser to match composer.lock")
	}
	if result.HasDependencies == nil || *result.HasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", result.HasDependencies)
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
}

func TestComposerLockParserExtractsSections(t *testing.T) {
	parser, err := newComposerLockParser(composerLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newComposerLockParser returned error: %v", err)
	}

	result, err := parser.Match("composer.lock", mustReadTestdataFile(t, "php", "composer-lock-packages-dev", "composer.lock"))
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected parser to match composer.lock")
	}
	want := []Dependency{
		{Name: "monolog/monolog@3.6.0", Section: "packages"},
		{Name: "phpunit/phpunit@11.5.3", Section: "packages-dev"},
	}
	if !slices.Equal(result.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
}

func TestComposerLockParserPreservesDuplicatesAcrossSections(t *testing.T) {
	parser, err := newComposerLockParser(composerLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newComposerLockParser returned error: %v", err)
	}

	result, err := parser.Match("composer.lock", mustReadTestdataFile(t, "php", "composer-lock-duplicate-across-groups", "composer.lock"))
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if !result.Matched {
		t.Fatalf("expected parser to match composer.lock")
	}
	want := []Dependency{
		{Name: "monolog/monolog@3.6.0", Section: "packages"},
		{Name: "phpunit/phpunit@11.5.3", Section: "packages"},
		{Name: "monolog/monolog@3.6.0", Section: "packages-dev"},
	}
	if !slices.Equal(result.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
}

func TestComposerLockParserFixtureCoverage(t *testing.T) {
	parser, err := newComposerLockParser(composerLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newComposerLockParser returned error: %v", err)
	}

	testCases := []struct {
		name       string
		fixtureDir string
		wantDeps   []Dependency
		wantHas    *bool
	}{
		{
			name:       "basic packages",
			fixtureDir: "composer-lock-basic",
			wantDeps: []Dependency{
				{Name: "monolog/monolog@3.6.0", Section: "packages"},
				{Name: "psr/log@3.0.0", Section: "packages"},
			},
			wantHas:    boolPtr(true),
		},
		{
			name:       "includes packages dev",
			fixtureDir: "composer-lock-packages-dev",
			wantDeps: []Dependency{
				{Name: "monolog/monolog@3.6.0", Section: "packages"},
				{Name: "phpunit/phpunit@11.5.3", Section: "packages-dev"},
			},
			wantHas:    boolPtr(true),
		},
		{
			name:       "falls back to name when version missing",
			fixtureDir: "composer-lock-missing-version",
			wantDeps: []Dependency{
				{Name: "monolog/monolog", Section: "packages"},
			},
			wantHas:    boolPtr(true),
		},
		{
			name:       "dedupes duplicate packages across groups",
			fixtureDir: "composer-lock-duplicate-across-groups",
			wantDeps: []Dependency{
				{Name: "monolog/monolog@3.6.0", Section: "packages"},
				{Name: "phpunit/phpunit@11.5.3", Section: "packages"},
				{Name: "monolog/monolog@3.6.0", Section: "packages-dev"},
			},
			wantHas:    boolPtr(true),
		},
		{
			name:       "reports conclusive empty",
			fixtureDir: "composer-lock-empty",
			wantDeps:   nil,
			wantHas:    boolPtr(false),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := mustReadTestdataFile(t, "php", tc.fixtureDir, "composer.lock")
			result, err := parser.Match("composer.lock", content)
			if err != nil {
				t.Fatalf("Match returned error: %v", err)
			}
			if !result.Matched {
				t.Fatalf("expected parser to match composer.lock")
			}
			if tc.wantHas == nil {
				if result.HasDependencies != nil {
					t.Fatalf("expected has_dependencies=nil, got %+v", result.HasDependencies)
				}
			} else if result.HasDependencies == nil || *result.HasDependencies != *tc.wantHas {
				t.Fatalf("unexpected has_dependencies: got %+v want %+v", result.HasDependencies, tc.wantHas)
			}
			if !slices.Equal(result.Dependencies, tc.wantDeps) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, tc.wantDeps)
			}
		})
	}
}

func mustReadTestdataFile(t *testing.T, parts ...string) []byte {
	t.Helper()

	pathParts := append([]string{"..", "..", "testdata"}, parts...)
	data, err := os.ReadFile(filepath.Join(pathParts...))
	if err != nil {
		t.Fatalf("read fixture failed: %v", err)
	}
	return data
}
