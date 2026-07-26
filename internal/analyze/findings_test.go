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
		{fixture: "go-sum-missing", checkID: "go-sum-missing", path: "go.mod"},
		{fixture: "composer-lock-missing", checkID: "php-composer-lockfile-missing-for-application", path: "composer.json"},
		{fixture: "gemfile-lock-missing", checkID: "ruby-gemfile-lockfile-missing-for-application", path: "Gemfile"},
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
			if !slices.ContainsFunc(result.CheckRuns, func(run CheckRun) bool {
				return run.CheckID == test.checkID && run.Status == CheckCompleted
			}) {
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

func TestConflictingJavaScriptLockfilesCheckReportsDifferentFamilies(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "findings", "js-conflicting-lockfiles"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("expected one finding, got %#v", result.Findings)
	}
	finding := result.Findings[0]
	if finding.CheckID != "javascript-conflicting-lockfiles" || finding.Subject.ProjectRoot != "." {
		t.Fatalf("unexpected finding: %#v", finding)
	}
	wantLocations := []FindingLocation{{Path: "package.json"}, {Path: "package-lock.json"}, {Path: "pnpm-lock.yaml"}}
	if !slices.Equal(finding.Locations, wantLocations) {
		t.Fatalf("locations: got %#v want %#v", finding.Locations, wantLocations)
	}
	if finding.Evidence["lockfile_families"] != "npm, pnpm" || finding.Evidence["conflicting_lockfiles"] != "package-lock.json, pnpm-lock.yaml" {
		t.Fatalf("unexpected evidence: %#v", finding.Evidence)
	}
	if finding.Fingerprint == "" {
		t.Fatal("expected stable finding fingerprint")
	}
	if !slices.ContainsFunc(result.CheckRuns, func(run CheckRun) bool {
		return run.CheckID == "javascript-conflicting-lockfiles" && run.Status == CheckCompleted
	}) {
		t.Fatalf("expected completed check run, got %#v", result.CheckRuns)
	}
}

