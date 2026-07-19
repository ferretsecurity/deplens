package analyze

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed default_rules.yaml
var defaultRulesYAML []byte

const (
	FormManifest             SourceForm = "manifest"
	FormRequirements         SourceForm = "requirements"
	FormLockfile             SourceForm = "lockfile"
	FormConstraintFile       SourceForm = "constraint-file"
	FormChecksumFile         SourceForm = "checksum-file"
	FormVersionCatalog       SourceForm = "version-catalog"
	FormWorkspaceDefinition  SourceForm = "workspace-definition"
	FormBuildDefinition      SourceForm = "build-definition"
	FormAutomationDefinition SourceForm = "automation-definition"
	FormDeploymentDefinition SourceForm = "deployment-definition"
	FormToolConfig           SourceForm = "tool-config"
	FormSourceCode           SourceForm = "source-code"
	FormMarkup               SourceForm = "markup"
	FormVendoredFile         SourceForm = "vendored-file"
	FormOther                SourceForm = "other"

	RoleDeclaration   SourceRole = "declaration"
	RoleConstraint    SourceRole = "constraint"
	RoleResolution    SourceRole = "resolution"
	RoleIntegrity     SourceRole = "integrity"
	RoleConfiguration SourceRole = "configuration"
	RoleWorkspace     SourceRole = "workspace"
	RoleUsage         SourceRole = "usage"
	RoleInventory     SourceRole = "inventory"
)

type detector struct {
	ID             DetectorID
	PackageType    PackageType
	Form           SourceForm
	Roles          []SourceRole
	FilenameRegexp *regexp.Regexp
	PathGlob       string
	Analyzer       sourceAnalyzer
}

type Ruleset struct {
	detectors   []detector
	detectorIDs []DetectorID
}

type rulesFile struct {
	Rules []ruleConfig `yaml:"rules"`
}

type ruleConfig struct {
	ID            string          `yaml:"id"`
	PackageType   string          `yaml:"package-type"`
	Form          string          `yaml:"form"`
	Roles         []string        `yaml:"roles"`
	FilenameRegex string          `yaml:"filename-regex"`
	PathGlob      string          `yaml:"path-glob"`
	Analyzer      *analyzerConfig `yaml:"analyzer"`
}

type analyzerConfig struct {
	Type   string
	config yaml.Node
}

func (c *analyzerConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("must be a mapping")
	}
	c.config = yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	seenType := false
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Value == "type" {
			if seenType {
				return fmt.Errorf("type: duplicate field")
			}
			seenType = true
			if err := value.Decode(&c.Type); err != nil {
				return fmt.Errorf("type: %w", err)
			}
			continue
		}
		c.config.Content = append(c.config.Content, key, value)
	}
	return nil
}

type uvLockMatcherConfig struct{}
type poetryLockMatcherConfig struct{}
type pipfileLockMatcherConfig struct{}
type packageLockMatcherConfig struct{}
type pnpmLockMatcherConfig struct{}
type composerLockMatcherConfig struct{}
type cargoLockMatcherConfig struct{}
type yarnLockMatcherConfig struct{}

type sourceAnalyzer interface {
	Analyze(path string, content []byte) (sourceAnalyzerResult, error)
}

type sourceAnalyzerResult struct {
	Recognized   bool
	Analysis     SourceAnalysis
	Dependencies []DependencyReference
	Diagnostics  []Diagnostic
}

func LoadDefaultRules() (Ruleset, error) {
	return loadRules("embedded default rules", defaultRulesYAML)
}

func LoadRulesFile(path string) (Ruleset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Ruleset{}, fmt.Errorf("read rules file %q: %w", path, err)
	}
	return loadRules(path, data)
}

