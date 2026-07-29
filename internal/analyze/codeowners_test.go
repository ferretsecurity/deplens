package analyze

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestDefaultCodeownersCheckReportsOneFindingPerUncoveredSource(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")
	mustWriteFile(t, filepath.Join(root, "requirements.txt"), "requests==2.32.3\n")

	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	findings := findingsForCheck(result.Findings, "dependency-source-codeowners-missing")
	if len(findings) != 2 {
		t.Fatalf("expected one finding per source, got %#v", findings)
	}
	if findings[0].Locations[0].Path != "package.json" || findings[1].Locations[0].Path != "requirements.txt" {
		t.Fatalf("unexpected finding order or locations: %#v", findings)
	}
	for _, finding := range findings {
		if finding.Subject.ProjectRoot != "." || finding.Evidence["reason"] != "codeowners-file-missing" || finding.Fingerprint == "" {
			t.Fatalf("unexpected finding: %#v", finding)
		}
	}
	runs := filterCheckRuns(result.CheckRuns, "dependency-source-codeowners-missing")
	if len(runs) != 1 || runs[0].Status != CheckCompleted || runs[0].Subject.ProjectRoot != "." {
		t.Fatalf("unexpected check runs: %#v", runs)
	}
}

func TestDefaultCodeownersCheckUsesGitHubAndGitLabFixtures(t *testing.T) {
	tests := []struct {
		fixture  string
		path     string
		platform string
		reason   string
	}{
		{fixture: "codeowners-github", path: "package-lock.json", platform: "github", reason: "explicitly-unowned"},
		{fixture: "codeowners-gitlab", path: "apps/package-lock.json", platform: "gitlab", reason: "explicitly-unowned"},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			result, err := Scan(filepath.Join("..", "..", "testdata", "findings", test.fixture), nil, mustLoadDefaultRules(t))
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			findings := findingsForCheck(result.Findings, "dependency-source-codeowners-missing")
			if len(findings) != 1 || findings[0].Locations[0].Path != test.path {
				t.Fatalf("unexpected ownership findings: %#v", findings)
			}
			if findings[0].Evidence["codeowners_platform"] != test.platform || findings[0].Evidence["reason"] != test.reason {
				t.Fatalf("unexpected evidence: %#v", findings[0].Evidence)
			}
		})
	}
}

func TestCodeownersPolicyFileIsReadEvenWhenItsDirectoryIsIgnored(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".github", "CODEOWNERS"), "* @dependency-team\n")
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")

	result, err := Scan(root, []string{".github"}, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if findings := findingsForCheck(result.Findings, "dependency-source-codeowners-missing"); len(findings) != 0 {
		t.Fatalf("expected ignored .github policy to cover the source, got %#v", findings)
	}
}

func TestGitHubCodeownersMatcherUsesLastMatchAndSkipsInvalidLines(t *testing.T) {
	matcher := newGitHubCodeownersMatcher([]byte(`
* @global-owner
[invalid] @ignored
/legal legal@ownership.engineering
/apps/ @application-team
/apps/private
`))
	tests := []struct {
		path    string
		covered bool
		reason  string
	}{
		{path: "README.md", covered: true},
		{path: "legal/NOTICE", covered: true},
		{path: "apps/public/package.json", covered: true},
		{path: "apps/private/package.json", reason: "explicitly-unowned"},
	}
	for _, test := range tests {
		covered, reason := matcher.ownership(test.path)
		if covered != test.covered || reason != test.reason {
			t.Fatalf("%s: got covered=%t reason=%q", test.path, covered, reason)
		}
	}
}

func TestGitHubCodeownersMatcherRejectsOversizedFile(t *testing.T) {
	matcher := newGitHubCodeownersMatcher([]byte(strings.Repeat(" ", gitHubCodeownersMaxSize)))
	covered, reason := matcher.ownership("package.json")
	if covered || reason != "codeowners-file-too-large" {
		t.Fatalf("got covered=%t reason=%q", covered, reason)
	}
}

