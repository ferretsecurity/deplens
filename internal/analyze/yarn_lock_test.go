package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestYarnLockDetectManifestFileExtractsClassicEntries(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

left-pad@^1.3.0:
	version "1.3.0"

lodash@^4.17.0, lodash@~4.17.21:
	version "4.17.21"
`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-yarn") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if want := []string{"left-pad@1.3.0", "lodash@4.17.21"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestYarnLockDetectManifestFileDeduplicatesClassicSelectors(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

left-pad@^1.3.0, left-pad@~1.3.0:
	version "1.3.0"

left-pad@1.3.0:
	version "1.3.0"
`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"left-pad@1.3.0"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestYarnLockDetectManifestFileClassicFallsBackToNameWhenVersionMissing(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

left-pad@^1.3.0:
	resolved "https://registry.yarnpkg.com/left-pad/-/left-pad-1.3.0.tgz"
`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"left-pad"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestYarnLockDetectManifestFileClassicEmitsMixedVersionedAndVersionMissingEntries(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

left-pad@^1.3.0:
	version "1.3.0"

lodash@^4.17.0:
	resolved "https://registry.yarnpkg.com/lodash/-/lodash-4.17.21.tgz"
`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"left-pad@1.3.0", "lodash"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestYarnLockDetectManifestFileExtractsClassicScopedGroupedSelectors(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# yarn lockfile v1

"@babel/code-frame@^7.0.0", "@babel/code-frame@^7.27.1":
	version "7.27.1"
`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"@babel/code-frame@7.27.1"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestYarnLockDetectManifestFileClassicHeaderOnlyIsConclusiveEmpty(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, "# yarn lockfile v1\n")

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-yarn") {
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

func TestYarnLockDetectManifestFileExtractsModernEntries(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10

"react@npm:^18.3.1":
  version: "18.3.1"
  resolution: "react@npm:18.3.1"

"typescript@patch:typescript@npm%3A5.4.5#~builtin<compat/typescript>":
  version: "5.4.5"
  resolution: "typescript@patch:typescript@npm%3A5.4.5#~builtin<compat/typescript>"

"@babel/code-frame@npm:^7.27.1":
  version: "7.27.1"
  resolution: "@babel/code-frame@npm:7.27.1"
`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-yarn") {
		t.Fatalf("unexpected manifest type: got %q", got)
	}
	if want := []string{"@babel/code-frame@7.27.1", "react@18.3.1", "typescript@5.4.5"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestYarnLockDetectManifestFileModernFallsBackToNameWhenVersionMissing(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10

"string-width@npm:^4.2.3":
  resolution: "string-width@npm:4.2.3"
`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"string-width"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestYarnLockDetectManifestFileModernGroupedSelectorsFallBackToNameWithoutResolution(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10

" left-pad@npm:^1.3.0, left-pad@npm:~1.3.0 ":
  version: "1.3.0"

" @babel/core@npm:^7.0.0, @babel/core@npm:^7.27.0 ":
  version: "7.27.0"
`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"@babel/core@7.27.0", "left-pad@1.3.0"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestYarnLockDetectManifestFileModernMatchesWithLeadingCommentPreamble(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `# This file is generated by running "yarn install"
# Manual changes might be lost
__metadata:
  version: 8
  cacheKey: 10

"react@npm:^18.3.1":
  version: "18.3.1"
  resolution: "react@npm:18.3.1"
`)

	_, deps, hasDependencies, _, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if want := []string{"react@18.3.1"}; !slices.Equal(dependencyNames(deps), want) {
		t.Fatalf("unexpected dependencies: got %+v want %+v", deps, want)
	}
	if hasDependencies == nil || !*hasDependencies {
		t.Fatalf("expected has_dependencies=true, got %+v", hasDependencies)
	}
}

func TestYarnLockDetectManifestFileModernMetadataOnlyIsConclusiveEmpty(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10
`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-yarn") {
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

func TestYarnLockDetectManifestFileRejectsMalformedModernYAML(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
"react@npm:^18.3.1"
  version: "18.3.1"
`)

	_, _, _, _, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err == nil {
		t.Fatalf("expected error")
	}
	if ok {
		t.Fatalf("expected no match on malformed content")
	}
	if got := err.Error(); !strings.Contains(got, "yarn.lock") {
		t.Fatalf("expected error to mention yarn.lock, got %q", got)
	}
}

func TestYarnLockDetectManifestFileRejectsStructurallyInvalidModernEntry(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `__metadata:
  version: 8
  cacheKey: 10

"react@npm:^18.3.1": oops
`)

	_, _, _, _, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err == nil {
		t.Fatalf("expected error")
	}
	if ok {
		t.Fatalf("expected no match on malformed content")
	}
	if got := err.Error(); !strings.Contains(got, "yarn.lock") {
		t.Fatalf("expected error to mention yarn.lock, got %q", got)
	}
}

func TestYarnLockDetectManifestFileReturnsNoMatchForUnrecognizedContent(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, `not a yarn lockfile
just some text
`)

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if ok {
		t.Fatalf("expected no match, got %q", got)
	}
	if deps != nil {
		t.Fatalf("expected no dependencies, got %+v", deps)
	}
	if hasDependencies != nil {
		t.Fatalf("expected unknown has_dependencies, got %+v", hasDependencies)
	}
	if warnings != nil {
		t.Fatalf("expected no warnings, got %+v", warnings)
	}
}

func TestYarnLockDetectManifestFileClassicHeaderMatchesWithUTF8BOM(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, "\ufeff# yarn lockfile v1\n")

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-yarn") {
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

func TestYarnLockDetectManifestFileClassicHeaderMatchesWithGeneratedPreamble(t *testing.T) {
	ruleset := mustLoadYarnLockRules(t)
	filePath := filepath.Join(t.TempDir(), "yarn.lock")

	mustWriteFile(t, filePath, "# THIS IS AN AUTOGENERATED FILE. DO NOT EDIT THIS FILE DIRECTLY.\n# yarn lockfile v1\n")

	got, deps, hasDependencies, warnings, ok, err := ruleset.DetectManifestFile(filePath, "yarn.lock")
	if err != nil {
		t.Fatalf("DetectManifestFile failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected match")
	}
	if got != ManifestType("js-yarn") {
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

func mustLoadYarnLockRules(t *testing.T) Ruleset {
	t.Helper()

	ruleset, err := loadRules("test.yaml", []byte(`
rules:
  - name: js-yarn
    filename-regex: '^yarn\.lock$'
    yarn-lock: {}
`))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	return ruleset
}