func loadRules(source string, data []byte) (Ruleset, error) {
	var raw rulesFile
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&raw); err != nil {
		return Ruleset{}, fmt.Errorf("parse rules from %s: %w", source, err)
	}
	if len(raw.Rules) == 0 {
		return Ruleset{}, fmt.Errorf("%s: rules: must contain at least one rule", source)
	}

	detectors := make([]detector, 0, len(raw.Rules))
	seenIDs := make(map[DetectorID]struct{}, len(raw.Rules))
	for ruleIdx, rawRule := range raw.Rules {
		fieldPath := fmt.Sprintf("rules[%d]", ruleIdx)
		if strings.TrimSpace(rawRule.ID) == "" {
			return Ruleset{}, fmt.Errorf("%s: %s.id: required", source, fieldPath)
		}
		if rawRule.ID != strings.TrimSpace(rawRule.ID) {
			return Ruleset{}, fmt.Errorf("%s: %s.id: must not have surrounding whitespace", source, fieldPath)
		}
		id := DetectorID(rawRule.ID)
		if _, exists := seenIDs[id]; exists {
			return Ruleset{}, fmt.Errorf("%s: %s.id: duplicate value %q", source, fieldPath, id)
		}
		seenIDs[id] = struct{}{}
		if rawRule.FilenameRegex == "" && rawRule.PathGlob == "" {
			return Ruleset{}, fmt.Errorf("%s: %s: at least one selector is required", source, fieldPath)
		}
		form := SourceForm(rawRule.Form)
		if !validSourceForm(form) {
			return Ruleset{}, fmt.Errorf("%s: %s.form: invalid value %q", source, fieldPath, rawRule.Form)
		}
		roles, err := parseSourceRoles(rawRule.Roles)
		if err != nil {
			return Ruleset{}, fmt.Errorf("%s: %s.roles: %w", source, fieldPath, err)
		}

		var compiled *regexp.Regexp
		if rawRule.FilenameRegex != "" {
			compiled, err = regexp.Compile(rawRule.FilenameRegex)
			if err != nil {
				return Ruleset{}, fmt.Errorf("%s: %s.filename-regex: compile %q: %w", source, fieldPath, rawRule.FilenameRegex, err)
			}
		}
		if rawRule.PathGlob != "" {
			if err := validatePathGlob(rawRule.PathGlob); err != nil {
				return Ruleset{}, fmt.Errorf("%s: %s.path-glob: compile %q: %w", source, fieldPath, rawRule.PathGlob, err)
			}
		}

		analyzer, err := compileSourceAnalyzer(rawRule)
		if err != nil {
			return Ruleset{}, fmt.Errorf("%s: %s.analyzer: %w", source, fieldPath, err)
		}
		detectors = append(detectors, detector{
			ID:             id,
			PackageType:    dependencyTypeFromRule(rawRule),
			Form:           form,
			Roles:          roles,
			FilenameRegexp: compiled,
			PathGlob:       rawRule.PathGlob,
			Analyzer:       analyzer,
		})
	}

	return Ruleset{detectors: detectors, detectorIDs: detectorIDsFromDetectors(detectors)}, nil
}

func (r Ruleset) DetectorIDs() []DetectorID {
	return append([]DetectorID(nil), r.detectorIDs...)
}

func (r Ruleset) MatchSelectorOnlySource(name string) (DetectorID, bool) {
	for _, d := range r.detectors {
		if d.PathGlob == "" && d.Analyzer == nil && d.matches(name, "") {
			return d.ID, true
		}
	}
	return "", false
}

func (r Ruleset) AnalyzeDependencySource(path, name string) (DependencySourceResult, bool, error) {
	return r.analyzeDependencySource(path, name, normalizeRelativePath(name))
}

func (r Ruleset) AnalyzeDependencySourceAtRelativePath(path, name, relPath string) (DependencySourceResult, bool, error) {
	return r.analyzeDependencySource(path, name, normalizeRelativePath(relPath))
}

