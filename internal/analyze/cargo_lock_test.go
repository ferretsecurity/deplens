package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCargoLockAnalyzeDependencySourceExtractsPackageVersions(t *testing.T) {
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

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "Cargo.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("rust-cargo-lock") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if want := []string{"serde@1.0.217", "tokio@1.43.0"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if present == nil || !*present {
		t.Fatalf("expected presence=present, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestCargoLockAnalyzeDependencySourceReturnsConclusiveEmptyForVersionOnlyFiles(t *testing.T) {
	ruleset := mustLoadCargoLockRules(t)
	filePath := filepath.Join(t.TempDir(), "Cargo.lock")

	mustWriteFile(t, filePath, "version = 3\n")

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "Cargo.lock")
	if err != nil {
		t.Fatalf("AnalyzeDependencySource failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != DetectorID("rust-cargo-lock") {
		t.Fatalf("unexpected dependency source type: got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if present == nil || *present {
		t.Fatalf("expected presence=absent, got %+v", present)
	}
	if diagnosticMessages != nil {
		t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
	}
}

func TestCargoLockParserSetsStructuredFields(t *testing.T) {
	parser, err := newCargoLockParser(cargoLockMatcherConfig{})
	if err != nil {
		t.Fatalf("newCargoLockParser: %v", err)
	}
	result, err := parser.Analyze("Cargo.lock", []byte(`
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
	if dep.OriginKind != "registry" {
		t.Errorf("Source: got %q want %q", dep.OriginKind, "registry")
	}
	if dep.Attributes["checksum"] != "abc123" {
		t.Errorf("Attributes[checksum]: got %q want %q", dep.Attributes["checksum"], "abc123")
	}
}

func TestCargoLockAnalyzeDependencySourceRejectsMalformedTOML(t *testing.T) {
	ruleset := mustLoadCargoLockRules(t)
	filePath := filepath.Join(t.TempDir(), "Cargo.lock")

	mustWriteFile(t, filePath, "version = 3\n[[package]]\nname = \"serde\"\nversion = ")

	got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, filePath, "Cargo.lock")
	if err != nil {
		t.Fatalf("expected warning, got error: %v", err)
	}
	if !ok || got != DetectorID("rust-cargo-lock") {
		t.Fatalf("expected cargo lock warning match, got detector=%q ok=%v", got, ok)
	}
	if deps != nil || present != nil {
		t.Fatalf("expected no dependency result, got deps=%+v presence=%+v", deps, present)
	}
	if len(diagnosticMessages) != 1 || !strings.Contains(diagnosticMessages[0], "Cargo.lock") {
		t.Fatalf("unexpected diagnostics: %+v", diagnosticMessages)
	}
}

func TestCargoLockParserFixtureCoverage(t *testing.T) {
	ruleset := mustLoadCargoLockRules(t)

	testCases := []struct {
		name        string
		fixtureDir  string
		wantDeps    []string
		wantPresent *bool
	}{
		{
			name:        "extracts package versions",
			fixtureDir:  "cargo-lock-with-deps",
			wantDeps:    []string{"serde@1.0.217", "tokio@1.43.0"},
			wantPresent: boolPtr(true),
		},
		{
			name:        "reports conclusive empty",
			fixtureDir:  "cargo-lock-empty",
			wantDeps:    nil,
			wantPresent: boolPtr(false),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "testdata", "rust", tc.fixtureDir, "Cargo.lock")
			got, deps, present, diagnosticMessages, ok, err := analyzeSourceParts(ruleset, path, "Cargo.lock")
			if err != nil {
				t.Fatalf("AnalyzeDependencySource failed: %v", err)
			}
			if !ok {
				t.Fatalf("expected match")
			}
			if got != DetectorID("rust-cargo-lock") {
				t.Fatalf("unexpected dependency source type: got %q", got)
			}
			if !slices.Equal(dependencyNames(deps), tc.wantDeps) {
				t.Fatalf("unexpected dependencies: got %+v want %+v", deps, tc.wantDeps)
			}
			if tc.wantPresent == nil {
				if present != nil {
					t.Fatalf("expected presence=unknown, got %+v", present)
				}
			} else if present == nil || *present != *tc.wantPresent {
				t.Fatalf("unexpected presence: got %+v want %+v", present, tc.wantPresent)
			}
			if diagnosticMessages != nil {
				t.Fatalf("expected no diagnostics, got %+v", diagnosticMessages)
			}
		})
	}
}

func mustLoadCargoLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte("rules:\n    - id: rust-cargo-lock\n      filename-regex: '^Cargo\\.lock$'\n      form: other\n      roles:\n        - inventory\n      analyzer:\n        type: cargo-lock\n"))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	return ruleset
}
