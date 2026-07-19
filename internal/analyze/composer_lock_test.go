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

	result, err := parser.Analyze("composer.lock", []byte(`{
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
	if !result.Recognized {
		t.Fatalf("expected parser to match composer.lock")
	}
	if result.Analysis.Presence != PresencePresent {
		t.Fatalf("expected presence=present, got %+v", result.Analysis)
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

	result, err := parser.Analyze("composer.lock", []byte(`{"packages":[]}`))
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected parser to match composer.lock")
	}
	if result.Analysis.Presence != PresenceAbsent {
		t.Fatalf("expected presence=absent, got %+v", result.Analysis)
	}
	if len(result.Dependencies) != 0 {
		t.Fatalf("expected no dependencies, got %+v", result.Dependencies)
	}
}

func TestComposerLockParserSetsStructuredFields(t *testing.T) {
	parser, _ := newComposerLockParser(composerLockMatcherConfig{})
	result, _ := parser.Analyze("composer.lock", []byte(`{
        "packages": [
            {"name": "vendor/pkg", "version": "1.0.0", "type": "library"}
        ],
        "packages-dev": []
    }`))
	dep := result.Dependencies[0]
	if dep.Raw != "vendor/pkg@1.0.0" {
		t.Errorf("Raw: got %q", dep.Raw)
	}
	if dep.Name != "vendor/pkg" {
		t.Errorf("Name: got %q", dep.Name)
	}
	if dep.Version != "1.0.0" {
		t.Errorf("Version: got %q", dep.Version)
	}
	if dep.SourceGroup != "packages" {
		t.Errorf("SourceGroup: got %q", dep.SourceGroup)
	}
	if dep.Attributes["package_type"] != "library" {
		t.Errorf("Attributes[package_type]: got %q", dep.Attributes["package_type"])
	}
}

func TestComposerLockParserExtractsSections(t *testing.T) {
	parser, err := newComposerLockParser(composerLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newComposerLockParser returned error: %v", err)
	}

	result, err := parser.Analyze("composer.lock", mustReadTestdataFile(t, "php", "composer-lock-packages-dev", "composer.lock"))
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected parser to match composer.lock")
	}
	want := []DependencyReference{
		{Raw: "monolog/monolog@3.6.0", Name: "monolog/monolog", Version: "3.6.0", SourceGroup: "packages"},
		{Raw: "phpunit/phpunit@11.5.3", Name: "phpunit/phpunit", Version: "11.5.3", SourceGroup: "packages-dev"},
	}
	if !equalDependencies(result.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
}

func TestComposerLockParserPreservesDuplicatesAcrossSections(t *testing.T) {
	parser, err := newComposerLockParser(composerLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newComposerLockParser returned error: %v", err)
	}

	result, err := parser.Analyze("composer.lock", mustReadTestdataFile(t, "php", "composer-lock-duplicate-across-groups", "composer.lock"))
	if err != nil {
		t.Fatalf("Match returned error: %v", err)
	}
	if !result.Recognized {
		t.Fatalf("expected parser to match composer.lock")
	}
	want := []DependencyReference{
		{Raw: "monolog/monolog@3.6.0", Name: "monolog/monolog", Version: "3.6.0", SourceGroup: "packages"},
		{Raw: "phpunit/phpunit@11.5.3", Name: "phpunit/phpunit", Version: "11.5.3", SourceGroup: "packages"},
		{Raw: "monolog/monolog@3.6.0", Name: "monolog/monolog", Version: "3.6.0", SourceGroup: "packages-dev"},
	}
	if !equalDependencies(result.Dependencies, want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", result.Dependencies, want)
	}
}

func TestComposerLockParserFixtureCoverage(t *testing.T) {
	parser, err := newComposerLockParser(composerLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newComposerLockParser returned error: %v", err)
	}

	testCases := []struct {
		name        string
		fixtureDir  string
		wantDeps    []DependencyReference
		wantPresent *bool
	}{
		{
			name:       "basic packages",
			fixtureDir: "composer-lock-basic",
			wantDeps: []DependencyReference{
				{Raw: "monolog/monolog@3.6.0", Name: "monolog/monolog", Version: "3.6.0", SourceGroup: "packages"},
				{Raw: "psr/log@3.0.0", Name: "psr/log", Version: "3.0.0", SourceGroup: "packages"},
			},
			wantPresent: boolPtr(true),
		},
		{
			name:       "includes packages dev",
			fixtureDir: "composer-lock-packages-dev",
			wantDeps: []DependencyReference{
				{Raw: "monolog/monolog@3.6.0", Name: "monolog/monolog", Version: "3.6.0", SourceGroup: "packages"},
				{Raw: "phpunit/phpunit@11.5.3", Name: "phpunit/phpunit", Version: "11.5.3", SourceGroup: "packages-dev"},
			},
			wantPresent: boolPtr(true),
		},
		{
			name:       "falls back to name when version missing",
			fixtureDir: "composer-lock-missing-version",
			wantDeps: []DependencyReference{
				{Raw: "monolog/monolog", Name: "monolog/monolog", SourceGroup: "packages"},
			},
			wantPresent: boolPtr(true),
		},
		{
			name:       "dedupes duplicate packages across groups",
			fixtureDir: "composer-lock-duplicate-across-groups",
			wantDeps: []DependencyReference{
				{Raw: "monolog/monolog@3.6.0", Name: "monolog/monolog", Version: "3.6.0", SourceGroup: "packages"},
				{Raw: "phpunit/phpunit@11.5.3", Name: "phpunit/phpunit", Version: "11.5.3", SourceGroup: "packages"},
				{Raw: "monolog/monolog@3.6.0", Name: "monolog/monolog", Version: "3.6.0", SourceGroup: "packages-dev"},
			},
			wantPresent: boolPtr(true),
		},
		{
			name:        "reports conclusive empty",
			fixtureDir:  "composer-lock-empty",
			wantDeps:    nil,
			wantPresent: boolPtr(false),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			content := mustReadTestdataFile(t, "php", tc.fixtureDir, "composer.lock")
			result, err := parser.Analyze("composer.lock", content)
			if err != nil {
				t.Fatalf("Match returned error: %v", err)
			}
			if !result.Recognized {
				t.Fatalf("expected parser to match composer.lock")
			}
			if tc.wantPresent == nil {
				if result.Analysis.Presence != "" && result.Analysis.Presence != PresenceUnknown {
					t.Fatalf("expected presence=unknown, got %+v", result.Analysis)
				}
			} else if result.Analysis.Presence != presenceAnalysis(*tc.wantPresent).Presence {
				t.Fatalf("unexpected presence: got %+v want %+v", result.Analysis, tc.wantPresent)
			}
			if !equalDependencies(result.Dependencies, tc.wantDeps) {
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
