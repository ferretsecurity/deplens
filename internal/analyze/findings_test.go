package analyze

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestMissingLockfileDefaultChecks(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	tests := []struct {
		fixture string
		checkID CheckID
		path    string
	}{
		{fixture: "npm-missing", checkID: "javascript-npm-lockfile-missing", path: "package.json"},
		{fixture: "pnpm-missing", checkID: "javascript-pnpm-lockfile-missing", path: "package.json"},
		{fixture: "yarn-missing", checkID: "javascript-yarn-lockfile-missing", path: "package.json"},
		{fixture: "yarn-config-missing", checkID: "javascript-yarn-lockfile-missing", path: "package.json"},
		{fixture: "uv-missing", checkID: "python-uv-lockfile-missing", path: "pyproject.toml"},
		{fixture: "cargo-missing", checkID: "rust-cargo-lockfile-missing-for-application", path: "Cargo.toml"},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "findings", test.fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			if len(result.Findings) != 1 {
				t.Fatalf("expected one finding, got %#v", result.Findings)
			}
			finding := result.Findings[0]
			if finding.CheckID != test.checkID || len(finding.Locations) != 1 || finding.Locations[0].Path != test.path || finding.Fingerprint == "" {
				t.Fatalf("unexpected finding: %#v", finding)
			}
			if len(result.CheckRuns) != 1 || result.CheckRuns[0].Status != CheckCompleted {
				t.Fatalf("unexpected check runs: %#v", result.CheckRuns)
			}
		})
	}
}

func TestMissingLockfileChecksDoNotFlagSatisfiedOrDependencyFreeProjects(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	for _, fixture := range []string{"npm-present", "dependency-free"} {
		t.Run(fixture, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "findings", fixture), nil, ruleset)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			if len(result.Findings) != 0 {
				t.Fatalf("expected no findings, got %#v", result.Findings)
			}
		})
	}
}

func TestCargoMissingLockfileCheckSkipsLibrary(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "findings", "cargo-library"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", result.Findings)
	}
	if len(result.CheckRuns) != 1 || result.CheckRuns[0].Status != CheckSkipped || result.CheckRuns[0].ReasonCode != "project-role-unknown" {
		t.Fatalf("expected role skip, got %#v", result.CheckRuns)
	}
}

func TestMissingLockfileChecksSkipAmbiguousManager(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "findings", "ambiguous-js"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", result.Findings)
	}
	if !slices.ContainsFunc(result.CheckRuns, func(run CheckRun) bool {
		return run.CheckID == "javascript-pnpm-lockfile-missing" && run.Status == CheckSkipped && run.ReasonCode == "ambiguous-package-manager"
	}) {
		t.Fatalf("expected ambiguous manager skip, got %#v", result.CheckRuns)
	}
}

func TestMissingLockfileChecksReportFailedRunsForMalformedManifest(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), `{"private":`)
	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("expected no findings, got %#v", result.Findings)
	}
	if len(result.CheckRuns) != 3 {
		t.Fatalf("expected one failed run for each JavaScript check, got %#v", result.CheckRuns)
	}
	for _, run := range result.CheckRuns {
		if run.Status != CheckFailed || run.ReasonCode != "source-analysis-failed" {
			t.Fatalf("unexpected failed run: %#v", run)
		}
	}
}

func TestMissingLockfileCheckUsesWorkspaceOwner(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "findings", "pnpm-workspace"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected one finding, got %#v", result.Findings)
	}
	finding := result.Findings[0]
	if finding.CheckID != "javascript-pnpm-lockfile-missing" || finding.Subject.ProjectRoot != "." || len(finding.Locations) != 1 || finding.Locations[0].Path != "package.json" {
		t.Fatalf("expected workspace-root finding, got %#v", finding)
	}
}

func TestMissingLockfileChecksUseUVAndCargoWorkspaceOwners(t *testing.T) {
	tests := []struct {
		fixture string
		checkID CheckID
	}{
		{fixture: "uv-workspace", checkID: "python-uv-lockfile-missing"},
		{fixture: "cargo-workspace", checkID: "rust-cargo-lockfile-missing-for-application"},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "findings", test.fixture), nil, mustLoadDefaultRules(t))
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			if len(result.Findings) != 1 || result.Findings[0].CheckID != test.checkID || result.Findings[0].Subject.ProjectRoot != "." {
				t.Fatalf("expected one workspace-root finding, got %#v", result.Findings)
			}
			if test.fixture == "uv-workspace" && slices.ContainsFunc(result.Sources, func(source DependencySourceResult) bool {
				return source.Path == "pyproject.toml"
			}) {
				t.Fatalf("dependency-free workspace root must remain a policy input, not a dependency source: %#v", result.Sources)
			}
		})
	}
}

func TestUnrelatedAncestorLockfileDoesNotSatisfyNestedProject(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "findings", "unrelated-ancestor-lock"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].Subject.ProjectRoot != "nested" || len(result.Findings[0].Locations) != 1 || result.Findings[0].Locations[0].Path != "nested/package.json" {
		t.Fatalf("expected nested-project finding, got %#v", result.Findings)
	}
}

func TestFindingFingerprintDoesNotDependOnSummary(t *testing.T) {
	subject := projectSubject("apps/web")
	first := newMissingLockfileFinding(check{ID: "check", Summary: "first", Severity: SeverityMedium}, subject, "apps/web/package.json", "npm", "package-lock.json")
	second := newMissingLockfileFinding(check{ID: "check", Summary: "changed", Severity: SeverityHigh}, subject, "apps/web/alternate.json", "npm", "package-lock.json")
	if first.Fingerprint != second.Fingerprint {
		t.Fatalf("fingerprint changed with presentation fields: %q != %q", first.Fingerprint, second.Fingerprint)
	}
}

func TestJavaScriptMissingLockfileCheckDoesNotRequirePrivatePackage(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "findings", "npm-missing"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].CheckID != "javascript-npm-lockfile-missing" {
		t.Fatalf("expected public npm project finding, got %#v", result.Findings)
	}
}

func TestUVCheckParsesPyprojectPolicyInputWithoutRecognizedSource(t *testing.T) {
	ruleset, err := loadRules("checks.yaml", []byte(`
rules:
  - id: unrelated
    form: other
    roles: [inventory]
    filename-regex: '^unrelated$'
checks:
  - id: python-uv-lockfile-missing
    summary: uv project has dependencies but no uv lockfile
    severity: medium
    evaluator:
      type: uv-lockfile-missing
    remediation: Run uv lock.
`))
	if err != nil {
		t.Fatalf("loadRules failed: %v", err)
	}
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "pyproject.toml"), "[project]\ndependencies = [\"httpx\"]\n[tool.uv]\n")
	result, err := Scan(root, nil, ruleset)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no recognized dependency sources, got %#v", result.Sources)
	}
	if len(result.Findings) != 1 || result.Findings[0].Subject.ProjectRoot != "." || result.Findings[0].Locations[0].Path != "pyproject.toml" {
		t.Fatalf("expected uv finding from policy input, got %#v", result.Findings)
	}
}