func (r Ruleset) analyzeDependencySource(filePath, name, relPath string) (DependencySourceResult, bool, error) {
	var content []byte
	contentLoaded := false

	for _, d := range r.detectors {
		if !d.matches(name, relPath) {
			continue
		}
		base := DependencySourceResult{Detector: d.ID, Path: relPath, Form: d.Form, Roles: append([]SourceRole(nil), d.Roles...)}
		if d.Analyzer == nil {
			base.Analysis = identifiedAnalysis()
			return base, true, nil
		}
		if filePath == "" {
			continue
		}
		if !contentLoaded {
			data, err := os.ReadFile(filePath)
			if err != nil {
				base.Analysis = failedAnalysis()
				base.Diagnostics = []Diagnostic{{Severity: DiagnosticError, Code: "source-read-failed", Message: fmt.Sprintf("read candidate file %q: %v", filePath, err)}}
				return base, true, nil
			}
			content = data
			contentLoaded = true
		}
		result, err := d.Analyzer.Analyze(filePath, content)
		if err != nil {
			base.Analysis = failedAnalysis()
			base.Diagnostics = []Diagnostic{{Severity: DiagnosticError, Code: "source-analysis-failed", Message: err.Error()}}
			return base, true, nil
		}
		if result.Recognized {
			if err := validateSourceAnalyzerResult(result); err != nil {
				return DependencySourceResult{}, false, fmt.Errorf("detector %q returned invalid analysis: %w", d.ID, err)
			}
			applyDependencyType(result.Dependencies, d.PackageType)
			applyDependencyVERS(result.Dependencies)
			base.Analysis = result.Analysis
			base.Dependencies = result.Dependencies
			base.Diagnostics = result.Diagnostics
			return base, true, nil
		}
	}
	return DependencySourceResult{}, false, nil
}

func validSourceForm(form SourceForm) bool {
	switch form {
	case FormManifest, FormRequirements, FormLockfile, FormConstraintFile, FormChecksumFile,
		FormVersionCatalog, FormWorkspaceDefinition, FormBuildDefinition, FormAutomationDefinition,
		FormDeploymentDefinition, FormToolConfig, FormSourceCode, FormMarkup, FormVendoredFile, FormOther:
		return true
	default:
		return false
	}
}

func parseSourceRoles(raw []string) ([]SourceRole, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("must contain at least one role")
	}
	roles := make([]SourceRole, 0, len(raw))
	seen := make(map[SourceRole]struct{}, len(raw))
	for _, value := range raw {
		role := SourceRole(value)
		if !validSourceRole(role) {
			return nil, fmt.Errorf("invalid value %q", value)
		}
		if _, exists := seen[role]; exists {
			return nil, fmt.Errorf("duplicate value %q", value)
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	return roles, nil
}

func validSourceRole(role SourceRole) bool {
	switch role {
	case RoleDeclaration, RoleConstraint, RoleResolution, RoleIntegrity, RoleConfiguration, RoleWorkspace, RoleUsage, RoleInventory:
		return true
	default:
		return false
	}
}

func validSourceAnalysis(analysis SourceAnalysis) bool {
	switch analysis {
	case SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionUnsupported},
		SourceAnalysis{Presence: PresenceUnknown, Extraction: ExtractionFailed},
		SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionUnsupported},
		SourceAnalysis{Presence: PresenceAbsent, Extraction: ExtractionComplete},
		SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionUnsupported},
		SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionComplete},
		SourceAnalysis{Presence: PresencePresent, Extraction: ExtractionPartial}:
		return true
	default:
		return false
	}
}

func validateSourceAnalyzerResult(result sourceAnalyzerResult) error {
	if !validSourceAnalysis(result.Analysis) {
		return fmt.Errorf("invalid state %q/%q", result.Analysis.Presence, result.Analysis.Extraction)
	}
	if len(result.Dependencies) > 0 && result.Analysis.Presence != PresencePresent {
		return fmt.Errorf("%d dependencies require presence %q", len(result.Dependencies), PresencePresent)
	}
	if result.Analysis.Presence == PresenceAbsent && len(result.Dependencies) > 0 {
		return fmt.Errorf("absent presence cannot include dependencies")
	}
	if result.Analysis.Extraction == ExtractionPartial && (len(result.Dependencies) == 0 || len(result.Diagnostics) == 0) {
		return fmt.Errorf("partial extraction requires dependencies and diagnostics")
	}
	if result.Analysis.Extraction == ExtractionFailed && len(result.Diagnostics) == 0 {
		return fmt.Errorf("failed extraction requires a diagnostic")
	}
	hasWarning, hasError := false, false
	for index, diagnostic := range result.Diagnostics {
		if diagnostic.Severity != DiagnosticWarning && diagnostic.Severity != DiagnosticError {
			return fmt.Errorf("diagnostic %d has invalid severity %q", index, diagnostic.Severity)
		}
		if !diagnosticCodeRegexp.MatchString(diagnostic.Code) {
			return fmt.Errorf("diagnostic %d has invalid code %q", index, diagnostic.Code)
		}
		if strings.TrimSpace(diagnostic.Message) == "" {
			return fmt.Errorf("diagnostic %d has an empty message", index)
		}
		hasWarning = hasWarning || diagnostic.Severity == DiagnosticWarning
		hasError = hasError || diagnostic.Severity == DiagnosticError
	}
	if result.Analysis.Extraction == ExtractionPartial && !hasWarning {
		return fmt.Errorf("partial extraction requires a warning diagnostic")
	}
	if result.Analysis.Extraction == ExtractionFailed && !hasError {
		return fmt.Errorf("failed extraction requires an error diagnostic")
	}
	return nil
}

