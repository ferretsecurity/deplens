package analyze

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/hmarr/codeowners"
	"gopkg.in/yaml.v3"
)

type codeownersPlatform string

const (
	codeownersPlatformAuto   codeownersPlatform = "auto"
	codeownersPlatformCommon codeownersPlatform = "common"
	codeownersPlatformGitHub codeownersPlatform = "github"
	codeownersPlatformGitLab codeownersPlatform = "gitlab"

	gitHubCodeownersMaxSize = 3 * 1024 * 1024
)

var (
	codeownersCandidatePaths  = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS", ".gitlab/CODEOWNERS"}
	gitHubCodeownersPaths     = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}
	gitLabCodeownersPaths     = []string{"CODEOWNERS", "docs/CODEOWNERS", ".gitlab/CODEOWNERS"}
	commonCodeownersPaths     = []string{"CODEOWNERS", "docs/CODEOWNERS"}
	gitLabSectionHeaderRegexp = regexp.MustCompile(`^(\^)?\[([^\]]+)\](?:\[(\d+)\])?(?:\s+(.*))?$`)
	gitLabOwnerRegexp         = regexp.MustCompile(`^@[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*$`)
	gitLabRoleRegexp          = regexp.MustCompile(`^@@(?:developer|developers|maintainer|maintainers|owner|owners)$`)
	codeownersEmailRegexp     = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}$`)
	gitHubUsernameRegexp      = regexp.MustCompile(`^@[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*$`)
	gitHubTeamRegexp          = regexp.MustCompile(`^@[A-Za-z0-9]+(?:-[A-Za-z0-9]+)*/[A-Za-z0-9_]+(?:-[A-Za-z0-9_]+)*$`)
)

type codeownersEvaluatorConfig struct {
	Platform string `yaml:"platform"`
}

type codeownersMatcher interface {
	ownership(path string) (covered bool, reason string)
}

type resolvedCodeowners struct {
	matcher  codeownersMatcher
	path     string
	platform codeownersPlatform
}

type codeownersResolutionError struct {
	reason string
	detail string
}

func (e codeownersResolutionError) Error() string {
	return e.detail
}

func compileCodeownersEvaluatorConfig(node yaml.Node) (codeownersPlatform, error) {
	for index := 0; index < len(node.Content); index += 2 {
		if node.Content[index].Value != "platform" {
			return "", fmt.Errorf("dependency-source-codeowners configuration: unknown field %q", node.Content[index].Value)
		}
	}
	var raw codeownersEvaluatorConfig
	if err := node.Decode(&raw); err != nil {
		return "", fmt.Errorf("dependency-source-codeowners configuration: %w", err)
	}
	platform := codeownersPlatform(raw.Platform)
	if platform == "" {
		platform = codeownersPlatformAuto
	}
	switch platform {
	case codeownersPlatformAuto, codeownersPlatformGitHub, codeownersPlatformGitLab:
		return platform, nil
	default:
		return "", fmt.Errorf("dependency-source-codeowners configuration: platform: invalid value %q", raw.Platform)
	}
}

func collectCodeownersPolicyInputs(root string) map[string]policyInput {
	result := make(map[string]policyInput)
	for _, relative := range codeownersCandidatePaths {
		fullPath := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(fullPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			result[relative] = policyInput{path: relative, readError: err}
			continue
		}
		if !info.Mode().IsRegular() {
			result[relative] = policyInput{
				path: relative, readError: fmt.Errorf("CODEOWNERS path is not a regular file"),
			}
			continue
		}
		content, readErr := os.ReadFile(fullPath)
		result[relative] = policyInput{path: relative, content: content, readError: readErr}
	}
	return result
}

func evaluateDependencySourceCodeowners(sources []DependencySourceResult, inputs map[string]policyInput, configured check) ([]CheckRun, []Finding) {
	subject := projectSubject("")
	if len(sources) == 0 {
		return []CheckRun{completedRun(configured.ID, subject)}, []Finding{}
	}
	resolved, err := resolveCodeowners(inputs, sources, configured.CodeownersPlatform)
	if err != nil {
		var resolutionErr codeownersResolutionError
		if value, ok := err.(codeownersResolutionError); ok {
			resolutionErr = value
		} else {
			resolutionErr = codeownersResolutionError{reason: "codeowners-analysis-failed", detail: err.Error()}
		}
		return []CheckRun{{
			CheckID: configured.ID, Subject: subject, Status: CheckFailed,
			ReasonCode: resolutionErr.reason, Detail: resolutionErr.detail,
		}}, []Finding{}
	}

	findings := make([]Finding, 0)
	for _, source := range sources {
		covered := false
		reason := "codeowners-file-missing"
		if resolved.matcher != nil {
			covered, reason = resolved.matcher.ownership(source.Path)
		}
		if covered {
			continue
		}
		evidence := map[string]string{
			"source_path": source.Path,
			"reason":      reason,
		}
		if resolved.path != "" {
			evidence["codeowners_path"] = resolved.path
		}
		if resolved.platform != "" {
			evidence["codeowners_platform"] = string(resolved.platform)
		}
		findings = append(findings, Finding{
			CheckID: configured.ID, Severity: configured.Severity, Summary: configured.Summary, Subject: subject,
			Locations: []FindingLocation{{Path: source.Path}}, Evidence: evidence, Remediation: configured.Remediation,
			Fingerprint: findingFingerprint(configured.ID, subject, source.Path, ""),
		})
	}
	return []CheckRun{completedRun(configured.ID, subject)}, findings
}

func resolveCodeowners(inputs map[string]policyInput, sources []DependencySourceResult, platform codeownersPlatform) (resolvedCodeowners, error) {
	if platform == "" {
		platform = codeownersPlatformAuto
	}
	switch platform {
	case codeownersPlatformGitHub:
		return resolveLocatedCodeowners(inputs, platform, gitHubCodeownersPaths)
	case codeownersPlatformGitLab:
		return resolveLocatedCodeowners(inputs, platform, gitLabCodeownersPaths)
	case codeownersPlatformAuto:
		return resolveAutomaticCodeowners(inputs, sources)
	default:
		return resolvedCodeowners{}, codeownersResolutionError{
			reason: "codeowners-platform-invalid",
			detail: fmt.Sprintf("unsupported CODEOWNERS platform %q", platform),
		}
	}
}

func resolveLocatedCodeowners(inputs map[string]policyInput, platform codeownersPlatform, paths []string) (resolvedCodeowners, error) {
	input, selected := firstCodeownersInput(inputs, paths)
	if !selected {
		return resolvedCodeowners{platform: platform}, nil
	}
	if input.readError != nil {
		return resolvedCodeowners{}, codeownersReadError(input)
	}
	resolved := resolvedCodeowners{path: input.path, platform: platform}
	if platform == codeownersPlatformGitHub {
		resolved.matcher = newGitHubCodeownersMatcher(input.content)
	} else {
		resolved.matcher = newGitLabCodeownersMatcher(input.content)
	}
	return resolved, nil
}

func resolveAutomaticCodeowners(inputs map[string]policyInput, sources []DependencySourceResult) (resolvedCodeowners, error) {
	_, hasGitHub := inputs[".github/CODEOWNERS"]
	_, hasGitLab := inputs[".gitlab/CODEOWNERS"]
	if hasGitHub && hasGitLab {
		return resolvedCodeowners{}, codeownersResolutionError{
			reason: "codeowners-platform-ambiguous",
			detail: "both .github/CODEOWNERS and .gitlab/CODEOWNERS are present; configure evaluator.platform explicitly",
		}
	}
	if hasGitHub {
		return resolveLocatedCodeowners(inputs, codeownersPlatformGitHub, gitHubCodeownersPaths)
	}
	if hasGitLab {
		return resolveLocatedCodeowners(inputs, codeownersPlatformGitLab, gitLabCodeownersPaths)
	}

	input, selected := firstCodeownersInput(inputs, commonCodeownersPaths)
	if !selected {
		return resolvedCodeowners{}, nil
	}
	if input.readError != nil {
		return resolvedCodeowners{}, codeownersReadError(input)
	}
	githubMatcher := newGitHubCodeownersMatcher(input.content)
	gitlabMatcher := newGitLabCodeownersMatcher(input.content)
	for _, source := range sources {
		githubCovered, _ := githubMatcher.ownership(source.Path)
		gitlabCovered, _ := gitlabMatcher.ownership(source.Path)
		if githubCovered != gitlabCovered {
			return resolvedCodeowners{}, codeownersResolutionError{
				reason: "codeowners-platform-ambiguous",
				detail: fmt.Sprintf("GitHub and GitLab CODEOWNERS semantics disagree for %q; configure evaluator.platform explicitly", source.Path),
			}
		}
	}
	return resolvedCodeowners{matcher: githubMatcher, path: input.path, platform: codeownersPlatformCommon}, nil
}

func firstCodeownersInput(inputs map[string]policyInput, paths []string) (policyInput, bool) {
	for _, candidate := range paths {
		if input, ok := inputs[candidate]; ok {
			return input, true
		}
	}
	return policyInput{}, false
}

func codeownersReadError(input policyInput) codeownersResolutionError {
	return codeownersResolutionError{
		reason: "codeowners-read-failed",
		detail: fmt.Sprintf("read CODEOWNERS file %q: %v", input.path, input.readError),
	}
}

type gitHubCodeownersMatcher struct {
	rules    codeowners.Ruleset
	tooLarge bool
}

func newGitHubCodeownersMatcher(content []byte) gitHubCodeownersMatcher {
	if len(content) >= gitHubCodeownersMaxSize {
		return gitHubCodeownersMatcher{tooLarge: true}
	}
	var rules codeowners.Ruleset
	for _, line := range bytes.Split(content, []byte{'\n'}) {
		parsed, err := codeowners.ParseFile(bytes.NewReader(line), codeowners.WithOwnerMatchers(gitHubOwnerMatchers))
		if err != nil {
			continue
		}
		rules = append(rules, parsed...)
	}
	return gitHubCodeownersMatcher{rules: rules}
}

var gitHubOwnerMatchers = []codeowners.OwnerMatcher{
	codeowners.OwnerMatchFunc(func(value string) (codeowners.Owner, error) {
		if !codeownersEmailRegexp.MatchString(value) {
			return codeowners.Owner{}, codeowners.ErrNoMatch
		}
		return codeowners.Owner{Value: value, Type: codeowners.EmailOwner}, nil
	}),
	codeowners.OwnerMatchFunc(func(value string) (codeowners.Owner, error) {
		if !gitHubTeamRegexp.MatchString(value) {
			return codeowners.Owner{}, codeowners.ErrNoMatch
		}
		return codeowners.Owner{Value: strings.TrimPrefix(value, "@"), Type: codeowners.TeamOwner}, nil
	}),
	codeowners.OwnerMatchFunc(func(value string) (codeowners.Owner, error) {
		username := strings.TrimPrefix(value, "@")
		if !gitHubUsernameRegexp.MatchString(value) || len(username) > 39 {
			return codeowners.Owner{}, codeowners.ErrNoMatch
		}
		return codeowners.Owner{Value: username, Type: codeowners.UsernameOwner}, nil
	}),
}

func (m gitHubCodeownersMatcher) ownership(sourcePath string) (bool, string) {
	if m.tooLarge {
		return false, "codeowners-file-too-large"
	}
	rule, err := m.rules.Match(sourcePath)
	if err != nil || rule == nil {
		return false, "no-matching-rule"
	}
	if len(rule.Owners) == 0 {
		return false, "explicitly-unowned"
	}
	return true, ""
}

type gitLabCodeownersMatcher struct {
	sections []*gitLabCodeownersSection
}

type gitLabCodeownersSection struct {
	entries []gitLabCodeownersEntry
}

type gitLabCodeownersEntry struct {
	pattern string
	owned   bool
	exclude bool
}

func newGitLabCodeownersMatcher(content []byte) gitLabCodeownersMatcher {
	defaultSection := &gitLabCodeownersSection{}
	sections := []*gitLabCodeownersSection{defaultSection}
	byName := make(map[string]*gitLabCodeownersSection)
	current := defaultSection
	currentDefault := false

	for _, rawLine := range bytes.Split(content, []byte{'\n'}) {
		line := strings.TrimSpace(string(rawLine))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if match := gitLabSectionHeaderRegexp.FindStringSubmatch(line); match != nil {
			key := strings.ToLower(strings.TrimSpace(match[2]))
			if key != "" {
				section := byName[key]
				if section == nil {
					section = &gitLabCodeownersSection{}
					byName[key] = section
					sections = append(sections, section)
				}
				current = section
				currentDefault = hasValidGitLabOwner(splitCodeownersFields(match[4]))
				continue
			}
		}

		fields := splitCodeownersFields(line)
		if len(fields) == 0 {
			continue
		}
		pattern := fields[0]
		if strings.HasPrefix(pattern, "!") {
			pattern = strings.TrimPrefix(pattern, "!")
			if pattern != "" {
				current.entries = append(current.entries, gitLabCodeownersEntry{pattern: normalizeGitLabPattern(pattern), exclude: true})
			}
			continue
		}
		owned := hasValidGitLabOwner(fields[1:])
		current.entries = append(current.entries, gitLabCodeownersEntry{
			pattern: normalizeGitLabPattern(pattern), owned: owned || currentDefault,
		})
	}
	return gitLabCodeownersMatcher{sections: sections}
}

func (m gitLabCodeownersMatcher) ownership(sourcePath string) (bool, string) {
	matched := false
	for _, section := range m.sections {
		owned, sectionMatched := section.ownership(sourcePath)
		matched = matched || sectionMatched
		if owned {
			return true, ""
		}
	}
	if matched {
		return false, "explicitly-unowned"
	}
	return false, "no-matching-rule"
}

func (s gitLabCodeownersSection) ownership(sourcePath string) (bool, bool) {
	owned := false
	matched := false
	excluded := false
	for _, entry := range s.entries {
		if excluded || entry.pattern == "" || !doublestar.MatchUnvalidated(entry.pattern, sourcePath) {
			continue
		}
		matched = true
		if entry.exclude {
			excluded = true
			owned = false
			continue
		}
		owned = entry.owned
	}
	return owned, matched
}

func normalizeGitLabPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ""
	}
	anchored := strings.HasPrefix(pattern, "/")
	pattern = strings.TrimPrefix(pattern, "/")
	if strings.HasSuffix(pattern, "/") {
		pattern += "**"
	}
	if !anchored {
		pattern = "**/" + pattern
	}
	pattern = escapeGitLabBraces(pattern)
	if !doublestar.ValidatePattern(pattern) {
		return ""
	}
	return pattern
}

func escapeGitLabBraces(pattern string) string {
	var result strings.Builder
	escaped := false
	for _, char := range pattern {
		if (char == '{' || char == '}') && !escaped {
			result.WriteRune('\\')
		}
		result.WriteRune(char)
		if char == '\\' {
			escaped = !escaped
		} else {
			escaped = false
		}
	}
	return result.String()
}

func hasValidGitLabOwner(fields []string) bool {
	for _, field := range fields {
		if gitLabOwnerRegexp.MatchString(field) || gitLabRoleRegexp.MatchString(field) || codeownersEmailRegexp.MatchString(field) {
			return true
		}
	}
	return false
}

func splitCodeownersFields(value string) []string {
	fields := make([]string, 0)
	var current strings.Builder
	escaped := false
	for _, char := range value {
		if escaped {
			if unicode.IsSpace(char) {
				current.WriteRune(char)
			} else {
				current.WriteRune('\\')
				current.WriteRune(char)
			}
			escaped = false
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if unicode.IsSpace(char) {
			if current.Len() > 0 {
				fields = append(fields, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(char)
	}
	if escaped {
		current.WriteRune('\\')
	}
	if current.Len() > 0 {
		fields = append(fields, current.String())
	}
	return fields
}
