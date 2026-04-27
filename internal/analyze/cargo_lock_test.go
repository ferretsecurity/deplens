package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCargoLockDetectManifestFileExtractsPackageVersions(t *testing.T) {
	ruleset := mustLoadCargoLockRules(t)
	filePath := filepath.Join(t.TempDir(), "Cargo.lock")

	mustWriteFile(t, filePath, `
version = 3

[[package]]
name = "serde"
version = "1.0.217"

[[package]]
name = "tokio"
version = "1.43.0"
`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "Cargo.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("rust-cargo-lock") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if want := []string{"serde@1.0.217", "tokio@1.43.0"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestCargoLockDetectManifestFileReturnsConclusiveEmptyForVersionOnlyFiles(t *testing.T) {
	ruleset := mustLoadCargoLockRules(t)
	filePath := filepath.Join(t.TempDir(), "Cargo.lock")

	mustWriteFile(t, filePath, "version = 3\n")

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "Cargo.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("rust-cargo-lock") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if hasDependencies == nil || *hasDependencies {
		t.Fatalf("expected has_dependencies=false, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestCargoLockParserSetsStructuredFields(t *testing.T) {
	parser, err := newCargoLockParser(cargoLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newCargoLockParser: %v", err)
	}
	result, err := parser.Match("Cargo.lock", []byte(`
version = 3

[[package]]
name = "serde"
version = "1.0.217"
source = "registry+https://github.com/rust-lang/crates.io-index"
checksum = "abc123"
`))
	if err != nil {
		t.Fatalf("Match failed: %v", err)
	}
	if len(result.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(result.Dependencies))
	}
	dep := result.Dependencies[0]
	if dep.Raw != "serde@1.0.217" {
		t.Errorf("Raw: got %q want %q", dep.Raw, "serde@1.0.217")
	}
	if dep.Name != "serde" {
		t.Errorf("Name: got %q want %q", dep.Name, "serde")
	}
	if dep.Version != "1.0.217" {
		t.Errorf("Version: got %q want %q", dep.Version, "1.0.217")
	}
	if dep.Source != "registry" {
		t.Errorf("Source: got %q want %q", dep.Source, "registry")
	}
	if dep.Extras["checksum"] != "abc123" {
		t.Errorf("Extras[checksum]: got %q want %q", dep.Extras["checksum"], "abc123")
	}
}

func TestCargoLockDetectManifestFileRejectsMalformedTOML(t *testing.T) {
	ruleset := mustLoadCargoLockRules(t)
	filePath := filepath.Join(t.TempDir(), "Cargo.lock")

	mustWriteFile(t, filePath, "version = 3\n[[package]]\nname = \"serde\"\nversion = ")

	_, _, _, _, _, err := ruleset.DetectManifestFile(filePath, "Cargo.lock")
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if got := err.Error(); !strings.Contains(got, "Cargo.lock") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCargoLockParserFixtureCoverage(t *testing.T) {
	ruleset := mustLoadCargoLockRules(t)

	testCases := []struct {
		name       string
		fixtureDir string
		wantDeps   []string
		wantHas    *bool
	}{
		{
			name:       "extracts package versions",
			fixtureDir: "cargo-lock-with-deps",
			wantDeps:   []string{"serde@1.0.217", "tokio@1.43.0"},
			wantHas:    boolPtr(true),
		},
		{
			name:       "reports conclusive empty",
			fixtureDir: "cargo-lock-empty",
			wantDeps:   nil,
			wantHas:    boolPtr(false),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "rust", tc.fixtureDir, "Cargo.lock")
			got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(path, "Cargo.lock")
			if err != nil {
				t.Fatalf("DetectManifestFile failed: %v", err)
			}
			if !ok {
				t.Fatalf("expected match")
			}
			if got != ManifestType("rust-cargo-lock") {
				t.Fatalf("unexpected manifest type: got %q", got)
			}
			if !slices.Equal(dependencyNames(deps), tc.wantDeps) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", deps, tc.wantDeps)
			}
			if tc.wantHas == nil {
				if hasDependencies != nil {
					t.Fatalf("expected has_dependencies=nil, got %+v", hasDependencies)
				}
			} else if hasDependencies == nil || *hasDependencies != *tc.wantHas {
				t.Fatalf("unexpected has_dependencies: got %+v want %+v", hasDependencies, tc.wantHas)
			}
			if warnings != nil {
				t.Fatalf("expected no warnings, got %+v", warnings)
			}
		})
	}
}

func mustLoadCargoLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte(`
rules:
  - name: rust-cargo-lock
    filename-regex: '^Cargo\.lock$'
    cargo-lock: {}
`))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	return ruleset
}