var diagnosticCodeRegexp = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func dependencyTypeFromRule(rawRule ruleConfig) PackageType {
	if rawRule.PackageType == "" {
		return ""
	}
	packageType := PackageType(rawRule.PackageType)
	if !isKnownPackageType(packageType) {
		log.Printf("warning: rule %q uses unknown package-type %q; preserving value", rawRule.ID, rawRule.PackageType)
	}
	return packageType
}

func applyDependencyType(dependencies []DependencyReference, packageType PackageType) {
	if packageType == "" {
		return
	}
	for idx := range dependencies {
		if dependencies[idx].PackageType == "" {
			dependencies[idx].PackageType = packageType
		}
	}
}

// Source of truth: https://raw.githubusercontent.com/package-url/purl-spec/main/purl-types-index.json
// This local snapshot is advisory only; unknown values are preserved and logged.
func isKnownPackageType(packageType PackageType) bool {
	switch packageType {
	case "alpm", "apk", "bazel", "bitbucket", "bitnami", "cargo", "chrome-extension", "cocoapods",
		"composer", "conan", "conda", "cpan", "cran", "deb", "docker", "gem", "generic", "github",
		"golang", "hackage", "hex", "huggingface", "julia", "luarocks", "maven", "mlflow", "npm",
		"nuget", "oci", "opam", "otp", "pub", "pypi", "qpkg", "rpm", "swift", "swid", "vcpkg",
		"vscode-extension", "yocto":
		return true
	default:
		return false
	}
}

func (d detector) matches(name, relPath string) bool {
	if d.FilenameRegexp != nil && !d.FilenameRegexp.MatchString(name) {
		return false
	}
	if d.PathGlob != "" && !pathGlobMatches(d.PathGlob, relPath) {
		return false
	}
	return true
}

func pathGlobMatches(pattern, relPath string) bool {
	return matchPathGlobSegments(strings.Split(pattern, "/"), strings.Split(relPath, "/"))
}

func matchPathGlobSegments(patternSegments, pathSegments []string) bool {
	if len(patternSegments) == 0 {
		return len(pathSegments) == 0
	}
	if patternSegments[0] == "**" {
		if matchPathGlobSegments(patternSegments[1:], pathSegments) {
			return true
		}
		for i := 0; i < len(pathSegments); i++ {
			if matchPathGlobSegments(patternSegments[1:], pathSegments[i+1:]) {
				return true
			}
		}
		return false
	}
	if len(pathSegments) == 0 {
		return false
	}
	ok, err := path.Match(patternSegments[0], pathSegments[0])
	return err == nil && ok && matchPathGlobSegments(patternSegments[1:], pathSegments[1:])
}

func validatePathGlob(pattern string) error {
	for _, segment := range strings.Split(pattern, "/") {
		if segment == "" {
			return fmt.Errorf("empty path segment")
		}
		if segment == "**" {
			continue
		}
		if strings.Contains(segment, "**") {
			return fmt.Errorf("invalid recursive wildcard segment %q", segment)
		}
		if _, err := path.Match(segment, ""); err != nil {
			return err
		}
	}
	return nil
}

func detectorIDsFromDetectors(detectors []detector) []DetectorID {
	ids := make([]DetectorID, 0, len(detectors))
	for _, d := range detectors {
		ids = append(ids, d.ID)
	}
	return ids
}