func TestConflictingJavaScriptLockfilesCheckTreatsNPMFilesAsOneFamily(t *testing.T) {
	result, err := Scan(filepath.Join("..", "..", "testdata", "findings", "js-same-family-lockfiles"), nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if slices.ContainsFunc(result.Findings, func(finding Finding) bool {
		return finding.CheckID == "javascript-conflicting-lockfiles"
	}) {
		t.Fatalf("expected package-lock.json and npm-shrinkwrap.json to be one family, got %#v", result.Findings)
	}
}

func TestConflictingJavaScriptLockfilesCheckUsesWorkspaceOwner(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), `{"name":"workspace","workspaces":["packages/*"]}`)
	mustWriteFile(t, filepath.Join(root, "package-lock.json"), `{"name":"workspace","lockfileVersion":3,"packages":{}}`)
	mustWriteFile(t, filepath.Join(root, "pnpm-lock.yaml"), "lockfileVersion: '9.0'\nimporters: {}\n")
	mustWriteFile(t, filepath.Join(root, "packages", "app", "package.json"), `{"name":"app","dependencies":{"left-pad":"1.3.0"}}`)

	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(result.Findings) != 1 || result.Findings[0].CheckID != "javascript-conflicting-lockfiles" || result.Findings[0].Subject.ProjectRoot != "." {
		t.Fatalf("expected one workspace-root conflict, got %#v", result.Findings)
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
	if len(result.CheckRuns) != 4 {
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

func TestGoSumCheckHandlesLocalReplacementsAndNestedModules(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)

	t.Run("local dependencies only", func(t *testing.T) {
		root := t.TempDir()
		mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25.0\n\nrequire example.com/local v0.0.0\n\nreplace example.com/local => ../local\n")
		result, err := Scan(root, nil, ruleset)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if len(result.Findings) != 0 || len(result.CheckRuns) != 1 || result.CheckRuns[0].Status != CheckSkipped || result.CheckRuns[0].ReasonCode != "local-dependencies-only" {
			t.Fatalf("expected local-only skip, got findings=%#v runs=%#v", result.Findings, result.CheckRuns)
		}
	})

	t.Run("external dependency remains eligible", func(t *testing.T) {
		root := t.TempDir()
		mustWriteFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.25.0\n\nrequire (\n\texample.com/local v0.0.0\n\tgithub.com/google/uuid v1.6.0\n)\n\nreplace example.com/local => ../local\n")
		result, err := Scan(root, nil, ruleset)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if len(result.Findings) != 1 || result.Findings[0].CheckID != "go-sum-missing" {
			t.Fatalf("expected go.sum finding, got %#v", result.Findings)
		}
	})

	t.Run("ancestor checksum does not satisfy nested module", func(t *testing.T) {
		root := t.TempDir()
		mustWriteFile(t, filepath.Join(root, "go.sum"), "github.com/google/uuid v1.6.0 h1:checksum\n")
		mustWriteFile(t, filepath.Join(root, "nested", "go.mod"), "module example.com/nested\n\ngo 1.25.0\n\nrequire github.com/google/uuid v1.6.0\n")
		result, err := Scan(root, nil, ruleset)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		if len(result.Findings) != 1 || result.Findings[0].Subject.ProjectRoot != "nested" || result.Findings[0].Locations[0].Path != "nested/go.mod" {
			t.Fatalf("expected nested finding, got %#v", result.Findings)
		}
	})

	t.Run("malformed module", func(t *testing.T) {
		root := t.TempDir()
		mustWriteFile(t, filepath.Join(root, "go.mod"), "module\n")
		result, err := Scan(root, nil, ruleset)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		runs := filterCheckRuns(result.CheckRuns, "go-sum-missing")
		if len(result.Findings) != 0 || len(runs) != 1 || runs[0].Status != CheckFailed || runs[0].ReasonCode != "source-analysis-failed" {
			t.Fatalf("expected failed Go check without a finding, got findings=%#v runs=%#v", result.Findings, runs)
		}
	})
}

func TestComposerLockfileCheckClassifiesApplicationsConservatively(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	tests := []struct {
		name         string
		content      string
		lock         bool
		status       CheckRunStatus
		reason       string
		wantFindings int
	}{
		{name: "satisfied project", content: `{"type":"project","require":{"vendor/package":"^1"}}`, lock: true, status: CheckCompleted},
		{name: "missing lockfile", content: `{"type":"project","require":{"vendor/package":"^1"}}`, status: CheckCompleted, wantFindings: 1},
		{name: "platform only", content: `{"type":"project","require":{"php":"^8.3","php-debug":"*","ext-json":"*","lib-curl":"*","composer":"^2","composer-plugin-api":"^2","composer-runtime-api":"^2"}}`},
		{name: "library", content: `{"type":"library","require":{"vendor/package":"^1"}}`, status: CheckSkipped, reason: "not-application"},
		{name: "unknown role", content: `{"require":{"vendor/package":"^1"}}`, status: CheckSkipped, reason: "project-role-unknown"},
		{name: "lock disabled", content: `{"type":"project","config":{"lock":false},"require":{"vendor/package":"^1"}}`, status: CheckSkipped, reason: "lockfile-disabled"},
		{name: "malformed manifest", content: `{`, status: CheckFailed, reason: "source-analysis-failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			mustWriteFile(t, filepath.Join(root, "composer.json"), test.content)
			if test.lock {
				mustWriteFile(t, filepath.Join(root, "composer.lock"), `{"packages":[]}`)
			}
			result, err := Scan(root, nil, ruleset)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			runs := filterCheckRuns(result.CheckRuns, "php-composer-lockfile-missing-for-application")
			if test.status == "" {
				if len(runs) != 0 {
					t.Fatalf("expected no composer check run, got %#v", runs)
				}
			} else if len(runs) != 1 || runs[0].Status != test.status || runs[0].ReasonCode != test.reason {
				t.Fatalf("unexpected composer runs: %#v", runs)
			}
			if len(result.Findings) != test.wantFindings {
				t.Fatalf("expected %d findings, got %#v", test.wantFindings, result.Findings)
			}
		})
	}
}

func TestGemfileLockfileCheckSkipsLibraries(t *testing.T) {
	ruleset := mustLoadDefaultRules(t)
	t.Run("satisfied application", func(t *testing.T) {
		root := t.TempDir()
		mustWriteFile(t, filepath.Join(root, "Gemfile"), "source 'https://rubygems.org'\ngem 'rack', '~> 3.0'\n")
		mustWriteFile(t, filepath.Join(root, "Gemfile.lock"), "GEM\n")
		result, err := Scan(root, nil, ruleset)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		runs := filterCheckRuns(result.CheckRuns, "ruby-gemfile-lockfile-missing-for-application")
		if len(result.Findings) != 0 || len(runs) != 1 || runs[0].Status != CheckCompleted {
			t.Fatalf("expected satisfied Gemfile, got findings=%#v runs=%#v", result.Findings, runs)
		}
	})

	t.Run("gemspec library", func(t *testing.T) {
		root := t.TempDir()
		mustWriteFile(t, filepath.Join(root, "Gemfile"), "gemspec\ngem 'rspec'\n")
		result, err := Scan(root, nil, ruleset)
		if err != nil {
			t.Fatalf("Scan failed: %v", err)
		}
		runs := filterCheckRuns(result.CheckRuns, "ruby-gemfile-lockfile-missing-for-application")
		if len(result.Findings) != 0 || len(runs) != 1 || runs[0].Status != CheckSkipped || runs[0].ReasonCode != "not-application" {
			t.Fatalf("expected Gemfile library skip, got findings=%#v runs=%#v", result.Findings, runs)
		}
	})
}

func filterCheckRuns(runs []CheckRun, id CheckID) []CheckRun {
	return slices.DeleteFunc(append([]CheckRun(nil), runs...), func(run CheckRun) bool { return run.CheckID != id })
}