func TestGitHubCodeownersMatcherRejectsImpossibleUsername(t *testing.T) {
	matcher := newGitHubCodeownersMatcher([]byte(`* @invalid_owner`))
	covered, reason := matcher.ownership("package.json")
	if covered || reason != "no-matching-rule" {
		t.Fatalf("got covered=%t reason=%q", covered, reason)
	}
}

func TestGitLabCodeownersMatcherSupportsSectionsDefaultsAndExclusions(t *testing.T) {
	matcher := newGitLabCodeownersMatcher([]byte(`
[Dependencies] @dependency-team
/apps/
!/apps/generated/

^[Automation] @automation
/apps/generated/lockfiles/

[Spaces]
path\ with\ spaces/ @space-owner
`))
	tests := []struct {
		path    string
		covered bool
		reason  string
	}{
		{path: "apps/package.json", covered: true},
		{path: "apps/generated/package.json", reason: "explicitly-unowned"},
		{path: "apps/generated/lockfiles/package-lock.json", covered: true},
		{path: "path with spaces/package.json", covered: true},
		{path: "docs/package.json", reason: "no-matching-rule"},
	}
	for _, test := range tests {
		covered, reason := matcher.ownership(test.path)
		if covered != test.covered || reason != test.reason {
			t.Fatalf("%s: got covered=%t reason=%q", test.path, covered, reason)
		}
	}
}

func TestGitLabCodeownersMatcherParsesOwnersAfterInlineCommentMarker(t *testing.T) {
	matcher := newGitLabCodeownersMatcher([]byte(`/package.json invalid-owner # responsibility: @dependency-team`))
	covered, reason := matcher.ownership("package.json")
	if !covered || reason != "" {
		t.Fatalf("got covered=%t reason=%q", covered, reason)
	}
}

func TestGitLabCodeownersMatcherCombinesDuplicateSectionsAndAcceptsRoles(t *testing.T) {
	matcher := newGitLabCodeownersMatcher([]byte(`
[Dependencies]
*.json invalid-owner @@maintainer
[DEPENDENCIES]
*.toml @group/nested
`))
	for _, path := range []string{"package.json", "nested/pyproject.toml"} {
		covered, reason := matcher.ownership(path)
		if !covered || reason != "" {
			t.Fatalf("%s: got covered=%t reason=%q", path, covered, reason)
		}
	}
}

func TestGitLabCodeownersMatcherLetsZeroOwnerEntryOverrideEarlierOwner(t *testing.T) {
	matcher := newGitLabCodeownersMatcher([]byte(`
* @dependency-team
/package.json invalid-owner
`))
	covered, reason := matcher.ownership("package.json")
	if covered || reason != "explicitly-unowned" {
		t.Fatalf("got covered=%t reason=%q", covered, reason)
	}
}

func TestGitLabCodeownersMatcherTreatsBracesLiterally(t *testing.T) {
	matcher := newGitLabCodeownersMatcher([]byte(`{package.json,package-lock.json} @dependency-team`))
	if covered, _ := matcher.ownership("package.json"); covered {
		t.Fatal("GitLab does not enable brace alternation")
	}
	if covered, reason := matcher.ownership("{package.json,package-lock.json}"); !covered || reason != "" {
		t.Fatalf("expected literal brace path to be covered, got covered=%t reason=%q", covered, reason)
	}
}

func TestGitLabCodeownersMatcherRejectsWhitespaceOnlySection(t *testing.T) {
	matcher := newGitLabCodeownersMatcher([]byte("* @global\n[ ]\n/package.json\n"))
	covered, reason := matcher.ownership("package.json")
	if covered || reason != "explicitly-unowned" {
		t.Fatalf("got covered=%t reason=%q", covered, reason)
	}
}

func TestGitLabCodeownersMatcherParsesLongSingleLine(t *testing.T) {
	matcher := newGitLabCodeownersMatcher([]byte(strings.Repeat(" ", 70*1024) + "* @dependency-team"))
	covered, reason := matcher.ownership("package.json")
	if !covered || reason != "" {
		t.Fatalf("got covered=%t reason=%q", covered, reason)
	}
}

func TestCodeownersLocationPrecedence(t *testing.T) {
	inputs := map[string]policyInput{
		".github/CODEOWNERS": {path: ".github/CODEOWNERS", content: []byte("* @github")},
		"CODEOWNERS":         {path: "CODEOWNERS", content: []byte("* @root")},
		"docs/CODEOWNERS":    {path: "docs/CODEOWNERS", content: []byte("* @docs")},
		".gitlab/CODEOWNERS": {path: ".gitlab/CODEOWNERS", content: []byte("* @gitlab")},
	}
	github, err := resolveCodeowners(inputs, nil, codeownersPlatformGitHub)
	if err != nil || github.path != ".github/CODEOWNERS" {
		t.Fatalf("unexpected GitHub resolution: %#v err=%v", github, err)
	}
	gitlab, err := resolveCodeowners(inputs, nil, codeownersPlatformGitLab)
	if err != nil || gitlab.path != "CODEOWNERS" {
		t.Fatalf("unexpected GitLab resolution: %#v err=%v", gitlab, err)
	}
}

func TestAutomaticCodeownersResolutionRequiresOverrideForConflictingSignals(t *testing.T) {
	inputs := map[string]policyInput{
		".github/CODEOWNERS": {path: ".github/CODEOWNERS", content: []byte("* @github")},
		".gitlab/CODEOWNERS": {path: ".gitlab/CODEOWNERS", content: []byte("* @gitlab")},
	}
	_, err := resolveCodeowners(inputs, []DependencySourceResult{{Path: "package.json"}}, codeownersPlatformAuto)
	var resolutionErr codeownersResolutionError
	if !errors.As(err, &resolutionErr) || resolutionErr.reason != "codeowners-platform-ambiguous" {
		t.Fatalf("expected platform ambiguity, got %v", err)
	}
}

func TestAutomaticCodeownersResolutionFailsWhenRootFileSemanticsDisagree(t *testing.T) {
	inputs := map[string]policyInput{
		"CODEOWNERS": {path: "CODEOWNERS", content: []byte("* @global\n[Dependencies] @deps\n/package.json\n")},
	}
	_, err := resolveCodeowners(inputs, []DependencySourceResult{{Path: "package.json"}}, codeownersPlatformAuto)
	var resolutionErr codeownersResolutionError
	if !errors.As(err, &resolutionErr) || resolutionErr.reason != "codeowners-platform-ambiguous" {
		t.Fatalf("expected platform ambiguity, got %v", err)
	}
}

func TestDefaultCodeownersCheckFailsWithoutFindingsForAmbiguousPlatform(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".github", "CODEOWNERS"), "* @github-team\n")
	mustWriteFile(t, filepath.Join(root, ".gitlab", "CODEOWNERS"), "* @gitlab-team\n")
	mustWriteFile(t, filepath.Join(root, "package.json"), "{}")

	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if findings := findingsForCheck(result.Findings, "dependency-source-codeowners-missing"); len(findings) != 0 {
		t.Fatalf("expected no uncertain findings, got %#v", findings)
	}
	runs := filterCheckRuns(result.CheckRuns, "dependency-source-codeowners-missing")
	if len(runs) != 1 || runs[0].Status != CheckFailed || runs[0].ReasonCode != "codeowners-platform-ambiguous" {
		t.Fatalf("unexpected check runs: %#v", runs)
	}
}

func TestDefaultCodeownersCheckIgnoresPolicyAmbiguityWithoutSources(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, ".github", "CODEOWNERS"), "* @github-team\n")
	mustWriteFile(t, filepath.Join(root, ".gitlab", "CODEOWNERS"), "* @gitlab-team\n")

	result, err := Scan(root, nil, mustLoadDefaultRules(t))
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	runs := filterCheckRuns(result.CheckRuns, "dependency-source-codeowners-missing")
	if len(runs) != 1 || runs[0].Status != CheckCompleted {
		t.Fatalf("unexpected check runs: %#v", runs)
	}
}

func TestCodeownersPolicyInputRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "actual-codeowners"), "* @dependency-team\n")
	if err := os.MkdirAll(filepath.Join(root, ".github"), 0o755); err != nil {
		t.Fatalf("mkdir .github: %v", err)
	}
	if err := os.Symlink("../actual-codeowners", filepath.Join(root, ".github", "CODEOWNERS")); err != nil {
		t.Fatalf("create CODEOWNERS symlink: %v", err)
	}
	input := collectCodeownersPolicyInputs(root)[".github/CODEOWNERS"]
	if input.readError == nil {
		t.Fatalf("expected symlink to be rejected, got %#v", input)
	}
}

func TestAutomaticCodeownersResolutionAcceptsCommonSyntax(t *testing.T) {
	inputs := map[string]policyInput{
		"CODEOWNERS": {path: "CODEOWNERS", content: []byte("* @dependency-team\n")},
	}
	resolved, err := resolveCodeowners(inputs, []DependencySourceResult{{Path: "package.json"}, {Path: "apps/package.json"}}, codeownersPlatformAuto)
	if err != nil {
		t.Fatalf("resolveCodeowners failed: %v", err)
	}
	if resolved.platform != "common" {
		t.Fatalf("expected common platform, got %#v", resolved)
	}
	for _, path := range []string{"package.json", "apps/package.json"} {
		covered, reason := resolved.matcher.ownership(path)
		if !covered || reason != "" {
			t.Fatalf("%s: got covered=%t reason=%q", path, covered, reason)
		}
	}
}

func TestRuleSchemaAcceptsAndValidatesCodeownersPlatform(t *testing.T) {
	base := `
rules:
  - id: source
    form: manifest
    roles: [declaration]
    filename-regex: '^package\.json$'
checks:
  - id: dependency-source-codeowners-missing
    summary: Dependency source has no code owner
    severity: medium
    evaluator:
      type: dependency-source-codeowners
      platform: gitlab
    remediation: Add an owner.
`
	ruleset, err := loadRules("codeowners.yaml", []byte(base))
	if err != nil {
		t.Fatalf("expected configuration to load: %v", err)
	}
	if len(ruleset.checks) != 1 || ruleset.checks[0].CodeownersPlatform != codeownersPlatformGitLab {
		t.Fatalf("unexpected checks: %#v", ruleset.checks)
	}
	for _, replacement := range []string{"platform: bitbucket", "platform: gitlab\n      unknown: value"} {
		_, err := loadRules("invalid.yaml", []byte(strings.Replace(base, "platform: gitlab", replacement, 1)))
		if err == nil {
			t.Fatalf("expected %q to be rejected", replacement)
		}
	}
}

func TestCodeownersFindingFingerprintIgnoresPresentationFields(t *testing.T) {
	sources := []DependencySourceResult{{Path: "package.json"}}
	_, first := evaluateDependencySourceCodeowners(sources, nil, check{
		ID: "dependency-source-codeowners-missing", Summary: "First summary", Severity: SeverityLow,
	})
	_, second := evaluateDependencySourceCodeowners(sources, nil, check{
		ID: "dependency-source-codeowners-missing", Summary: "Changed summary", Severity: SeverityHigh,
	})
	if len(first) != 1 || len(second) != 1 || first[0].Fingerprint != second[0].Fingerprint {
		t.Fatalf("fingerprint changed with presentation fields: %#v %#v", first, second)
	}
}

func findingsForCheck(findings []Finding, id CheckID) []Finding {
	return slices.DeleteFunc(append([]Finding(nil), findings...), func(finding Finding) bool {
		return finding.CheckID != id
	})
}
